package ocr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joseph-ayodele/receipts-tracker/constants"
)

func (e *Extractor) extractPDF(ctx context.Context, path string) (ExtractionResult, error) {
	// 1) try pdftotext
	text, pages, warn, err := e.pdfToText(ctx, path)
	if err == nil && len(strings.TrimSpace(text)) >= 20 {
		return ExtractionResult{
			Text:       Normalize(text),
			Pages:      pages,
			SourceType: constants.PDF,
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
		return ExtractionResult{Warnings: w, SourceType: constants.PDF}, fmt.Errorf("pdf ocr failed: %w", err2)
	}
	return ExtractionResult{
		Text:       Normalize(text2),
		Pages:      pages2,
		SourceType: constants.PDF,
		Method:     "pdf-ocr",
		Language:   e.cfg.TesseractLang,
		Warnings:   append(warn, warn2...),
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
	defer func(path string) {
		if rmErr := os.RemoveAll(path); rmErr != nil {
			// keep this a warning; OCR succeeded already
			fmt.Printf("warning: failed to remove temp dir %q: %v\n", path, rmErr)
		}
	}(tmpDir)

	prefix := filepath.Join(tmpDir, "page")
	// pdftoppm -r <DPI> -png <in.pdf> <tmp/page>
	_, errb, runErr := e.runner.Run(ctx, e.cfg.Pdftoppm, "-r", fmt.Sprintf("%d", e.cfg.DPI), "-png", path, prefix)
	if runErr != nil {
		return "", 0, []string{string(errb)}, runErr
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
		res, err := e.extractImage(ctx, img)
		if err != nil {
			warns = append(warns, err.Error())
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\f\n") // clear page break marker
		}
		b.WriteString(res.Text)
		// include any page-level warnings (e.g., TSV errors)
		warns = append(warns, res.Warnings...)
	}

	// pages reported = number rendered (even if some failed OCR)
	return b.String(), len(matches), warns, nil
}
