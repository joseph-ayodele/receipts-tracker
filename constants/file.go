package constants

import "strings"

const (
	PDF = "PDF"
	IMAGE = "IMAGE"
	TXT   = "TXT"
)

// extToFormat is the single source of truth for supported extensions and their formats.
var extToFormat = map[string]string{
	"pdf":  PDF,
	"txt":  TXT,
	"jpg":  IMAGE,
	"jpeg": IMAGE,
	"png":  IMAGE,
	"heic":  IMAGE,
	"heif":  IMAGE,
	"heics": IMAGE,
	"heifs": IMAGE,
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

// IsHEICExt returns true if the extension is in the HEIC/HEIF family.
func IsHEICExt(ext string) bool {
	switch NormalizeExt(ext) {
	case "heic", "heif", "heics", "heifs":
		return true
	default:
		return false
	}
}
