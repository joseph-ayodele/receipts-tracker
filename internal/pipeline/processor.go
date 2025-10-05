package processor

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	parse "github.com/joseph-ayodele/receipts-tracker/internal/pipeline/parsefields"
	"github.com/joseph-ayodele/receipts-tracker/internal/pipeline/textextract"
)

// Processor coordinates OCR (text extract) then LLM parse (fields).
type Processor struct {
	Logger *slog.Logger
	OCR    *textextract.Pipeline
	Parse  *parse.Pipeline
}

func NewProcessor(logger *slog.Logger, ocr *textextract.Pipeline, parse *parse.Pipeline) *Processor {
	if logger == nil {
		logger = slog.Default()
	}
	return &Processor{Logger: logger, OCR: ocr, Parse: parse}
}

// ProcessFile runs OCR for a fileID (creating/advancing extract_job),
// then runs LLM parse on the resulting job, and upserts the receipt.
// Returns the final jobID (same one started by OCR).
func (p *Processor) ProcessFile(ctx context.Context, fileID uuid.UUID) (uuid.UUID, error) {
	// 1) OCR stage → creates job + stores ocr_text + confidence
	jobID, ocrRes, err := p.OCR.Run(ctx, fileID)
	if err != nil {
		p.Logger.Error("processor.ocr.failed", "file_id", fileID, "err", err)
		return jobID, err
	}
	p.Logger.Info("processor.ocr.ok",
		"file_id", fileID,
		"job_id", jobID,
		"method", ocrRes.Method,
		"pages", ocrRes.Pages,
		"confidence", ocrRes.Confidence,
	)

	// 2) LLM parse stage → reads job.ocr_text, decides whether to attach file (based on confidence), and upserts receipt.
	if _, err := p.Parse.Run(ctx, jobID); err != nil {
		p.Logger.Error("processor.parse.failed", "job_id", jobID, "err", err)
		return jobID, err
	}
	p.Logger.Info("processor.parse.ok", "job_id", jobID)
	return jobID, nil
}
