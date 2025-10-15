package common

import (
	"os"
	"strconv"
	"time"
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

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
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
