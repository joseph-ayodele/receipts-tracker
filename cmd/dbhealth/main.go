package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	repo "receipts-tracker/internal/repository"
)

func main() {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		fmt.Println("ERROR: DB_URL env var is required")
		fmt.Println("  mac/Linux (bash/zsh): export DB_URL=postgres://USER:PASS@HOST:PORT/DB")
		fmt.Println("  Windows (PowerShell): $env:DB_URL='postgres://USER:PASS@HOST:PORT/DB'")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg := repo.Config{
		URL:             dbURL,
		MaxConns:        10,
		MinConns:        0,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
		HealthTimeout:   3 * time.Second,
	}

	pool, err := repo.NewPool(ctx, cfg)
	if err != nil {
		fmt.Printf("ERROR: creating pool: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := repo.HealthCheck(ctx, pool, cfg.HealthTimeout); err != nil {
		fmt.Printf("DB health: FAIL (%v)\n", err)
		os.Exit(1)
	}
	fmt.Println("DB health: OK")

	cats, err := repo.ListCategories(ctx, pool)
	if err != nil {
		fmt.Printf("ERROR: listing categories: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("categories count: %d\n", len(cats))
	for _, c := range cats {
		fmt.Printf("- [%d] %s\n", c.ID, c.Name)
	}
}
