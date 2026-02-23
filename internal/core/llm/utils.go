package llm

import (
	"encoding/base64"
	"mime"
	"os"
	"path/filepath"

	"github.com/joseph-ayodele/receipts-tracker/constants"
)

// ResolveVisionContent returns base64 data URLs to attach as vision inputs.
// It handles both single-image files and multi-page PDFs (via VisionImagePaths).
// Returns nil if no vision attachment should be made.
func ResolveVisionContent(req ExtractRequest) []string {
	if len(req.VisionImagePaths) > 0 {
		var urls []string
		for _, p := range req.VisionImagePaths {
			u, _, err := readAsDataURL(p)
			if err == nil {
				urls = append(urls, u)
			}
		}
		return urls
	}
	attach, dataURL, _ := ShouldAttachImage(req)
	if attach {
		return []string{dataURL}
	}
	return nil
}

func ShouldAttachImage(req ExtractRequest) (attach bool, dataURL, mimeType string) {
	attach = req.FilePath != "" &&
		constants.MapExtToFormat(filepath.Ext(req.FilePath)) == constants.IMAGE &&
		(req.ForceVision || req.PrepConfidence < constants.ImageConfidenceThreshold)

	if !attach {
		return false, "", ""
	}

	// pick file path (prefer cached PNG for HEIC/HEIF)
	file := req.FilePath
	if constants.IsHEICExt(filepath.Ext(file)) && req.ArtifactCacheDir != "" && req.ContentHashHex != "" {
		cached := filepath.Join(req.ArtifactCacheDir, req.ContentHashHex+".png")
		if st, err := os.Stat(cached); err == nil && !st.IsDir() {
			file = cached
		} else {
			// still HEIC and no cached PNG → skip attach (OpenAI can't process HEIC)
			return false, "", ""
		}
	}

	// size gate
	if st, err := os.Stat(file); err == nil {
		if st.Size() > int64(constants.MaxVisionMBDefault)*1024*1024 {
			return false, "", ""
		}
	}

	u, mt, err := readAsDataURL(file)
	if err != nil {
		return false, "", ""
	}
	return true, u, mt
}

func readAsDataURL(path string) (string, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	ext := constants.NormalizeExt(filepath.Ext(path))
	mt := mime.TypeByExtension("." + ext)
	if mt == "" {
		// fallbacks
		switch ext {
		case "jpg", "jpeg":
			mt = "image/jpeg"
		case "png":
			mt = "image/png"
		default:
			mt = "application/octet-stream"
		}
	}
	data := base64.StdEncoding.EncodeToString(b)
	return "data:" + mt + ";base64," + data, mt, nil
}
