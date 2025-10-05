package processor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/constants"
	"github.com/joseph-ayodele/receipts-tracker/internal/extract"
	"github.com/joseph-ayodele/receipts-tracker/internal/ocr"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

type OCRStage struct {
	FilesRepo     repository.ReceiptFileRepository
	JobsRepo      repository.ExtractJobRepository
	TextExtractor extract.TextExtractor
	Logger        *slog.Logger
}

func NewOCRStage(files repository.ReceiptFileRepository, jobs repository.ExtractJobRepository, tx extract.TextExtractor, logger *slog.Logger) *OCRStage {
	if logger == nil {
		logger = slog.Default()
	}
	return &OCRStage{FilesRepo: files, JobsRepo: jobs, TextExtractor: tx, Logger: logger}
}

// Run starts an extract_job, runs OCR, and persists the OCR text.
// Returns the job ID and the extraction summary. LLM stage is NOT called.
func (p *OCRStage) Run(ctx context.Context, fileID uuid.UUID) (uuid.UUID, extract.TextExtractionResult, error) {
	// Lookup the file
	row, err := p.FilesRepo.GetByID(ctx, fileID)
	if err != nil {
		return uuid.Nil, extract.TextExtractionResult{}, fmt.Errorf("get file: %w", err)
	}

	format := constants.MapExtToFormat(row.FileExt)
	if format == "" {
		return uuid.Nil, extract.TextExtractionResult{}, fmt.Errorf("unsupported format: %s", row.FileExt)
	}

	// Start job in RUNNING
	job, err := p.JobsRepo.Start(ctx, row.ID, row.ProfileID, format, string(constants.JobStatusRunning))
	if err != nil {
		return uuid.Nil, extract.TextExtractionResult{}, err
	}

	// OCR
	res, err := p.TextExtractor.Extract(ctx, row.SourcePath)
	if err != nil {
		_ = p.JobsRepo.FinishOCR(ctx, job.ID, repository.OCROutcome{
			ErrorMessage: err.Error(),
		})
		return job.ID, res, err
	}

	// Decide if review is needed
	needsReview := false
	if format == constants.IMAGE {
		// for images, flag low-confidence OCR for review (or LLM fallback later)
		if res.Confidence > 0 && res.Confidence < ocr.ImageConfidenceThreshold {
			p.Logger.Warn("Image ocr confidence low; needs review", "file_id", fileID, "job_id", job.ID, "conf", res.Confidence)
			needsReview = true
		}
	}

	// Persist OCR result (mark OCR_OK)
	out := repository.OCROutcome{
		OCRText:     res.Text,
		Method:      res.Method,
		Confidence:  res.Confidence,
		NeedsReview: needsReview,
		ModelParams: map[string]any{"lang": res.Language},
	}
	if err := p.JobsRepo.FinishOCR(ctx, job.ID, out); err != nil {
		return job.ID, res, err
	}

	return job.ID, res, nil
}
