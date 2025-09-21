package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Category struct {
	ID   int32
	Name string
}

// ListCategories returns all categories ordered by name.
// Safe when the table is empty (returns [] with len 0).
func ListCategories(ctx context.Context, pool *pgxpool.Pool) ([]Category, error) {
	rows, err := pool.Query(ctx, `SELECT id, name FROM categories ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
