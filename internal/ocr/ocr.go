package ocr

import (
	"context"
	"fmt"
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
}

func NewExtractor(cfg Config) *Extractor {
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
	return &Extractor{cfg: cfg, runner: execRunner{}}
}

// Extract picks a strategy based on file extension.
func (e *Extractor) Extract(ctx context.Context, path string) (ExtractionResult, error) {
	start := time.Now()
	ext := constants.NormalizeExt(filepath.Ext(path))
	switch constants.MapExtToFormat(ext) {
	case constants.PDF:
		res, err := e.extractPDF(ctx, path)
		res.Duration = time.Since(start)
		return res, err
	case constants.IMAGE:
		var cleanup func()
		var warns []string
		if constants.IsHEICExt(ext) {
			out, w, c, err := convertHEICtoPNG(ctx, e.runner, e.cfg.HeicConverter, path)
			warns = append(warns, w...)
			if err != nil {
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
		return ExtractionResult{}, fmt.Errorf("unsupported extension: %q", ext)
	}
}
