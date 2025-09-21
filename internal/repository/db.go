package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
	HealthTimeout   time.Duration
}

// NewPool builds a pgx pool with sensible defaults.
// It does not ping; use HealthCheck after creation.
func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if cfg.URL == "" {
		return nil, errors.New("missing DB URL")
	}

	pc, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, err
	}

	if cfg.MaxConns > 0 {
		pc.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		pc.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		pc.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		pc.MaxConnIdleTime = cfg.MaxConnIdleTime
	}

	return pgxpool.NewWithConfig(ctx, pc)
}

// HealthCheck runs a quick query with a timeout.
func HealthCheck(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration) error {
	ctx2 := ctx
	cancel := func() {}
	if timeout > 0 {
		ctx2, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	var one int
	return pool.QueryRow(ctx2, "SELECT 1").Scan(&one)
}
