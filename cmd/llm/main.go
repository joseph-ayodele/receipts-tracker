package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/internal/llm"
	openail "github.com/joseph-ayodele/receipts-tracker/internal/llm/openai"
	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if len(os.Args) != 2 {
		logger.Error("usage: runllm <extract_job_id>")
		os.Exit(2)
	}
	jobID, err := uuid.Parse(os.Args[1])
	if err != nil {
		logger.Error("invalid extract_job_id", "arg", os.Args[1], "error", err)
		os.Exit(2)
	}

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		logger.Error("DB_URL env var is required")
		os.Exit(2)
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		logger.Error("OPENAI_API_KEY env var is required")
		os.Exit(2)
	}

	// DB open
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	entc, pool, err := repo.Open(ctx, repo.Config{
		DSN:             dbURL,
		MaxConns:        10,
		MinConns:        1,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
		DialTimeout:     3 * time.Second,
	}, logger)
	if err != nil {
		logger.Error("open db", "error", err)
		os.Exit(1)
	}
	defer func(entc *ent.Client) { _ = entc.Close() }(entc)
	defer pool.Close()

	// repos
	jobsRepo := repo.NewExtractJobRepository(entc, logger)
	filesRepo := repo.NewReceiptFileRepository(entc, logger)
	profRepo := repo.NewProfileRepository(entc, logger)
	catRepo := repo.NewCategoryRepository(entc, logger)

	// load job
	job, err := jobsRepo.GetByID(ctx, jobID)
	if err != nil {
		logger.Error("load extract_job", "job_id", jobID, "error", err)
		os.Exit(1)
	}
	if *job.OcrText == "" {
		logger.Error("extract_job has empty ocr_text; run OCR first", "job_id", jobID)
		os.Exit(1)
	}

	// load file (for path hints)
	fileRow, err := filesRepo.GetByID(ctx, job.FileID)
	if err != nil {
		logger.Error("load receipt_file", "file_id", job.FileID, "error", err)
		os.Exit(1)
	}

	// load profile (for default currency)
	profileRow, err := profRepo.GetByID(ctx, job.ProfileID)
	if err != nil {
		logger.Error("load profile", "profile_id", job.ProfileID, "error", err)
		os.Exit(1)
	}

	// categories
	cats, err := catRepo.ListCategories(ctx)
	if err != nil {
		logger.Error("list categories", "error", err)
		os.Exit(1)
	}
	allowed := make([]string, 0, len(cats))
	for _, c := range cats {
		allowed = append(allowed, c.Name)
	}

	// build request
	req := llm.ExtractRequest{
		OCRText:           *job.OcrText,
		FilenameHint:      filepath.Base(fileRow.SourcePath),
		FolderHint:        filepath.Dir(fileRow.SourcePath),
		CountryHint:       "", // optional; set if you have it
		AllowedCategories: allowed,
		DefaultCurrency:   profileRow.DefaultCurrency,
		Timezone:          "",                        // optional; can be set from config later
		PrepConfidence:    *job.ExtractionConfidence, // 0..1 (if you stored it)
		FilePath:          fileRow.SourcePath,        // vision future-path
	}

	client := openail.New(openail.Config{
		Model:        getenv("OPENAI_MODEL", "gpt-4o-mini"),
		APIKey:       os.Getenv("OPENAI_API_KEY"),
		Temperature:  0.0,
		Timeout:      45 * time.Second,
		EnableVision: false, // future
	}, logger)

	fields, raw, err := client.ExtractFields(ctx, req)
	if err != nil {
		logger.Error("llm extract failed", "job_id", jobID, "error", err, "raw", string(raw))
		os.Exit(1)
	}

	logger.Info("llm extract ok",
		"job_id", jobID,
		"merchant", fields.MerchantName,
		"date", fields.TxDate,
		"total", fields.Total,
		"currency", fields.CurrencyCode,
		"category", fields.Category,
	)

	// Print raw JSON to stdout (so you can redirect to a file if you want)
	os.Stdout.Write(raw)
	os.Stdout.Write([]byte("\n"))
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
