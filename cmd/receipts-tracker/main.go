package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joseph-ayodele/receipts-tracker/internal/async"
	"github.com/joseph-ayodele/receipts-tracker/internal/extract"
	"github.com/joseph-ayodele/receipts-tracker/internal/ingest"
	"github.com/joseph-ayodele/receipts-tracker/internal/llm/openai"
	"github.com/joseph-ayodele/receipts-tracker/internal/ocr"
	pipeline "github.com/joseph-ayodele/receipts-tracker/internal/pipeline"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
	svc "github.com/joseph-ayodele/receipts-tracker/internal/server"
)

func main() {
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

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		logger.Error("missing DB_URL environment variable")
		os.Exit(1)
	}
	addr := os.Getenv("GRPC_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if !strings.HasPrefix(addr, ":") {
		addr = ":" + addr
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		logger.Error("OPENAI_API_KEY env var is required")
		os.Exit(2)
	}

	heicConv := getenv("HEIC_CONVERTER", "magick")
	tessdata := os.Getenv("TESSDATA_PREFIX")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbConfig := repo.Config{
		DSN:             dbURL,
		MaxConns:        20,
		MinConns:        5,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
		DialTimeout:     3 * time.Second,
	}
	entc, pool, err := repo.Open(ctx, dbConfig, logger)
	if err != nil {
		logger.Error("failed to open database", "error", err, "db_url", dbURL)
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
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("failed to listen on address", "addr", addr, "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()

	profilesRepo := repo.NewProfileRepository(entc, logger)
	receiptsRepo := repo.NewReceiptRepository(entc, logger)
	filesRepo := repo.NewReceiptFileRepository(entc, logger)
	jobsRepo := repo.NewExtractJobRepository(entc, logger)

	// OCR text pipeline (already present in your codebase)
	ocrCfg := ocr.Config{
		HeicConverter:    heicConv, // <— key bit
		TessdataDir:      tessdata, // optional
		ArtifactCacheDir: "./tmp",
	}
	extractor := ocr.NewExtractor(ocrCfg, logger)
	ocrAdapter := extract.NewOCRAdapter(extractor, logger)
	ocrPipe := pipeline.NewOCRStage(filesRepo, jobsRepo, ocrAdapter, logger)

	// LLM parse pipeline (uses your OpenAI client)
	openaiClient := openai.NewClient(openai.Config{
		Model:       getenv("OPENAI_MODEL", "gpt-4o-mini"),
		APIKey:      os.Getenv("OPENAI_API_KEY"),
		Temperature: 0.0,
		Timeout:     45 * time.Second,
	}, logger)

	parseCfg := pipeline.Config{
		MinConfidence:    0.60,
		ArtifactCacheDir: "./tmp",
	}
	parsePipe := pipeline.NewParseStage(logger, parseCfg, jobsRepo, profilesRepo, jobsRepo, receiptsRepo, openaiClient)

	// Orchestrator
	processor := pipeline.NewProcessor(logger, ocrPipe, parsePipe)

	profilesService := svc.NewProfileService(profilesRepo, logger)
	v1.RegisterProfilesServiceServer(grpcServer, profilesService)
	receiptsService := svc.NewReceiptService(receiptsRepo, logger)
	v1.RegisterReceiptsServiceServer(grpcServer, receiptsService)

	queue := async.NewProcessorQueue(processor, logger,
		async.WithWorkers(6),
		async.WithQueueSize(512),
		async.WithProcessTimeout(3*time.Minute),
	)

	ingestor := ingest.NewFSIngestor(profilesRepo, filesRepo, logger)
	ingestionService := svc.NewIngestionService(ingestor, queue, profilesRepo, logger)
	v1.RegisterIngestionServiceServer(grpcServer, ingestionService)

	// Register gRPC health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	// Set the service as serving (empty string means overall server health)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	logger.Info("receipts-tracker listening", "addr", addr)
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
