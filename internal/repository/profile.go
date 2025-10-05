package repository

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent/profile"
	"github.com/joseph-ayodele/receipts-tracker/internal/entity"
	"github.com/joseph-ayodele/receipts-tracker/internal/utils"
)

type ProfileRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entity.Profile, error)
	CreateProfile(ctx context.Context, profile *entity.Profile) (*entity.Profile, error)
	ListProfiles(ctx context.Context) ([]*entity.Profile, error)
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

type profileRepository struct {
	client *ent.Client
	logger *slog.Logger
}

func NewProfileRepository(client *ent.Client, logger *slog.Logger) ProfileRepository {
	return &profileRepository{
		client: client,
		logger: logger,
	}
}

func (r *profileRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Profile, error) {
	p, err := r.client.Profile.
		Query().
		Where(profile.ID(id)).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return utils.ToProfile(p), nil
}

func (r *profileRepository) CreateProfile(ctx context.Context, profile *entity.Profile) (*entity.Profile, error) {
	builder := r.client.Profile.Create().
		SetName(profile.Name).
		SetDefaultCurrency(profile.DefaultCurrency)

	if profile.JobTitle != nil {
		builder = builder.SetJobTitle(*profile.JobTitle)
	}
	if profile.JobDescription != nil {
		builder = builder.SetJobDescription(*profile.JobDescription)
	}

	p, err := builder.Save(ctx)
	if err != nil {
		r.logger.Error("failed to create profile", "name", profile.Name, "currency", profile.DefaultCurrency, "error", err)
		return nil, err
	}
	return utils.ToProfile(p), nil
}

func (r *profileRepository) ListProfiles(ctx context.Context) ([]*entity.Profile, error) {
	plist, err := r.client.Profile.Query().Order(profile.ByCreatedAt()).All(ctx)
	if err != nil {
		r.logger.Error("failed to list profiles", "error", err)
		return nil, err
	}

	result := make([]*entity.Profile, len(plist))
	for i, p := range plist {
		result[i] = utils.ToProfile(p)
	}
	return result, nil
}

func (r *profileRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.client.Profile.Query().Where(profile.ID(id)).Exist(ctx)
	if err != nil {
		r.logger.Error("failed to check profile existence", "profile_id", id, "error", err)
		return false, err
	}
	return exists, nil
}
