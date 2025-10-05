package llm

import "context"

// ReceiptFields is the normalized shape we want from the LLM.
type ReceiptFields struct {
	MerchantName    string  `json:"merchant_name"`
	TxDate          string  `json:"tx_date"`            // ISO-8601 date (YYYY-MM-DD)
	Subtotal        string  `json:"subtotal,omitempty"` // decimal as string
	Tax             string  `json:"tax,omitempty"`      // decimal as string
	Total           string  `json:"total"`              // decimal as string
	CurrencyCode    string  `json:"currency_code"`      // ISO 4217 (3 letters)
	Category        string  `json:"category,omitempty"` // must be in allowed taxonomy (if provided)
	PaymentMethod   string  `json:"payment_method,omitempty"`
	PaymentLast4    string  `json:"payment_last4,omitempty"`
	Description     string  `json:"description,omitempty"`
	ModelConfidence float32 `json:"confidence,omitempty"` // 0..1 (optional; we also compute our own)
}

// ExtractRequest bundles the inputs & constraints for field extraction.
type ExtractRequest struct {
	OCRText           string   // normalized OCR text (from our OCR pipeline)
	FilenameHint      string   // e.g. "2024-07-21-costco.pdf"
	FolderHint        string   // optional, to help with routing / categorization
	CountryHint       string   // optional, e.g., "US"
	AllowedCategories []string // optional; if empty we won't constrain the enum
	DefaultCurrency   string   // fallback currency if not found, e.g., "USD"
	Timezone          string   // optional; helps parse local dates
	PrepConfidence    float32  // our confidence in the OCR text (0..1)
	FilePath          string   // original file path, if our confidence is low we upload the file directly
}

// FieldExtractor is the interface our pipeline depends on.
type FieldExtractor interface {
	ExtractFields(ctx context.Context, req ExtractRequest) (ReceiptFields, []byte /*rawJSON*/, error)
}
