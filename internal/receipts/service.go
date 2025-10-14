package receipts

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/internal/entity"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
	"github.com/joseph-ayodele/receipts-tracker/internal/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Service handles receipt business logic.
type Service struct {
	receiptRepo repository.ReceiptRepository
	logger      *slog.Logger
}

// NewService creates a new receipt service.
func NewService(receiptRepo repository.ReceiptRepository, logger *slog.Logger) *Service {
	return &Service{
		receiptRepo: receiptRepo,
		logger:      logger,
	}
}

// ListReceiptsRequest represents receipt listing parameters.
type ListReceiptsRequest struct {
	ProfileID string
	FromDate  *time.Time
	ToDate    *time.Time
}

// ListReceipts returns receipts for a profile.
func (s *Service) ListReceipts(ctx context.Context, req ListReceiptsRequest) ([]*entity.Receipt, error) {
	if strings.TrimSpace(req.ProfileID) == "" {
		s.logger.Error("list receipts request missing profile_id")
		return nil, status.Error(codes.InvalidArgument, "profile_id is required")
	}

	profileID, err := uuid.Parse(req.ProfileID)
	if err != nil {
		s.logger.Error("invalid profile_id format for list receipts", "profile_id", req.ProfileID, "error", err)
		return nil, status.Error(codes.InvalidArgument, "profile_id must be a UUID")
	}

	var fromDate, toDate *time.Time
	if req.FromDate != nil {
		from, err := utils.ParseYMD(req.FromDate.Format("2006-01-02"))
		if err != nil {
			s.logger.Error("invalid from_date format", "from_date", req.FromDate, "error", err)
			return nil, status.Errorf(codes.InvalidArgument, "from_date invalid (YYYY-MM-DD): %v", err)
		}
		fromDate = &from
	}
	if req.ToDate != nil {
		to, err := utils.ParseYMD(req.ToDate.Format("2006-01-02"))
		if err != nil {
			s.logger.Error("invalid to_date format", "to_date", req.ToDate, "error", err)
			return nil, status.Errorf(codes.InvalidArgument, "to_date invalid (YYYY-MM-DD): %v", err)
		}
		toDate = &to
	}

	s.logger.Info("listing receipts", "profile_id", profileID, "from_date", fromDate, "to_date", toDate)
	recs, err := s.receiptRepo.ListReceipts(ctx, profileID, fromDate, toDate)
	if err != nil {
		s.logger.Error("failed to list receipts", "profile_id", profileID, "error", err)
		return nil, status.Errorf(codes.Internal, "list receipts: %v", err)
	}

	s.logger.Info("receipts listed successfully", "profile_id", profileID, "count", len(recs))
	return recs, nil
}
