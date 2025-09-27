package server

import (
	"context"
	"log/slog"
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
	logger      *slog.Logger
}

func NewProfileService(profileRepo repository.ProfileRepository, logger *slog.Logger) *ProfileService {
	return &ProfileService{
		profileRepo: profileRepo,
		logger:      logger,
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

	s.logger.Info("creating profile", "name", name, "currency", cur)

	p, err := s.profileRepo.CreateProfile(ctx, name, cur)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create profile: %v", err)
	}

	s.logger.Info("profile created successfully", "profile_id", p.ID, "name", p.Name, "currency", p.DefaultCurrency)

	return &receiptspb.CreateProfileResponse{
		Profile: utils.ToPBProfile(p),
	}, nil
}

// ListProfiles lists all the profileRepo.
func (s *ProfileService) ListProfiles(ctx context.Context, _ *receiptspb.ListProfilesRequest) (*receiptspb.ListProfilesResponse, error) {
	s.logger.Info("listing profiles")

	plist, err := s.profileRepo.ListProfiles(ctx)
	if err != nil {
		// DB error already logged in repository layer
		return nil, status.Errorf(codes.Internal, "list profileRepo: %v", err)
	}

	s.logger.Info("profiles listed successfully", "count", len(plist))

	out := make([]*receiptspb.Profile, 0, len(plist))
	for _, p := range plist {
		out = append(out, utils.ToPBProfile(p))
	}
	return &receiptspb.ListProfilesResponse{Profiles: out}, nil
}
