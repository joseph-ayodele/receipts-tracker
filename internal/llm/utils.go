package llm

import (
	"encoding/base64"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/joseph-ayodele/receipts-tracker/constants"
)

func ShouldAttachImage(req ExtractRequest) (attach bool, dataURL, mimeType string) {
	attach = req.FilePath != "" &&
		constants.MapExtToFormat(filepath.Ext(req.FilePath)) == constants.IMAGE &&
		req.PrepConfidence < constants.ImageConfidenceThreshold

	if !attach {
		return false, "", ""
	}

	// pick candidate path (prefer cached PNG for HEIC/HEIF)
	candidate := req.FilePath
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(req.FilePath), "."))
	if (ext == "heic" || ext == "heif" || ext == "heics" || ext == "heifs") &&
		req.ArtifactCacheDir != "" && req.ContentHashHex != "" {
		cached := filepath.Join(req.ArtifactCacheDir, req.ContentHashHex+".png")
		if st, err := os.Stat(cached); err == nil && !st.IsDir() {
			candidate = cached
			ext = "png"
		} else {
			// still HEIC and no cached PNG → skip attach (OpenAI can't process HEIC)
			return false, "", ""
		}
	}

	// size gate
	if st, err := os.Stat(candidate); err == nil {
		if st.Size() > int64(constants.MaxVisionMBDefault)*1024*1024 {
			return false, "", ""
		}
	}

	u, mt, err := readAsDataURL(candidate)
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
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
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
