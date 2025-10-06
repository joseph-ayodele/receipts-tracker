package entity

import (
	"time"

	"github.com/google/uuid"
)

// Receipt represents a receipt for data transfer between layers.
type Receipt struct {
	ID            uuid.UUID `json:"id"`
	ProfileID     uuid.UUID `json:"profile_id"`
	MerchantName  string    `json:"merchant_name"`
	TxDate        time.Time `json:"tx_date"`
	Subtotal      *float64  `json:"subtotal,omitempty"`
	Tax           *float64  `json:"tax,omitempty"`
	Total         float64   `json:"total"`
	CurrencyCode  string    `json:"currency_code"`
	CategoryName  string    `json:"category_name"`
	PaymentMethod *string   `json:"payment_method,omitempty"`
	PaymentLast4  *string   `json:"payment_last4,omitempty"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
