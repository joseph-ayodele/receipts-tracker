package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ExtractJob represents an extract job for data transfer between layers.
type ExtractJob struct {
	ID                   uuid.UUID       `json:"id"`
	FileID               uuid.UUID       `json:"file_id"`
	ProfileID            uuid.UUID       `json:"profile_id"`
	ReceiptID            *uuid.UUID      `json:"receipt_id,omitempty"`
	Format               string          `json:"format"`
	StartedAt            time.Time       `json:"started_at"`
	FinishedAt           *time.Time      `json:"finished_at,omitempty"`
	Status               *string         `json:"status,omitempty"`
	ErrorMessage         *string         `json:"error_message,omitempty"`
	ExtractionConfidence *float32        `json:"extraction_confidence,omitempty"`
	NeedsReview          bool            `json:"needs_review"`
	OCRText              *string         `json:"ocr_text,omitempty"`
	ExtractedJSON        json.RawMessage `json:"extracted_json,omitempty"`
	ModelName            *string         `json:"model_name,omitempty"`
	ModelParams          json.RawMessage `json:"model_params,omitempty"`
}
