package llm

// BuildReceiptJSONSchema returns a JSON-Schema (draft 2020-12 subset) as a generic map.
// We pass this to OpenAI as a structured output constraint and also use it locally to validate.
func BuildReceiptJSONSchema(allowedCategories []string) map[string]any {
	props := map[string]any{
		"merchant_name":  map[string]any{"type": "string", "minLength": 1},
		"tx_date":        map[string]any{"type": "string", "pattern": `^\d{4}-\d{2}-\d{2}$`},
		"subtotal":       decimalProp(),
		"discount":       decimalProp(), // optional
		"shipping_fee":   decimalProp(), // optional
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
