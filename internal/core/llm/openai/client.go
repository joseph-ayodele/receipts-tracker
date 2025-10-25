package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/internal/core/llm"
)

// ExtractFields implements llm.FieldExtractor using text-only chat/completions.
// If PrepConfidence is low and FilePath is provided, we LOG that a vision path
// would be preferable, but we DO NOT switch behavior yet (future step).
func (c *Client) ExtractFields(ctx context.Context, req llm.ExtractRequest) (llm.ReceiptFields, []byte, error) {
	reqID := uuid.New().String()
	start := time.Now()

	c.logger.Info("starting llm extraction",
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

	// decide whether to attach the image (low OCR confidence + image + vision enabled)
	attach, dataURL, _ := llm.ShouldAttachImage(req)

	// build schema + prompts
	schema := llm.BuildReceiptJSONSchema(req.AllowedCategories)
	sys := llm.BuildSystemPrompt(req)
	user := llm.BuildUserPrompt(req, attach)

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
	c.logger.Debug("openai request payload", "attach", attach, "ocr_conf", req.PrepConfidence,
		"model", c.cfg.Model)

	// 4) POST
	endpoint := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	headers := map[string]string{
		"Authorization": "Bearer " + c.cfg.APIKey,
		"Content-Type":  "application/json",
	}

	raw, status, httpErr := llm.SendJSON(ctx, c.http, endpoint, body, headers, c.logger)
	if httpErr != nil {
		c.logger.Error("llm extract http_error",
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
		c.logger.Error("llm extract decode_error",
			"req_id", reqID, "error", err, "raw_bytes", len(raw),
			"elapsed_ms", time.Since(start).Milliseconds(),
		)
		return llm.ReceiptFields{}, raw, fmt.Errorf("decode openai response: %w", err)
	}
	if len(cc.Choices) == 0 {
		c.logger.Error("llm extract no_choices",
			"req_id", reqID, "raw", string(raw),
			"elapsed_ms", time.Since(start).Milliseconds(),
		)
		return llm.ReceiptFields{}, raw, fmt.Errorf("no choices in openai response")
	}
	content := strings.TrimSpace(cc.Choices[0].Message.Content)
	rawContent := []byte(content)

	// 6) validate strictly → optional lenient sanitize → numeric normalization retry
	if err := llm.ValidateJSONAgainstSchema(schema, rawContent); err != nil {
		// First try: numeric normalization retry for pattern failures on money fields
		if isPatternFailureOnMoney(err) {
			normalized := normalizeMoneyFields(rawContent)
			if err2 := llm.ValidateJSONAgainstSchema(schema, normalized); err2 == nil {
				c.logger.Info("llm numeric normalization applied",
					"stage", "llm_normalize",
					"normalized", true,
					"fields", []string{"subtotal", "tax", "discount", "other_fees", "tip", "total"},
				)
				rawContent = normalized
			} else {
				c.logger.Error("llm extract schema_validation_failed_after_normalize",
					"req_id", reqID, "error", err2, "content", string(rawContent),
					"elapsed_ms", time.Since(start).Milliseconds(),
				)
				return llm.ReceiptFields{}, rawContent, fmt.Errorf("schema validation failed after normalize: %w", err2)
			}
		} else if c.cfg.LenientOptional {
			// Second try: lenient sanitize for other validation errors
			cleaned, dropped, sErr := llm.NormalizeAndSanitizeJSON(rawContent, c.logger)
			if sErr == nil {
				if vErr := llm.ValidateJSONAgainstSchema(schema, cleaned); vErr == nil {
					c.logger.Warn("llm extract lenient_sanitize_applied",
						"req_id", reqID, "dropped", dropped,
						"elapsed_ms", time.Since(start).Milliseconds(),
					)
					rawContent = cleaned
				} else {
					c.logger.Error("llm extract schema_validation_failed",
						"req_id", reqID, "error", vErr, "content", string(rawContent),
						"elapsed_ms", time.Since(start).Milliseconds(),
					)
					return llm.ReceiptFields{}, rawContent, fmt.Errorf("schema validation failed: %w", vErr)
				}
			} else {
				c.logger.Error("llm extract sanitize_failed",
					"req_id", reqID, "error", sErr,
					"elapsed_ms", time.Since(start).Milliseconds(),
				)
				return llm.ReceiptFields{}, rawContent, fmt.Errorf("sanitize failed: %w", sErr)
			}
		} else {
			c.logger.Error("llm extract schema_validation_failed",
				"req_id", reqID, "error", err, "content", string(rawContent),
				"elapsed_ms", time.Since(start).Milliseconds(),
			)
			return llm.ReceiptFields{}, rawContent, fmt.Errorf("schema validation failed: %w", err)
		}
	}

	// 7) unmarshal into fields
	var out llm.ReceiptFields
	if err := json.Unmarshal(rawContent, &out); err != nil {
		c.logger.Error("llm extract unmarshal_failed",
			"req_id", reqID, "error", err,
			"elapsed_ms", time.Since(start).Milliseconds(),
		)
		return llm.ReceiptFields{}, rawContent, fmt.Errorf("unmarshal fields: %w", err)
	}

	c.logger.Info("llm extract successful",
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

// normalizeMoneyFields strips currency symbols, commas, spaces, and parentheses from numeric fields
func normalizeMoneyFields(raw []byte) []byte {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return raw
	}

	norm := func(s string) string {
		// strip currency symbols, commas, spaces; turn (123.45) into -123.45
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
			s = "-" + strings.TrimSuffix(strings.TrimPrefix(s, "("), ")")
		}
		s = strings.ReplaceAll(s, ",", "")
		s = strings.TrimPrefix(s, "$")
		s = strings.ReplaceAll(s, " ", "")
		return s
	}
	keys := []string{"subtotal", "tax", "discount", "other_fees", "tip", "total"}
	for _, k := range keys {
		if v, ok := obj[k]; ok {
			if str, ok := v.(string); ok && str != "" {
				obj[k] = norm(str)
			}
		}
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return out
}

// isPatternFailureOnMoney detects if validation error is due to pattern mismatch on money fields
func isPatternFailureOnMoney(err error) bool {
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "pattern") {
		return false
	}
	for _, k := range []string{"/subtotal", "/tax", "/discount", "/other_fees", "/tip", "/total"} {
		if strings.Contains(msg, k) {
			return true
		}
	}
	return false
}
