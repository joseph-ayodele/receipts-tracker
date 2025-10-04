package ocr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// convertHEICtoPNG converts a HEIC/HEIF file to a temporary PNG using the chosen converter.
// converter: "heif-convert" | "magick" | "sips"
//
// Returns (outPath, warnings, cleanup, err). Call cleanup() to remove temp files.
func convertHEICtoPNG(ctx context.Context, r Runner, converter, in string) (string, []string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "rt-heic-*")
	if err != nil {
		return "", nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	out := filepath.Join(tmpDir, "page.png")

	switch converter {
	case "heif-convert":
		if _, errb, err2 := r.Run(ctx, "heif-convert", in, out); err2 != nil {
			return "", []string{string(errb)}, cleanup, fmt.Errorf("heif-convert failed: %w", err2)
		}
	case "magick":
		if _, errb, err2 := r.Run(ctx, "magick", in, out); err2 != nil {
			return "", []string{string(errb)}, cleanup, fmt.Errorf("magick convert failed: %w", err2)
		}
	case "sips":
		if _, errb, err2 := r.Run(ctx, "sips", "-s", "format", "png", in, "--out", out); err2 != nil {
			return "", []string{string(errb)}, cleanup, fmt.Errorf("sips convert failed: %w", err2)
		}
	default:
		return "", nil, cleanup, fmt.Errorf("HEIC not supported: set ocr.Config.HeicConverter to one of: heif-convert | magick | sips")
	}

	if _, statErr := os.Stat(out); statErr != nil {
		return "", nil, cleanup, fmt.Errorf("HEIC conversion produced no output: %v", statErr)
	}
	return out, nil, cleanup, nil
}

// naive heuristic confidence based on decoded text characteristics
func heuristicConfidence(txt string) float32 {
	// very simple: boost if we see common receipt artifacts
	// (date-ish, currency-ish, amount-ish). Each adds ~0.15.
	txtL := strings.ToLower(txt)
	score := float32(0.2) // base
	if hasDatePattern(txtL) {
		score += 0.2
	}
	if hasCurrencyPattern(txtL) {
		score += 0.15
	}
	if hasAmountPattern(txtL) {
		score += 0.15
	}
	if len(txt) > 120 {
		score += 0.1
	} // enough content
	if score > 1.0 {
		score = 1.0
	}
	return score
}
