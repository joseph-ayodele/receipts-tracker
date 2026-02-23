package core

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/joseph-ayodele/receipts-tracker/constants"
	"github.com/joseph-ayodele/receipts-tracker/internal/core/llm"
	"github.com/joseph-ayodele/receipts-tracker/internal/core/ocr"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
	"github.com/joseph-ayodele/receipts-tracker/internal/tools"
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
	visionDirect     bool // skip OCR; send files directly to LLM as vision input
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
	visionDirect bool,
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
		visionDirect:     visionDirect,
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
	// Skip LLM parsing if no LLM client is configured
	if p.llmExtractor == nil {
		p.logger.Info("skipping LLM parse - no LLM client configured", "job_id", jobID)
		return jobID, nil
	}

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

	// In vision-direct mode skip OCR entirely; mark job as OCR_OK with empty text
	// so runLLMParse can proceed and attach the raw file as a vision input.
	if p.visionDirect {
		p.logger.Info("vision-direct: skipping OCR", "file_id", fileID, "job_id", job.ID, "format", format)
		if err := p.jobsRepo.FinishOCR(ctx, job.ID, repository.OCROutcome{
			OCRText:    "",
			Method:     "vision-direct",
			Confidence: 0,
		}); err != nil {
			return job.ID, ocr.ExtractionResult{}, err
		}
		return job.ID, ocr.ExtractionResult{Method: "vision-direct"}, nil
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
			JobTitle:       tools.StrOrEmpty(prof.JobTitle),
			JobDescription: tools.StrOrEmpty(prof.JobDescription),
		},
	}

	// Vision-direct: attach the file instead of relying on OCR text.
	if p.visionDirect {
		switch job.Format {
		case constants.IMAGE:
			// HEIC files must be converted to PNG before vision attachment —
			// OpenAI cannot process HEIC and ShouldAttachImage requires a cached PNG.
			if constants.IsHEICExt(filepath.Ext(file.SourcePath)) {
				pngPath, heicCleanup, convErr := p.ocrExtractor.ConvertHEICForVision(ctx, file.SourcePath, req.ContentHashHex)
				if convErr != nil {
					p.logger.Warn("vision-direct: heic→png failed, vision unavailable", "job_id", job.ID, "err", convErr)
				} else {
					if heicCleanup != nil {
						defer heicCleanup()
					}
					p.logger.Debug("vision-direct: heic converted", "job_id", job.ID, "png", pngPath)
				}
			}
			req.ForceVision = true
		case constants.PDF:
			// Rasterize PDF pages and attach them as vision images.
			const maxVisionPages = 5
			pages, cleanup, rErr := p.ocrExtractor.RenderPDFPages(ctx, file.SourcePath, maxVisionPages)
			if rErr != nil {
				p.logger.Warn("vision-direct: pdf render failed, falling back to empty text", "job_id", job.ID, "err", rErr)
			} else {
				defer cleanup()
				req.VisionImagePaths = pages
				p.logger.Debug("vision-direct: pdf rendered", "job_id", job.ID, "pages", len(pages))
			}
		}
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

	// sanitize item list
	fields.Description = sanitizeDescription(fields.Description)

	// If other_fees missing/zero but OCR shows fee lines, aggregate them
	if parseDecimal(fields.OtherFees) == 0 && strings.Contains(strings.ToLower(*job.OcrText), "fee") {
		if fees, ok := aggregateFeesFromOCR(*job.OcrText); ok && fees > 0 {
			fields.OtherFees = fmt.Sprintf("%.2f", fees)
			p.logger.Info("post LLM adjustment applied",
				"stage", "post_llm_adjust",
				"reason", "fees_aggregated",
				"other_fees", fields.OtherFees,
			)
		}
	}

	// Reconcile totals deterministically
	reconcileTotals(*job.OcrText, &fields, p.logger)

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

// Tender offset keywords for detecting gift cards only
var tenderKeywords = []string{"gift card", "gift-card", "giftcard", "payment", "installment"}

// First alternative handles comma-formatted numbers (e.g. "1,302.41").
// Second alternative handles plain integers/decimals of any length (e.g. "1302.41", "-50.00").
var moneyRe = regexp.MustCompile(`(?i)[$\s]*(-?\d{1,3}(?:,\d{3})+(?:\.\d{1,2})?|-?\d+(?:\.\d{1,2})?)`)

var feeLineRe = regexp.MustCompile(`(?i)\b(Cleaning|Service|Resort|Booking|Host|Processing)\s+fee\b.*?([$(]?-?\s*\d[\d,]*(?:\.\d{1,2})?\)?)`)

func parseDecimal(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	m := moneyRe.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0
	}
	clean := strings.ReplaceAll(m[1], ",", "")
	v, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0
	}
	return v
}

func computeArithmeticTotal(f *llm.ReceiptFields) (float64, bool) {
	if f == nil {
		return 0, false
	}
	sub := parseDecimal(f.Subtotal)
	tax := parseDecimal(f.Tax)
	fees := parseDecimal(f.OtherFees)
	disc := parseDecimal(f.Discount)
	known := 0
	for _, v := range []float64{sub, tax, fees, disc} {
		if v != 0 {
			known++
		}
	}
	if known == 0 {
		return 0, false
	}
	total := sub + tax + fees - disc
	// clamp tiny float noise
	if math.Abs(total) < 0.005 {
		total = 0
	}
	return total, true
}

func containsAnyLower(s string, needles ...string) bool {
	s = strings.ToLower(s)
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func aggregateFeesFromOCR(ocr string) (float64, bool) {
	var sum float64
	var found bool
	for _, m := range feeLineRe.FindAllStringSubmatch(ocr, -1) {
		amt := parseDecimal(m[2])
		if amt > 0 {
			sum += amt
			found = true
		}
	}
	// clamp tiny noise
	if math.Abs(sum) < 0.005 {
		return 0, false
	}
	return sum, found
}

func reconcileTotals(ocrText string, f *llm.ReceiptFields, log *slog.Logger) {
	if f == nil {
		return
	}
	model := parseDecimal(f.Total)
	arith, ok := computeArithmeticTotal(f)
	hadGiftCard := containsAnyLower(ocrText, tenderKeywords...)

	shouldOverride := false
	reason := ""
	// Only override when arithmetic > model: the LLM under-reported total (e.g. anchored
	// on a $0 gift-card-reduced charge). When arith < model the LLM likely read the true
	// total correctly but mis-extracted a component — trust the model total in that case.
	if ok && arith > model+0.01 {
		shouldOverride = true
		reason = "component_mismatch"
	}
	if ok && hadGiftCard {
		// When gift card present, prefer arithmetic even if model ~ subtotal/0
		shouldOverride = true
		if reason == "" {
			reason = "tender_offset"
		}
	}

	if shouldOverride && arith > 0 {
		f.Total = fmt.Sprintf("%.2f", arith)
		if f.CurrencyCode == "" {
			f.CurrencyCode = "USD"
		}
		if log != nil {
			log.Info("post LLM adjustment applied",
				"stage", "post_llm_adjust",
				"reason", reason,
				"new_total", f.Total,
				"had_gift_card", hadGiftCard,
			)
		}
	}
}

// Optional: remove dangling ellipsis when only one item is present
func sanitizeDescription(desc string) string {
	d := strings.TrimSpace(desc)
	// If only one token (no commas/newlines), strip trailing ellipsis
	if !strings.Contains(d, ",") && !strings.Contains(d, "\n") {
		d = strings.TrimSuffix(d, "…")
		d = strings.TrimSuffix(d, "...")
	}
	return d
}
