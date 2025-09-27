package server

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/internal/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	receiptspb "github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"

	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

type Service struct {
	receiptspb.UnimplementedReceiptsServiceServer
	profiles repo.ProfileRepository
	receipts repo.ReceiptRepository
}

func New(profiles repo.ProfileRepository, receipts repo.ReceiptRepository) *Service {
	return &Service{profiles: profiles, receipts: receipts}
}

func (s *Service) CreateProfile(ctx context.Context, req *receiptspb.CreateProfileRequest) (*receiptspb.CreateProfileResponse, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	cur := strings.TrimSpace(req.GetDefaultCurrency())
	if cur == "" {
		cur = "USD"
	}
	if len(cur) != 3 {
		return nil, status.Error(codes.InvalidArgument, "default_currency must be 3 letters (ISO 4217)")
	}
	cur = strings.ToUpper(cur)

	p, err := s.profiles.CreateProfile(ctx, name, cur)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create profile: %v", err)
	}

	return &receiptspb.CreateProfileResponse{
		Profile: utils.ToPBProfile(p),
	}, nil
}

func (s *Service) ListProfiles(ctx context.Context, _ *receiptspb.ListProfilesRequest) (*receiptspb.ListProfilesResponse, error) {
	plist, err := s.profiles.ListProfiles(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list profiles: %v", err)
	}
	out := make([]*receiptspb.Profile, 0, len(plist))
	for _, p := range plist {
		out = append(out, utils.ToPBProfile(p))
	}
	return &receiptspb.ListProfilesResponse{Profiles: out}, nil
}

func (s *Service) ListReceipts(ctx context.Context, req *receiptspb.ListReceiptsRequest) (*receiptspb.ListReceiptsResponse, error) {
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

	recs, err := s.receipts.ListReceipts(ctx, profileID, fromDate, toDate)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list receipts: %v", err)
	}

	out := make([]*receiptspb.Receipt, 0, len(recs))
	for _, r := range recs {
		out = append(out, utils.ToPBReceipt(r))
	}
	return &receiptspb.ListReceiptsResponse{Receipts: out}, nil
}

func (s *Service) ExportReceipts(context.Context, *receiptspb.ExportReceiptsRequest) (*receiptspb.ExportReceiptsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ExportReceipts not implemented yet (Step 8)")
}
