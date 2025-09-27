package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"google.golang.org/grpc"

	"github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
	svc "github.com/joseph-ayodele/receipts-tracker/internal/server"
)

func main() {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL env var is required")
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
		log.Fatalf("opening DB: %v", err)
	}
	defer func(entc *ent.Client) {
		err := entc.Close()
		if err != nil {
			log.Printf("closing ent client: %v", err)
		}
	}(entc)
	defer pool.Close()

	// gRPC server
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}
	grpcServer := grpc.NewServer()

	profilesRepo := repo.NewProfileRepository(entc)
	receiptsRepo := repo.NewReceiptRepository(entc)

	profilesService := svc.NewProfileService(profilesRepo)
	v1.RegisterProfilesServiceServer(grpcServer, profilesService)
	receiptsService := svc.NewReceiptService(receiptsRepo)
	v1.RegisterReceiptsServiceServer(grpcServer, receiptsService)

	log.Printf("receiptsd listening on %s", addr)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC serve error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	grpcServer.GracefulStop()
}
