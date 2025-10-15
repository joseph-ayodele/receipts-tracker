package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/internal/core/extract"
	"github.com/joseph-ayodele/receipts-tracker/internal/core/ocr"
	processor2 "github.com/joseph-ayodele/receipts-tracker/internal/core/pipeline"

	"github.com/joseph-ayodele/receipts-tracker/internal/llm/openai"
	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if len(os.Args) < 2 {
		logger.Error("usage: runllm <file_id> [times]")
		os.Exit(2)
	}
	fileID, err := uuid.Parse(os.Args[1])
	if err != nil {
		logger.Error("invalid file_id", "arg", os.Args[1], "error", err)
		os.Exit(2)
	}
	times := 10
	if len(os.Args) >= 3 {
		if n, err := strconv.Atoi(os.Args[2]); err == nil && n > 0 {
			times = n
		}
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

	// --- DB/open
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	entc, pool, err := repo.Open(ctx, repo.Config{
		DSN:             dbURL,
		MaxConns:        15,
		MinConns:        2,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
		DialTimeout:     3 * time.Second,
	}, logger)
	if err != nil {
		logger.Error("open db", "error", err)
		os.Exit(1)
	}
	defer repo.Close(entc, pool, logger)

	// repos
	profilesRepo := repo.NewProfileRepository(entc, logger)
	receiptsRepo := repo.NewReceiptRepository(entc, logger)
	filesRepo := repo.NewReceiptFileRepository(entc, logger)
	jobsRepo := repo.NewExtractJobRepository(entc, logger)

	// pull the file row for logging/context (pipeline will refetch as needed)
	fileRow, err := filesRepo.GetByID(ctx, fileID)
	if err != nil {
		logger.Error("load receipt_file", "file_id", fileID, "error", err)
		os.Exit(1)
	}

	// --- Wire OCR + LLM same as server
	cacheDir := getenv("ARTIFACT_CACHE_DIR", "./tmp")
	heicConv := getenv("HEIC_CONVERTER", "magick")
	tessdata := os.Getenv("TESSDATA_PREFIX")

	ocrCfg := ocr.Config{
		HeicConverter:    heicConv,
		TessdataDir:      tessdata,
		ArtifactCacheDir: cacheDir,
	}
	ocrExtractor := ocr.NewExtractor(ocrCfg, logger)
	textAdapter := extract.NewOCRAdapter(ocrExtractor, logger)
	ocrStage := processor2.NewOCRStage(filesRepo, jobsRepo, textAdapter, logger)

	openaiClient := openai.NewClient(openai.Config{
		Model:           getenv("OPENAI_MODEL", "gpt-4o-mini"),
		APIKey:          os.Getenv("OPENAI_API_KEY"),
		Temperature:     0.0,
		Timeout:         45 * time.Second,
		LenientOptional: true,
		MaxVisionMB:     10,
	}, logger)

	parseCfg := processor2.Config{
		MinConfidence:    0.60,
		ArtifactCacheDir: cacheDir,
	}
	parseStage := processor2.NewParseStage(logger, parseCfg, jobsRepo, profilesRepo, jobsRepo, receiptsRepo, openaiClient)

	processor := processor2.NewProcessor(logger, ocrStage, parseStage)

	// --- Loop N times on the SAME file_id
	base := filepath.Base(fileRow.SourcePath)
	for i := 1; i <= times; i++ {
		runCtx, cancelRun := context.WithTimeout(context.Background(), 2*time.Minute)
		start := time.Now()
		logger.Info("pipeline.run.start", "iter", i, "file_id", fileID, "basename", base)

		_, err := processor.ProcessFile(runCtx, fileID) // your pipeline method
		cancelRun()

		if err != nil {
			logger.Error("pipeline.run.error", "iter", i, "err", err)
		} else {
			logger.Info("pipeline.run.ok", "iter", i, "elapsed_ms", time.Since(start).Milliseconds())
		}

		time.Sleep(750 * time.Millisecond)
	}

	logger.Info("done", "file_id", fileID.String(), "times", times)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
