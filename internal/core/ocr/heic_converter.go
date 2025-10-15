package ocr

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

type ctxKey string

const (
	ctxKeyContentHash ctxKey = "ocr.content_hash_hex"
)

// WithContentHash stores the hex-encoded SHA256 for downstream reuse.
func WithContentHash(ctx context.Context, hex string) context.Context {
	return context.WithValue(ctx, ctxKeyContentHash, hex)
}

func contentHashFromCtx(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeyContentHash).(string)
	return v, ok
}

// convertHEICtoPNG converts a HEIC/HEIF file to PNG.
// If cacheDir and hashHex are non-empty, it will persist (and reuse) the PNG at
//
//	{cacheDir}/{hashHex}.png
//
// Returns (outPath, warnings, cleanup, err).
// - When caching is used (file exists or was created), cleanup is nil.
// - When caching is not used, a temp directory is created and cleanup removes it.
func convertHEICtoPNG(
	ctx context.Context,
	r Runner,
	logger *slog.Logger,
	converter string,
	in string,
	cacheDir string,
	hashHex string,
) (string, []string, func(), error) {
	// Reuse cached artifact if possible
	if cacheDir != "" && hashHex != "" {
		cached := filepath.Join(cacheDir, hashHex+".png")
		if st, err := os.Stat(cached); err == nil && !st.IsDir() {
			logger.Debug("using cached heic->png", "cache", cached)
			return cached, nil, nil, nil
		}
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			return "", nil, nil, err
		}
		// Fall through to produce tmp then persist to cache
	}

	// Produce a temporary PNG (legacy temp-path behavior)
	tmpDir, err := os.MkdirTemp("", "rt-heic-*")
	if err != nil {
		return "", nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	out := filepath.Join(tmpDir, "page.png")

	switch converter {
	case "heif-convert":
		if _, errb, err2 := r.Run(ctx, "heif-convert", logger, in, out); err2 != nil {
			return "", []string{string(errb)}, cleanup, fmt.Errorf("heif-convert failed: %w", err2)
		}
	case "magick":
		if _, errb, err2 := r.Run(ctx, "magick", logger, in, out); err2 != nil {
			return "", []string{string(errb)}, cleanup, fmt.Errorf("magick convert failed: %w", err2)
		}
	case "sips":
		if _, errb, err2 := r.Run(ctx, "sips", logger, "-s", "format", "png", in, "--out", out); err2 != nil {
			return "", []string{string(errb)}, cleanup, fmt.Errorf("sips convert failed: %w", err2)
		}
	default:
		return "", nil, cleanup, fmt.Errorf("HEIC not supported: set ocr.Config.HeicConverter to one of: heif-convert | magick | sips")
	}

	if _, statErr := os.Stat(out); statErr != nil {
		return "", nil, cleanup, fmt.Errorf("HEIC conversion produced no output: %v", statErr)
	}

	// If we have cache info, persist the temp PNG into the cache and return the cached path.
	if cacheDir != "" && hashHex != "" {
		cached := filepath.Join(cacheDir, hashHex+".png")

		// try atomic rename; if it fails (e.g., EXDEV), copy then remove tmp
		if err := os.Rename(out, cached); err != nil {
			// if another process already wrote it, use existing and discard tmp
			if st, statErr := os.Stat(cached); statErr == nil && !st.IsDir() {
				cleanup()
				logger.Debug("cached heic->png already present", "cache", cached)
				return cached, nil, nil, nil
			}
			inF, e := os.Open(out)
			if e != nil {
				cleanup()
				return "", nil, nil, e
			}
			defer func(inF *os.File) {
				err := inF.Close()
				if err != nil {
					logger.Warn("failed to close heic->png temp file", "file", inF.Name(), "error", err)
				}
			}(inF)

			outF, e := os.Create(cached)
			if e != nil {
				err := inF.Close()
				if err != nil {
					return "", nil, nil, err
				}
				cleanup()
				return "", nil, nil, e
			}
			if _, e = io.Copy(outF, inF); e != nil {
				err := outF.Close()
				if err != nil {
					return "", nil, nil, err
				}
				cleanup()
				return "", nil, nil, e
			}
			if e = outF.Close(); e != nil {
				cleanup()
				return "", nil, nil, e
			}
			// Remove the temp dir now that we persisted
			cleanup()
		}

		logger.Debug("cached heic->png", "cache", cached)
		return cached, nil, nil, nil
	}

	// No cache parameters → return temp path and cleanup func
	return out, nil, cleanup, nil
}
