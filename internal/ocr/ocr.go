package ocr

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/joseph-ayodele/receipts-tracker/constants"
)

type Config struct {
	Pdftotext string // binary name or absolute path; if empty -> "pdftotext"
	Pdftoppm  string // binary name or absolute path; if empty -> "pdftoppm"
	Tesseract string // binary name or absolute path; if empty -> "tesseract"

	TesseractLang string // default "eng"
	DPI           int    // rasterization DPI for scanned PDFs, default 300
	MaxPages      int    // 0 = no limit

	TessdataDir         string
	HeicConverter       string
	EnableTSVConfidence bool

	PSM int // e.g., 6 is good for uniform block of text
	OEM int // 1 = LSTM; leave 0 to use default

	ArtifactCacheDir string
}

type ExtractionResult struct {
	Text       string
	Pages      int
	SourceType string // constants.PDF | constants.IMAGE
	Method     string // "pdf-text" | "pdf-ocr" | "image-ocr"
	Language   string
	Duration   time.Duration
	Warnings   []string
	Confidence float32
}

type Extractor struct {
	cfg    Config
	runner Runner
	logger *slog.Logger
}

func NewExtractor(cfg Config, logger *slog.Logger) *Extractor {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Pdftotext == "" {
		cfg.Pdftotext = "pdftotext"
	}
	if cfg.Pdftoppm == "" {
		cfg.Pdftoppm = "pdftoppm"
	}
	if cfg.Tesseract == "" {
		cfg.Tesseract = "tesseract"
	}
	if cfg.TesseractLang == "" {
		cfg.TesseractLang = "eng"
	}
	if cfg.DPI <= 0 {
		cfg.DPI = 300
	}
	if cfg.ArtifactCacheDir == "" {
		cfg.ArtifactCacheDir = "./tmp"
	}
	return &Extractor{cfg: cfg, runner: execRunner{}, logger: logger}
}

// Extract picks a strategy based on file extension.
func (e *Extractor) Extract(ctx context.Context, path string) (ExtractionResult, error) {
	start := time.Now()
	ext := constants.NormalizeExt(filepath.Ext(path))
	e.logger.Debug("starting ocr extraction", "path", path, "method", "auto", "ext", ext)
	switch constants.MapExtToFormat(ext) {
	case constants.PDF:
		res, err := e.extractPDF(ctx, path)
		res.Duration = time.Since(start)
		return res, err
	case constants.IMAGE:
		var cleanup func()
		var warns []string
		if constants.IsHEICExt(ext) {
			hashHex, _ := contentHashFromCtx(ctx)
			out, w, c, err := convertHEICtoPNG(ctx, e.runner, e.logger, e.cfg.HeicConverter, path, e.cfg.ArtifactCacheDir, hashHex)
			warns = append(warns, w...)
			if err != nil {
				e.logger.Error("heic conversion failed", "path", path, "error", err)
				return ExtractionResult{SourceType: constants.IMAGE, Warnings: warns}, err
			}
			cleanup = c
			path = out
		}
		if cleanup != nil {
			defer cleanup()
		}
		res, err := e.extractImage(ctx, path)
		res.Duration = time.Since(start)
		res.Warnings = append(res.Warnings, warns...)
		return res, err
	default:
		e.logger.Error("unsupported ocr extension", "extension", ext)
		return ExtractionResult{}, fmt.Errorf("unsupported extension: %q", ext)
	}
}
