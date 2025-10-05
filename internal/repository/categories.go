package repository

import (
	"context"
	"log/slog"

	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent/category"
	"github.com/joseph-ayodele/receipts-tracker/internal/entity"
	"github.com/joseph-ayodele/receipts-tracker/internal/utils"
)

type CategoryRepository interface {
	ListCategories(ctx context.Context) ([]*entity.Category, error)
	ListByType(ctx context.Context, catType string) ([]*entity.Category, error)
	FindByName(ctx context.Context, name string) (*entity.Category, error)
}

type categoryRepository struct {
	client *ent.Client
	logger *slog.Logger
}

func NewCategoryRepository(client *ent.Client, logger *slog.Logger) CategoryRepository {
	return &categoryRepository{
		client: client,
		logger: logger,
	}
}

func (r *categoryRepository) ListCategories(ctx context.Context) ([]*entity.Category, error) {
	categories, err := r.client.Category.
		Query().
		Order(category.ByName()).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*entity.Category, len(categories))
	for i, cat := range categories {
		result[i] = utils.ToCategory(cat)
	}
	return result, nil
}

func (r *categoryRepository) ListByType(ctx context.Context, t string) ([]*entity.Category, error) {
	categories, err := r.client.Category.Query().
		Where(category.CategoryTypeEQ(category.CategoryType(t))).
		Order(category.ByName()).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*entity.Category, len(categories))
	for i, cat := range categories {
		result[i] = utils.ToCategory(cat)
	}
	return result, nil
}

func (r *categoryRepository) FindByName(ctx context.Context, name string) (*entity.Category, error) {
	cat, err := r.client.Category.Query().
		Where(category.Name(name)).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return utils.ToCategory(cat), nil
}
