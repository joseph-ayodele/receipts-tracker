package repository

import (
	"context"

	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent/profile"
)

type ProfileRepository interface {
	CreateProfile(ctx context.Context, name, defaultCurrency string) (*ent.Profile, error)
	ListProfiles(ctx context.Context) ([]*ent.Profile, error)
}

type profileRepository struct {
	client *ent.Client
}

func NewProfileRepository(client *ent.Client) ProfileRepository {
	return &profileRepository{client: client}
}

func (r *profileRepository) CreateProfile(ctx context.Context, name, defaultCurrency string) (*ent.Profile, error) {
	return r.client.Profile.Create().SetName(name).SetDefaultCurrency(defaultCurrency).Save(ctx)
}

func (r *profileRepository) ListProfiles(ctx context.Context) ([]*ent.Profile, error) {
	return r.client.Profile.Query().Order(profile.ByCreatedAt()).All(ctx)
}

