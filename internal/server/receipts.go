package server

import (
	"context"
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
}

func NewReceiptService(receiptRepo repository.ReceiptRepository) *ReceiptService {
	return &ReceiptService{
		receiptRepo: receiptRepo,
	}
}

func (s *ReceiptService) ListReceipts(ctx context.Context, req *receiptspb.ListReceiptsRequest) (*receiptspb.ListReceiptsResponse, error) {
	if strings.TrimSpace(req.GetProfileId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id is required")
	}
	profileID, err := uuid.Parse(req.GetProfileId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "profile_id must be a UUID")
	}

	var fromDate, toDate *time.Time
	if fd := strings.TrimSpace(req.GetFromDate()); fd != "" {
		from, err := utils.ParseYMD(fd)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "from_date invalid (YYYY-MM-DD): %v", err)
		}
		fromDate = &from
	}
	if td := strings.TrimSpace(req.GetToDate()); td != "" {
		to, err := utils.ParseYMD(td)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "to_date invalid (YYYY-MM-DD): %v", err)
		}
		toDate = &to
	}

	recs, err := s.receiptRepo.ListReceipts(ctx, profileID, fromDate, toDate)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list receiptRepo: %v", err)
	}

	out := make([]*receiptspb.Receipt, 0, len(recs))
	for _, r := range recs {
		out = append(out, utils.ToPBReceipt(r))
	}
	return &receiptspb.ListReceiptsResponse{Receipts: out}, nil
}

func (s *ReceiptService) ExportReceipts(context.Context, *receiptspb.ExportReceiptsRequest) (*receiptspb.ExportReceiptsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ExportReceipts not implemented yet (Step 8)")
}
