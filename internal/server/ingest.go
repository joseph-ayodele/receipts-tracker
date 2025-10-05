package server

import (
	"context"
	"strings"
	"time"

	"log/slog"

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
	logger      *slog.Logger
}

func NewIngestionService(ing ingest.Ingestor, p repository.ProfileRepository, logger *slog.Logger) *IngestionService {
	return &IngestionService{
		ingestor:    ing,
		profileRepo: p,
		logger:      logger,
	}
}

// IngestFile implements v1.IngestionServiceServer
func (s *IngestionService) IngestFile(ctx context.Context, req *v1.IngestFileRequest) (*v1.IngestResponse, error) {
	pid := strings.TrimSpace(req.GetProfileId())
	if pid == "" {
		s.logger.Error("ingest request missing profile_id")
		return nil, status.Error(codes.InvalidArgument, "profile_id is required")
	}
	profileID, err := uuid.Parse(pid)
	if err != nil {
		s.logger.Error("invalid profile_id format for ingest", "profile_id", pid, "error", err)
		return nil, status.Error(codes.InvalidArgument, "profile_id must be a UUID")
	}

	path := strings.TrimSpace(req.GetPath())
	if path == "" {
		s.logger.Error("ingest request missing path", "profile_id", profileID)
		return nil, status.Error(codes.InvalidArgument, "path is required")
	}

	if exists, _ := s.profileRepo.Exists(ctx, profileID); !exists {
		s.logger.Error("profile not found for ingest", "profile_id", profileID)
		return nil, status.Error(codes.InvalidArgument, "profile not found")
	}

	s.logger.Info("starting file ingest", "profile_id", profileID, "path", path)
	r, err := s.ingestor.IngestPath(ctx, profileID, path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "ingest: %v", err)
	}
	s.logger.Info("file ingest succeeded", "profile_id", profileID, "file_id", r.FileID, "deduplicated", r.Deduplicated)

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
		s.logger.Error("ingest directory request missing profile_id")
		return nil, status.Error(codes.InvalidArgument, "profile_id is required")
	}
	profileID, err := uuid.Parse(pid)
	if err != nil {
		s.logger.Error("invalid profile_id format for ingest directory", "profile_id", pid, "error", err)
		return nil, status.Error(codes.InvalidArgument, "profile_id must be a UUID")
	}
	root := strings.TrimSpace(req.GetRootPath())
	if root == "" {
		s.logger.Error("ingest directory request missing root_path", "profile_id", profileID)
		return nil, status.Error(codes.InvalidArgument, "root_path is required")
	}

	// default skipHidden := true when field not present (optional bool)
	skipHidden := true
	if req.SkipHidden != false {
		skipHidden = req.GetSkipHidden()
	}

	if exists, _ := s.profileRepo.Exists(ctx, profileID); !exists {
		s.logger.Error("profile not found for ingest directory", "profile_id", profileID)
		return nil, status.Error(codes.InvalidArgument, "profile not found")
	}

	s.logger.Info("starting directory ingest", "profile_id", profileID, "root", root, "skip_hidden", skipHidden)
	results, stats, err := s.ingestor.IngestDirectory(ctx, profileID, root, skipHidden)
	if err != nil {
		// DB and file errors are already logged in repository/ingest layers
		return nil, status.Errorf(codes.InvalidArgument, "ingest directory: %v", err)
	}
	s.logger.Info("directory ingest completed", "profile_id", profileID, "scanned", stats.Scanned, "matched", stats.Matched, "succeeded", stats.Succeeded, "deduplicated", stats.Deduplicated, "failed", stats.Failed)

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
