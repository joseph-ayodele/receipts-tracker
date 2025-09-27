package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	ent "github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

type Usecase struct {
	Profiles    repository.ProfileRepository
	Files       repository.ReceiptFileRepository
	AllowedExts map[string]struct{}
}

func NewUsecase(p repository.ProfileRepository, f repository.ReceiptFileRepository) *Usecase {
	return &Usecase{Profiles: p, Files: f}
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

// Implement receiptsv1.Ingestor-style method (no ent types in signature).
func (u *Usecase) IngestPath(ctx context.Context, profileID uuid.UUID, path string) (fileID string, dedup bool, hexHash string, ext string, uploadedAt time.Time, sourcePath string, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false, "", "", time.Time{}, "", fmt.Errorf("abs path: %w", err)
	}
	ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(abs)), ".")
	if ext == "" || !u.allowed(ext) {
		return "", false, "", "", time.Time{}, "", fmt.Errorf("unsupported or missing extension: %q", ext)
	}
	//exists, err := u.Profiles.Exists(ctx, entprofile.ID(profileID))
	//if err != nil {
	//	return "", false, "", "", time.Time{}, "", fmt.Errorf("check profile: %w", err)
	//}
	//if !exists {
	//	return "", false, "", "", time.Time{}, "", fmt.Errorf("profile not found")
	//}

	f, err := os.Open(abs)
	if err != nil {
		return "", false, "", "", time.Time{}, "", fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", false, "", "", time.Time{}, "", fmt.Errorf("hash: %w", err)
	}
	sum := h.Sum(nil)
	hexHash = hex.EncodeToString(sum)
	now := time.Now().UTC()

	row, dedup, err := u.Files.UpsertByHash(ctx, profileID, abs, ext, sum, now)
	if err != nil {
		return "", false, "", "", time.Time{}, "", err
	}
	return row.ID.String(), dedup, hexHash, row.FileExt, row.UploadedAt, row.SourcePath, nil
}

// Optional: if you still need the ent row internally for other flows.
func (u *Usecase) ingestEnt(ctx context.Context, profileID uuid.UUID, path string) (*ent.ReceiptFile, bool, string, error) {
	_, dedup, hexHash, _, _, _, err := u.IngestPath(ctx, profileID, path)
	if err != nil {
		return nil, false, "", err
	}
	// If needed, you can look it back up via Files repo.
	return nil, dedup, hexHash, nil
}
