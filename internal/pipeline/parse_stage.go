package processor

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/joseph-ayodele/receipts-tracker/constants"
	"github.com/joseph-ayodele/receipts-tracker/internal/llm"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

// Config holds thresholds and behavior flags for the parse stage.
type Config struct {
	MinConfidence    float32 // default 0.60
	ArtifactCacheDir string  // default "./cache" if empty
}

type ParseStage struct {
	Logger         *slog.Logger
	Cfg            Config
	JobsRepo       repository.ExtractJobRepository
	ProfilesRepo   repository.ProfileRepository
	ReceiptsRepo   repository.ReceiptRepository
	ExtractJobRepo repository.ExtractJobRepository
	Extractor      llm.FieldExtractor
}

func NewParseStage(
	logger *slog.Logger,
	cfg Config,
	jobs repository.ExtractJobRepository,
	profiles repository.ProfileRepository,
	extractJobRepo repository.ExtractJobRepository,
	recs repository.ReceiptRepository,
	fe llm.FieldExtractor,
) *ParseStage {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.MinConfidence <= 0 {
		cfg.MinConfidence = 0.60
	}
	if cfg.ArtifactCacheDir == "" {
		cfg.ArtifactCacheDir = "./tmp"
	}
	return &ParseStage{
		Logger:         logger,
		Cfg:            cfg,
		JobsRepo:       jobs,
		ExtractJobRepo: extractJobRepo,
		ProfilesRepo:   profiles,
		ReceiptsRepo:   recs,
		Extractor:      fe,
	}
}

// Run executes the LLM parse stage for an existing OCR job (jobID).
// Preconditions: job is OCR_OK with non-empty ocr_text and a valid file link.
// Effects: writes extracted_json, extraction_confidence, needs_review;
//
//	upserts receipts row and links file -> receipt.
func (p *ParseStage) Run(ctx context.Context, jobID uuid.UUID) (uuid.UUID, error) {
	job, file, err := p.JobsRepo.GetWithFile(ctx, jobID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("load job: %w", err)
	}
	if *job.Status != string(constants.JobStatusOCROK) && job.OcrText == nil {
		return job.ID, fmt.Errorf("job not ready for parse: status=%s ocr_text_empty=%t", *job.Status, job.OcrText == nil)
	}

	// Load profile + allowed categories
	prof, err := p.ProfilesRepo.GetByID(ctx, file.ProfileID)
	if err != nil {
		return job.ID, fmt.Errorf("load profile: %w", err)
	}
	allowed := constants.AsStringSlice()

	// Build LLM request
	req := llm.ExtractRequest{
		OCRText:           *job.OcrText,
		FilenameHint:      filepath.Base(file.SourcePath),
		FolderHint:        filepath.Dir(file.SourcePath),
		AllowedCategories: allowed,
		DefaultCurrency:   prof.DefaultCurrency,
		Timezone:          "",                        // optional
		PrepConfidence:    *job.ExtractionConfidence, // from OCR stage if set, else 0
		FilePath:          file.SourcePath,           // enable file fallback when OCR is weak
		ContentHashHex:    hex.EncodeToString(file.ContentHash),
		ArtifactCacheDir:  p.Cfg.ArtifactCacheDir,
		Profile: llm.ProfileContext{
			ProfileName:    prof.Name,
			JobTitle:       *prof.JobTitle,
			JobDescription: *prof.JobDescription,
		},
	}

	p.Logger.Info("parse fields start",
		"job_id", job.ID, "file_id", file.ID,
		"ocr_bytes", len(*job.OcrText), "allowed_categories", len(allowed),
	)

	fields, raw, err := p.Extractor.ExtractFields(ctx, req)
	if err != nil {
		_ = p.JobsRepo.FinishParseFailure(ctx, job.ID, err.Error(), raw)
		return job.ID, fmt.Errorf("llm extract: %w", err)
	}

	// Resolve category using canonical mapping
	needsReview := false
	canon, ok := constants.Canonicalize(fields.Category)
	if !ok {
		needsReview = true
		p.Logger.Warn("category unknown", "label", fields.Category)
		canon = constants.Other
	}

	// Heuristic needs_review
	if fields.MerchantName == "" || fields.TxDate == "" || fields.Total == "" {
		needsReview = true
	}
	if fields.ModelConfidence > 0 && fields.ModelConfidence < p.Cfg.MinConfidence {
		needsReview = true
	}

	// Upsert receipt and link file
	request := &repository.CreateReceiptRequest{
		File:          file,
		JobID:         job.ID,
		ReceiptFields: fields,
		CategoryName:  string(canon),
	}
	rec, err := p.ReceiptsRepo.UpsertFromFields(ctx, request)
	if err != nil {
		_ = p.JobsRepo.FinishParseFailure(ctx, job.ID, err.Error(), raw)
		return job.ID, fmt.Errorf("upsert receipt: %w", err)
	}
	// Ensure job -> receipt link is set (idempotent)
	if err := p.ExtractJobRepo.SetReceiptID(ctx, job.ID, rec.ID); err != nil {
		_ = p.JobsRepo.FinishParseFailure(ctx, job.ID, fmt.Sprintf("link job->receipt: %v", err), raw)
		return job.ID, err
	}

	// Persist parse success on job
	if err := p.JobsRepo.FinishParseSuccess(ctx, job.ID, fields, needsReview, raw, map[string]any{
		"model":      "openai", // your client sets more detail if you prefer
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return job.ID, err
	}

	p.Logger.Info("parsed fields successfully",
		"job_id", job.ID, "receipt_id", rec.ID,
		"merchant", fields.MerchantName,
		"date", fields.TxDate, "total", fields.Total,
		"category", string(canon), "needs_review", needsReview,
		"confidence", fields.ModelConfidence,
	)
	return job.ID, nil
}
