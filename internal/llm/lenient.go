package llm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	reLast4   = regexp.MustCompile(`^\d{4}$`)
	reDecimal = regexp.MustCompile(`^-?\d+(\.\d{1,2})?$`)
	optMoney  = []string{"subtotal", "discount", "other_fees", "tax", "tip"} // optional only
)

// SanitizeOptionalFields removes or normalizes optional fields that don't meet our stricter schema,
// so the overall document can still validate. We only touch OPTIONALS.
func SanitizeOptionalFields(doc []byte) ([]byte, []string, error) {
	var m map[string]any
	if err := json.Unmarshal(doc, &m); err != nil {
		return nil, nil, err
	}

	var dropped []string
	var changed []string

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

	for _, k := range optMoney {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case nil:
				delete(m, k)
				dropped = append(dropped, k)
			case float64:
				m[k] = fmt.Sprintf("%.2f", t)
				changed = append(changed, k)
			case string:
				s := strings.TrimSpace(t)
				if s == "" || strings.EqualFold(s, "null") {
					delete(m, k)
					dropped = append(dropped, k)
					continue
				}
				// accept numbers like "7", "7.0", "7.08", or signed
				if !reDecimal.MatchString(s) {
					// try parse and reformat
					if f, err := strconv.ParseFloat(s, 64); err == nil {
						m[k] = fmt.Sprintf("%.2f", f)
						changed = append(changed, k)
					} else {
						delete(m, k)
						dropped = append(dropped, k)
					}
				} else {
					// normalize to two decimals
					if f, err := strconv.ParseFloat(s, 64); err == nil {
						m[k] = fmt.Sprintf("%.2f", f)
						if s != m[k] {
							changed = append(changed, k)
						}
					}
				}
			default:
				// unknown type -> drop
				delete(m, k)
				dropped = append(dropped, k)
			}
		}
	}

	b, err := json.Marshal(m)
	if err != nil {
		return nil, nil, err
	}
	return b, dropped, nil
}
