package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// SendJSON sends a JSON request to a full URL with optional headers and returns the raw response body.
// It does not assume any provider (OpenAI/Azure/etc.). Callers decide the URL and headers.
func SendJSON(ctx context.Context, client *http.Client, url string, body any, headers map[string]string, logger *slog.Logger) ([]byte, int, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}

	reqID := uuid.New().String()
	start := time.Now()

	bs, err := json.Marshal(body)
	if err != nil {
		logger.Error("llm.http.encode_error", "req_id", reqID, "error", err)
		return nil, 0, fmt.Errorf("encode json: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bs))
	if err != nil {
		logger.Error("llm.http.build_request_error", "req_id", reqID, "error", err)
		return nil, 0, fmt.Errorf("build request: %w", err)
	}

	// Default headers; allow caller overrides.
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	logger.Info("llm.http.request",
		"req_id", reqID,
		"url", url,
		"content_length", len(bs),
	)

	resp, err := client.Do(req)
	if err != nil {
		logger.Error("llm.http.send_error", "req_id", reqID, "error", err, "elapsed_ms", time.Since(start).Milliseconds())
		return nil, 0, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logger.Warn("llm.http.response_body_close_error", "req_id", reqID, "error", err)
		}
	}(resp.Body)

	raw, _ := io.ReadAll(resp.Body)

	logger.Info("llm.http.response",
		"req_id", reqID,
		"status", resp.StatusCode,
		"bytes", len(raw),
		"elapsed_ms", time.Since(start).Milliseconds(),
	)

	if resp.StatusCode/100 != 2 {
		return raw, resp.StatusCode, fmt.Errorf("non-2xx status: %d", resp.StatusCode)
	}
	return raw, resp.StatusCode, nil
}
