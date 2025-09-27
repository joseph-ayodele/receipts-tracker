package constants

import "strings"

// FileTypes holds the allowed file types for the format field in ExtractJob.
var FileTypes = []string{"PDF", "IMAGE", "TXT"}

// AllowedExtensions holds the default allowed file extensions for receipts ingestion.
var AllowedExtensions = map[string]struct{}{
	"pdf":  {},
	"jpg":  {},
	"jpeg": {},
	"png":  {},
}

// NormalizeExt lowercases and trims the dot from a file extension.
func NormalizeExt(ext string) string {
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}
