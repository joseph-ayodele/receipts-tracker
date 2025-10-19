package common

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	repo "github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

// Config holds all application configuration
type Config struct {
	Database DatabaseConfig
	Server   ServerConfig
	OCR      OCRConfig
	LLM      LLMConfig
}

// DatabaseConfig holds database-related configuration
type DatabaseConfig struct {
	DSN              string
	MaxConns         int32
	MinConns         int32
	MaxConnLifetime  time.Duration
	MaxConnIdleTime  time.Duration
	DialTimeout      time.Duration
	StatementTimeout time.Duration
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	GRPCAddr string
}

// OCRConfig holds OCR-related configuration
type OCRConfig struct {
	HeicConverter    string
	TessdataDir      string
	ArtifactCacheDir string
}

// LLMConfig holds LLM-related configuration
type LLMConfig struct {
	Model       string
	APIKey      string
	Temperature float32
	Timeout     time.Duration
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			DSN:              getEnv("DB_URL", ""),
			MaxConns:         getEnvAsInt32("DB_MAX_CONNS", 20),
			MinConns:         getEnvAsInt32("DB_MIN_CONNS", 5),
			MaxConnLifetime:  getEnvAsDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute),
			MaxConnIdleTime:  getEnvAsDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute),
			DialTimeout:      getEnvAsDuration("DB_DIAL_TIMEOUT", 3*time.Second),
			StatementTimeout: getEnvAsDuration("DB_STATEMENT_TIMEOUT", 0),
		},
		Server: ServerConfig{
			GRPCAddr: getEnv("GRPC_ADDR", ":8080"),
		},
		OCR: OCRConfig{
			HeicConverter:    getEnv("HEIC_CONVERTER", "magick"),
			TessdataDir:      getEnv("TESSDATA_PREFIX", ""),
			ArtifactCacheDir: getEnv("ARTIFACT_CACHE_DIR", "./tmp"),
		},
		LLM: LLMConfig{
			Model:       getEnv("OPENAI_MODEL", "gpt-4o-mini"),
			APIKey:      getEnv("OPENAI_API_KEY", ""),
			Temperature: getEnvAsFloat32("OPENAI_TEMPERATURE", 0.0),
			Timeout:     getEnvAsDuration("OPENAI_TIMEOUT", 45*time.Second),
		},
	}
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt32(key string, defaultValue int32) int32 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseInt(value, 10, 32); err == nil {
			return int32(intVal)
		}
	}
	return defaultValue
}

func getEnvAsFloat32(key string, defaultValue float32) float32 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 32); err == nil {
			return float32(floatVal)
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// ValidateConfig validates the loaded configuration
func (c *Config) Validate() error {
	if c.Database.DSN == "" {
		return NewAppError("CONFIG_ERROR", "DB_URL is required", ErrInvalidInput)
	}
	if c.LLM.APIKey == "" {
		return NewAppError("CONFIG_ERROR", "OPENAI_API_KEY is required", ErrInvalidInput)
	}
	if c.Server.GRPCAddr == "" {
		return NewAppError("CONFIG_ERROR", "GRPC_ADDR is required", ErrInvalidInput)
	}
	return nil
}

// ShouldUseInMemory returns true if the application should use in-memory SQLite
func ShouldUseInMemory(cfg *Config, useInmemFlag bool) bool {
	return useInmemFlag || cfg.Database.DSN == ""
}

// DatabaseResult holds the initialized database client and cleanup function
type DatabaseResult struct {
	Client  *ent.Client
	Cleanup func()
}

// InitDatabase initializes either SQLite in-memory or Postgres database
func InitDatabase(ctx context.Context, cfg *Config, useInmemFlag bool, logger *slog.Logger) (*DatabaseResult, error) {
	useInMemory := ShouldUseInMemory(cfg, useInmemFlag)

	if useInMemory {
		return initSQLiteInMemory(ctx, logger)
	}

	return initPostgres(ctx, cfg, logger)
}

// initSQLiteInMemory initializes SQLite in-memory database
func initSQLiteInMemory(ctx context.Context, logger *slog.Logger) (*DatabaseResult, error) {
	logger.Info("initializing in-memory SQLite database")

	entc, db, err := repo.OpenSQLiteInMemory(logger)
	if err != nil {
		return nil, err
	}

	// Run migrations for SQLite
	err = repo.MigrateSQLite(ctx, entc)
	if err != nil {
		return nil, err
	}

	// Cleanup function for in-memory mode
	cleanup := func() {
		if entc != nil {
			err := entc.Close()
			if err != nil {
				return
			}
		}
		if db != nil {
			err := db.Close()
			if err != nil {
				return
			}
		}
	}

	logger.Info("in-memory SQLite database initialized successfully")
	return &DatabaseResult{
		Client:  entc,
		Cleanup: cleanup,
	}, nil
}

// initPostgres initializes Postgres database
func initPostgres(ctx context.Context, cfg *Config, logger *slog.Logger) (*DatabaseResult, error) {
	logger.Info("initializing Postgres database", "dsn", cfg.Database.DSN)

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
		return nil, err
	}

	// Ping DB to ensure connectivity
	err = repo.HealthCheck(ctx, pool, 5*time.Second, logger)
	if err != nil {
		return nil, err
	}

	// Cleanup function for Postgres mode
	cleanup := func() {
		repo.Close(entc, pool, logger)
	}

	logger.Info("Postgres database initialized successfully")
	return &DatabaseResult{
		Client:  entc,
		Cleanup: cleanup,
	}, nil
}
