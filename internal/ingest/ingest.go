package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	ent "github.com/joseph-ayodele/receipts-tracker/gen/ent"
	entreceiptfile "github.com/joseph-ayodele/receipts-tracker/gen/ent/receiptfile"
)

// IngestPath computes sha256(file), normalizes ext, and inserts receipt_files
// if (profile_id, content_hash) is new; else returns the existing row.
// Returns (row, deduplicated, nil).
func IngestPath(ctx context.Context, entc *ent.Client, profileID uuid.UUID, path string) (*ent.ReceiptFile, bool, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, false, fmt.Errorf("abs path: %w", err)
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(abs)), ".")
	if ext == "" {
		return nil, false, fmt.Errorf("file has no extension: %s", abs)
	}
	// Optional: enforce allowed set (pdf/jpg/png)
	switch ext {
	case "pdf", "jpg", "jpeg", "png":
	default:
		return nil, false, fmt.Errorf("unsupported extension: %s", ext)
	}

	f, err := os.Open(abs)
	if err != nil {
		return nil, false, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, false, fmt.Errorf("hash: %w", err)
	}
	sum := h.Sum(nil)
	now := time.Now().UTC()

	// Check existing (profile_id, content_hash)
	existing, err := entc.ReceiptFile.Query().
		Where(
			entreceiptfile.ProfileID(profileID),
			entreceiptfile.ContentHash(sum),
		).
		Only(ctx)
	if err == nil {
		return existing, true, nil
	}

	// Create new
	row, err := entc.ReceiptFile.
		Create().
		SetProfileID(profileID).
		SetSourcePath(abs).
		SetContentHash(sum).
		SetFileExt(ext).
		SetUploadedAt(now).
		Save(ctx)
	if err != nil {
		return nil, false, err
	}
	return row, false, nil
}

// HashHex computes sha256 hex for a file (helper for testing/logging).
func HashHex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
