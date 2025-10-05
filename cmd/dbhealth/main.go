package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		//AddSource: true,
	}))
	slog.SetDefault(logger)

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		slog.Error("missing DB_URL environment variable")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Open pgx pool + ent client
	entc, pool, err := repo.Open(ctx, repo.Config{
		DSN:             dbURL,
		MaxConns:        20,
		MinConns:        5,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
		DialTimeout:     3 * time.Second,
	}, logger)
	if err != nil {
		slog.Error("failed to open database", "error", err, "db_url", dbURL)
		os.Exit(1)
	}
	defer func(entc *ent.Client) {
		err := entc.Close()
		if err != nil {
			slog.Error("failed to close ent client", "error", err)
		}
	}(entc)
	defer pool.Close()

	// Health check via pool
	if err := repo.HealthCheck(ctx, pool, 1*time.Second, logger); err != nil {
		slog.Error("DB health check failed", "error", err)
		os.Exit(1)
	}
	slog.Info("DB health check passed")

	// Create category repository and list categories
	catRepo := repo.NewCategoryRepository(entc, logger)
	cats, err := catRepo.ListCategories(ctx)
	if err != nil {
		slog.Error("failed to list categories", "error", err)
		os.Exit(1)
	}

	slog.Info("categories listed", "count", len(cats))
	for _, c := range cats {
		slog.Info("category", "id", c.ID, "name", c.Name)
	}
}
