package server

import (
	"context"
	"log/slog"
	"strings"

	"github.com/joseph-ayodele/receipts-tracker/constants"
	"github.com/joseph-ayodele/receipts-tracker/internal/entity"
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

func extractProfileInput(req *receiptspb.CreateProfileRequest) (*entity.Profile, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	jobTitle := strings.TrimSpace(req.GetJobTitle())
	jobDesc := strings.TrimSpace(req.GetJobDescription())

	cur := strings.ToUpper(strings.TrimSpace(req.GetDefaultCurrency()))
	if cur == "" {
		cur = constants.DefaultCurrency
	} else if len(cur) != 3 {
		return nil, status.Error(codes.InvalidArgument, "default currency must be 3 letters (ISO 4217)")
	}

	jobTitlePtr := &jobTitle
	jobDescPtr := &jobDesc
	if jobTitle == "" {
		jobTitlePtr = nil
	}
	if jobDesc == "" {
		jobDescPtr = nil
	}

	return &entity.Profile{
		Name:            name,
		DefaultCurrency: cur,
		JobTitle:        jobTitlePtr,
		JobDescription:  jobDescPtr,
	}, nil
}

// CreateProfile creates a new profile.
func (s *ProfileService) CreateProfile(ctx context.Context, req *receiptspb.CreateProfileRequest) (*receiptspb.CreateProfileResponse, error) {
	profile, err := extractProfileInput(req)
	if err != nil {
		return nil, err
	}

	p, err := s.profileRepo.CreateProfile(ctx, profile)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create profile: %v", err)
	}

	s.logger.Info("profile created successfully", "profile_id", p.ID, "name", p.Name)

	return &receiptspb.CreateProfileResponse{
		Profile: utils.ToPBProfileFromEntity(p),
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
		out = append(out, utils.ToPBProfileFromEntity(p))
	}
	return &receiptspb.ListProfilesResponse{Profiles: out}, nil
}
