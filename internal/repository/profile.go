package repository

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent/profile"
)

type Profile struct {
	Name            string
	DefaultCurrency string
	JobTitle        string
	JobDescription  string
}

type ProfileRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*ent.Profile, error)
	CreateProfile(ctx context.Context, profile *Profile) (*ent.Profile, error)
	ListProfiles(ctx context.Context) ([]*ent.Profile, error)
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

func (r *profileRepository) GetByID(ctx context.Context, id uuid.UUID) (*ent.Profile, error) {
	return r.client.Profile.
		Query().
		Where(profile.ID(id)).
		Only(ctx)
}

func (r *profileRepository) CreateProfile(ctx context.Context, profile *Profile) (*ent.Profile, error) {
	p, err := r.client.Profile.Create().
		SetName(profile.Name).
		SetDefaultCurrency(profile.DefaultCurrency).
		SetJobTitle(profile.JobTitle).
		SetJobDescription(profile.JobDescription).
		Save(ctx)
	if err != nil {
		r.logger.Error("failed to create profile", "name", profile.Name, "currency", profile.DefaultCurrency, "error", err)
		return nil, err
	}
	return p, nil
}

func (r *profileRepository) ListProfiles(ctx context.Context) ([]*ent.Profile, error) {
	plist, err := r.client.Profile.Query().Order(profile.ByCreatedAt()).All(ctx)
	if err != nil {
		r.logger.Error("failed to list profiles", "error", err)
		return nil, err
	}
	return plist, nil
}

func (r *profileRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.client.Profile.Query().Where(profile.ID(id)).Exist(ctx)
	if err != nil {
		r.logger.Error("failed to check profile existence", "profile_id", id, "error", err)
		return false, err
	}
	return exists, nil
}
