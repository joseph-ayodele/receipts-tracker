package entity

import (
	"time"

	"github.com/google/uuid"
)

// Profile represents a profile for data transfer between layers.
type Profile struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	JobTitle        *string   `json:"job_title,omitempty"`
	JobDescription  *string   `json:"job_description,omitempty"`
	DefaultCurrency string    `json:"default_currency"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
