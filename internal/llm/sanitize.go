package llm

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
)

// NormalizeAndSanitizeJSON
// - Renames known synonyms (shipping_fees -> other_fees)
// - Drops null/empty optionals
// - Coerces numeric -> string for money-ish fields
// - Removes unknown keys (strict additionalProperties = false friendliness)
func NormalizeAndSanitizeJSON(raw []byte, logger *slog.Logger) ([]byte, []string, error) {
	if logger == nil {
		logger = slog.Default()
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, nil, fmt.Errorf("sanitize: decode: %w", err)
	}

	dropped := make([]string, 0, 8)
	renamed := func(from, to string) {
		if v, ok := m[from]; ok {
			// don't overwrite existing value if already present
			if _, exists := m[to]; !exists {
				m[to] = v
			}
			delete(m, from)
			dropped = append(dropped, from+"->"+to)
		}
	}

	// 1) rename synonyms to your schema
	renamed("shipping_fee", "other_fees")
	renamed("shipping_fees", "other_fees")
	renamed("fees", "other_fees")

	// 2) drop null / "" for optionals; coerce money fields to strings
	moneyFields := []string{"subtotal", "tax", "total", "tip", "other_fees", "discount"}
	coerceMoney := func(k string) {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case float64:
				m[k] = fmt.Sprintf("%.2f", t)
			case int:
				m[k] = fmt.Sprintf("%d", t)
			case string:
				s := strings.TrimSpace(t)
				if s == "" {
					delete(m, k)
					dropped = append(dropped, k+"(empty)")
				} else {
					m[k] = s
				}
			case nil:
				delete(m, k)
				dropped = append(dropped, k+"(null)")
			default:
				// unexpected type -> drop
				delete(m, k)
				dropped = append(dropped, k+"(type)")
			}
		}
	}
	for _, k := range moneyFields {
		coerceMoney(k)
	}

	// 3) normalize payment fields lightly
	if v, ok := m["payment_method"].(string); ok {
		pm := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(v), " ", "_"))
		if pm != "" {
			m["payment_method"] = pm
		} else {
			delete(m, "payment_method")
			dropped = append(dropped, "payment_method(empty)")
		}
	}
	if v, ok := m["payment_last4"].(string); ok {
		s := strings.TrimSpace(v)
		// keep only last 4 digits if longer/shorter noise
		if len(s) >= 4 {
			m["payment_last4"] = s[len(s)-4:]
		} else {
			// too short → drop
			delete(m, "payment_last4")
			dropped = append(dropped, "payment_last4(short)")
		}
	}

	// 4) remove unknown keys (everything not in the schema set below)
	allowed := map[string]struct{}{
		"merchant_name": {}, "tx_date": {}, "subtotal": {}, "tax": {}, "total": {},
		"currency_code": {}, "category": {}, "payment_method": {}, "payment_last4": {},
		"description": {}, "tip": {}, "other_fees": {}, "discount": {},
		"confidence": {}, // harmless if model added it; your validator can ignore or allow
	}
	for k := range maps.Clone(m) {
		if _, ok := allowed[k]; !ok {
			delete(m, k)
			dropped = append(dropped, k+"(unknown)")
		}
	}

	// 5) trim obvious strings
	trimKeys := []string{"merchant_name", "tx_date", "currency_code", "category", "description"}
	for _, k := range trimKeys {
		if v, ok := m[k].(string); ok {
			s := strings.TrimSpace(v)
			if s == "" {
				delete(m, k)
				dropped = append(dropped, k+"(empty)")
			} else {
				m[k] = s
			}
		}
	}

	out, err := json.Marshal(m)
	if err != nil {
		return nil, dropped, fmt.Errorf("sanitize: encode: %w", err)
	}
	if len(dropped) > 0 {
		logger.Warn("llm.extract.normalize_sanitize", "dropped", slices.Collect(dropped))
	}
	return out, dropped, nil
}
