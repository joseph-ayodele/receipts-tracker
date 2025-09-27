package ingest

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/joseph-ayodele/receipts-tracker/constants"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

// ValidateProfile checks if a profile exists by ID.
func ValidateProfile(ctx context.Context, repo repository.ProfileRepository, profileID uuid.UUID) error {
	exists, err := repo.Exists(ctx, profileID)
	if err != nil {
		log.Printf("check profile error: %v", err)
		return err
	}
	if !exists {
		log.Printf("profile not found: %v", profileID)
		return fmt.Errorf("profile not found")
	}
	return nil
}

// AllowedExt checks if a file extension is in the allowed set (defaults to pdf/jpg/jpeg/png).
func AllowedExt(ext string) bool {
	ext = constants.NormalizeExt(ext)
	_, ok := constants.AllowedExtensions[ext]
	return ok
}

// IsHidden checks if a file or directory is hidden (starts with '.').
func IsHidden(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, ".")
}
