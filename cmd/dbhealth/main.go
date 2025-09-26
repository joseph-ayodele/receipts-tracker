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
		fmt.Println("  mac/Linux (bash/zsh): export DB_URL=postgres://USER:PASS@HOST:PORT/DB?sslmode=require")
		fmt.Println("  Windows (PowerShell): $env:DB_URL='postgres://USER:PASS@HOST:PORT/DB?sslmode=require'")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := repo.HealthCheck(ctx, dbURL, 3*time.Second); err != nil {
		fmt.Printf("DB health: FAIL (%v)\n", err)
		os.Exit(1)
	}
	fmt.Println("DB health: OK")

	// ent client for typed queries
	entc, err := repo.NewEntClient(dbURL)
	if err != nil {
		fmt.Printf("ERROR: creating ent client: %v\n", err)
		os.Exit(1)
	}
	defer entc.Close()

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
