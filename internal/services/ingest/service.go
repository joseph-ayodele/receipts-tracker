package ingest

import (
	"context"
	"strings"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/internal/core/async"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Service handles ingestion business logic.
type Service struct {
	ingestor    Ingestor
	profileRepo repository.ProfileRepository
	queue       async.Queue
	logger      *slog.Logger
}

// NewService creates a new ingest service.
func NewService(ing Ingestor, p repository.ProfileRepository, q async.Queue, logger *slog.Logger) *Service {
	return &Service{
		ingestor:    ing,
		profileRepo: p,
		queue:       q,
		logger:      logger,
	}
}

// FileIngestRequest represents file ingestion parameters.
type FileIngestRequest struct {
	ProfileID      string
	Path           string
	SkipDuplicates bool
}

// DirectoryIngestResult represents directory ingestion results.
type DirectoryIngestResult struct {
	Statistics DirStats
	Results    []IngestionResult
}

// IngestFile ingests a single file.
func (s *Service) IngestFile(ctx context.Context, req FileIngestRequest) (IngestionResult, error) {
	profileID, err := uuid.Parse(strings.TrimSpace(req.ProfileID))
	if err != nil {
		s.logger.Error("invalid profile_id format for ingest", "profile_id", req.ProfileID, "error", err)
		return IngestionResult{}, status.Error(codes.InvalidArgument, "profile_id must be a UUID")
	}

	path := strings.TrimSpace(req.Path)
	if path == "" {
		s.logger.Error("ingest request missing path", "profile_id", profileID)
		return IngestionResult{}, status.Error(codes.InvalidArgument, "path is required")
	}

	if exists, _ := s.profileRepo.Exists(ctx, profileID); !exists {
		s.logger.Error("profile not found for ingest", "profile_id", profileID)
		return IngestionResult{}, status.Error(codes.InvalidArgument, "profile not found")
	}

	s.logger.Info("starting file ingest", "profile_id", profileID, "path", path)
	r, err := s.ingestor.IngestPath(ctx, profileID, path)
	if err != nil {
		return IngestionResult{}, status.Errorf(codes.InvalidArgument, "ingest: %v", err)
	}

	s.logger.Info("file ingest succeeded", "profile_id", profileID, "file_id", r.FileID, "deduplicated", r.Deduplicated)

	return r, nil
}

// DirectoryIngestRequest represents directory ingestion parameters.
type DirectoryIngestRequest struct {
	ProfileID      string
	RootPath       string
	SkipHidden     bool
	SkipDuplicates bool
}

// IngestDirectory ingests all files in a directory.
func (s *Service) IngestDirectory(ctx context.Context, req DirectoryIngestRequest) (*DirectoryIngestResult, error) {
	profileID, err := uuid.Parse(strings.TrimSpace(req.ProfileID))
	if err != nil {
		s.logger.Error("invalid profile_id format for ingest directory", "profile_id", req.ProfileID, "error", err)
		return nil, status.Error(codes.InvalidArgument, "profile_id must be a UUID")
	}

	root := strings.TrimSpace(req.RootPath)
	if root == "" {
		s.logger.Error("ingest directory request missing root_path", "profile_id", profileID)
		return nil, status.Error(codes.InvalidArgument, "root_path is required")
	}

	// default skipHidden := true when field not present (optional bool)
	skipHidden := req.SkipHidden
	if !req.SkipHidden {
		skipHidden = true
	}
	// Note: skipDuplicates is not used in directory ingestion business logic
	// as duplicate handling is done at the file level during processing

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

	return &DirectoryIngestResult{
		Statistics: stats,
		Results:    results,
	}, nil
}

// ProcessIngestedFile contains the business logic for processing an ingested file
func (s *Service) ProcessIngestedFile(ctx context.Context, result *IngestionResult, skipDuplicates bool) error {
	if result.Err != "" || result.FileID == "" {
		return nil // Skip processing if there was an error or no file ID
	}

	fileUUID, err := uuid.Parse(result.FileID)
	if err != nil {
		s.logger.Error("invalid file_id: cannot enqueue", "file_id", result.FileID, "error", err)
		return status.Error(codes.InvalidArgument, "invalid file_id")
	}

	if result.Deduplicated && skipDuplicates {
		s.logger.Info("skipping processing (duplicate)", "file_id", result.FileID, "path", result.SourcePath)
		return nil
	}

	if err := s.queue.Enqueue(ctx, async.Job{
		FileID:      fileUUID,
		Force:       !skipDuplicates && result.Deduplicated,
		SubmittedAt: time.Now(),
	}); err != nil {
		s.logger.Error("enqueue failed for file", "file_id", result.FileID, "err", err)
		return status.Errorf(codes.Internal, "enqueue failed: %v", err)
	}

	return nil
}
