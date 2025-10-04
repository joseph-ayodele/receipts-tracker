package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joseph-ayodele/receipts-tracker/internal/ingest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
	svc "github.com/joseph-ayodele/receipts-tracker/internal/server"
)

func main() {
	// Setup structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		//AddSource: true,
	}))
	slog.SetDefault(logger)

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
	entc, pool, err := svc.ConnectDB(ctx, dbURL, logger)
	if err != nil {
		slog.Error("failed to open database", "error", err, "db_url", dbURL)
		os.Exit(1)
	}
	defer svc.CloseDB(entc, pool, logger)

	// Ping DB to ensure connectivity
	err = svc.PingDB(ctx, pool, logger, 5*time.Second)
	if err != nil {
		slog.Error("failed to ping database", "error", err)
		os.Exit(1)
	}

	// gRPC server
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to listen on address", "addr", addr, "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()

	profilesRepo := repo.NewProfileRepository(entc, logger)
	receiptsRepo := repo.NewReceiptRepository(entc, logger)
	receiptsFileRepo := repo.NewReceiptFileRepository(entc, logger)

	profilesService := svc.NewProfileService(profilesRepo, logger)
	v1.RegisterProfilesServiceServer(grpcServer, profilesService)
	receiptsService := svc.NewReceiptService(receiptsRepo, logger)
	v1.RegisterReceiptsServiceServer(grpcServer, receiptsService)

	ingestor := ingest.NewFSIngestor(profilesRepo, receiptsFileRepo, logger)
	ingestionService := svc.NewIngestionService(ingestor, profilesRepo, logger)
	v1.RegisterIngestionServiceServer(grpcServer, ingestionService)

	// Register gRPC health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	// Set the service as serving (empty string means overall server health)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	logger.Info("receiptsd listening", "addr", addr)
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
