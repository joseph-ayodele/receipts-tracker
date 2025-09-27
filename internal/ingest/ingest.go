package ingest

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// IngestionResult is the per-file ingest outcome.
type IngestionResult struct {
	SourcePath   string
	FileID       string
	Deduplicated bool
	HashHex      string
	FileExt      string
	UploadedAt   time.Time
	Err          string
}

// DirStats summarizes a directory ingest.
type DirStats struct {
	Scanned      uint32
	Matched      uint32
	Succeeded    uint32
	Deduplicated uint32
	Failed       uint32
}

// Ingestor is the behavior the service depends on.
type Ingestor interface {
	// IngestPath a single path.
	IngestPath(ctx context.Context, profileID uuid.UUID, path string) (IngestionResult, error)
	// IngestDirectory ingests all matching files under root.
	IngestDirectory(ctx context.Context, profileID uuid.UUID, root string, skipHidden bool) ([]IngestionResult, DirStats, error)
}