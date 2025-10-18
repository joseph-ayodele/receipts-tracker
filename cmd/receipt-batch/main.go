package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/internal/common"
	"github.com/joseph-ayodele/receipts-tracker/internal/core"
	"github.com/joseph-ayodele/receipts-tracker/internal/core/llm/openai"
	"github.com/joseph-ayodele/receipts-tracker/internal/core/ocr"
	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
	"github.com/joseph-ayodele/receipts-tracker/internal/services/export"
	ingest2 "github.com/joseph-ayodele/receipts-tracker/internal/services/ingest"
)

// printError prints an error message to stderr, falling back to stdout if stderr fails
func printError(format string, args ...interface{}) {
	if _, err := fmt.Fprintf(os.Stderr, format, args...); err != nil {
		fmt.Printf(format, args...)
	}
}

func main() {
	// Parse CLI flags
	var (
		inmem   = flag.Bool("inmem", false, "use in-memory SQLite database")
		dir     = flag.String("dir", "", "directory to process receipts from (required)")
		out     = flag.String("out", "", "output XLSX file path (optional, defaults to parent directory)")
		fromStr = flag.String("from", "", "from date YYYY-MM-DD")
		toStr   = flag.String("to", "", "to date YYYY-MM-DD")
	)
	flag.Parse()

	// Validate required flags
	if *dir == "" {
		printError("Error: --dir is required\n")
		os.Exit(1)
	}

	// If output file not specified, use parent directory with default filename
	if *out == "" {
		parentDir := filepath.Dir(*dir)
		*out = filepath.Join(parentDir, "receipts.xlsx")
	}

	// Parse date filters
	var from, to *time.Time
	if *fromStr != "" {
		if parsed, err := time.Parse("2006-01-02", *fromStr); err != nil {
			printError("Error: invalid --from date format, use YYYY-MM-DD: %v\n", err)
			os.Exit(1)
		} else {
			from = &parsed
		}
	}
	if *toStr != "" {
		if parsed, err := time.Parse("2006-01-02", *toStr); err != nil {
			printError("Error: invalid --to date format, use YYYY-MM-DD: %v\n", err)
			os.Exit(1)
		} else {
			to = &parsed
		}
	}

	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx := context.Background()

	// Load configuration for OpenAI and other services
	cfg := common.LoadConfig()

	// Initialize database using common utility
	dbResult, err := common.InitDatabase(ctx, cfg, *inmem, logger)
	if err != nil {
		logger.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer dbResult.Cleanup()

	entc := dbResult.Client

	// Wire repositories
	profilesRepo := repo.NewProfileRepository(entc, logger)
	receiptsRepo := repo.NewReceiptRepository(entc, logger)
	filesRepo := repo.NewReceiptFileRepository(entc, logger)
	jobsRepo := repo.NewExtractJobRepository(entc, logger)

	// Create or fetch default profile
	profileName := "Local Batch"
	defaultCurrency := "USD"
	profile, err := profilesRepo.GetOrCreateByName(ctx, profileName, defaultCurrency)
	if err != nil {
		logger.Error("failed to get or create profile", "error", err)
		os.Exit(1)
	}
	logger.Info("using profile", "id", profile.ID, "name", profile.Name)

	// Setup OCR
	ocrCfg := ocr.Config{
		HeicConverter:    cfg.OCR.HeicConverter,
		TessdataDir:      cfg.OCR.TessdataDir,
		ArtifactCacheDir: cfg.OCR.ArtifactCacheDir,
	}
	extractor := ocr.NewExtractor(ocrCfg, logger)

	// Setup OpenAI client (graceful if missing)
	var openaiClient *openai.Client
	if cfg.LLM.APIKey != "" {
		openaiClient = openai.NewClient(openai.Config{
			Model:       cfg.LLM.Model,
			APIKey:      cfg.LLM.APIKey,
			Temperature: cfg.LLM.Temperature,
			Timeout:     cfg.LLM.Timeout,
		}, logger)
		logger.Info("OpenAI client initialized", "model", cfg.LLM.Model)
	} else {
		logger.Warn("OpenAI API key not configured, LLM parsing will be skipped")
	}

	// Setup processor
	processor := core.NewProcessor(logger, extractor, openaiClient, filesRepo, jobsRepo, profilesRepo, receiptsRepo, jobsRepo, 0.60, "./tmp")

	// Setup ingestor
	ingestor := ingest2.NewFSIngestor(profilesRepo, filesRepo, logger)

	// Ingest directory
	logger.Info("starting ingestion", "dir", *dir, "profile", profile.ID)
	ingestionResults, stats, err := ingestor.IngestDirectory(ctx, profile.ID, *dir, true)
	if err != nil {
		logger.Error("failed to ingest directory", "error", err)
		os.Exit(1)
	}

	// Extract file IDs from ingestion results
	var ingested []uuid.UUID
	for _, result := range ingestionResults {
		if result.Err == "" {
			fileID, err := uuid.Parse(result.FileID)
			if err != nil {
				logger.Error("failed to parse file ID", "file_id", result.FileID, "error", err)
				continue
			}
			ingested = append(ingested, fileID)
		}
	}
	logger.Info("ingestion complete",
		"files_ingested", len(ingested),
		"scanned", stats.Scanned,
		"matched", stats.Matched,
		"succeeded", stats.Succeeded,
		"failed", stats.Failed,
		"deduplicated", stats.Deduplicated)

	// Process each ingested file
	processed := 0
	failures := 0

	for _, fileID := range ingested {
		logger.Info("processing file", "file_id", fileID)
		_, err := processor.ProcessFile(ctx, fileID)
		if err != nil {
			logger.Error("failed to process file", "file_id", fileID, "error", err)
			failures++
		} else {
			processed++
		}
	}

	// Export to XLSX
	logger.Info("exporting to XLSX", "output", *out)
	exportService := export.NewService(entc, receiptsRepo, filesRepo, logger)

	xlsxBytes, err := exportService.ExportReceiptsXLSX(ctx, profile.ID, from, to)
	if err != nil {
		logger.Error("failed to export receipts", "error", err)
		os.Exit(1)
	}

	// Write to file
	err = os.WriteFile(*out, xlsxBytes, 0644)
	if err != nil {
		logger.Error("failed to write output file", "error", err)
		os.Exit(1)
	}

	// Log summary
	logger.Info("batch processing complete",
		"files_ingested", len(ingested),
		"files_processed", processed,
		"failures", failures,
		"output_file", *out)

	fmt.Printf("Batch processing complete!\n")
	fmt.Printf("- Files ingested: %d\n", len(ingested))
	fmt.Printf("- Files processed: %d\n", processed)
	fmt.Printf("- Failures: %d\n", failures)
	fmt.Printf("- Output: %s\n", *out)
}
