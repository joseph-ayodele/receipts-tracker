package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/internal/llm"
)

// ExtractFields implements llm.FieldExtractor using text-only chat/completions.
// If PrepConfidence is low and FilePath is provided, we LOG that a vision path
// would be preferable, but we DO NOT switch behavior yet (future step).
func (c *Client) ExtractFields(ctx context.Context, req llm.ExtractRequest) (llm.ReceiptFields, []byte, error) {
	rid := uuid.New().String()
	start := time.Now()

	c.log.Info("llm.extract.start",
		"req_id", rid,
		"model", c.cfg.Model,
		"temp", c.cfg.Temperature,
		"text_len", len(req.OCRText),
		"has_file_path", req.FilePath != "",
		"prep_confidence", req.PrepConfidence,
		"allowed_categories", len(req.AllowedCategories),
		"default_currency", req.DefaultCurrency,
		"timezone", req.Timezone,
	)

	if req.FilePath != "" && req.PrepConfidence > 0 && req.PrepConfidence < c.cfg.LowConfThreshold {
		c.log.Warn("llm.extract.low_ocr_confidence",
			"req_id", rid, "prep_confidence", req.PrepConfidence,
			"hint", "vision path not implemented; proceeding with text-only")
	}

	schema := llm.BuildReceiptJSONSchema(req.AllowedCategories)
	sys := buildSystemPrompt(req)
	user := buildUserPrompt(req.OCRText, req.FilenameHint, req.FolderHint)

	body := map[string]any{
		"model":           c.cfg.Model,
		"temperature":     c.cfg.Temperature,
		"response_format": map[string]any{"type": "json_object"},
		"messages": []map[string]any{
			{"role": "system", "content": sys},
			{"role": "user", "content": user + "\n\nReturn ONLY JSON that matches the provided schema."},
			{"role": "system", "content": "JSON Schema:\n" + mustJSON(schema)},
		},
	}

	endpoint := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	raw, httpErr := c.post(ctx, endpoint, body)
	if httpErr != nil {
		c.log.Error("llm.extract.http_error",
			"req_id", rid, "error", httpErr,
			"elapsed_ms", time.Since(start).Milliseconds(),
		)
		return llm.ReceiptFields{}, nil, httpErr
	}

	var cc struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &cc); err != nil {
		c.log.Error("llm.extract.decode_error",
			"req_id", rid, "error", err, "raw_bytes", len(raw),
			"elapsed_ms", time.Since(start).Milliseconds(),
		)
		return llm.ReceiptFields{}, raw, fmt.Errorf("decode openai response: %w", err)
	}
	if len(cc.Choices) == 0 {
		c.log.Error("llm.extract.no_choices",
			"req_id", rid, "raw", string(raw),
			"elapsed_ms", time.Since(start).Milliseconds(),
		)
		return llm.ReceiptFields{}, raw, fmt.Errorf("no choices in openai response")
	}
	content := strings.TrimSpace(cc.Choices[0].Message.Content)
	rawContent := []byte(content)

	// Validate strictly first.
	if err := llm.ValidateJSONAgainstSchema(schema, rawContent); err != nil {
		if c.cfg.LenientOptional {
			// Try a lenient sanitize: drop/normalize optional offenders and re-validate.
			cleaned, dropped, sErr := llm.SanitizeOptionalFields(rawContent)
			if sErr == nil {
				if vErr := llm.ValidateJSONAgainstSchema(schema, cleaned); vErr == nil {
					c.log.Warn("llm.extract.lenient_sanitize_applied",
						"req_id", rid, "dropped", dropped,
						"elapsed_ms", time.Since(start).Milliseconds(),
					)
					rawContent = cleaned
				} else {
					c.log.Error("llm.extract.schema_validation_failed",
						"req_id", rid, "error", vErr, "content", string(rawContent),
						"elapsed_ms", time.Since(start).Milliseconds(),
					)
					return llm.ReceiptFields{}, rawContent, fmt.Errorf("schema validation failed: %w", vErr)
				}
			} else {
				c.log.Error("llm.extract.sanitize_failed",
					"req_id", rid, "error", sErr,
					"elapsed_ms", time.Since(start).Milliseconds(),
				)
				return llm.ReceiptFields{}, rawContent, fmt.Errorf("sanitize failed: %w", sErr)
			}
		} else {
			c.log.Error("llm.extract.schema_validation_failed",
				"req_id", rid, "error", err, "content", string(rawContent),
				"elapsed_ms", time.Since(start).Milliseconds(),
			)
			return llm.ReceiptFields{}, rawContent, fmt.Errorf("schema validation failed: %w", err)
		}
	}

	var out llm.ReceiptFields
	if err := json.Unmarshal(rawContent, &out); err != nil {
		c.log.Error("llm.extract.unmarshal_failed",
			"req_id", rid, "error", err,
			"elapsed_ms", time.Since(start).Milliseconds(),
		)
		return llm.ReceiptFields{}, rawContent, fmt.Errorf("unmarshal fields: %w", err)
	}

	c.log.Info("llm.extract.ok",
		"req_id", rid,
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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai http error: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			c.log.Warn("openai response body close error", "error", err)
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

func buildSystemPrompt(req llm.ExtractRequest) string {
	var catLine string
	if len(req.AllowedCategories) > 0 {
		catLine = "Allowed categories (enum): " + strings.Join(req.AllowedCategories, ", ") + ". "
	} else {
		catLine = "Category must be a short, sensible label if present. "
	}
	if req.DefaultCurrency == "" {
		req.DefaultCurrency = "USD"
	}
	ctxBits := []string{}
	if req.Profile.ProfileName != "" {
		ctxBits = append(ctxBits, "Profile: "+req.Profile.ProfileName+".")
	}
	if req.Profile.JobTitle != "" {
		ctxBits = append(ctxBits, "Job Title: "+req.Profile.JobTitle+".")
	}
	if req.Profile.JobDescription != "" {
		ctxBits = append(ctxBits, "Job Description: "+req.Profile.JobDescription+".")
	}

	parts := []string{
		"You are a receipts parser. Return ONLY JSON that matches the JSON Schema provided.",
		"Use ISO-8601 dates (YYYY-MM-DD).",
		"Currency must be a 3-letter ISO 4217 code; default to " + req.DefaultCurrency + " if uncertain.",
		catLine,
		"Business context: " + strings.Join(ctxBits, " "),
		// concise, generic, tax-friendly:
		"For 'description', write a few words explaining the business need (concise, generic, professional) for the irs. Avoid addresses, timestamps, names.",

		// money fields behavior:
		"If a tip appears, include it under 'tip'.",
		"If taxes appear, put them in 'tax' (never include taxes in 'other_fees').",
		"Sum non-tax, non-tip surcharges into 'other_fees' (e.g., booking, airport, regulatory).",
		"Include 'discount' if visible (positive amount representing the discount magnitude).",

		// formatting hygiene:
		"Never output null. If a field is not present, omit it.",
	}
	if tz := strings.TrimSpace(req.Timezone); tz != "" {
		parts = append(parts, "If dates are ambiguous, prefer timezone: "+tz+".")
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
