package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

// DBConfig holds database connection configuration
type DBConfig struct {
	DSN             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
	DialTimeout     time.Duration
}

// ConnectDB establishes a connection to the database using the provided DSN and returns the Ent client and connection pool
func ConnectDB(ctx context.Context, dbURL string, logger *slog.Logger) (*ent.Client, *pgxpool.Pool, error) {
	config := DBConfig{
		DSN:             dbURL,
		MaxConns:        20,
		MinConns:        5,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
		DialTimeout:     3 * time.Second,
	}

	logger.Info("connecting to database", "dsn", dbURL)
	entc, pool, err := repo.Open(ctx, repo.Config{
		DSN:             config.DSN,
		MaxConns:        config.MaxConns,
		MinConns:        config.MinConns,
		MaxConnLifetime: config.MaxConnLifetime,
		MaxConnIdleTime: config.MaxConnIdleTime,
		DialTimeout:     config.DialTimeout,
	})
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return nil, nil, err
	}

	logger.Info("successfully connected to database")
	return entc, pool, nil
}

// PingDB pings the database to ensure it's responsive
func PingDB(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger, timeout time.Duration) error {
	logger.Debug("pinging database")
	err := repo.HealthCheck(ctx, pool, timeout)
	if err != nil {
		logger.Error("database ping failed", "error", err)
		return err
	}
	logger.Debug("database ping successful")
	return nil
}

// CloseDB closes the database connections gracefully
func CloseDB(entc *ent.Client, pool *pgxpool.Pool, logger *slog.Logger) {
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
