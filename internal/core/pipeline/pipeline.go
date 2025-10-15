package pipeline

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

// Processor coordinates OCR (text extract) then LLM parse (fields).
type Processor struct {
	logger *slog.Logger
	ocr    *OCRStage
	parse  *ParseStage
}

func NewProcessor(logger *slog.Logger, ocr *OCRStage, parse *ParseStage) *Processor {
	if logger == nil {
		logger = slog.Default()
	}
	return &Processor{logger: logger, ocr: ocr, parse: parse}
}

// ProcessFile runs OCR for a fileID (creating/advancing extract_job),
// then runs LLM parse on the resulting job, and upserts the receipt.
// Returns the final jobID (same one started by OCR).
func (p *Processor) ProcessFile(ctx context.Context, fileID uuid.UUID) (uuid.UUID, error) {
	// 1) OCR stage → creates job + stores ocr_text + confidence
	jobID, ocrRes, err := p.ocr.Run(ctx, fileID)
	if err != nil {
		p.logger.Error("processor.ocr.failed", "file_id", fileID, "err", err)
		return jobID, err
	}
	p.logger.Debug("processor extract stage success",
		"file_id", fileID,
		"job_id", jobID,
		"method", ocrRes.Method,
		"pages", ocrRes.Pages,
		"confidence", ocrRes.Confidence,
	)

	// 2) LLM parse stage → reads job.ocr_text, decides whether to attach file (based on confidence), and upserts receipt.
	if _, err := p.parse.Run(ctx, jobID); err != nil {
		p.logger.Error("processor.parse.failed", "job_id", jobID, "err", err)
		return jobID, err
	}
	p.logger.Debug("processor parse stage success", "job_id", jobID)
	return jobID, nil
}
