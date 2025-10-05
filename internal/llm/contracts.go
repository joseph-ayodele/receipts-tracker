package llm

import "context"

type ProfileContext struct {
	ProfileName    string `json:"profile_name,omitempty"`
	JobTitle       string `json:"job_title,omitempty"`
	JobDescription string `json:"job_description,omitempty"`
}

// ReceiptFields is the normalized shape we want from the LLM.
type ReceiptFields struct {
	MerchantName    string  `json:"merchant_name"`
	TxDate          string  `json:"tx_date"`                // YYYY-MM-DD
	Subtotal        string  `json:"subtotal,omitempty"`     // decimal
	Discount        string  `json:"discount,omitempty"`     // decimal NEGATIVE or POSITIVE total reduction
	ShippingFee     string  `json:"shipping_fee,omitempty"` // decimal
	Tax             string  `json:"tax,omitempty"`          // decimal
	Total           string  `json:"total"`                  // decimal
	CurrencyCode    string  `json:"currency_code"`          // ISO 4217
	Category        string  `json:"category,omitempty"`     // must match AllowedCategories if provided
	PaymentMethod   string  `json:"payment_method,omitempty"`
	PaymentLast4    string  `json:"payment_last4,omitempty"` // 4 digits
	Description     string  `json:"description,omitempty"`   // business need (tax-friendly)
	ModelConfidence float32 `json:"confidence,omitempty"`    // optional (0..1)
}

type ExtractRequest struct {
	OCRText           string
	FilenameHint      string
	FolderHint        string
	AllowedCategories []string
	DefaultCurrency   string
	Timezone          string

	PrepConfidence float32
	FilePath       string

	Profile ProfileContext
}

// FieldExtractor is the interface our pipeline depends on.
type FieldExtractor interface {
	ExtractFields(ctx context.Context, req ExtractRequest) (ReceiptFields, []byte /*rawJSON*/, error)
}
