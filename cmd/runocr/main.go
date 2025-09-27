package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/internal/extract"
	"github.com/joseph-ayodele/receipts-tracker/internal/ocr"
	"github.com/joseph-ayodele/receipts-tracker/internal/pipeline/textextract"
	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if len(os.Args) != 2 {
		logger.Error("usage", "cmd", "runocr <file-id-uuid>")
		os.Exit(2)
	}
	fileID, err := uuid.Parse(os.Args[1])
	if err != nil {
		logger.Error("invalid file id (must be UUID)", "arg", os.Args[1], "error", err)
		os.Exit(2)
	}

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		logger.Error("DB_URL required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	entc, pool, err := repo.Open(ctx, repo.Config{
		DSN:             dbURL,
		MaxConns:        10,
		MinConns:        1,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
		DialTimeout:     3 * time.Second,
	})
	if err != nil {
		logger.Error("open db", "error", err)
		os.Exit(1)
	}
	defer func(entc *ent.Client) {
		if cerr := entc.Close(); cerr != nil {
			logger.Error("close ent client", "error", cerr)
		}
	}(entc)
	defer pool.Close()

	filesRepo := repo.NewReceiptFileRepository(entc, logger)
	jobsRepo := repo.NewExtractJobRepository(entc, logger)

	// Build OCR extractor (Stage 1) and adapt it to TextExtractor.
	ocrx := ocr.NewExtractor(ocr.Config{}) // <-- single argument (matches your code)
	textExtractor := extract.NewOCRAdapter(ocrx, logger)

	p := textextract.NewPipeline(filesRepo, jobsRepo, textExtractor, logger)

	start := time.Now()
	jobID, res, err := p.Run(ctx, fileID)
	dur := time.Since(start)

	if err != nil {
		logger.Error("text extraction failed",
			"job_id", jobID, "error", err, "duration_ms", dur.Milliseconds())
		os.Exit(1)
	}

	logger.Info("text extraction OK",
		"job_id", jobID,
		"method", res.Method,
		"pages", res.Pages,
		"bytes", len(res.Text),
		"duration_ms", dur.Milliseconds(),
	)
}
