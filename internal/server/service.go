package server

import (
	"context"
	"fmt"
	receiptsv2 "receipts-tracker/gen/receipts/v1"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"receipts-tracker/internal/repository"
)

type ReceiptsService struct {
	receiptsv2.UnimplementedReceiptsServiceServer
	pool   repository.Pool
	logger *zap.Logger
}

func NewReceiptsService(pool repository.Pool, logger *zap.Logger) *ReceiptsService {
	return &ReceiptsService{pool: pool, logger: logger}
}

func (s *ReceiptsService) CreateProfile(ctx context.Context, req *receiptsv2.CreateProfileRequest) (*receiptsv2.CreateProfileResponse, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	defCur := req.GetDefaultCurrency()

	p, err := repository.CreateProfile(ctx, s.pool, name, defCur)
	if err != nil {
		s.logger.Warn("create profile failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "create profile failed")
	}

	return &receiptsv2.CreateProfileResponse{
		Profile: &receiptsv2.Profile{
			Id:              p.ID,
			Name:            p.Name,
			DefaultCurrency: p.DefaultCurrency,
			CreatedAt:       p.CreatedAt.Format(time.RFC3339Nano),
			UpdatedAt:       p.UpdatedAt.Format(time.RFC3339Nano),
		},
	}, nil
}

func (s *ReceiptsService) ListProfiles(ctx context.Context, _ *receiptsv2.ListProfilesRequest) (*receiptsv2.ListProfilesResponse, error) {
	ps, err := repository.ListProfiles(ctx, s.pool)
	if err != nil {
		s.logger.Warn("list profiles failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "list profiles failed")
	}
	out := make([]*receiptsv2.Profile, 0, len(ps))
	for _, p := range ps {
		out = append(out, &receiptsv2.Profile{
			Id:              p.ID,
			Name:            p.Name,
			DefaultCurrency: p.DefaultCurrency,
			CreatedAt:       p.CreatedAt.Format(time.RFC3339Nano),
			UpdatedAt:       p.UpdatedAt.Format(time.RFC3339Nano),
		})
	}
	return &receiptsv2.ListProfilesResponse{Profiles: out}, nil
}

func (s *ReceiptsService) ListReceipts(ctx context.Context, req *receiptsv2.ListReceiptsRequest) (*receiptsv2.ListReceiptsResponse, error) {
	profileID := req.GetProfileId()
	if profileID == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id is required")
	}

	var fromPtr, toPtr *time.Time
	parseDate := func(s string) (*time.Time, error) {
		if s == "" {
			return nil, nil
		}
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return nil, fmt.Errorf("invalid date %q: %w", s, err)
		}
		return &t, nil
	}

	var err error
	if fromPtr, err = parseDate(req.GetFromDate()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if toPtr, err = parseDate(req.GetToDate()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	recs, err := repository.ListReceipts(ctx, s.pool, profileID, fromPtr, toPtr)
	if err != nil {
		s.logger.Warn("list receipts failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "list receipts failed")
	}

	out := make([]*receiptsv2.Receipt, 0, len(recs))
	for _, r := range recs {
		out = append(out, &receiptsv2.Receipt{
			Id:           r.ID,
			ProfileId:    r.ProfileID,
			MerchantName: r.MerchantName,
			TxDate:       r.TxDate.Format("2006-01-02"),
			Total:        r.Total, // already decimal string
			CurrencyCode: r.CurrencyCode,
			CreatedAt:    r.CreatedAt.Format(time.RFC3339Nano),
			UpdatedAt:    r.UpdatedAt.Format(time.RFC3339Nano),
		})
	}
	return &receiptsv2.ListReceiptsResponse{Receipts: out}, nil
}

func (s *ReceiptsService) ExportReceipts(context.Context, *receiptsv2.ExportReceiptsRequest) (*receiptsv2.ExportReceiptsResponse, error) {
	// Implemented in Step 8
	return nil, status.Error(codes.Unimplemented, "ExportReceipts not implemented yet")
}
