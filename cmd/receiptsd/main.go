package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/internal/ingest"
	"google.golang.org/grpc"

	"github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
	svc "github.com/joseph-ayodele/receipts-tracker/internal/server"
)

func main() {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		slog.Error("missing DB_URL environment variable")
		os.Exit(1)
	}
	addr := os.Getenv("GRPC_ADDR")
	if addr == "" {
		addr = ":8080"
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

	// gRPC server
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to listen on address", "addr", addr, "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()

	profilesRepo := repo.NewProfileRepository(entc)
	receiptsRepo := repo.NewReceiptRepository(entc)
	receiptsFileRepo := repo.NewReceiptFileRepository(entc)

	profilesService := svc.NewProfileService(profilesRepo)
	v1.RegisterProfilesServiceServer(grpcServer, profilesService)
	receiptsService := svc.NewReceiptService(receiptsRepo)
	v1.RegisterReceiptsServiceServer(grpcServer, receiptsService)

	ingestor := ingest.NewFSIngestor(profilesRepo, receiptsFileRepo)
	ingestionService := svc.NewIngestionService(ingestor, profilesRepo)
	v1.RegisterIngestionServiceServer(grpcServer, ingestionService)

	slog.Info("receiptsd listening", "addr", addr)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("gRPC serve error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down gracefully")
	grpcServer.GracefulStop()
}
