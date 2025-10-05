package entity

import (
	"time"

	"github.com/google/uuid"
)

// ReceiptFile represents a receipt file for data transfer between layers.
type ReceiptFile struct {
	ID          uuid.UUID `json:"id"`
	ProfileID   uuid.UUID `json:"profile_id"`
	SourcePath  string    `json:"source_path"`
	ContentHash []byte    `json:"content_hash"`
	Filename    string    `json:"filename"`
	FileExt     string    `json:"file_ext"`
	FileSize    int       `json:"file_size"`
	UploadedAt  time.Time `json:"uploaded_at"`
}
