package repository

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent/receipt"
	"github.com/joseph-ayodele/receipts-tracker/internal/entity"
	"github.com/joseph-ayodele/receipts-tracker/internal/llm"
	"github.com/joseph-ayodele/receipts-tracker/internal/tools"
)

// CreateReceiptRequest wraps parameters for creating a receipt.
type CreateReceiptRequest struct {
	File          *ent.ReceiptFile
	JobID         uuid.UUID
	ReceiptFields llm.ReceiptFields
	CategoryName  string
}

type ReceiptRepository interface {
	ListReceipts(ctx context.Context, profileID uuid.UUID, fromDate, toDate *time.Time) ([]*entity.Receipt, error)
	UpsertFromFields(ctx context.Context, request *CreateReceiptRequest) (*entity.Receipt, error)
	// GetCurrentByFileID fetches the current receipt by file_id
	GetCurrentByFileID(ctx context.Context, fileID uuid.UUID) (*entity.Receipt, error)
}

type receiptRepository struct {
	client *ent.Client
	logger *slog.Logger
}

func NewReceiptRepository(client *ent.Client, logger *slog.Logger) ReceiptRepository {
	return &receiptRepository{
		client: client,
		logger: logger,
	}
}

func (r *receiptRepository) ListReceipts(ctx context.Context, profileID uuid.UUID, fromDate, toDate *time.Time) ([]*entity.Receipt, error) {
	q := r.client.Receipt.Query().
		Where(
			receipt.ProfileID(profileID),
			receipt.IsCurrent(true),
		)
	if fromDate != nil {
		q = q.Where(receipt.TxDateGTE(*fromDate))
	}
	if toDate != nil {
		q = q.Where(receipt.TxDateLTE(*toDate))
	}
	recs, err := q.Order(receipt.ByTxDate()).All(ctx)
	if err != nil {
		r.logger.Error("failed to list receipts", "profile_id", profileID, "error", err)
		return nil, err
	}

	result := make([]*entity.Receipt, len(recs))
	for i, rec := range recs {
		result[i] = tools.ToReceipt(rec)
	}
	return result, nil
}

func (r *receiptRepository) GetCurrentByFileID(ctx context.Context, fileID uuid.UUID) (*entity.Receipt, error) {
	rec, err := r.client.Receipt.Query().
		Where(
			receipt.FileIDEQ(fileID),
			receipt.IsCurrent(true),
		).
		Only(ctx)
	if err != nil {
		r.logger.Error("failed to fetch current receipt by file_id",
			"file_id", fileID, "error", err)
		return nil, err
	}

	return tools.ToReceipt(rec), nil
}

// UpsertFromFields creates a new receipt version, demoting any existing current receipts
// for the same logical receipt within a transaction for atomic de-duplication
func (r *receiptRepository) UpsertFromFields(ctx context.Context, request *CreateReceiptRequest) (*entity.Receipt, error) {
	f := request.ReceiptFields
	file := request.File

	// parse date
	txDate, err := time.Parse("2006-01-02", f.TxDate)
	if err != nil {
		return nil, err
	}

	// convert money fields
	dec := func(s string) *float64 {
		if s == "" {
			return nil
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil
		}
		return &v
	}

	total := *dec(f.Total)

	// Transaction to ensure atomic de-dupe
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func(tx *ent.Tx) {
		err := tx.Rollback()
		if err != nil {
			r.logger.Debug("transaction rollback error (may be benign)", "error", err)
		}
	}(tx)

	// Find and demote existing current receipts that match this logical receipt
	var demotedCount int

	if file.ID != uuid.Nil { // De-dupe by file_id
		demoted, err := tx.Receipt.Update().
			Where(
				receipt.FileIDEQ(file.ID),
				receipt.IsCurrent(true),
			).
			SetIsCurrent(false).
			SetUpdatedAt(time.Now()).
			Save(ctx)
		if err != nil {
			return nil, err
		}
		demotedCount = demoted

		r.logger.Info("demoted previous versions by file_id",
			"file_id", file.ID, "demoted_count", demotedCount)
	} else { // De-dupe by natural key
		demoted, err := tx.Receipt.Update().
			Where(
				receipt.ProfileID(file.ProfileID),
				receipt.MerchantName(f.MerchantName),
				receipt.TxDate(txDate),
				receipt.Total(total),
				receipt.CurrencyCode(f.CurrencyCode),
				receipt.IsCurrent(true),
			).
			SetIsCurrent(false).
			SetUpdatedAt(time.Now()).
			Save(ctx)
		if err != nil {
			return nil, err
		}
		demotedCount = demoted

		r.logger.Info("demoted previous versions by natural key",
			"profile_id", file.ProfileID, "merchant", f.MerchantName,
			"tx_date", f.TxDate, "total", f.Total, "currency", f.CurrencyCode,
			"demoted_count", demotedCount)
	}

	// Create new current version
	builder := tx.Receipt.Create().
		SetProfileID(file.ProfileID).
		SetNillableFileID(&file.ID).
		SetFilePath(file.SourcePath).
		SetMerchantName(f.MerchantName).
		SetTxDate(txDate).
		SetCurrencyCode(f.CurrencyCode).
		SetCategoryName(request.CategoryName).
		SetTotal(total).
		SetNillableSubtotal(dec(f.Subtotal)).
		SetNillableTax(dec(f.Tax)).
		SetIsCurrent(true)

	if f.Description != "" {
		builder = builder.SetDescription(f.Description)
	}

	rec, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Log version transitions for observability
	if demotedCount > 0 {
		r.logger.Info("created new receipt version",
			"receipt_id", rec.ID, "file_id", file.ID,
			"previous_versions_demoted", demotedCount)
	}

	return tools.ToReceipt(rec), nil
}
