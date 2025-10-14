package server

import (
	"context"
	"time"

	"log/slog"

	v1 "github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
	"github.com/joseph-ayodele/receipts-tracker/internal/ingest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type IngestionServer struct {
	v1.UnimplementedIngestionServiceServer
	svc    *ingest.Service
	logger *slog.Logger
}

func NewIngestionServer(svc *ingest.Service, logger *slog.Logger) *IngestionServer {
	return &IngestionServer{
		svc:    svc,
		logger: logger,
	}
}

// IngestFile implements v1.IngestionServiceServer
func (s *IngestionServer) IngestFile(ctx context.Context, req *v1.IngestFileRequest) (*v1.IngestResponse, error) {
	// Convert gRPC request to service request
	serviceReq := ingest.IngestFileRequest{
		ProfileID:      req.GetProfileId(),
		Path:           req.GetPath(),
		SkipDuplicates: req.GetSkipDuplicates(),
	}

	// Call service layer (pure business logic)
	r, err := s.svc.IngestFile(ctx, serviceReq)
	if err != nil {
		return nil, err
	}

	// Convert service response to gRPC response
	resp := &v1.IngestResponse{
		FileId:         r.FileID,
		Deduplicated:   r.Deduplicated,
		ContentHashHex: r.HashHex,
		FileExt:        r.FileExt,
		UploadedAt:     r.UploadedAt.UTC().Format(time.RFC3339),
		SourcePath:     r.SourcePath,
		Error:          "",
	}

	// Handle file processing if ingestion was successful
	if r.Err == "" && r.FileID != "" {
		if err := s.svc.ProcessIngestedFile(ctx, &r, req.GetSkipDuplicates()); err != nil {
			s.logger.Error("file processing failed", "file_id", r.FileID, "err", err)
			resp.Error = err.Error()
		}
	}

	return resp, nil
}

func (s *IngestionServer) IngestDirectory(ctx context.Context, req *v1.IngestDirectoryRequest) (*v1.IngestDirectoryResponse, error) {
	// Convert gRPC request to service request
	serviceReq := ingest.IngestDirectoryRequest{
		ProfileID:      req.GetProfileId(),
		RootPath:       req.GetRootPath(),
		SkipHidden:     req.GetSkipHidden(),
		SkipDuplicates: req.GetSkipDuplicates(),
	}

	// Call service layer (pure business logic)
	result, err := s.svc.IngestDirectory(ctx, serviceReq)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, status.Error(codes.Internal, "service returned nil result")
	}

	// Convert service response to gRPC response
	out := &v1.IngestDirectoryResponse{
		Scanned:      result.Statistics.Scanned,
		Matched:      result.Statistics.Matched,
		Succeeded:    result.Statistics.Succeeded,
		Deduplicated: result.Statistics.Deduplicated,
		Failed:       result.Statistics.Failed,
		Results:      make([]*v1.IngestResponse, 0, len(result.Results)),
	}

	// Process each ingested file
	for _, r := range result.Results {
		item := &v1.IngestResponse{
			FileId:         r.FileID,
			Deduplicated:   r.Deduplicated,
			ContentHashHex: r.HashHex,
			FileExt:        r.FileExt,
			UploadedAt:     r.UploadedAt.UTC().Format(time.RFC3339),
			SourcePath:     r.SourcePath,
			Error:          r.Err,
		}
		out.Results = append(out.Results, item)

		// Handle file processing if ingestion was successful
		if r.Err == "" && r.FileID != "" {
			if err := s.svc.ProcessIngestedFile(ctx, &r, req.GetSkipDuplicates()); err != nil {
				s.logger.Error("file processing failed", "file_id", r.FileID, "err", err)
				item.Error = err.Error()
			}
		}
	}

	return out, nil
}
