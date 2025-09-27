package constants

import "strings"

// extToFormat is the single source of truth for supported extensions and their formats.
var extToFormat = map[string]string{
	"pdf":  "PDF",
	"jpg":  "IMAGE",
	"jpeg": "IMAGE",
	"png":  "IMAGE",
	"txt":  "TXT",
}

// NormalizeExt lowercases and trims the dot from a file extension.
func NormalizeExt(ext string) string {
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}

// IsAllowedExt checks if a file extension is supported.
func IsAllowedExt(ext string) bool {
	ext = NormalizeExt(ext)
	_, ok := extToFormat[ext]
	return ok
}

// MapExtToFormat returns the logical format for a given extension,
// or an empty string if unsupported.
func MapExtToFormat(ext string) string {
	ext = NormalizeExt(ext)
	return extToFormat[ext]
}
