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

type IngestionServer struct {
	v1.UnimplementedIngestionServiceServer
	uc *ingest.Usecase
}

func NewIngestionServerWithUsecase(uc *ingest.Usecase) *IngestionServer {
	return &IngestionServer{uc: uc}
}

func (s *IngestionServer) Ingest(ctx context.Context, req *v1.IngestRequest) (*v1.IngestResponse, error) {
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

	row, dedup, shaHex, err := s.uc.IngestPath(ctx, profileID, path)
	if err != nil {
		// Distinguish invalid vs internal briefly; expand later if needed.
		return nil, status.Errorf(codes.InvalidArgument, "ingest: %v", err)
	}

	return &v1.IngestResponse{
		FileId:         row.ID.String(),
		Deduplicated:   dedup,
		ContentHashHex: shaHex,
		FileExt:        row.FileExt,
		UploadedAt:     row.UploadedAt.UTC().Format(time.RFC3339),
		SourcePath:     row.SourcePath,
	}, nil
}
