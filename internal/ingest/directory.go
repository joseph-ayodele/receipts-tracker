package ingest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type FileResult struct {
	Path        string
	FileID      string
	Deduplicated bool
	HashHex     string
	Err         string
}

type DirStats struct {
	Scanned      uint32
	Matched      uint32
	Succeeded    uint32
	Deduplicated uint32
	Failed       uint32
}

// IngestDirectory walks root, filters by includeExts (or defaults), skips hidden if requested,
// and calls IngestPath for each file. Returns per-file results + aggregate stats.
func (u *Usecase) IngestDirectory(ctx context.Context, profileID uuid.UUID, root string, includeExts []string, skipHidden bool) ([]FileResult, DirStats, error) {
	if strings.TrimSpace(root) == "" {
		return nil, DirStats{}, errors.New("root_path is required")
	}

	// Build ext set
	exts := map[string]struct{}{}
	if len(includeExts) == 0 {
		exts = map[string]struct{}{"pdf": {}, "jpg": {}, "jpeg": {}, "png": {}}
	} else {
		for _, e := range includeExts {
			e = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(e), "."))
			if e != "" {
				exts[e] = struct{}{}
			}
		}
	}

	var results []FileResult
	var stats DirStats

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		stats.Scanned++
		if walkErr != nil {
			results = append(results, FileResult{Path: path, Err: walkErr.Error()})
			stats.Failed++
			return nil // continue walking
		}
		// skip hidden dirs/files if requested
		if skipHidden && isHidden(path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			// hidden file: just skip
			return nil
		}
		// only files
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
		if _, ok := exts[ext]; !ok {
			return nil
		}
		stats.Matched++

		// per-file ingest with context
		fileID, dedup, hex, _, uploadedAt, src, err := u.IngestPath(ctx, profileID, path)
		if err != nil {
			results = append(results, FileResult{Path: path, Err: err.Error()})
			stats.Failed++
			return nil
		}
		_ = uploadedAt // not used in this summary; kept for parity
		_ = src

		results = append(results, FileResult{
			Path:         path,
			FileID:       fileID,
			Deduplicated: dedup,
			HashHex:      hex,
		})
		stats.Succeeded++
		if dedup {
			stats.Deduplicated++
		}
		return nil
	})

	if err != nil {
		return results, stats, fmt.Errorf("walk: %w", err)
	}
	return results, stats, nil
}

func isHidden(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, ".")
}
