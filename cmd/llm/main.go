package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joseph-ayodele/receipts-tracker/internal/llm"
	openail "github.com/joseph-ayodele/receipts-tracker/internal/llm/openai"
)

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if os.Getenv("OPENAI_API_KEY") == "" {
		logger.Error("OPENAI_API_KEY is required in env")
		os.Exit(2)
	}

	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Println("usage: llmtest <ocr_text_file> [original_file_path]")
		os.Exit(2)
	}

	ocrBytes, err := os.ReadFile(os.Args[1])
	if err != nil {
		logger.Error("read ocr text file", "path", os.Args[1], "error", err)
		os.Exit(1)
	}
	ocrText := string(ocrBytes)

	var originalPath string
	if len(os.Args) == 3 {
		originalPath = os.Args[2]
	}

	client := openail.New(openail.Config{
		Model:        getenv("OPENAI_MODEL", "gpt-4o-mini"),
		APIKey:       os.Getenv("OPENAI_API_KEY"),
		Temperature:  0.0,
		Timeout:      45 * time.Second,
		EnableVision: false, // future
	}, logger)

	req := llm.ExtractRequest{
		OCRText:           ocrText,
		FilenameHint:      "demo.pdf",
		FolderHint:        "/Users/me/Receipts",
		CountryHint:       "US",
		AllowedCategories: []string{"Travel", "Meals", "Supplies", "Utilities", "Other"},
		DefaultCurrency:   "USD",
		Timezone:          "America/Chicago",
		PrepConfidence:    0.35,         // simulate low OCR confidence
		FilePath:          originalPath, // optional
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()

	fields, raw, err := client.ExtractFields(ctx, req)
	if err != nil {
		logger.Error("llm extract failed", "error", err, "raw", string(raw))
		os.Exit(1)
	}

	logger.Info("llm extract ok",
		"merchant", fields.MerchantName,
		"date", fields.TxDate,
		"total", fields.Total,
		"currency", fields.CurrencyCode,
		"category", fields.Category,
	)
	fmt.Println(string(raw))
}
