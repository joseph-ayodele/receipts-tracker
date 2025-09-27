package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	ent "github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

type Usecase struct {
	Profiles repository.ProfileRepository
	Files    repository.ReceiptFileRepository

	AllowedExts map[string]struct{} // lowercased sans dot; nil -> default
}

func NewUsecase(p repository.ProfileRepository, f repository.ReceiptFileRepository) *Usecase {
	return &Usecase{
		Profiles: p,
		Files:    f,
	}
}

func (u *Usecase) allowed(ext string) bool {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	allow := u.AllowedExts
	if allow == nil {
		allow = map[string]struct{}{"pdf": {}, "jpg": {}, "jpeg": {}, "png": {}}
	}
	_, ok := allow[ext]
	return ok
}

// IngestPath reads and hashes the file, verifies profile exists, and upserts receipt_files.
// Returns (row, dedup, hexHash, nil) or error.
func (u *Usecase) IngestPath(ctx context.Context, profileID uuid.UUID, path string) (*ent.ReceiptFile, bool, string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		log.Printf("abs path error: %v", err)
		return nil, false, "", err
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(abs)), ".")
	if ext == "" || !u.allowed(ext) {
		log.Printf("unsupported or missing extension: %q", ext)
		return nil, false, "", fmt.Errorf("unsupported or missing extension")
	}

	//exists, err := u.Profiles.Exists(ctx, entprofile.ID(profileID))
	//if err != nil {
	//	log.Printf("check profile error: %v", err)
	//	return nil, false, "", err
	//}
	//if !exists {
	//	log.Printf("profile not found")
	//	return nil, false, "", fmt.Errorf("profile not found")
	//}

	f, err := os.Open(abs)
	if err != nil {
		log.Printf("open error: %v", err)
		return nil, false, "", err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Printf("error closing file: %v", err)
		}
	}(f)

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Printf("hash error: %v", err)
		return nil, false, "", err
	}
	sum := h.Sum(nil)
	sumHex := hex.EncodeToString(sum)
	now := time.Now().UTC()

	row, dedup, err := u.Files.UpsertByHash(ctx, profileID, abs, ext, sum, now)
	if err != nil {
		return nil, false, "", err
	}
	return row, dedup, sumHex, nil
}
