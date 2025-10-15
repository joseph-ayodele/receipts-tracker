package core

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/joseph-ayodele/receipts-tracker/constants"
	"github.com/joseph-ayodele/receipts-tracker/internal/core/llm"
	"github.com/joseph-ayodele/receipts-tracker/internal/core/ocr"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

// Processor coordinates OCR (text extract) then LLM parse (fields).
type Processor struct {
	logger           *slog.Logger
	ocrExtractor     *ocr.Extractor
	llmExtractor     llm.FieldExtractor
	filesRepo        repository.ReceiptFileRepository
	jobsRepo         repository.ExtractJobRepository
	profilesRepo     repository.ProfileRepository
	receiptsRepo     repository.ReceiptRepository
	extractJobRepo   repository.ExtractJobRepository
	minConfidence    float32
	artifactCacheDir string
}

func NewProcessor(
	logger *slog.Logger,
	ocrExtractor *ocr.Extractor,
	llmExtractor llm.FieldExtractor,
	filesRepo repository.ReceiptFileRepository,
	jobsRepo repository.ExtractJobRepository,
	profilesRepo repository.ProfileRepository,
	receiptsRepo repository.ReceiptRepository,
	extractJobRepo repository.ExtractJobRepository,
	minConfidence float32,
	artifactCacheDir string,
) *Processor {
	if logger == nil {
		logger = slog.Default()
	}
	if minConfidence == 0 {
		minConfidence = 0.60
	}
	if artifactCacheDir == "" {
		artifactCacheDir = "./tmp"
	}
	return &Processor{
		logger:           logger,
		ocrExtractor:     ocrExtractor,
		llmExtractor:     llmExtractor,
		filesRepo:        filesRepo,
		jobsRepo:         jobsRepo,
		profilesRepo:     profilesRepo,
		receiptsRepo:     receiptsRepo,
		extractJobRepo:   extractJobRepo,
		minConfidence:    minConfidence,
		artifactCacheDir: artifactCacheDir,
	}
}

// ProcessFile runs OCR for a fileID (creating/advancing extract_job),
// then runs LLM parse on the resulting job, and upserts the receipt.
// Returns the final jobID (same one started by OCR).
func (p *Processor) ProcessFile(ctx context.Context, fileID uuid.UUID) (uuid.UUID, error) {
	// 1) OCR stage → creates job + stores ocr_text + confidence
	jobID, ocrRes, err := p.runOCR(ctx, fileID)
	if err != nil {
		p.logger.Error("processor.ocr.failed", "file_id", fileID, "err", err)
		return jobID, err
	}
	p.logger.Debug("processor ocr success",
		"file_id", fileID,
		"job_id", jobID,
		"method", ocrRes.Method,
		"pages", ocrRes.Pages,
		"confidence", ocrRes.Confidence,
	)

	// 2) LLM parse stage → reads job.ocr_text, decides whether to attach file (based on confidence), and upserts receipt.
	if _, err := p.runLLMParse(ctx, jobID); err != nil {
		p.logger.Error("processor.parse.failed", "job_id", jobID, "err", err)
		return jobID, err
	}
	p.logger.Debug("processor parse success", "job_id", jobID)
	return jobID, nil
}

// runOCR starts an extract_job, runs OCR, and persists the OCR text.
// Returns the job ID and the extraction summary.
func (p *Processor) runOCR(ctx context.Context, fileID uuid.UUID) (uuid.UUID, ocr.ExtractionResult, error) {
	// Lookup the file
	row, err := p.filesRepo.GetByID(ctx, fileID)
	if err != nil {
		return uuid.Nil, ocr.ExtractionResult{}, fmt.Errorf("get file: %w", err)
	}
	hashHex := hex.EncodeToString(row.ContentHash)
	ctx = ocr.WithContentHash(ctx, hashHex)

	format := constants.MapExtToFormat(row.FileExt)
	if format == "" {
		return uuid.Nil, ocr.ExtractionResult{}, fmt.Errorf("unsupported format: %s", row.FileExt)
	}

	// Start job in RUNNING
	job, err := p.jobsRepo.Start(ctx, row.ID, row.ProfileID, format, string(constants.JobStatusRunning))
	if err != nil {
		return uuid.Nil, ocr.ExtractionResult{}, err
	}

	// OCR
	res, err := p.ocrExtractor.Extract(ctx, row.SourcePath)
	if err != nil {
		_ = p.jobsRepo.FinishOCR(ctx, job.ID, repository.OCROutcome{
			ErrorMessage: err.Error(),
		})
		return job.ID, res, err
	}

	// Decide if review is needed
	needsReview := false
	if format == constants.IMAGE {
		// for images, flag low-confidence OCR for review (or LLM fallback later)
		if res.Confidence > 0 && res.Confidence < constants.ImageConfidenceThreshold {
			p.logger.Warn("Image ocr confidence low; needs review", "file_id", fileID, "job_id", job.ID, "conf", res.Confidence)
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
	if err := p.jobsRepo.FinishOCR(ctx, job.ID, out); err != nil {
		return job.ID, res, err
	}

	return job.ID, res, nil
}

// RunOCROnly performs OCR extraction only, without LLM parsing.
func (p *Processor) RunOCROnly(ctx context.Context, fileID uuid.UUID) (uuid.UUID, ocr.ExtractionResult, error) {
	return p.runOCR(ctx, fileID)
}

// runLLMParse executes the LLM parse stage for an existing OCR job (jobID).
// Preconditions: job is OCR_OK with non-empty ocr_text and a valid file link.
// Effects: writes extracted_json, extraction_confidence, needs_review;
// upserts receipts row and links file -> receipt.
func (p *Processor) runLLMParse(ctx context.Context, jobID uuid.UUID) (uuid.UUID, error) {
	job, file, err := p.jobsRepo.GetWithFile(ctx, jobID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("load job: %w", err)
	}
	if *job.Status != string(constants.JobStatusOCROK) && job.OcrText == nil {
		return job.ID, fmt.Errorf("job not ready for parse: status=%s ocr_text_empty=%t", *job.Status, job.OcrText == nil)
	}

	// Load profile + allowed categories
	prof, err := p.profilesRepo.GetByID(ctx, file.ProfileID)
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
		ArtifactCacheDir:  p.artifactCacheDir,
		Profile: llm.ProfileContext{
			ProfileName:    prof.Name,
			JobTitle:       *prof.JobTitle,
			JobDescription: *prof.JobDescription,
		},
	}

	p.logger.Debug("parse fields start",
		"job_id", job.ID, "file_id", file.ID,
		"ocr_bytes", len(*job.OcrText), "allowed_categories", len(allowed),
	)

	fields, raw, err := p.llmExtractor.ExtractFields(ctx, req)
	if err != nil {
		_ = p.jobsRepo.FinishParseFailure(ctx, job.ID, err.Error(), raw)
		return job.ID, fmt.Errorf("llm extract: %w", err)
	}

	// Resolve category using canonical mapping
	needsReview := false
	canon, ok := constants.Canonicalize(fields.Category)
	if !ok {
		needsReview = true
		p.logger.Warn("category unknown", "label", fields.Category)
		canon = constants.Other
	}

	// Heuristic needs_review
	if fields.MerchantName == "" || fields.TxDate == "" || fields.Total == "" {
		needsReview = true
	}
	if fields.ModelConfidence > 0 && fields.ModelConfidence < p.minConfidence {
		needsReview = true
	}

	// Upsert receipt and link file
	request := &repository.CreateReceiptRequest{
		File:          file,
		JobID:         job.ID,
		ReceiptFields: fields,
		CategoryName:  string(canon),
	}
	rec, err := p.receiptsRepo.UpsertFromFields(ctx, request)
	if err != nil {
		_ = p.jobsRepo.FinishParseFailure(ctx, job.ID, err.Error(), raw)
		return job.ID, fmt.Errorf("upsert receipt: %w", err)
	}
	// Ensure job -> receipt link is set (idempotent)
	if err := p.extractJobRepo.SetReceiptID(ctx, job.ID, rec.ID); err != nil {
		_ = p.jobsRepo.FinishParseFailure(ctx, job.ID, fmt.Sprintf("link job->receipt: %v", err), raw)
		return job.ID, err
	}

	// Persist parse success on job
	if err := p.jobsRepo.FinishParseSuccess(ctx, job.ID, fields, needsReview, raw, map[string]any{
		"model":      "openai", // your client sets more detail if you prefer
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return job.ID, err
	}

	p.logger.Info("parsed fields successfully",
		"job_id", job.ID, "receipt_id", rec.ID,
		"merchant", fields.MerchantName,
		"date", fields.TxDate, "total", fields.Total,
		"category", string(canon), "needs_review", needsReview,
		"confidence", fields.ModelConfidence,
	)
	return job.ID, nil
}
