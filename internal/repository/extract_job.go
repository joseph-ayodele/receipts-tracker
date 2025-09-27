package repository

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
)

type ExtractJobRepository interface {
	Start(ctx context.Context, fileID, profileID uuid.UUID, format, status string) (*ent.ExtractJob, error)
	FinishOCRSuccess(ctx context.Context, jobID uuid.UUID, ocrText string, method string, modelParams map[string]any) error
	FinishFailure(ctx context.Context, jobID uuid.UUID, message string) error
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

func (r *extractJobRepo) FinishOCRSuccess(ctx context.Context, jobID uuid.UUID, ocrText string, method string, modelParams map[string]any) error {
	var params []byte
	if modelParams != nil {
		if b, err := json.Marshal(modelParams); err == nil {
			params = b
		}
	}
	// Note: field names assume your Ent schema generated: OcrText, ModelName, ModelParams, FinishedAt, Status
	_, err := r.ent.ExtractJob.
		UpdateOneID(jobID).
		SetOcrText(ocrText).
		SetModelName(method).
		SetModelParams(params).
		SetFinishedAt(time.Now()).
		SetStatus("OCR_OK").
		Save(ctx)
	if err != nil {
		r.log.Error("extract_job finish(OK) failed", "job_id", jobID, "err", err)
		return err
	}
	r.log.Info("extract_job finished (OCR_OK)", "job_id", jobID, "method", method)
	return nil
}

func (r *extractJobRepo) FinishFailure(ctx context.Context, jobID uuid.UUID, message string) error {
	_, err := r.ent.ExtractJob.
		UpdateOneID(jobID).
		SetFinishedAt(time.Now()).
		SetStatus("FAILED").
		SetErrorMessage(message).
		Save(ctx)
	if err != nil {
		r.log.Error("extract_job finish(FAILED) failed", "job_id", jobID, "err", err)
		return err
	}
	r.log.Warn("extract_job finished (FAILED)", "job_id", jobID, "error", message)
	return nil
}
