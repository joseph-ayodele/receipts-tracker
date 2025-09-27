package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

func main() {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Println("ERROR: DB_URL env var is required")
		log.Println("  mac/Linux (bash/zsh): export DB_URL=postgres://USER:PASS@HOST:PORT/DB?sslmode=disable")
		log.Println("  Windows (PowerShell): $env:DB_URL='postgres://USER:PASS@HOST:PORT/DB?sslmode=disable'")
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
		log.Fatalf("opening DB: %v", err)
	}
	defer func(entc *ent.Client) {
		err := entc.Close()
		if err != nil {
			log.Printf("ERROR: closing ent client: %v", err)
		}
	}(entc)
	defer pool.Close()

	// Health check via pool
	if err := repo.HealthCheck(ctx, pool, 1*time.Second); err != nil {
		log.Fatalf("DB health: FAIL (%v)", err)
	}
	log.Println("DB health: OK")

	// typed query using ent client
	cats, err := repo.ListCategories(ctx, entc)
	if err != nil {
		log.Fatalf("listing categories: %v", err)
	}

	log.Printf("categories count: %d", len(cats))
	for _, c := range cats {
		log.Printf("- [%d] %s", c.ID, c.Name)
	}
}
