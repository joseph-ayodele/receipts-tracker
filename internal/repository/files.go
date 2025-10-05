package repository

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	entfile "github.com/joseph-ayodele/receipts-tracker/gen/ent/receiptfile"
	"github.com/joseph-ayodele/receipts-tracker/internal/entity"
	"github.com/joseph-ayodele/receipts-tracker/internal/utils"
)

// CreateReceiptFileRequest wraps parameters for creating a receipt file.
type CreateReceiptFileRequest struct {
	ProfileID   uuid.UUID `json:"profile_id"`
	SourcePath  string    `json:"source_path"`
	Filename    string    `json:"filename"`
	FileExt     string    `json:"file_ext"`
	FileSize    int       `json:"file_size"`
	ContentHash []byte    `json:"content_hash"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

type ReceiptFileRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entity.ReceiptFile, error)
	GetByProfileAndHash(ctx context.Context, profileID uuid.UUID, hash []byte) (*entity.ReceiptFile, error)
	Create(ctx context.Context, request *CreateReceiptFileRequest) (*entity.ReceiptFile, error)
	UpsertByHash(ctx context.Context, request *CreateReceiptFileRequest) (*entity.ReceiptFile, bool, error)
	SetReceiptID(ctx context.Context, fileID, receiptID uuid.UUID) error
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

func (r *receiptFileRepo) GetByID(ctx context.Context, id uuid.UUID) (*entity.ReceiptFile, error) {
	file, err := r.ent.ReceiptFile.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return utils.ToReceiptFile(file), nil
}

func (r *receiptFileRepo) GetByProfileAndHash(ctx context.Context, profileID uuid.UUID, hash []byte) (*entity.ReceiptFile, error) {
	row, err := r.ent.ReceiptFile.Query().
		Where(
			entfile.ProfileID(profileID),
			entfile.ContentHash(hash),
		).Only(ctx)
	if err != nil {
		r.logger.Warn("receipt not found", "profile_id", profileID, "error", err)
		return nil, err
	}
	return utils.ToReceiptFile(row), nil
}

func (r *receiptFileRepo) Create(ctx context.Context, request *CreateReceiptFileRequest) (*entity.ReceiptFile, error) {
	row, err := r.ent.ReceiptFile.Create().
		SetProfileID(request.ProfileID).
		SetSourcePath(request.SourcePath).
		SetFilename(request.Filename).
		SetFileExt(request.FileExt).
		SetFileSize(request.FileSize).
		SetContentHash(request.ContentHash).
		SetUploadedAt(request.UploadedAt).
		Save(ctx)
	if err != nil {
		r.logger.Error("failed to create receipt file", "profile_id", request.ProfileID, "source_path", request.SourcePath, "filename", request.Filename, "error", err)
		return nil, err
	}
	return utils.ToReceiptFile(row), nil
}

func (r *receiptFileRepo) UpsertByHash(ctx context.Context, request *CreateReceiptFileRequest) (*entity.ReceiptFile, bool, error) {
	if existing, err := r.GetByProfileAndHash(ctx, request.ProfileID, request.ContentHash); err == nil {
		return existing, true, nil
	}
	row, err := r.Create(ctx, request)
	if err != nil {
		r.logger.Error("failed to upsert receipt file by hash", "profile_id", request.ProfileID, "source_path", request.SourcePath, "filename", request.Filename, "error", err)
		return nil, false, err
	}
	return row, false, nil
}

func (r *receiptFileRepo) SetReceiptID(ctx context.Context, fileID, receiptID uuid.UUID) error {
	// TODO: Schema may need receipt_id field on receipt_files table to link files to receipts
	// For now, this is a placeholder - the linking happens through the receipt creation process
	r.logger.Warn("SetReceiptID called but not implemented - links handled via receipt creation", "file_id", fileID, "receipt_id", receiptID)
	return nil
}
