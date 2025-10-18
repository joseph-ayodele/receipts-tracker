package repository

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

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

	logger.Info("successfully initialized connection to database")
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

// OpenSQLiteInMemory opens a shared in-memory SQLite and returns ent.Client + *sql.DB (or nil for pool).
func OpenSQLiteInMemory(log *slog.Logger) (*ent.Client, *sql.DB, error) {
	log.Info("opening in-memory SQLite database", "dsn", "file:memdb1?mode=memory&cache=shared")

	// Use the shared in-memory database DSN
	db, err := sql.Open("sqlite", "file:memdb1?mode=memory&cache=shared")
	if err != nil {
		log.Error("failed to open SQLite database", "error", err)
		return nil, nil, err
	}

	// Ensure database is closed on error after successful opening
	var success bool
	defer func() {
		if !success {
			if closeErr := db.Close(); closeErr != nil {
				log.Error("failed to close database after error", "error", closeErr)
			}
		}
	}()

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		log.Error("failed to enable foreign keys", "error", err)
		return nil, nil, err
	}

	// Configure connection settings
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	// Wrap with Ent SQL driver and open with SQLite dialect
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := ent.NewClient(ent.Driver(drv))

	success = true
	log.Info("successfully initialized in-memory SQLite database")
	return client, db, nil
}

// MigrateSQLite runs auto-migration with Ent for SQLite (create all tables/indices).
func MigrateSQLite(ctx context.Context, c *ent.Client) error {
	if err := c.Schema.Create(ctx); err != nil {
		return err
	}
	return nil
}
