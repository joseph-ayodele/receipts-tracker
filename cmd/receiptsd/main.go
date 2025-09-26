package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"receipts-tracker/gen/receipts/v1"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"go.uber.org/zap"

	"receipts-tracker/internal/repository"
	"receipts-tracker/internal/server"
)

func main() {
	// Logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	log := logger.Sugar()

	// Env
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL env var is required")
	}

	// Context with signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// DB Pool
	cfg := repository.DBConfig{
		URL:             dbURL,
		MaxConns:        10,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
		HealthTimeout:   3 * time.Second,
	}
	pool, err := repository.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("creating DB pool: %v", err)
	}
	defer pool.Close()

	// Healthcheck DB on startup
	if err := repository.HealthCheck(ctx, pool, cfg.HealthTimeout); err != nil {
		log.Fatalf("DB health failed: %v", err)
	}
	log.Infow("DB health OK")

	// gRPC server
	grpcServer := grpc.NewServer()
	// Health service
	hs := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, hs)
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	// Reflection for grpcurl
	reflection.Register(grpcServer)

	// Business service
	svc := server.NewReceiptsService(pool, logger)
	receiptsv1.RegisterReceiptsServiceServer(grpcServer, svc)

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	log.Infof("gRPC serving on :%s", port)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("grpc serve: %v", err)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down...")
	grpcServer.GracefulStop()
	fmt.Println("stopped.")
}
