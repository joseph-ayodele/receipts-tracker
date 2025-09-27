package ocr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Config struct {
	Pdftotext string // binary name or absolute path; if empty -> "pdftotext"
	Pdftoppm  string // binary name or absolute path; if empty -> "pdftoppm"
	Tesseract string // binary name or absolute path; if empty -> "tesseract"

	TesseractLang string // default "eng"
	DPI           int    // rasterization DPI for scanned PDFs, default 300
	MaxPages      int    // 0 = no limit
}

type ExtractionResult struct {
	Text       string
	Pages      int
	SourceType string // "PDF" | "IMAGE"
	Method     string // "pdf-text" | "pdf-ocr" | "image-ocr"
	Language   string
	Duration   time.Duration
	Warnings   []string
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

// For tests.
func (e *Extractor) WithRunner(r Runner) *Extractor {
	e.runner = r
	return e
}

// Extract picks a strategy based on file extension.
func (e *Extractor) Extract(ctx context.Context, path string) (ExtractionResult, error) {
	start := time.Now()
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "pdf":
		res, err := e.extractPDF(ctx, path)
		res.Duration = time.Since(start)
		return res, err
	case "jpg", "jpeg", "png":
		res, err := e.extractImage(ctx, path)
		res.Duration = time.Since(start)
		return res, err
	default:
		return ExtractionResult{}, fmt.Errorf("unsupported extension: %q", ext)
	}
}

func (e *Extractor) extractPDF(ctx context.Context, path string) (ExtractionResult, error) {
	// 1) try pdftotext
	text, pages, warn, err := e.pdfToText(ctx, path)
	if err == nil && len(strings.TrimSpace(text)) >= 20 {
		return ExtractionResult{
			Text:       Normalize(text),
			Pages:      pages,
			SourceType: "PDF",
			Method:     "pdf-text",
			Language:   "", // n/a for text
			Warnings:   warn,
		}, nil
	}

	// 2) fallback: rasterize + tesseract
	text2, pages2, warn2, err2 := e.pdfToOCR(ctx, path)
	if err2 != nil {
		// If text was short but present, surface both warnings for debugging
		w := append(warn, warn2...)
		if err != nil {
			w = append(w, err.Error())
		}
		return ExtractionResult{Warnings: w, SourceType: "PDF"}, fmt.Errorf("pdf ocr failed: %w", err2)
	}
	return ExtractionResult{
		Text:       Normalize(text2),
		Pages:      pages2,
		SourceType: "PDF",
		Method:     "pdf-ocr",
		Language:   e.cfg.TesseractLang,
		Warnings:   append(warn, warn2...),
	}, nil
}

func (e *Extractor) extractImage(ctx context.Context, path string) (ExtractionResult, error) {
	text, warn, err := e.tesseractOCR(ctx, path)
	if err != nil {
		return ExtractionResult{SourceType: "IMAGE", Warnings: warn}, err
	}
	return ExtractionResult{
		Text:       Normalize(text),
		Pages:      1,
		SourceType: "IMAGE",
		Method:     "image-ocr",
		Language:   e.cfg.TesseractLang,
		Warnings:   warn,
	}, nil
}

func (e *Extractor) pdfToText(ctx context.Context, path string) (text string, pages int, warnings []string, err error) {
	// pdftotext -layout -enc UTF-8 -eol unix <path> -
	out, errb, err := e.runner.Run(ctx, e.cfg.Pdftotext, "-layout", "-enc", "UTF-8", "-eol", "unix", path, "-")
	if err != nil {
		return "", 0, []string{string(errb)}, err
	}
	text = string(out)
	// A form-feed \f is used as page separator by default
	pages = 1 + strings.Count(text, "\f")
	return text, pages, nil, nil
}

func (e *Extractor) pdfToOCR(ctx context.Context, path string) (text string, pages int, warnings []string, err error) {
	tmpDir, err := os.MkdirTemp("", "rt-pp-*")
	if err != nil {
		return "", 0, nil, err
	}
	defer os.RemoveAll(tmpDir)

	prefix := filepath.Join(tmpDir, "page")
	// pdftoppm -r 300 -png <in.pdf> <tmp/page>
	_, errb, err := e.runner.Run(ctx, e.cfg.Pdftoppm, "-r", fmt.Sprintf("%d", e.cfg.DPI), "-png", path, prefix)
	if err != nil {
		return "", 0, []string{string(errb)}, err
	}

	// collect generated pngs (prefix-1.png, prefix-2.png, ...)
	matches, _ := filepath.Glob(prefix + "-*.png")
	sort.Strings(matches)
	if e.cfg.MaxPages > 0 && len(matches) > e.cfg.MaxPages {
		matches = matches[:e.cfg.MaxPages]
	}
	if len(matches) == 0 {
		return "", 0, []string{"pdftoppm produced no images"}, fmt.Errorf("no pages rendered")
	}

	var b strings.Builder
	var warns []string
	for _, img := range matches {
		txt, w, err := e.tesseractOCR(ctx, img)
		if err != nil {
			warns = append(warns, err.Error())
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\f\n") // keep a clear page break marker
		}
		b.WriteString(txt)
		warns = append(warns, w...)
	}
	pages = len(matches)
	return b.String(), pages, warns, nil
}

var reBoxNoise = regexp.MustCompile(`(?m)^\s*[_\-]{3,}\s*$`)

func (e *Extractor) tesseractOCR(ctx context.Context, path string) (string, []string, error) {
	// tesseract <file> stdout -l <lang>
	out, errb, err := e.runner.Run(ctx, e.cfg.Tesseract, path, "stdout", "-l", e.cfg.TesseractLang)
	if err != nil {
		return "", []string{string(errb)}, fmt.Errorf("tesseract: %w", err)
	}
	// minor cleanup of obvious line noise
	txt := reBoxNoise.ReplaceAllString(string(out), "")
	return txt, nil, nil
}
