package repository

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/constants"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent/extractjob"
	"github.com/joseph-ayodele/receipts-tracker/internal/llm"
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
	GetByID(ctx context.Context, jobID uuid.UUID) (*ent.ExtractJob, error)
	Start(ctx context.Context, fileID, profileID uuid.UUID, format, status string) (*ent.ExtractJob, error)
	FinishOCR(ctx context.Context, jobID uuid.UUID, outcome OCROutcome) error
	GetWithFile(ctx context.Context, jobID uuid.UUID) (*ent.ExtractJob, *ent.ReceiptFile, error)
	SetReceiptID(ctx context.Context, jobID, receiptID uuid.UUID) error
	FinishParseSuccess(ctx context.Context, jobID uuid.UUID, fields llm.ReceiptFields, needsReview bool, raw []byte, modelParams map[string]any) error
	FinishParseFailure(ctx context.Context, jobID uuid.UUID, errMsg string, raw []byte) error
}

type extractJobRepo struct {
	ent    *ent.Client
	logger *slog.Logger
}

func NewExtractJobRepository(entc *ent.Client, logger *slog.Logger) ExtractJobRepository {
	return &extractJobRepo{ent: entc, logger: logger}
}

func (r *extractJobRepo) GetByID(ctx context.Context, jobID uuid.UUID) (*ent.ExtractJob, error) {
	return r.ent.ExtractJob.
		Query().
		Where(extractjob.ID(jobID)).
		Only(ctx)
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
		r.logger.Error("extract_job start failed", "file_id", fileID, "err", err)
		return nil, err
	}
	r.logger.Info("extract_job started", "job_id", job.ID, "file_id", fileID, "format", format)
	return job, nil
}

func (r *extractJobRepo) FinishOCR(ctx context.Context, jobID uuid.UUID, outcome OCROutcome) error {
	u := r.ent.ExtractJob.UpdateOneID(jobID).SetFinishedAt(time.Now())
	if outcome.ErrorMessage != "" {
		return u.
			SetStatus(string(constants.JobStatusFailed)).
			SetErrorMessage(outcome.ErrorMessage).
			Exec(ctx)
	}

	var params []byte
	if outcome.ModelParams != nil {
		if b, err := json.Marshal(outcome.ModelParams); err == nil {
			params = b
		}
	}

	return u.
		SetStatus(string(constants.JobStatusOCROK)).
		SetOcrText(outcome.OCRText).
		SetModelName(outcome.Method).
		SetModelParams(params).
		SetExtractionConfidence(outcome.Confidence).
		SetNeedsReview(outcome.NeedsReview).
		Exec(ctx)
}

func (r *extractJobRepo) GetWithFile(ctx context.Context, jobID uuid.UUID) (*ent.ExtractJob, *ent.ReceiptFile, error) {
	job, err := r.ent.ExtractJob.Get(ctx, jobID)
	if err != nil {
		return nil, nil, err
	}
	file, err := r.ent.ReceiptFile.Get(ctx, job.FileID)
	if err != nil {
		return nil, nil, err
	}
	return job, file, nil
}

func (r *extractJobRepo) FinishParseSuccess(ctx context.Context, jobID uuid.UUID, fields llm.ReceiptFields, needsReview bool, _ []byte, modelParams map[string]any) error {
	mp, _ := json.Marshal(modelParams)
	fb, _ := json.Marshal(fields)
	return r.ent.ExtractJob.
		UpdateOneID(jobID).
		SetStatus("PARSE_OK").
		SetNeedsReview(needsReview).
		SetExtractionConfidence(fields.ModelConfidence).
		SetExtractedJSON(fb).
		SetModelName("openai").
		SetModelParams(mp).
		Exec(ctx)
}

func (r *extractJobRepo) FinishParseFailure(ctx context.Context, jobID uuid.UUID, errMsg string, raw []byte) error {
	return r.ent.ExtractJob.
		UpdateOneID(jobID).
		SetStatus("PARSE_ERR").
		SetErrorMessage(errMsg).
		SetExtractedJSON(raw).
		Exec(ctx)
}

func (r *extractJobRepo) SetReceiptID(ctx context.Context, jobID, receiptID uuid.UUID) error {
	err := r.ent.ExtractJob.UpdateOneID(jobID).SetReceiptID(receiptID).Exec(ctx)
	if err != nil {
		r.logger.Error("failed to set receipt ID on job", "job_id", jobID, "receipt_id", receiptID, "error", err)
		return err
	}
	r.logger.Info("receipt ID set successfully on job", "job_id", jobID, "receipt_id", receiptID)
	return nil
}
