package ocr

import (
	"regexp"
	"strings"
)

var (
	reDate   = regexp.MustCompile(`\b(20\d{2}?\d{2})\b`)
	reCurr   = regexp.MustCompile(`\b(usd|eur|gbp|cad|aud|inr|jpy)\b|[$£€]`)
	reAmount = regexp.MustCompile(`\b\d{1,3}(,\d{3})*(\.\d{2})\b|\b\d+\.\d{2}\b`)
)

func hasDatePattern(s string) bool     { return reDate.MatchString(s) }
func hasCurrencyPattern(s string) bool { return reCurr.MatchString(s) }
func hasAmountPattern(s string) bool   { return reAmount.MatchString(s) }

// naive heuristic confidence based on decoded text characteristics
func heuristicConfidence(txt string) float32 {
	// very simple: boost if we see common receipt artifacts
	// (date-ish, currency-ish, amount-ish). Each adds ~0.15.
	txtL := strings.ToLower(txt)
	score := float32(0.2) // base
	if hasDatePattern(txtL) {
		score += 0.2
	}
	if hasCurrencyPattern(txtL) {
		score += 0.15
	}
	if hasAmountPattern(txtL) {
		score += 0.15
	}
	if len(txt) > 120 {
		score += 0.1
	} // enough content
	if score > 1.0 {
		score = 1.0
	}
	return score
}
