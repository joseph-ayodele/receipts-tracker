package server

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	v1 "github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
	"github.com/joseph-ayodele/receipts-tracker/internal/ingest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DirectoryIngestor interface {
	IngestPath(ctx context.Context, profileID uuid.UUID, path string) (fileID string, dedup bool, hex string, ext string, uploadedAt time.Time, sourcePath string, err error)
	IngestDirectory(ctx context.Context, profileID uuid.UUID, root string, includeExts []string, skipHidden bool) ([]ingest.FileResult, ingest.DirStats, error)
}

type IngestionService struct {
	v1.UnimplementedIngestionServiceServer
	ing DirectoryIngestor
}

func NewIngestionService(ing DirectoryIngestor) *IngestionService {
	return &IngestionService{ing: ing}
}

func (s *IngestionService) Ingest(ctx context.Context, req *v1.IngestRequest) (*v1.IngestResponse, error) {
	pid := strings.TrimSpace(req.GetProfileId())
	if pid == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id is required")
	}
	profileID, err := uuid.Parse(pid)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "profile_id must be a UUID")
	}
	path := strings.TrimSpace(req.GetPath())
	if path == "" {
		return nil, status.Error(codes.InvalidArgument, "path is required")
	}

	fileID, dedup, hex, ext, uploadedAt, source, err := s.ing.IngestPath(ctx, profileID, path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "ingest: %v", err)
	}

	return &v1.IngestResponse{
		FileId:         fileID,
		Deduplicated:   dedup,
		ContentHashHex: hex,
		FileExt:        ext,
		UploadedAt:     uploadedAt.UTC().Format(time.RFC3339),
		SourcePath:     source,
	}, nil
}

func (s *IngestionService) IngestDirectory(ctx context.Context, req *v1.IngestDirectoryRequest) (*v1.IngestDirectoryResponse, error) {
	pid := strings.TrimSpace(req.GetProfileId())
	if pid == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id is required")
	}
	profileID, err := uuid.Parse(pid)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "profile_id must be a UUID")
	}
	root := strings.TrimSpace(req.GetRootPath())
	if root == "" {
		return nil, status.Error(codes.InvalidArgument, "root_path is required")
	}

	// normalize include_exts
	include := make([]string, 0, len(req.GetIncludeExts()))
	for _, e := range req.GetIncludeExts() {
		e = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(e)), ".")
		if e != "" {
			include = append(include, e)
		}
	}

	// default skipHidden := true when field not present (optional bool)
	skipHidden := true
	if req.SkipHidden != false {
		skipHidden = req.GetSkipHidden()
	}

	results, stats, err := s.ing.IngestDirectory(ctx, profileID, root, include, skipHidden)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "ingest directory: %v", err)
	}

	out := &v1.IngestDirectoryResponse{
		Scanned:      stats.Scanned,
		Matched:      stats.Matched,
		Succeeded:    stats.Succeeded,
		Deduplicated: stats.Deduplicated,
		Failed:       stats.Failed,
		Results:      make([]*v1.IngestResponse, 0, len(results)),
	}
	for _, r := range results {
		out.Results = append(out.Results, &v1.IngestResponse{
			FileId:         r.FileID,
			Deduplicated:   r.Deduplicated,
			ContentHashHex: r.HashHex,
			//FileExt:        r.FileExt,
			//UploadedAt:     r.UploadedAt.UTC().Format(time.RFC3339),
			SourcePath:     r.Path,
			Error:          r.Err,
		})
	}
	return out, nil
}
