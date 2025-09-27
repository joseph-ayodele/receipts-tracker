package extract

import (
	"context"
	"log/slog"

	"github.com/joseph-ayodele/receipts-tracker/internal/ocr"
)

type OCRAdapter struct {
	e *ocr.Extractor
}

func NewOCRAdapter(e *ocr.Extractor, _ *slog.Logger) *OCRAdapter {
	return &OCRAdapter{e: e}
}

func (a *OCRAdapter) Extract(ctx context.Context, path string) (TextExtractionResult, error) {
	r, err := a.e.Extract(ctx, path)
	return TextExtractionResult{
		Text:       r.Text,
		Pages:      r.Pages,
		SourceType: r.SourceType,
		Method:     r.Method,
		Language:   r.Language,
		Duration:   r.Duration,
		Warnings:   r.Warnings,
	}, err
}
