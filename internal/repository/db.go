package repository

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver

	entpkg "receipts-tracker/db/ent"
)

type Config struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
	HealthTimeout   time.Duration
}

// NewEntClient returns an ent client backed by the pgx stdlib driver.
func NewEntClient(dsn string) (*entpkg.Client, error) {
	client, err := entpkg.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	// Optional tuning
	if db, ok := client.DB().(*sql.DB); ok && db != nil {
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(30 * time.Minute)
	}

	return client, nil
}

// HealthCheck pings using database/sql to catch DSN issues early.
func HealthCheck(ctx context.Context, dsn string, timeout time.Duration) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	if timeout > 0 {
		ctx2, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return db.PingContext(ctx2)
	}
	return db.PingContext(ctx)
}
