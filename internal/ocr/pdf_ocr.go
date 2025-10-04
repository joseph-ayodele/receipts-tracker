package ocr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
		err := os.RemoveAll(path)
		if err != nil {
			fmt.Printf("warning: failed to remove temp dir %q: %v\n", path, err)
		}
	}(tmpDir)

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
