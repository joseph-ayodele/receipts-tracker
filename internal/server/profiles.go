package server

import (
	"context"
	"log/slog"

	receiptspb "github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
	"github.com/joseph-ayodele/receipts-tracker/internal/profiles"
	"github.com/joseph-ayodele/receipts-tracker/internal/utils"
)

type ProfileServer struct {
	receiptspb.UnimplementedProfilesServiceServer
	svc    *profiles.Service
	logger *slog.Logger
}

func NewProfileServer(svc *profiles.Service, logger *slog.Logger) *ProfileServer {
	return &ProfileServer{
		svc:    svc,
		logger: logger,
	}
}

// CreateProfile creates a new profile.
func (s *ProfileServer) CreateProfile(ctx context.Context, req *receiptspb.CreateProfileRequest) (*receiptspb.CreateProfileResponse, error) {
	// Convert gRPC request to service request
	serviceReq := profiles.CreateProfileRequest{
		Name:            req.GetName(),
		JobTitle:        req.GetJobTitle(),
		JobDescription:  req.GetJobDescription(),
		DefaultCurrency: req.GetDefaultCurrency(),
	}

	// Call service layer (pure business logic)
	p, err := s.svc.CreateProfile(ctx, serviceReq)
	if err != nil {
		return nil, err
	}

	return &receiptspb.CreateProfileResponse{
		Profile: utils.ToPBProfileFromEntity(p),
	}, nil
}

// ListProfiles lists all the profiles.
func (s *ProfileServer) ListProfiles(ctx context.Context, _ *receiptspb.ListProfilesRequest) (*receiptspb.ListProfilesResponse, error) {
	// Call service layer (pure business logic)
	plist, err := s.svc.ListProfiles(ctx)
	if err != nil {
		return nil, err
	}

	// Convert service response to gRPC response
	out := make([]*receiptspb.Profile, 0, len(plist))
	for _, p := range plist {
		out = append(out, utils.ToPBProfileFromEntity(p))
	}
	return &receiptspb.ListProfilesResponse{Profiles: out}, nil
}
