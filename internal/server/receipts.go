package server

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/joseph-ayodele/receipts-tracker/internal/receipts"
	"github.com/joseph-ayodele/receipts-tracker/internal/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	receiptspb "github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
)

type ReceiptServer struct {
	receiptspb.UnimplementedReceiptsServiceServer
	svc    *receipts.Service
	logger *slog.Logger
}

func NewReceiptServer(svc *receipts.Service, logger *slog.Logger) *ReceiptServer {
	return &ReceiptServer{
		svc:    svc,
		logger: logger,
	}
}

func (s *ReceiptServer) ListReceipts(ctx context.Context, req *receiptspb.ListReceiptsRequest) (*receiptspb.ListReceiptsResponse, error) {
	// Parse optional dates from gRPC request
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

	// Convert gRPC request to service request
	serviceReq := receipts.ListReceiptsRequest{
		ProfileID: req.GetProfileId(),
		FromDate:  fromDate,
		ToDate:    toDate,
	}

	// Call service layer (pure business logic)
	recs, err := s.svc.ListReceipts(ctx, serviceReq)
	if err != nil {
		return nil, err
	}

	// Convert service response to gRPC response
	out := make([]*receiptspb.Receipt, 0, len(recs))
	for _, r := range recs {
		out = append(out, utils.ToPBReceiptFromEntity(r))
	}
	return &receiptspb.ListReceiptsResponse{Receipts: out}, nil
}

func (s *ReceiptServer) ExportReceipts(context.Context, *receiptspb.ExportReceiptsRequest) (*receiptspb.ExportReceiptsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ExportReceipts not implemented yet (Step 8)")
}
