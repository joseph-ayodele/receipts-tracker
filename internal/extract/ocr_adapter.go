package extract

import (
	"context"

	"log/slog"

	"github.com/joseph-ayodele/receipts-tracker/internal/ocr"
)

type OCRAdapter struct {
	e *ocr.Extractor
}

func NewOCRAdapter(cfg ocr.Config, logger *slog.Logger) *OCRAdapter {
	return &OCRAdapter{e: ocr.NewExtractor(cfg, logger)}
}

func (a *OCRAdapter) Extract(ctx context.Context, path string) (TextExtractionResult, error) {
	r, err := a.e.Extract(ctx, path)
	if err != nil {
		return TextExtractionResult{}, err
	}
	return TextExtractionResult{
		Text:       r.Text,
		Pages:      r.Pages,
		SourceType: r.SourceType,
		Method:     r.Method,
		Language:   r.Language,
		Duration:   r.Duration,
		Warnings:   r.Warnings,
		Confidence: r.Confidence,
	}, nil
}
