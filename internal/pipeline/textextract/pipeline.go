package textextract

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/constants"
	"github.com/joseph-ayodele/receipts-tracker/internal/extract"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

type Pipeline struct {
	FilesRepo     repository.ReceiptFileRepository
	JobsRepo      repository.ExtractJobRepository
	TextExtractor extract.TextExtractor
	Log           *slog.Logger
}

func NewPipeline(files repository.ReceiptFileRepository, jobs repository.ExtractJobRepository, tx extract.TextExtractor, log *slog.Logger) *Pipeline {
	if log == nil {
		log = slog.Default()
	}
	return &Pipeline{FilesRepo: files, JobsRepo: jobs, TextExtractor: tx, Log: log}
}

// Run starts an extract_job, runs OCR, and persists the OCR text.
// Returns the job ID and the extraction summary. LLM stage is NOT called.
func (p *Pipeline) Run(ctx context.Context, fileID uuid.UUID) (uuid.UUID, extract.TextExtractionResult, error) {
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
		_ = p.JobsRepo.FinishFailure(ctx, job.ID, err.Error())
		return job.ID, res, err
	}

	// Persist OCR result (mark OCR_OK)
	params := map[string]any{"lang": res.Language}
	if err := p.JobsRepo.FinishOCRSuccess(ctx, job.ID, res.Text, res.Method, params); err != nil {
		return job.ID, res, err
	}

	return job.ID, res, nil
}
