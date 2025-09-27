package server

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
	"github.com/joseph-ayodele/receipts-tracker/internal/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	receiptspb "github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
)

type ReceiptService struct {
	receiptspb.UnimplementedReceiptsServiceServer
	receiptRepo repository.ReceiptRepository
	logger      *slog.Logger
}

func NewReceiptService(receiptRepo repository.ReceiptRepository, logger *slog.Logger) *ReceiptService {
	return &ReceiptService{
		receiptRepo: receiptRepo,
		logger:      logger,
	}
}

func (s *ReceiptService) ListReceipts(ctx context.Context, req *receiptspb.ListReceiptsRequest) (*receiptspb.ListReceiptsResponse, error) {
	if strings.TrimSpace(req.GetProfileId()) == "" {
		s.logger.Error("list receipts request missing profile_id")
		return nil, status.Error(codes.InvalidArgument, "profile_id is required")
	}
	profileID, err := uuid.Parse(req.GetProfileId())
	if err != nil {
		s.logger.Error("invalid profile_id format for list receipts", "profile_id", req.GetProfileId(), "error", err)
		return nil, status.Error(codes.InvalidArgument, "profile_id must be a UUID")
	}

	var fromDate, toDate *time.Time
	if fd := strings.TrimSpace(req.GetFromDate()); fd != "" {
		from, err := utils.ParseYMD(fd)
		if err != nil {
			s.logger.Error("invalid from_date format", "from_date", fd, "error", err)
			return nil, status.Errorf(codes.InvalidArgument, "from_date invalid (YYYY-MM-DD): %v", err)
		}
		fromDate = &from
	}
	if td := strings.TrimSpace(req.GetToDate()); td != "" {
		to, err := utils.ParseYMD(td)
		if err != nil {
			s.logger.Error("invalid to_date format", "to_date", td, "error", err)
			return nil, status.Errorf(codes.InvalidArgument, "to_date invalid (YYYY-MM-DD): %v", err)
		}
		toDate = &to
	}

	s.logger.Info("listing receipts", "profile_id", profileID, "from_date", fromDate, "to_date", toDate)
	recs, err := s.receiptRepo.ListReceipts(ctx, profileID, fromDate, toDate)
	if err != nil {
		s.logger.Error("failed to list receipts", "profile_id", profileID, "error", err)
		return nil, status.Errorf(codes.Internal, "list receiptRepo: %v", err)
	}
	s.logger.Info("receipts listed successfully", "profile_id", profileID, "count", len(recs))

	out := make([]*receiptspb.Receipt, 0, len(recs))
	for _, r := range recs {
		out = append(out, utils.ToPBReceipt(r))
	}
	return &receiptspb.ListReceiptsResponse{Receipts: out}, nil
}

func (s *ReceiptService) ExportReceipts(context.Context, *receiptspb.ExportReceiptsRequest) (*receiptspb.ExportReceiptsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ExportReceipts not implemented yet (Step 8)")
}
