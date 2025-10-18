package repository

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent/profile"
	"github.com/joseph-ayodele/receipts-tracker/internal/entity"
	"github.com/joseph-ayodele/receipts-tracker/internal/tools"
)

type ProfileRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entity.Profile, error)
	GetOrCreate(ctx context.Context, profile *entity.Profile) (*entity.Profile, error)
	GetOrCreateByName(ctx context.Context, name, defaultCurrency string) (*ent.Profile, error)
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
	return tools.ToProfile(p), nil
}

func (r *profileRepository) GetOrCreate(ctx context.Context, p *entity.Profile) (*entity.Profile, error) {
	existing, err := r.client.Profile.Query().Where(profile.Name(p.Name)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			builder := r.client.Profile.Create().
				SetName(p.Name).
				SetDefaultCurrency(p.DefaultCurrency)
			if p.JobTitle != nil {
				builder = builder.SetJobTitle(*p.JobTitle)
			}
			if p.JobDescription != nil {
				builder = builder.SetJobDescription(*p.JobDescription)
			}
			created, err := builder.Save(ctx)
			if err != nil {
				r.logger.Error("failed to create profile in GetOrCreate", "name", p.Name, "error", err)
				return nil, err
			}
			return tools.ToProfile(created), nil
		} else {
			return nil, err
		}
	}
	return tools.ToProfile(existing), nil
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
	return tools.ToProfile(p), nil
}

func (r *profileRepository) ListProfiles(ctx context.Context) ([]*entity.Profile, error) {
	plist, err := r.client.Profile.Query().Order(profile.ByCreatedAt()).All(ctx)
	if err != nil {
		r.logger.Error("failed to list profiles", "error", err)
		return nil, err
	}

	result := make([]*entity.Profile, len(plist))
	for i, p := range plist {
		result[i] = tools.ToProfile(p)
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

func (r *profileRepository) GetOrCreateByName(ctx context.Context, name, defaultCurrency string) (*ent.Profile, error) {
	existing, err := r.client.Profile.Query().Where(profile.Name(name)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			created, err := r.client.Profile.Create().
				SetName(name).
				SetDefaultCurrency(defaultCurrency).
				Save(ctx)
			if err != nil {
				r.logger.Error("failed to create profile in GetOrCreateByName", "name", name, "error", err)
				return nil, err
			}
			return created, nil
		} else {
			return nil, err
		}
	}
	return existing, nil
}
