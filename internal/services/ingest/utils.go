package ingest

import (
	"path/filepath"
	"strings"

	"github.com/joseph-ayodele/receipts-tracker/constants"
)

// AllowedExt checks if a file extension is in the allowed set.
func AllowedExt(ext string) bool {
	return constants.IsAllowedExt(ext)
}

// IsHidden checks if a file or directory is hidden (starts with '.').
func IsHidden(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, ".")
}
