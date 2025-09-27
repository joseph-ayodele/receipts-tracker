package repository

import (
	"context"

	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent/category"
)

type Category struct {
	ID   int32
	Name string
}

// ListCategories returns all categories ordered by name.
func ListCategories(ctx context.Context, client *ent.Client) ([]*ent.Category, error) {
	return client.Category.
		Query().
		Order(category.ByName()).
		All(ctx)
}
