package ocr

import (
	"regexp"
	"strings"
)

var (
	reCRLF        = regexp.MustCompile(`\r\n?`)
	reTabs        = regexp.MustCompile(`\t+`)
	reMultiSpace  = regexp.MustCompile(` {2,}`)
	reMultiBlank  = regexp.MustCompile(`\n{3,}`)
	reO0Artifacts = regexp.MustCompile(`\b0([1-9])\b`) // loose "0" vs "O" artifacts
)

var reBoxNoise = regexp.MustCompile(`(?m)^\s*[_\-]{3,}\s*$`)

// Normalize collapses noisy whitespace and fixes common OCR artifacts.
// Conservative: keeps line breaks; collapses >2 newlines into a single blank line.
func Normalize(s string) string {
	if s == "" {
		return s
	}
	s = reCRLF.ReplaceAllString(s, "\n")
	s = reTabs.ReplaceAllString(s, " ")
	s = reMultiSpace.ReplaceAllString(s, " ")
	// collapse too many blank lines
	s = reMultiBlank.ReplaceAllString(s, "\n\n")
	// trim trailing spaces on lines
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	s = strings.Join(lines, "\n")
	// very light artifact fix (you can extend this later or make configurable)
	s = reO0Artifacts.ReplaceAllString(s, "O$1")
	return strings.TrimSpace(s)
}
