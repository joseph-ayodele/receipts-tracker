package extract

import (
	"context"

	"log/slog"

	"github.com/joseph-ayodele/receipts-tracker/internal/core/ocr"
)

type OCRAdapter struct {
	extractor *ocr.Extractor
	logger    *slog.Logger
}

func NewOCRAdapter(e *ocr.Extractor, l *slog.Logger) *OCRAdapter {
	return &OCRAdapter{
		extractor: e,
		logger:    l,
	}
}

func (a *OCRAdapter) Extract(ctx context.Context, path string) (TextExtractionResult, error) {
	r, err := a.extractor.Extract(ctx, path)
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
