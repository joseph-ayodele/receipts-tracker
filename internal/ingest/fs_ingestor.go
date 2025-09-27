package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
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
	AllowedExts map[string]struct{} // lowercased sans '.'; nil -> default set
}

func NewFSIngestor(p repository.ProfileRepository, f repository.ReceiptFileRepository) *FSIngestor {
	return &FSIngestor{
		ProfileRepo: p,
		FilesRepo:   f,
	}
}

func (i *FSIngestor) IngestPath(ctx context.Context, profileID uuid.UUID, path string) (IngestionResult, error) {
	var out IngestionResult

	abs, err := filepath.Abs(path)
	if err != nil {
		log.Printf("abs path error: %v", err)
		return out, err
	}

	ext := constants.NormalizeExt(filepath.Ext(abs))
	if ext == "" || !AllowedExt(ext) {
		log.Printf("unsupported or missing extension: %q", ext)
		return out, fmt.Errorf("unsupported or missing extension")
	}

	if err := ValidateProfile(ctx, i.ProfileRepo, profileID); err != nil {
		return out, err
	}

	f, err := os.Open(abs)
	if err != nil {
		log.Printf("open error: %v", err)
		return out, err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Printf("close file error: %v", err)
		}
	}(f)

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Printf("hash error: %v", err)
		return out, err
	}
	sum := h.Sum(nil)
	now := time.Now().UTC()

	row, dedup, err := i.FilesRepo.UpsertByHash(ctx, profileID, abs, ext, sum, now)
	if err != nil {
		return out, err
	}

	out = IngestionResult{
		SourcePath:   row.SourcePath,
		FileID:       row.ID.String(),
		Deduplicated: dedup,
		HashHex:      hex.EncodeToString(sum),
		FileExt:      row.FileExt,
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
