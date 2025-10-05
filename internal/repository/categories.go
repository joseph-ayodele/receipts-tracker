package repository

import (
	"context"
	"log/slog"

	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent/category"
)

type Category struct {
	ID   int32
	Name string
}

type CategoryRepository interface {
	ListCategories(ctx context.Context) ([]*ent.Category, error)
	ListByType(ctx context.Context, catType string) ([]*ent.Category, error)
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

func (r *categoryRepository) ListCategories(ctx context.Context) ([]*ent.Category, error) {
	return r.client.Category.
		Query().
		Order(category.ByName()).
		All(ctx)
}

func (r *categoryRepository) ListByType(ctx context.Context, t string) ([]*ent.Category, error) {
	return r.client.Category.Query().
		Where(category.CategoryTypeEQ(category.CategoryType(t))).
		Order(category.ByName()).
		All(ctx)
}

func (r *categoryRepository) FindByName(ctx context.Context, name string) (*ent.Category, error) {
	return r.client.Category.Query().
		Where(category.Name(name)).
		Only(ctx)
}
