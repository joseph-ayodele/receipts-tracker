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
		norm := Normalize(text)
		confidence := heuristicConfidence(norm) // heuristic only (no OCR confidence)
		return ExtractionResult{
			Text:       norm,
			Pages:      pages,
			SourceType: constants.PDF,
			Method:     "pdf-text",
			Language:   "", // n/a for text
			Warnings:   warn,
			Confidence: confidence,
		}, nil
	}

	// 2) fallback: rasterize + tesseract (per page via extractImage)
	pageResults, warn2, err2 := e.pdfToOCRPages(ctx, path)
	if err2 != nil {
		w := append(warn, warn2...)
		if err != nil {
			w = append(w, err.Error())
		}
		return ExtractionResult{Warnings: w, SourceType: constants.PDF}, fmt.Errorf("pdf ocr failed: %w", err2)
	}

	// concatenate text and aggregate confidence
	var b strings.Builder
	var allWarns []string
	var sumConf float32
	var nConf int
	for i, pr := range pageResults {
		if i > 0 {
			b.WriteString("\n\f\n") // explicit page break
		}
		b.WriteString(pr.Text)
		allWarns = append(allWarns, pr.Warnings...)
		if pr.Confidence > 0 {
			sumConf += pr.Confidence
			nConf++
		}
	}
	norm := Normalize(b.String())
	confidence := float32(0)
	if nConf > 0 {
		confidence = sumConf / float32(nConf)
	} else {
		// fallback if page confidences weren’t computed
		confidence = heuristicConfidence(norm)
	}

	return ExtractionResult{
		Text:       norm,
		Pages:      len(pageResults),
		SourceType: constants.PDF,
		Method:     "pdf-ocr",
		Language:   e.cfg.TesseractLang,
		Warnings:   append(warn, append(warn2, allWarns...)...),
		Confidence: confidence,
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

// pdfToOCRPages renders PDF pages to PNG then runs extractImage() per page.
// It returns per-page ExtractionResult (with per-page Confidence) and aggregated warnings.
func (e *Extractor) pdfToOCRPages(ctx context.Context, path string) ([]ExtractionResult, []string, error) {
	tmpDir, err := os.MkdirTemp("", "rt-pp-*")
	if err != nil {
		return nil, nil, err
	}
	defer func(path string) {
		if rmErr := os.RemoveAll(path); rmErr != nil {
			// keep as warning; OCR may have succeeded already
			fmt.Printf("warning: failed to remove temp dir %q: %v\n", path, rmErr)
		}
	}(tmpDir)

	prefix := filepath.Join(tmpDir, "page")
	// pdftoppm -r <DPI> -png <in.pdf> <tmp/page>
	_, errb, runErr := e.runner.Run(ctx, e.cfg.Pdftoppm, "-r", fmt.Sprintf("%d", e.cfg.DPI), "-png", path, prefix)
	if runErr != nil {
		return nil, []string{string(errb)}, runErr
	}

	// collect generated pngs (prefix-1.png, prefix-2.png, ...)
	matches, _ := filepath.Glob(prefix + "-*.png")
	sort.Strings(matches)
	if e.cfg.MaxPages > 0 && len(matches) > e.cfg.MaxPages {
		matches = matches[:e.cfg.MaxPages]
	}
	if len(matches) == 0 {
		return nil, []string{"pdftoppm produced no images"}, fmt.Errorf("no pages rendered")
	}

	var results []ExtractionResult
	var warns []string
	for _, img := range matches {
		pr, err := e.extractImage(ctx, img)
		if err != nil {
			warns = append(warns, err.Error())
			continue
		}
		results = append(results, pr)
		warns = append(warns, pr.Warnings...)
	}
	return results, warns, nil
}
