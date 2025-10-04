package repository

import (
	"context"
	"log/slog"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
)

type Config struct {
	DSN              string
	MaxConns         int32
	MinConns         int32
	MaxConnLifetime  time.Duration
	MaxConnIdleTime  time.Duration
	DialTimeout      time.Duration
	StatementTimeout time.Duration
}

// Open creates a pgx pool, wraps it for Ent, and returns both.
func Open(ctx context.Context, cfg Config, logger *slog.Logger) (*ent.Client, *pgxpool.Pool, error) {
	logger.Info("connecting to database", "dsn", cfg.DSN)
	pc, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return nil, nil, err
	}

	pc.MaxConns = cfg.MaxConns
	pc.MinConns = cfg.MinConns
	pc.MaxConnLifetime = cfg.MaxConnLifetime
	pc.MaxConnIdleTime = cfg.MaxConnIdleTime
	pc.ConnConfig.RuntimeParams["application_name"] = "receipts-tracker"
	if cfg.StatementTimeout > 0 {
		pc.ConnConfig.RuntimeParams["statement_timeout"] = cfg.StatementTimeout.String()
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return nil, nil, err
	}

	// Wrap pool as *sql.DB for Ent
	db := stdlib.OpenDBFromPool(pool)
	drv := entsql.OpenDB(dialect.Postgres, db)
	client := ent.NewClient(ent.Driver(drv))

	logger.Info("successfully connected to database")
	return client, pool, nil
}

// Close closes the database connections gracefully
func Close(entc *ent.Client, pool *pgxpool.Pool, logger *slog.Logger) {
	logger.Info("closing database connections")
	if pool != nil {
		pool.Close()
	}
	if entc != nil {
		err := entc.Close()
		if err != nil {
			logger.Error("failed to close ent client", "error", err)
		}
	}
	logger.Info("database connections closed")
}

// HealthCheck pings using database/sql to catch DSN issues early.
func HealthCheck(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration, logger *slog.Logger) error {
	logger.Debug("pinging database")
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	logger.Debug("database ping successful")
	return pool.Ping(ctx)
}
