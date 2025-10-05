package openai

import (
	"log/slog"
	"net/http"
	"os"
	"time"
)

// Config for the OpenAI client.
type Config struct {
	APIKey      string        // if empty, falls back to env OPENAI_API_KEY
	BaseURL     string        // default https://api.openai.com/v1
	Model       string        // e.g., "gpt-4o-mini"
	Temperature float32       // 0..2
	Timeout     time.Duration // http client timeout

	// Future switches (not wired yet):
	EnableVision     bool    // when true and low OCR confidence + FilePath set, use vision path
	LowConfThreshold float32 // default 0.5 if zero; compare with req.PrepConfidence
}

type Client struct {
	cfg        Config
	httpClient *http.Client
	log        *slog.Logger
}

func New(cfg Config, log *slog.Logger) *Client {
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("OPENAI_API_KEY")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.LowConfThreshold <= 0 {
		cfg.LowConfThreshold = 0.5
	}
	if log == nil {
		log = slog.Default()
	}
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
		log:        log,
	}
}
