package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joseph-ayodele/receipts-tracker/internal/async"
	"github.com/joseph-ayodele/receipts-tracker/internal/common"
	"github.com/joseph-ayodele/receipts-tracker/internal/core/extract"
	"github.com/joseph-ayodele/receipts-tracker/internal/core/ocr"
	processor2 "github.com/joseph-ayodele/receipts-tracker/internal/core/pipeline"
	"github.com/joseph-ayodele/receipts-tracker/internal/llm/openai"
	"github.com/joseph-ayodele/receipts-tracker/internal/services/export"
	ingest2 "github.com/joseph-ayodele/receipts-tracker/internal/services/ingest"
	"github.com/joseph-ayodele/receipts-tracker/internal/services/profile"
	"github.com/joseph-ayodele/receipts-tracker/internal/services/receipt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
	svc "github.com/joseph-ayodele/receipts-tracker/internal/server"
)

func main() {
	// Load and validate configuration
	cfg := common.LoadConfig()
	if err := cfg.Validate(); err != nil {
		slog.Error("configuration validation failed", "error", err)
		os.Exit(1)
	}

	// Setup structured logger that outputs messages with variables but no time/level
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		//AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Remove time and level attributes, keep message and other variables
			if a.Key == slog.TimeKey || a.Key == slog.LevelKey {
				return slog.Attr{}
			}
			return a
		},
	}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Use configuration from config package
	dbConfig := repo.Config{
		DSN:             cfg.Database.DSN,
		MaxConns:        cfg.Database.MaxConns,
		MinConns:        cfg.Database.MinConns,
		MaxConnLifetime: cfg.Database.MaxConnLifetime,
		MaxConnIdleTime: cfg.Database.MaxConnIdleTime,
		DialTimeout:     cfg.Database.DialTimeout,
	}
	entc, pool, err := repo.Open(ctx, dbConfig, logger)
	if err != nil {
		logger.Error("failed to open database", "error", err, "dsn", cfg.Database.DSN)
		os.Exit(1)
	}
	defer repo.Close(entc, pool, logger)

	// Ping DB to ensure connectivity
	err = repo.HealthCheck(ctx, pool, 5*time.Second, logger)
	if err != nil {
		logger.Error("failed to ping database", "error", err)
		os.Exit(1)
	}

	// gRPC server
	lis, err := net.Listen("tcp", cfg.Server.GRPCAddr)
	if err != nil {
		logger.Error("failed to listen on address", "addr", cfg.Server.GRPCAddr, "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()

	profilesRepo := repo.NewProfileRepository(entc, logger)
	receiptsRepo := repo.NewReceiptRepository(entc, logger)
	filesRepo := repo.NewReceiptFileRepository(entc, logger)
	jobsRepo := repo.NewExtractJobRepository(entc, logger)

	// OCR text pipeline
	ocrCfg := ocr.Config{
		HeicConverter:    cfg.OCR.HeicConverter,
		TessdataDir:      cfg.OCR.TessdataDir,
		ArtifactCacheDir: cfg.OCR.ArtifactCacheDir,
	}
	extractor := ocr.NewExtractor(ocrCfg, logger)
	ocrAdapter := extract.NewOCRAdapter(extractor, logger)
	ocrPipe := processor2.NewOCRStage(filesRepo, jobsRepo, ocrAdapter, logger)

	// LLM parse pipeline
	openaiClient := openai.NewClient(openai.Config{
		Model:       cfg.LLM.Model,
		APIKey:      cfg.LLM.APIKey,
		Temperature: cfg.LLM.Temperature,
		Timeout:     cfg.LLM.Timeout,
	}, logger)

	parseCfg := processor2.Config{
		MinConfidence:    0.60,
		ArtifactCacheDir: "./tmp",
	}
	parsePipe := processor2.NewParseStage(logger, parseCfg, jobsRepo, profilesRepo, jobsRepo, receiptsRepo, openaiClient)

	// Orchestrator
	processor := processor2.NewProcessor(logger, ocrPipe, parsePipe)

	// Create service layers (business logic)
	profilesServiceLayer := profile.NewService(profilesRepo, logger)
	receiptsServiceLayer := receipt.NewService(receiptsRepo, logger)

	queue := async.NewProcessorQueue(processor, logger,
		async.WithWorkers(6),
		async.WithQueueSize(512),
		async.WithProcessTimeout(3*time.Minute),
	)

	ingestor := ingest2.NewFSIngestor(profilesRepo, filesRepo, logger)
	ingestionServiceLayer := ingest2.NewService(ingestor, profilesRepo, queue, logger)

	// Create server layers (gRPC protocol handling)
	profilesServer := svc.NewProfileServer(profilesServiceLayer, logger)
	v1.RegisterProfilesServiceServer(grpcServer, profilesServer)
	receiptsServer := svc.NewReceiptServer(receiptsServiceLayer, logger)
	v1.RegisterReceiptsServiceServer(grpcServer, receiptsServer)

	ingestionServer := svc.NewIngestionServer(ingestionServiceLayer, logger)
	v1.RegisterIngestionServiceServer(grpcServer, ingestionServer)

	exportService := export.NewService(entc, receiptsRepo, filesRepo, logger)
	exportServer := svc.NewExportServer(exportService, logger)
	v1.RegisterExportServiceServer(grpcServer, exportServer)

	// Register gRPC health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	// Set the service as serving (empty string means overall server health)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	logger.Info("receipts-tracker listening", "addr", cfg.Server.GRPCAddr)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("gRPC serve error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	queue.Shutdown(context.Background())
	grpcServer.GracefulStop()
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
