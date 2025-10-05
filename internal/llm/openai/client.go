package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/constants"
	"github.com/joseph-ayodele/receipts-tracker/internal/llm"
)

// ExtractFields implements llm.FieldExtractor using text-only chat/completions.
// If PrepConfidence is low and FilePath is provided, we LOG that a vision path
// would be preferable, but we DO NOT switch behavior yet (future step).
func (c *Client) ExtractFields(ctx context.Context, req llm.ExtractRequest) (llm.ReceiptFields, []byte, error) {
	reqID := uuid.New().String()
	start := time.Now()

	c.logger.Info("llm.extract.start",
		"req_id", reqID,
		"model", c.cfg.Model,
		"temp", c.cfg.Temperature,
		"text_len", len(req.OCRText),
		"has_file_path", req.FilePath != "",
		"prep_confidence", req.PrepConfidence,
		"allowed_categories", len(req.AllowedCategories),
		"default_currency", req.DefaultCurrency,
		"timezone", req.Timezone,
	)

	// 1) build schema + prompts
	schema := llm.BuildReceiptJSONSchema(req.AllowedCategories)
	sys := llm.BuildSystemPrompt(req)
	user := llm.BuildUserPrompt(req)

	// 2) decide whether to attach the image (low OCR confidence + image + vision enabled)
	attach, dataURL, mimeType := c.shouldAttachImage(req)

	c.logger.Info("llm.build_payload",
		"req_id", reqID,
		"attach", attach,
		"mime", mimeType,
		"ocr_conf", req.PrepConfidence,
		"model", c.cfg.Model,
	)

	// 3) build messages for /chat/completions
	//    - we keep the schema as a separate system message (your pattern)
	var userContent any
	if attach {
		userContent = []map[string]any{
			{"type": "text", "text": user},
			{"type": "input_image", "image_url": map[string]any{"url": dataURL}},
		}
	} else {
		userContent = user
	}

	body := map[string]any{
		"model":           c.cfg.Model,
		"temperature":     c.cfg.Temperature,
		"response_format": map[string]any{"type": "json_object"},
		"messages": []map[string]any{
			{"role": "system", "content": sys},
			{"role": "system", "content": "JSON Schema:\n" + mustJSON(schema)},
			{"role": "user", "content": userContent},
		},
	}

	// 4) POST
	endpoint := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	headers := map[string]string{
		"Authorization": "Bearer " + c.cfg.APIKey,
		"Content-Type":  "application/json",
	}
	raw, status, httpErr := llm.SendJSON(ctx, c.http, endpoint, body, headers, c.logger)
	if httpErr != nil {
		c.logger.Error("llm.extract.http_error",
			"req_id", reqID, "status", status, "error", httpErr,
			"elapsed_ms", time.Since(start).Milliseconds(),
		)
		return llm.ReceiptFields{}, nil, httpErr
	}

	// 5) decode response
	var cc struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &cc); err != nil {
		c.logger.Error("llm.extract.decode_error",
			"req_id", reqID, "error", err, "raw_bytes", len(raw),
			"elapsed_ms", time.Since(start).Milliseconds(),
		)
		return llm.ReceiptFields{}, raw, fmt.Errorf("decode openai response: %w", err)
	}
	if len(cc.Choices) == 0 {
		c.logger.Error("llm.extract.no_choices",
			"req_id", reqID, "raw", string(raw),
			"elapsed_ms", time.Since(start).Milliseconds(),
		)
		return llm.ReceiptFields{}, raw, fmt.Errorf("no choices in openai response")
	}
	content := strings.TrimSpace(cc.Choices[0].Message.Content)
	rawContent := []byte(content)

	// 6) validate strictly → optional lenient sanitize
	if err := llm.ValidateJSONAgainstSchema(schema, rawContent); err != nil {
		if c.cfg.LenientOptional {
			cleaned, dropped, sErr := llm.NormalizeAndSanitizeJSON(rawContent, c.logger)
			if sErr == nil {
				if vErr := llm.ValidateJSONAgainstSchema(schema, cleaned); vErr == nil {
					c.logger.Warn("llm.extract.lenient_sanitize_applied",
						"req_id", reqID, "dropped", dropped,
						"elapsed_ms", time.Since(start).Milliseconds(),
					)
					rawContent = cleaned
				} else {
					c.logger.Error("llm.extract.schema_validation_failed",
						"req_id", reqID, "error", vErr, "content", string(rawContent),
						"elapsed_ms", time.Since(start).Milliseconds(),
					)
					return llm.ReceiptFields{}, rawContent, fmt.Errorf("schema validation failed: %w", vErr)
				}
			} else {
				c.logger.Error("llm.extract.sanitize_failed",
					"req_id", reqID, "error", sErr,
					"elapsed_ms", time.Since(start).Milliseconds(),
				)
				return llm.ReceiptFields{}, rawContent, fmt.Errorf("sanitize failed: %w", sErr)
			}
		} else {
			c.logger.Error("llm.extract.schema_validation_failed",
				"req_id", reqID, "error", err, "content", string(rawContent),
				"elapsed_ms", time.Since(start).Milliseconds(),
			)
			return llm.ReceiptFields{}, rawContent, fmt.Errorf("schema validation failed: %w", err)
		}
	}

	// 7) unmarshal into fields
	var out llm.ReceiptFields
	if err := json.Unmarshal(rawContent, &out); err != nil {
		c.logger.Error("llm.extract.unmarshal_failed",
			"req_id", reqID, "error", err,
			"elapsed_ms", time.Since(start).Milliseconds(),
		)
		return llm.ReceiptFields{}, rawContent, fmt.Errorf("unmarshal fields: %w", err)
	}

	c.logger.Info("llm.extract.ok",
		"req_id", reqID,
		"merchant", out.MerchantName,
		"date", out.TxDate,
		"total", out.Total,
		"currency", out.CurrencyCode,
		"category", out.Category,
		"elapsed_ms", time.Since(start).Milliseconds(),
	)
	return out, rawContent, nil
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

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai http error: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			c.logger.Warn("openai response body close error", "error", err)
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

func mustJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func (c *Client) shouldAttachImage(req llm.ExtractRequest) (attach bool, dataURL, mimeType string) {
	attach = c.cfg.EnableVision &&
		req.FilePath != "" &&
		constants.MapExtToFormat(filepath.Ext(req.FilePath)) == constants.IMAGE &&
		req.PrepConfidence < c.cfg.LowConfThreshold

	if !attach {
		return false, "", ""
	}
	u, mt, err := readAsDataURL(req.FilePath)
	if err != nil {
		c.logger.Warn("llm.attach_failed_fallback_text", "file", req.FilePath, "err", err)
		return false, "", ""
	}
	return true, u, mt
}

func readAsDataURL(path string) (string, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	mt := mime.TypeByExtension("." + ext)
	if mt == "" {
		// fallbacks
		switch ext {
		case "jpg", "jpeg":
			mt = "image/jpeg"
		case "png":
			mt = "image/png"
		case "heic", "heif", "heics", "heifs":
			// If HEIC slipped through, still label something—OpenAI may not accept; we try image/heic
			mt = "image/heic"
		default:
			mt = "application/octet-stream"
		}
	}
	data := base64.StdEncoding.EncodeToString(b)
	return "data:" + mt + ";base64," + data, mt, nil
}
