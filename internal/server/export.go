package server

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/internal/services/export"

	v1 "github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
)

type ExportServer struct {
	v1.UnimplementedExportServiceServer
	svc    *export.Service
	logger *slog.Logger
}

func NewExportServer(svc *export.Service, logger *slog.Logger) *ExportServer {
	if logger == nil {
		logger = slog.Default()
	}
	return &ExportServer{svc: svc, logger: logger}
}

func (s *ExportServer) ExportReceipts(ctx context.Context, req *v1.ExportReceiptsRequest) (*v1.ExportReceiptsResponse, error) {
	pid := strings.TrimSpace(req.GetProfileId())
	profileID, err := uuid.Parse(pid)
	if err != nil || pid == "" {
		return nil, errInvalidArg("profile_id must be a UUID")
	}

	// Parse optional dates (YYYY-MM-DD). See user’s semantics:
	// - only from -> from..today (inclusive)
	// - only to   -> beginning..to (inclusive)
	// - none      -> all.
	var fromPtr, toPtr *time.Time
	if fd := strings.TrimSpace(req.GetFromDate()); fd != "" {
		t, err := time.Parse("2006-01-02", fd)
		if err != nil {
			return nil, errInvalidArg("from_date must be YYYY-MM-DD")
		}
		fromPtr = &t
	}
	if td := strings.TrimSpace(req.GetToDate()); td != "" {
		t, err := time.Parse("2006-01-02", td)
		if err != nil {
			return nil, errInvalidArg("to_date must be YYYY-MM-DD")
		}
		toPtr = &t
	}

	// If only from is given, make to = today
	if fromPtr != nil && toPtr == nil {
		today := time.Now().UTC()
		// use date only
		to := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
		toPtr = &to
	}

	// Call service
	xlsx, err := s.svc.ExportReceiptsXLSX(ctx, profileID, fromPtr, toPtr)
	if err != nil {
		s.logger.Error("export.xlsx.failed", "profile_id", pid, "err", err)
		return nil, errInternal(err.Error())
	}

	return &v1.ExportReceiptsResponse{Xlsx: xlsx}, nil
}

// --- minimal internal error helpers consistent with your gRPC style:

type invalidArg string

func (e invalidArg) Error() string   { return string(e) }
func errInvalidArg(msg string) error { return invalidArg(msg) }

type internalErr string

func (e internalErr) Error() string { return string(e) }
func errInternal(msg string) error  { return internalErr(msg) }
