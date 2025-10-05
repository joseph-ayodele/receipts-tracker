package llm

import (
	"strings"
)

// BuildSystemPrompt composes the system message with currency defaults, allowed categories,
// business context, and strict-but-practical formatting rules.
func BuildSystemPrompt(req ExtractRequest) string {
	// Categories line
	var catLine string
	if len(req.AllowedCategories) > 0 {
		catLine = "Allowed categories (enum): " + strings.Join(req.AllowedCategories, ", ") + ". "
	} else {
		catLine = "Category must be a short, sensible label if present. "
	}

	// Currency fallback
	defCur := req.DefaultCurrency
	if strings.TrimSpace(defCur) == "" {
		defCur = "USD"
	}

	// Business context
	var ctxBits []string
	if n := strings.TrimSpace(req.Profile.ProfileName); n != "" {
		ctxBits = append(ctxBits, "Profile: "+n+".")
	}
	if t := strings.TrimSpace(req.Profile.JobTitle); t != "" {
		ctxBits = append(ctxBits, "Job Title: "+t+".")
	}
	if d := strings.TrimSpace(req.Profile.JobDescription); d != "" {
		ctxBits = append(ctxBits, "Job Description: "+d+".")
	}

	parts := []string{
		"You are a receipts parser. Return ONLY JSON that matches the provided JSON Schema.",
		"Use ISO-8601 dates (YYYY-MM-DD).",
		"Currency must be a 3-letter ISO 4217 code; default to " + defCur + " if uncertain.",
		catLine,
		"Business context: " + strings.Join(ctxBits, " "),
		// Description guidance (concise, tax-appropriate)
		"For 'description', write a concise, tax-appropriate business need (about 8–16 words). Avoid personal names, addresses, or timestamps.",
		// Money fields behavior:
		"If a tip appears, include it under 'tip'.",
		"If taxes appear, put them in 'tax' (never include taxes in 'other_fees').",
		"Sum non-tax, non-tip surcharges into 'other_fees' (e.g., booking, airport, regulatory).",
		"Include 'discount' if visible (positive amount representing the discount).",
		// Formatting hygiene:
		"Never output null. If a field is not present, omit it.",
	}

	if tz := strings.TrimSpace(req.Timezone); tz != "" {
		parts = append(parts, "If dates are ambiguous, prefer timezone: "+tz+".")
	}
	return strings.Join(parts, " ")
}

// BuildUserPrompt packages filename/folder hints and the OCR text (truncated to ~3k).
func BuildUserPrompt(req ExtractRequest) string {
	ocr := strings.TrimSpace(req.OCRText)
	filename := strings.TrimSpace(req.FilenameHint)
	folder := strings.TrimSpace(req.FolderHint)

	// Build prompt
	var b strings.Builder
	if filename != "" {
		b.WriteString("Filename: ")
		b.WriteString(filename)
		b.WriteString("\n")
	}
	if folder != "" {
		b.WriteString("Folder path: ")
		b.WriteString(folder)
		b.WriteString("\n")
	}
	b.WriteString("\nOCR text (first ~3k chars):\n")
	if len(ocr) > 3000 {
		b.WriteString(ocr[:3000])
		b.WriteString("\n…(truncated)")
	} else {
		b.WriteString(ocr)
	}
	return b.String()
}

// BuildReceiptJSONSchema returns a JSON-Schema (draft 2020-12 subset) as a generic map.
// We pass this to OpenAI as a structured output constraint and also use it locally to validate.
func BuildReceiptJSONSchema(allowedCategories []string) map[string]any {
	props := map[string]any{
		"merchant_name":  map[string]any{"type": "string", "minLength": 1},
		"tx_date":        map[string]any{"type": "string", "pattern": `^\d{4}-\d{2}-\d{2}$`},
		"subtotal":       decimalProp(),
		"discount":       decimalProp(), // optional
		"other_fees":     decimalProp(), // optional
		"tip":            decimalProp(),
		"tax":            decimalProp(),
		"total":          decimalProp(),
		"currency_code":  map[string]any{"type": "string", "minLength": 3, "maxLength": 3},
		"category":       map[string]any{"type": "string"},
		"payment_method": map[string]any{"type": "string"},
		"payment_last4":  map[string]any{"type": "string", "minLength": 4, "maxLength": 4, "pattern": `^\d{4}$`},
		"description":    map[string]any{"type": "string"},
		"confidence":     map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
	}
	required := []string{"merchant_name", "tx_date", "total", "currency_code"}

	// Constrain category if a taxonomy is provided.
	if len(allowedCategories) > 0 {
		props["category"] = map[string]any{
			"type": "string",
			"enum": allowedCategories,
		}
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           props,
		"required":             required,
	}
}

func decimalProp() map[string]any {
	return map[string]any{
		"type":    "string",
		"pattern": `^-?\d+(\.\d{1,2})?$`, // allow negatives for discounts
	}
}
