package repository

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/constants"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
)

type OCROutcome struct {
	ErrorMessage string
	OCRText      string
	Method       string  // "pdf-text" | "pdf-ocr" | "image-ocr"
	Confidence   float32 // 0..1
	NeedsReview  bool
	ModelParams  map[string]any // e.g., {"lang":"eng"}
}

type ExtractJobRepository interface {
	Start(ctx context.Context, fileID, profileID uuid.UUID, format, status string) (*ent.ExtractJob, error)
	FinishOCR(ctx context.Context, jobID uuid.UUID, outcome OCROutcome) error
}

type extractJobRepo struct {
	ent *ent.Client
	log *slog.Logger
}

func NewExtractJobRepository(entc *ent.Client, log *slog.Logger) ExtractJobRepository {
	return &extractJobRepo{ent: entc, log: log}
}

func (r *extractJobRepo) Start(ctx context.Context, fileID, profileID uuid.UUID, format, status string) (*ent.ExtractJob, error) {
	job, err := r.ent.ExtractJob.
		Create().
		SetFileID(fileID).
		SetProfileID(profileID).
		SetFormat(format).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		r.log.Error("extract_job start failed", "file_id", fileID, "err", err)
		return nil, err
	}
	r.log.Info("extract_job started", "job_id", job.ID, "file_id", fileID, "format", format)
	return job, nil
}

func (r *extractJobRepo) FinishOCR(ctx context.Context, jobID uuid.UUID, outcome OCROutcome) error {
	u := r.ent.ExtractJob.UpdateOneID(jobID).SetFinishedAt(time.Now())
	if outcome.ErrorMessage != "" {
		_, err := u.
			SetStatus(string(constants.JobStatusFailed)).
			SetErrorMessage(outcome.ErrorMessage).
			Save(ctx)
		return err
	}

	var params []byte
	if outcome.ModelParams != nil {
		if b, err := json.Marshal(outcome.ModelParams); err == nil {
			params = b
		}
	}

	_, err := u.
		SetStatus(string(constants.JobStatusOCROK)).
		SetOcrText(outcome.OCRText).
		SetModelName(outcome.Method).
		SetModelParams(params).
		SetExtractionConfidence(outcome.Confidence).
		SetNeedsReview(outcome.NeedsReview).
		Save(ctx)
	return err
}
