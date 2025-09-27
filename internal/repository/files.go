package repository

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	entfile "github.com/joseph-ayodele/receipts-tracker/gen/ent/receiptfile"
)

type ReceiptFileRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*ent.ReceiptFile, error)
	GetByProfileAndHash(ctx context.Context, profileID uuid.UUID, hash []byte) (*ent.ReceiptFile, error)
	Create(ctx context.Context, profileID uuid.UUID, sourcePath, filename, ext string, size int, hash []byte, uploadedAt time.Time) (*ent.ReceiptFile, error)
	UpsertByHash(ctx context.Context, profileID uuid.UUID, sourcePath, filename, ext string, size int, hash []byte, uploadedAt time.Time) (*ent.ReceiptFile, bool, error)
}

type receiptFileRepo struct {
	ent    *ent.Client
	logger *slog.Logger
}

func NewReceiptFileRepository(entc *ent.Client, logger *slog.Logger) ReceiptFileRepository {
	return &receiptFileRepo{
		ent:    entc,
		logger: logger,
	}
}

func (r *receiptFileRepo) GetByID(ctx context.Context, id uuid.UUID) (*ent.ReceiptFile, error) {
	return r.ent.ReceiptFile.Get(ctx, id)
}

func (r *receiptFileRepo) GetByProfileAndHash(ctx context.Context, profileID uuid.UUID, hash []byte) (*ent.ReceiptFile, error) {
	row, err := r.ent.ReceiptFile.Query().
		Where(
			entfile.ProfileID(profileID),
			entfile.ContentHash(hash),
		).Only(ctx)
	if err != nil {
		r.logger.Error("failed to get receipt file by profile and hash", "profile_id", profileID, "error", err)
		return nil, err
	}
	return row, nil
}

func (r *receiptFileRepo) Create(ctx context.Context, profileID uuid.UUID, sourcePath, filename, ext string, size int, hash []byte, uploadedAt time.Time) (*ent.ReceiptFile, error) {
	row, err := r.ent.ReceiptFile.Create().
		SetProfileID(profileID).
		SetSourcePath(sourcePath).
		SetFilename(filename).
		SetFileExt(ext).
		SetFileSize(size).
		SetContentHash(hash).
		SetUploadedAt(uploadedAt).
		Save(ctx)
	if err != nil {
		r.logger.Error("failed to create receipt file", "profile_id", profileID, "source_path", sourcePath, "filename", filename, "error", err)
		return nil, err
	}
	return row, nil
}

func (r *receiptFileRepo) UpsertByHash(ctx context.Context, profileID uuid.UUID, sourcePath, filename, ext string, size int, hash []byte, uploadedAt time.Time) (*ent.ReceiptFile, bool, error) {
	if existing, err := r.GetByProfileAndHash(ctx, profileID, hash); err == nil {
		return existing, true, nil
	}
	row, err := r.Create(ctx, profileID, sourcePath, filename, ext, size, hash, uploadedAt)
	if err != nil {
		r.logger.Error("failed to upsert receipt file by hash", "profile_id", profileID, "source_path", sourcePath, "filename", filename, "error", err)
		return nil, false, err
	}
	return row, false, nil
}
