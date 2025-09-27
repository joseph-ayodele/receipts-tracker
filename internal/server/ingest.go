package server

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	v1 "github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
	"github.com/joseph-ayodele/receipts-tracker/internal/ingest"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type IngestionService struct {
	v1.UnimplementedIngestionServiceServer
	ingestor    ingest.Ingestor
	profileRepo repository.ProfileRepository
}

func NewIngestionService(ing ingest.Ingestor, p repository.ProfileRepository) *IngestionService {
	return &IngestionService{
		ingestor:    ing,
		profileRepo: p,
	}
}

// Ingest implements v1.IngestionServiceServer
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

	if exists, _ := s.profileRepo.Exists(ctx, profileID); !exists {
		return nil, status.Error(codes.InvalidArgument, "profile not found")
	}

	r, err := s.ingestor.IngestPath(ctx, profileID, path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "ingest: %v", err)
	}

	return &v1.IngestResponse{
		FileId:         r.FileID,
		Deduplicated:   r.Deduplicated,
		ContentHashHex: r.HashHex,
		FileExt:        r.FileExt,
		UploadedAt:     r.UploadedAt.UTC().Format(time.RFC3339),
		SourcePath:     r.SourcePath,
		Error:          "",
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
	

	// default skipHidden := true when field not present (optional bool)
	skipHidden := true
	if req.SkipHidden != false {
		skipHidden = req.GetSkipHidden()
	}

	if exists, _ := s.profileRepo.Exists(ctx, profileID); !exists {
		return nil, status.Error(codes.InvalidArgument, "profile not found")
	}

	results, stats, err := s.ingestor.IngestDirectory(ctx, profileID, root, skipHidden)
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
			FileExt:        r.FileExt,
			UploadedAt:     r.UploadedAt.UTC().Format(time.RFC3339),
			SourcePath:     r.SourcePath,
			Error:          r.Err,
		})
	}
	return out, nil
}
