package openai

import (
	"log/slog"
	"net/http"
	"os"
	"time"
)

// Config for the OpenAI client.
type Config struct {
	APIKey          string        // if empty, falls back to env OPENAI_API_KEY
	BaseURL         string        // default https://api.openai.com/v1
	Model           string        // e.g., "gpt-4o-mini"
	Temperature     float32       // 0..2
	Timeout         time.Duration // http client timeout
	LenientOptional bool
}

type Client struct {
	cfg    Config
	http   *http.Client
	logger *slog.Logger
}

func NewClient(cfg Config, logger *slog.Logger) *Client {
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("OPENAI_API_KEY")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-5-mini"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	if !cfg.LenientOptional {
		cfg.LenientOptional = true
	}
	return &Client{
		cfg:    cfg,
		http:   &http.Client{Timeout: cfg.Timeout},
		logger: logger,
	}
}
