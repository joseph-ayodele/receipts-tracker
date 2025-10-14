package profiles

import (
	"context"
	"log/slog"
	"strings"

	"github.com/joseph-ayodele/receipts-tracker/constants"
	"github.com/joseph-ayodele/receipts-tracker/internal/entity"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Service handles profile business logic.
type Service struct {
	profileRepo repository.ProfileRepository
	logger      *slog.Logger
}

// NewService creates a new profile service.
func NewService(profileRepo repository.ProfileRepository, logger *slog.Logger) *Service {
	return &Service{
		profileRepo: profileRepo,
		logger:      logger,
	}
}

// CreateProfileRequest represents profile creation parameters.
type CreateProfileRequest struct {
	Name            string
	JobTitle        string
	JobDescription  string
	DefaultCurrency string
}

// CreateProfile creates a new profile.
func (s *Service) CreateProfile(ctx context.Context, req CreateProfileRequest) (*entity.Profile, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	jobTitle := strings.TrimSpace(req.JobTitle)
	jobDesc := strings.TrimSpace(req.JobDescription)

	cur := strings.ToUpper(strings.TrimSpace(req.DefaultCurrency))
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

	profile := &entity.Profile{
		Name:            name,
		DefaultCurrency: cur,
		JobTitle:        jobTitlePtr,
		JobDescription:  jobDescPtr,
	}

	p, err := s.profileRepo.GetOrCreate(ctx, profile)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get or create profile: %v", err)
	}

	s.logger.Info("profile created successfully", "profile_id", p.ID, "name", p.Name)
	return p, nil
}

// ListProfiles returns all profiles.
func (s *Service) ListProfiles(ctx context.Context) ([]*entity.Profile, error) {
	s.logger.Info("listing profiles")

	plist, err := s.profileRepo.ListProfiles(ctx)
	if err != nil {
		// DB error already logged in repository layer
		return nil, status.Errorf(codes.Internal, "list profiles: %v", err)
	}

	s.logger.Info("profiles listed successfully", "count", len(plist))
	return plist, nil
}
