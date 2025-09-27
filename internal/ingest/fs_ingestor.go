package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/constants"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

// FSIngestor reads from the local filesystem.
type FSIngestor struct {
	ProfileRepo repository.ProfileRepository
	FilesRepo   repository.ReceiptFileRepository
	logger      *slog.Logger
}

func NewFSIngestor(p repository.ProfileRepository, f repository.ReceiptFileRepository, logger *slog.Logger) *FSIngestor {
	return &FSIngestor{
		ProfileRepo: p,
		FilesRepo:   f,
		logger:      logger,
	}
}

func (i *FSIngestor) IngestPath(ctx context.Context, profileID uuid.UUID, path string) (IngestionResult, error) {
	var out IngestionResult

	abs, err := filepath.Abs(path)
	if err != nil {
		i.logger.Error("abs path error", "error", err, "path", path)
		return out, err
	}

	ext := constants.NormalizeExt(filepath.Ext(abs))
	if ext == "" || !AllowedExt(ext) {
		i.logger.Warn("unsupported or missing extension", "ext", ext, "path", path)
		return out, fmt.Errorf("unsupported or missing extension")
	}

	f, err := os.Open(abs)
	if err != nil {
		i.logger.Error("file open error", "error", err, "path", path)
		return out, err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			i.logger.Error("close file error", "error", err, "path", path)
		}
	}(f)

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		i.logger.Error("hash error", "error", err, "path", path)
		return out, err
	}
	sum := h.Sum(nil)
	now := time.Now().UTC()

	stat, _ := f.Stat()
	filename := filepath.Base(abs)
	size := int(stat.Size())

	row, dedup, err := i.FilesRepo.UpsertByHash(ctx, profileID, abs, filename, ext, size, sum, now)
	if err != nil {
		return out, err
	}

	out = IngestionResult{
		SourcePath:   row.SourcePath,
		FileID:       row.ID.String(),
		Deduplicated: dedup,
		HashHex:      hex.EncodeToString(sum),
		FileExt:      row.FileExt,
		FileSize:     row.FileSize,
		Filename:     row.Filename,
		UploadedAt:   row.UploadedAt,
	}
	return out, nil
}

// IngestDirectory walks root, skips hidden if requested,
// and calls IngestPath for each file. Returns per-file results + aggregate stats.
func (i *FSIngestor) IngestDirectory(
	ctx context.Context,
	profileID uuid.UUID,
	root string,
	skipHidden bool,
) ([]IngestionResult, DirStats, error) {
	if strings.TrimSpace(root) == "" {
		return nil, DirStats{}, errors.New("root_path is required")
	}

	var results []IngestionResult
	var stats DirStats

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		stats.Scanned++
		if walkErr != nil {
			results = append(results, IngestionResult{SourcePath: path, Err: walkErr.Error()})
			stats.Failed++
			return nil
		}
		if skipHidden && IsHidden(path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}
		ext := constants.NormalizeExt(filepath.Ext(path))
		if !AllowedExt(ext) {
			return nil
		}
		stats.Matched++

		r, err := i.IngestPath(ctx, profileID, path)
		if err != nil {
			results = append(results, IngestionResult{SourcePath: path, Err: err.Error()})
			stats.Failed++
			return nil
		}

		results = append(results, r)
		stats.Succeeded++
		if r.Deduplicated {
			stats.Deduplicated++
		}
		return nil
	})

	if err != nil {
		return results, stats, fmt.Errorf("walk: %w", err)
	}
	return results, stats, nil
}
