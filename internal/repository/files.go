package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	entfile "github.com/joseph-ayodele/receipts-tracker/gen/ent/receiptfile"
)

type ReceiptFileRepository interface {
	GetByProfileAndHash(ctx context.Context, profileID uuid.UUID, hash []byte) (*ent.ReceiptFile, error)
	Create(ctx context.Context, profileID uuid.UUID, sourcePath, filename, ext string, size int, hash []byte, uploadedAt time.Time) (*ent.ReceiptFile, error)
	UpsertByHash(ctx context.Context, profileID uuid.UUID, sourcePath, filename, ext string, size int, hash []byte, uploadedAt time.Time) (*ent.ReceiptFile, bool, error)
}

type receiptFileRepo struct{ ent *ent.Client }

func NewReceiptFileRepository(entc *ent.Client) ReceiptFileRepository { return &receiptFileRepo{ent: entc} }

func (r *receiptFileRepo) GetByProfileAndHash(ctx context.Context, profileID uuid.UUID, hash []byte) (*ent.ReceiptFile, error) {
	return r.ent.ReceiptFile.Query().
		Where(
			entfile.ProfileID(profileID),
			entfile.ContentHash(hash),
		).Only(ctx)
}

func (r *receiptFileRepo) Create(ctx context.Context, profileID uuid.UUID, sourcePath, filename, ext string, size int, hash []byte, uploadedAt time.Time) (*ent.ReceiptFile, error) {
	return r.ent.ReceiptFile.Create().
		SetProfileID(profileID).
		SetSourcePath(sourcePath).
		SetFilename(filename).
		SetFileExt(ext).
		SetFileSize(size).
		SetContentHash(hash).
		SetUploadedAt(uploadedAt).
		Save(ctx)
}

func (r *receiptFileRepo) UpsertByHash(ctx context.Context, profileID uuid.UUID, sourcePath, filename, ext string, size int, hash []byte, uploadedAt time.Time) (*ent.ReceiptFile, bool, error) {
	if existing, err := r.GetByProfileAndHash(ctx, profileID, hash); err == nil {
		return existing, true, nil
	}
	row, err := r.Create(ctx, profileID, sourcePath, filename, ext, size, hash, uploadedAt)
	return row, false, err
}
