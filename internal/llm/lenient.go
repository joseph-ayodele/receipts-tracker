package llm

import (
	"encoding/json"
	"regexp"
	"strings"
)

var reLast4 = regexp.MustCompile(`^\d{4}$`)

// SanitizeOptionalFields removes or normalizes optional fields that don't meet our stricter schema,
// so the overall document can still validate. We only touch OPTIONALS.
func SanitizeOptionalFields(doc []byte, allowedCategories []string) ([]byte, []string, error) {
	var m map[string]any
	if err := json.Unmarshal(doc, &m); err != nil {
		return nil, nil, err
	}

	dropped := []string{}

	// payment_last4: if present but not 4 digits, drop it
	if v, ok := m["payment_last4"].(string); ok {
		s := strings.TrimSpace(v)
		if !reLast4.MatchString(s) {
			delete(m, "payment_last4")
			dropped = append(dropped, "payment_last4")
		} else {
			m["payment_last4"] = s
		}
	}

	// payment_method: normalize case a bit (optional, not strict)
	if v, ok := m["payment_method"].(string); ok {
		m["payment_method"] = strings.ToUpper(strings.TrimSpace(v))
	}

	// currency_code: required overall; still normalize casing if present
	if v, ok := m["currency_code"].(string); ok {
		m["currency_code"] = strings.ToUpper(strings.TrimSpace(v))
	}

	b, err := json.Marshal(m)
	if err != nil {
		return nil, nil, err
	}
	return b, dropped, nil
}
