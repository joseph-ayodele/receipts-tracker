package server

import (
	"context"
	"strings"

	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
	"github.com/joseph-ayodele/receipts-tracker/internal/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	receiptspb "github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
)

type ProfileService struct {
	receiptspb.UnimplementedProfilesServiceServer
	profileRepo repository.ProfileRepository
}

func NewProfileService(profileRepo repository.ProfileRepository) *ProfileService {
	return &ProfileService{
		profileRepo: profileRepo,
	}
}

// CreateProfile creates a new profile with the given name and default currency.
func (s *ProfileService) CreateProfile(ctx context.Context, req *receiptspb.CreateProfileRequest) (*receiptspb.CreateProfileResponse, error) {
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

	p, err := s.profileRepo.CreateProfile(ctx, name, cur)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create profile: %v", err)
	}

	return &receiptspb.CreateProfileResponse{
		Profile: utils.ToPBProfile(p),
	}, nil
}

// ListProfiles lists all the profileRepo.
func (s *ProfileService) ListProfiles(ctx context.Context, _ *receiptspb.ListProfilesRequest) (*receiptspb.ListProfilesResponse, error) {
	plist, err := s.profileRepo.ListProfiles(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list profileRepo: %v", err)
	}
	out := make([]*receiptspb.Profile, 0, len(plist))
	for _, p := range plist {
		out = append(out, utils.ToPBProfile(p))
	}
	return &receiptspb.ListProfilesResponse{Profiles: out}, nil
}
