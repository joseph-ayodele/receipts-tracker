package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

func main() {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		fmt.Println("ERROR: DB_URL env var is required")
		fmt.Println("  mac/Linux (bash/zsh): export DB_URL=postgres://USER:PASS@HOST:PORT/DB?sslmode=disable")
		fmt.Println("  Windows (PowerShell): $env:DB_URL='postgres://USER:PASS@HOST:PORT/DB?sslmode=disable'")
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
		// StatementTimeout: 2 * time.Second, // optional: server-side cap
	})
	if err != nil {
		fmt.Printf("ERROR: opening DB: %v\n", err)
		os.Exit(1)
	}
	defer func(entc *ent.Client) {
		err := entc.Close()
		if err != nil {
			fmt.Printf("ERROR: closing ent client: %v\n", err)
		}
	}(entc)
	defer pool.Close()

	// Health check via pool
	if err := repo.HealthCheck(ctx, pool, 1*time.Second); err != nil {
		fmt.Printf("DB health: FAIL (%v)\n", err)
		os.Exit(1)
	}
	fmt.Println("DB health: OK")

	// typed query using ent client
	cats, err := repo.ListCategories(ctx, entc)
	if err != nil {
		fmt.Printf("ERROR: listing categories: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("categories count: %d\n", len(cats))
	for _, c := range cats {
		fmt.Printf("- [%d] %s\n", c.ID, c.Name)
	}
}
