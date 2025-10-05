package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"log/slog"

	"github.com/joseph-ayodele/receipts-tracker/internal/llm"
)

// Config for the OpenAI client.
type Config struct {
	APIKey      string  // if empty, falls back to env OPENAI_API_KEY
	BaseURL     string  // default https://api.openai.com/v1
	Model       string  // e.g., "gpt-4o-mini" or "gpt-4.1-mini"
	Temperature float32 // 0..2
	Timeout     time.Duration
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
		cfg.Model = "gpt-4o-mini" // safe default; override in env or config
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
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

// ExtractFields implements llm.FieldExtractor using OpenAI "chat/completions" with JSON object format.
// We instruct the model with a strict schema and then validate locally.
func (c *Client) ExtractFields(ctx context.Context, req llm.ExtractRequest) (llm.ReceiptFields, []byte, error) {
	schema := llm.BuildReceiptJSONSchema(req.AllowedCategories)

	sys := buildSystemPrompt(req.AllowedCategories, req.DefaultCurrency, req.Timezone, req.CountryHint)
	user := buildUserPrompt(req.OCRText, req.FilenameHint, req.FolderHint)

	body := map[string]any{
		"model":       c.cfg.Model,
		"temperature": c.cfg.Temperature,
		// Ask for JSON only
		"response_format": map[string]any{"type": "json_object"},
		"messages": []map[string]any{
			{"role": "system", "content": sys},
			{"role": "user", "content": user + "\n\nReturn ONLY JSON that matches the provided schema."},
			// Stuff the schema as a visible instruction (since chat/completions doesn't accept json_schema directly).
			{"role": "system", "content": "JSON Schema:\n" + mustJSON(schema)},
		},
		// Low max-tokens is usually fine; leave unset to let the model decide, or set a cap:
		// "max_tokens": 800,
	}

	endpoint := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	raw, err := c.post(ctx, endpoint, body)
	if err != nil {
		return llm.ReceiptFields{}, nil, err
	}

	// Parse choices[0].message.content
	var cc struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &cc); err != nil {
		return llm.ReceiptFields{}, raw, fmt.Errorf("decode openai response: %w", err)
	}
	if len(cc.Choices) == 0 {
		return llm.ReceiptFields{}, raw, fmt.Errorf("no choices in openai response")
	}
	content := strings.TrimSpace(cc.Choices[0].Message.Content)

	// content should be JSON — validate against schema
	if err := llm.ValidateJSONAgainstSchema(schema, []byte(content)); err != nil {
		return llm.ReceiptFields{}, []byte(content), fmt.Errorf("schema validation failed: %w", err)
	}

	// Finally decode to our struct.
	var out llm.ReceiptFields
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return llm.ReceiptFields{}, []byte(content), fmt.Errorf("unmarshal fields: %w", err)
	}
	return out, []byte(content), nil
}

func (c *Client) post(ctx context.Context, url string, body map[string]any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai http error: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			c.log.Warn("close response body", "error", err)
		}
	}(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		return nil, fmt.Errorf("openai status %d: %s", resp.StatusCode, buf.String())
	}

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	return buf.Bytes(), nil
}

func buildSystemPrompt(categories []string, defaultCurrency, tz, country string) string {
	var catLine string
	if len(categories) > 0 {
		catLine = "Allowed categories (enum): " + strings.Join(categories, ", ") + ". "
	} else {
		catLine = "Category must be a short, sensible label if present. "
	}
	if defaultCurrency == "" {
		defaultCurrency = "USD"
	}
	parts := []string{
		"You are a receipts parser. Given OCR text & file hints, extract fields and return ONLY JSON that matches the JSON Schema provided.",
		"Use ISO-8601 dates (YYYY-MM-DD).",
		"Currency must be a 3-letter ISO 4217 code; default to " + defaultCurrency + " if uncertain.",
		catLine,
	}
	if tz != "" {
		parts = append(parts, "If you need to disambiguate dates, prefer timezone: "+tz+".")
	}
	if country != "" {
		parts = append(parts, "Country hint: "+country+".")
	}
	return strings.Join(parts, " ")
}

func buildUserPrompt(ocr, filename, folder string) string {
	var b strings.Builder
	b.WriteString("Filename: ")
	b.WriteString(filename)
	b.WriteString("\nFolder path: ")
	b.WriteString(folder)
	b.WriteString("\n\nOCR text (first ~3k chars):\n")
	if len(ocr) > 3000 {
		b.WriteString(ocr[:3000])
	} else {
		b.WriteString(ocr)
	}
	return b.String()
}

func mustJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
