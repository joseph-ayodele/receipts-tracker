package parsefields

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/joseph-ayodele/receipts-tracker/constants"
	"github.com/joseph-ayodele/receipts-tracker/internal/llm"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

// Config holds thresholds and behavior flags for the parse stage.
type Config struct {
	MinConfidence float32 // default 0.60
}

type Pipeline struct {
	Logger         *slog.Logger
	Cfg            Config
	JobsRepo       repository.ExtractJobRepository
	FilesRepo      repository.ReceiptFileRepository
	ProfilesRepo   repository.ProfileRepository
	CategoriesRepo repository.CategoryRepository
	ReceiptsRepo   repository.ReceiptRepository
	Extractor      llm.FieldExtractor
}

func NewPipeline(
	logger *slog.Logger,
	cfg Config,
	jobs repository.ExtractJobRepository,
	files repository.ReceiptFileRepository,
	profiles repository.ProfileRepository,
	cats repository.CategoryRepository,
	recs repository.ReceiptRepository,
	fe llm.FieldExtractor,
) *Pipeline {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.MinConfidence <= 0 {
		cfg.MinConfidence = 0.60
	}
	return &Pipeline{
		Logger:         logger,
		Cfg:            cfg,
		JobsRepo:       jobs,
		FilesRepo:      files,
		ProfilesRepo:   profiles,
		CategoriesRepo: cats,
		ReceiptsRepo:   recs,
		Extractor:      fe,
	}
}

// Run executes the LLM parse stage for an existing OCR job (jobID).
// Preconditions: job is OCR_OK with non-empty ocr_text and a valid file link.
// Effects: writes extracted_json, extraction_confidence, needs_review;
//
//	upserts receipts row and links file -> receipt.
func (p *Pipeline) Run(ctx context.Context, jobID uuid.UUID) (uuid.UUID, error) {
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
	cats, err := p.CategoriesRepo.ListCategories(ctx)
	if err != nil {
		return job.ID, fmt.Errorf("list categories: %w", err)
	}
	allowed := make([]string, 0, len(cats))
	for _, c := range cats {
		allowed = append(allowed, c.Name)
	}

	// Build LLM request
	req := llm.ExtractRequest{
		OCRText:           *job.OcrText,
		FilenameHint:      file.SourcePath, // filename signal
		FolderHint:        "",              // optional
		AllowedCategories: allowed,
		DefaultCurrency:   prof.DefaultCurrency,
		Timezone:          "",                        // optional
		PrepConfidence:    *job.ExtractionConfidence, // from OCR stage if set, else 0
		FilePath:          file.SourcePath,           // enable file fallback when OCR is weak
		Profile: llm.ProfileContext{
			ProfileName:    prof.Name,
			JobTitle:       *prof.JobTitle,
			JobDescription: *prof.JobDescription,
		},
	}

	p.Logger.Info("parsefields.start",
		"job_id", job.ID, "file_id", file.ID,
		"ocr_bytes", len(*job.OcrText), "allowed_categories", len(allowed),
	)

	fields, raw, err := p.Extractor.ExtractFields(ctx, req)
	if err != nil {
		_ = p.JobsRepo.FinishParseFailure(ctx, job.ID, err.Error(), raw)
		return job.ID, fmt.Errorf("llm extract: %w", err)
	}

	// Resolve category_id (strict). If not found, try "Other" if present, else mark review.
	var categoryID *int
	needsReview := false
	if fields.Category != "" {
		c, err := p.CategoriesRepo.FindByName(ctx, fields.Category)
		if err == nil && c != nil {
			catID := int(c.ID)
			categoryID = &catID
		} else {
			// fallback to "Other" if exists
			if other, err2 := p.CategoriesRepo.FindByName(ctx, "Other"); err2 == nil && other != nil {
				otherID := int(other.ID)
				categoryID = &otherID
			} else {
				needsReview = true
				p.Logger.Warn("category not found", "label", fields.Category)
			}
		}
	} else {
		needsReview = true
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
		CategoryID:    categoryID,
	}
	rec, err := p.ReceiptsRepo.UpsertFromFields(ctx, request)
	if err != nil {
		_ = p.JobsRepo.FinishParseFailure(ctx, job.ID, err.Error(), raw)
		return job.ID, fmt.Errorf("upsert receipt: %w", err)
	}
	// Ensure file -> receipt link is set (idempotent)
	if err := p.FilesRepo.SetReceiptID(ctx, file.ID, rec.ID); err != nil {
		_ = p.JobsRepo.FinishParseFailure(ctx, job.ID, fmt.Sprintf("link file->receipt: %v", err), raw)
		return job.ID, err
	}

	// Persist parse success on job
	if err := p.JobsRepo.FinishParseSuccess(ctx, job.ID, fields, needsReview, raw, map[string]any{
		"model":      "openai", // your client sets more detail if you prefer
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return job.ID, err
	}

	p.Logger.Info("parsefields.ok",
		"job_id", job.ID, "receipt_id", rec.ID,
		"merchant", fields.MerchantName,
		"date", fields.TxDate, "total", fields.Total,
		"category", fields.Category, "needs_review", needsReview,
		"confidence", fields.ModelConfidence,
	)
	return job.ID, nil
}

// --- helpers (parsing) if you need them elsewhere ---
func parseMoneyPtr(s string) *float64 {
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}
