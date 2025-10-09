package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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

	// decide whether to attach the image (low OCR confidence + image + vision enabled)
	attach, dataURL, mimeType := llm.ShouldAttachImage(req)

	// build schema + prompts
	schema := llm.BuildReceiptJSONSchema(req.AllowedCategories)
	sys := llm.BuildSystemPrompt(req)
	user := llm.BuildUserPrompt(req, attach)

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
			{"type": "image_url", "image_url": map[string]any{"url": dataURL}},
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

func mustJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
