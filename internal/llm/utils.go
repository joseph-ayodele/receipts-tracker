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
	u, mt, err := readAsDataURL(req.FilePath)
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
		case "heic", "heif", "heics", "heifs":
			// If HEIC slipped through, still label something—OpenAI may not accept; we try image/heic
			mt = "image/heic"
		default:
			mt = "application/octet-stream"
		}
	}
	data := base64.StdEncoding.EncodeToString(b)
	return "data:" + mt + ";base64," + data, mt, nil
}
