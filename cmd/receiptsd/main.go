package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	receiptspb "github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
	svc "github.com/joseph-ayodele/receipts-tracker/internal/server"
)

func main() {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "ERROR: DB_URL env var is required")
		os.Exit(2)
	}
	addr := os.Getenv("GRPC_ADDR")
	if addr == "" {
		addr = ":50051"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Open DB (ent client + pgx pool)
	entc, pool, err := repo.Open(ctx, repo.Config{
		DSN:             dbURL,
		MaxConns:        20,
		MinConns:        5,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
		DialTimeout:     3 * time.Second,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: opening DB: %v\n", err)
		os.Exit(1)
	}
	defer entc.Close()
	defer pool.Close()

	// gRPC server
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: listen %s: %v\n", addr, err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()

	service := svc.New(entc)
	receiptspb.RegisterReceiptsServiceServer(grpcServer, service)

	fmt.Printf("receiptsd listening on %s\n", addr)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			fmt.Fprintf(os.Stderr, "gRPC serve error: %v\n", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	fmt.Println("shutting down...")
	grpcServer.GracefulStop()
}
