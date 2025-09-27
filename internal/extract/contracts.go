package extract

import (
	"context"
	"time"
)

// TextExtractor is Stage 1: file -> text.
type TextExtractor interface {
	Extract(ctx context.Context, path string) (TextExtractionResult, error)
}

type TextExtractionResult struct {
	Text       string
	Pages      int
	SourceType string // "PDF" | "IMAGE"
	Method     string // "pdf-text" | "pdf-ocr" | "image-ocr"
	Language   string
	Duration   time.Duration
	Warnings   []string
}

// FieldExtractor is Stage 2: text -> JSON fields (LLM or rules).
type FieldExtractor interface {
	ExtractFields(ctx context.Context, text string, hints map[string]string) (FieldsResult, error)
}

type FieldsResult struct {
	// JSON string (validated against schema elsewhere) — exact shape TBD.
	JSON        string
	Confidence  float32
	ModelName   string
	ModelParams map[string]any
}
