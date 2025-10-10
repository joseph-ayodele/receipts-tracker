package llm

import (
	"strings"
)

// BuildSystemPrompt composes the system message with currency defaults, allowed categories,
// business context, and strict-but-practical formatting rules.
func BuildSystemPrompt(req ExtractRequest) string {
	// Categories line + rubric
	var catLine string
	if len(req.AllowedCategories) > 0 {
		catLine = "You MUST include a 'category' and it MUST be exactly one of the allowed enum. " +
			"If uncertain, choose 'Other'. Allowed categories (enum): " + strings.Join(req.AllowedCategories, ", ") + ". "
	} else {
		catLine = "You MUST include a 'category' that is a short, sensible label. If uncertain, use 'Other'. "
	}
	rubric := buildCategoryRubric(req.AllowedCategories)

	// Currency fallback
	defCur := strings.TrimSpace(req.DefaultCurrency)
	if defCur == "" {
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
		"Category selection rubric: " + rubric,
		"Business context: " + strings.Join(ctxBits, " "),

		// Description guidance (concise, tax-appropriate)
		"For 'description', list the visible item names (comma-separated) and then add a concise, tax-appropriate business need (about 8–16 words). Avoid personal names, addresses, or timestamps.",

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

// BuildUserPrompt packages filename/folder hints. When an image is attached we intentionally
// DO NOT include OCR text per business logic (low-confidence OCR is unhelpful).
func BuildUserPrompt(req ExtractRequest, imageAttached bool) string {
	filename := strings.TrimSpace(req.FilenameHint)
	folder := strings.TrimSpace(req.FolderHint)

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

	// Only include OCR text when no image is attached (useful for non-vision runs).
	if !imageAttached {
		ocr := strings.TrimSpace(req.OCRText)
		b.WriteString("\nOCR text (first ~3k chars):\n")
		if len(ocr) > 3000 {
			b.WriteString(ocr[:3000])
			b.WriteString("\n…(truncated)")
		} else {
			b.WriteString(ocr)
		}
	} else {
		// Small nudge that helps reduce variance in category choice without exposing chain-of-thought.
		b.WriteString("\nNote: An image of the receipt is attached. Use visible item names and the rubric to pick exactly one category from the enum; if uncertain, choose 'Other'.\n")
	}

	return b.String()
}

// BuildReceiptJSONSchema returns a JSON-Schema (draft 2020-12 subset) as a generic map.
// We REQUIRE 'category' and ensure it is non-empty; when a taxonomy is provided,
// category is restricted to the enum values.
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
		"category":       map[string]any{"type": "string", "minLength": 1},
		"payment_method": map[string]any{"type": "string"},
		"payment_last4":  map[string]any{"type": "string", "minLength": 4, "maxLength": 4, "pattern": `^\d{4}$`},
		"description":    map[string]any{"type": "string"},
		"confidence":     map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
	}

	// Constrain category if a taxonomy is provided.
	if len(allowedCategories) > 0 {
		props["category"] = map[string]any{
			"type": "string",
			"enum": allowedCategories,
		}
	}

	// Make category REQUIRED so the model can't omit it.
	required := []string{"merchant_name", "tx_date", "total", "currency_code", "category"}

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

// buildCategoryRubric emits short, high-precision rules only for categories present in the enum.
// It includes tie-breakers to avoid oscillating between similar buckets.
func buildCategoryRubric(allowed []string) string {
	if len(allowed) == 0 {
		// generic rubric
		return "Use item names to decide: consumables and stationery → 'Office Supplies'; " +
			"hardware/devices → 'Office Equipment'; recurring SaaS/apps → 'Software Subscription'; " +
			"workspace furniture/fixtures → 'Home Office'; telecom plans → 'Cell Phone Service' or 'Internet'; " +
			"food/drink → 'Meals'; postage/courier → 'Shipping Expenses'; training/courses/fees → 'Professional Development'; " +
			"transport/lodging → 'Travel Expenses'; otherwise → 'Other'. When torn between two, choose the narrower, more specific one; if still unsure, choose 'Other'."
	}

	// guidance for common categories — include only those present in the enum
	defs := map[string]string{
		"Office Supplies":          "Consumables & small accessories used up in daily work (pens, paper, cleaning wipes, duster, cables/chargers, batteries).",
		"Office Equipment":         "Durable hardware/devices or peripherals (printers, monitors, keyboards, headsets, routers).",
		"Home Office":              "Furniture/fixtures or workspace improvements for a home office (desk, chair, lamp, shelving). Not consumables.",
		"Software Subscription":    "Recurring SaaS/apps/licenses (monthly/annual software fees).",
		"Cell Phone Service":       "Mobile plan charges; physical accessories go to Supplies or Equipment.",
		"Internet":                 "ISP service fees and modem/router rental; purchasing a router is Equipment.",
		"Meals":                    "Food or drink purchases tied to business purposes.",
		"Shipping Expenses":        "Postage, courier, shipping labels, packaging for shipments.",
		"Professional Development": "Courses, training, certifications, conference fees (not travel/lodging/meals).",
		"Travel Expenses":          "Transportation, lodging, baggage, tolls, parking, ride-share, airfare.",
		"Other":                    "Use only when nothing else applies unambiguously.",
	}

	var parts []string
	for _, c := range allowed {
		if d, ok := defs[c]; ok {
			parts = append(parts, c+": "+d)
		}
	}
	// add crisp tie-breakers when similar buckets exist
	// (these will only matter if both categories are in the enum)
	if hasAll(allowed, "Office Supplies", "Home Office") {
		parts = append(parts, "Tie-breaker: if both 'Office Supplies' and 'Home Office' seem plausible, prefer 'Office Supplies' unless the item is clearly furniture/fixture.")
	}
	if hasAll(allowed, "Office Supplies", "Office Equipment") {
		parts = append(parts, "Tie-breaker: accessories/consumables → 'Office Supplies'; durable devices/peripherals → 'Office Equipment'.")
	}

	if len(parts) == 0 {
		return "Use item names to pick the closest category; if uncertain, choose 'Other'."
	}
	return strings.Join(parts, " | ")
}

func hasAll(list []string, a, b string) bool {
	foundA, foundB := false, false
	for _, x := range list {
		if x == a {
			foundA = true
		} else if x == b {
			foundB = true
		}
	}
	return foundA && foundB
}
