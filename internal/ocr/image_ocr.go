package ocr

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/joseph-ayodele/receipts-tracker/constants"
)

const ImageConfidenceThreshold = 0.6

func (e *Extractor) extractImage(ctx context.Context, path string) (ExtractionResult, error) {
	txt, warn, err := e.tesseractOCR(ctx, path)
	if err != nil {
		return ExtractionResult{SourceType: constants.IMAGE, Warnings: warn}, err
	}
	txt = Normalize(txt)

	// compute confidence
	var ocrConf float32
	var warn2 []string
	if e.cfg.EnableTSVConfidence {
		if c, w, err2 := e.tesseractTSVConfidence(ctx, path); err2 == nil {
			ocrConf = c
			warn = append(warn, w...)
		} else {
			warn2 = append(warn2, err2.Error())
		}
	}
	heurConf := heuristicConfidence(txt)

	// blend: weight OCR higher if present
	var conf float32
	if ocrConf > 0 {
		conf = 0.7*ocrConf + 0.3*heurConf
	} else {
		conf = heurConf
	}
	if conf > 1.0 {
		conf = 1.0
	}

	warn = append(warn, warn2...)

	return ExtractionResult{
		Text:       txt,
		Pages:      1,
		SourceType: constants.IMAGE,
		Method:     "image-ocr",
		Language:   e.cfg.TesseractLang,
		Warnings:   warn,
		Confidence: conf,
	}, nil
}

func (e *Extractor) tesseractOCR(ctx context.Context, path string) (string, []string, error) {
	args := []string{path, "stdout", "-l", e.cfg.TesseractLang}
	if e.cfg.TessdataDir != "" {
		args = append(args, "--tessdata-dir", e.cfg.TessdataDir)
	}

	// tesseract <file> stdout -l <lang>
	out, errb, err := e.runner.Run(ctx, e.cfg.Tesseract, args...)
	if err != nil {
		return "", []string{string(errb)}, fmt.Errorf("tesseract: %w", err)
	}

	// minor cleanup of obvious line noise
	txt := reBoxNoise.ReplaceAllString(string(out), "")
	return txt, nil, nil
}

// tesseractTSVConfidence runs tesseract in TSV mode and returns mean word conf in 0..1.
func (e *Extractor) tesseractTSVConfidence(ctx context.Context, path string) (float32, []string, error) {
	args := []string{path, "stdout", "-l", e.cfg.TesseractLang}
	if e.cfg.PSM > 0 {
		args = append(args, "--psm", fmt.Sprintf("%d", e.cfg.PSM))
	}
	if e.cfg.OEM > 0 {
		args = append(args, "--oem", fmt.Sprintf("%d", e.cfg.OEM))
	}
	if e.cfg.TessdataDir != "" {
		args = append(args, "--tessdata-dir", e.cfg.TessdataDir)
	}
	// TSV output
	args = append(args, "tsv")

	out, errb, err := e.runner.Run(ctx, e.cfg.Tesseract, args...)
	if err != nil {
		return 0, []string{string(errb)}, fmt.Errorf("tesseract TSV: %w", err)
	}
	lines := strings.Split(string(out), "\n")
	// conf column is the last; header line includes "conf"
	var sum, n float64
	for i, ln := range lines {
		if i == 0 || len(ln) == 0 {
			continue
		} // skip header
		cols := strings.Split(ln, "\t")
		if len(cols) < 12 {
			continue
		} // defensive
		confStr := cols[len(cols)-1]
		if confStr == "" || confStr == "-1" {
			continue
		}
		if v, err := strconv.ParseFloat(confStr, 64); err == nil {
			sum += v
			n++
		}
	}
	if n == 0 {
		return 0, nil, nil
	}
	mean := sum / n // 0..100
	return float32(mean / 100.0), nil, nil
}
