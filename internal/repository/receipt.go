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
	"github.com/joseph-ayodele/receipts-tracker/internal/utils"
)

// CreateReceiptRequest wraps parameters for creating a receipt.
type CreateReceiptRequest struct {
	File          *ent.ReceiptFile
	JobID         uuid.UUID
	ReceiptFields llm.ReceiptFields
	CategoryID    *int
}

type ReceiptRepository interface {
	ListReceipts(ctx context.Context, profileID uuid.UUID, fromDate, toDate *time.Time) ([]*entity.Receipt, error)
	UpsertFromFields(ctx context.Context, request *CreateReceiptRequest) (*entity.Receipt, error)
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
	q := r.client.Receipt.Query().Where(receipt.ProfileID(profileID))
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
		result[i] = utils.ToReceipt(rec)
	}
	return result, nil
}

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

	builder := r.client.Receipt.Create().
		SetProfileID(file.ProfileID).
		SetMerchantName(f.MerchantName).
		SetTxDate(txDate).
		SetCurrencyCode(f.CurrencyCode).
		SetTotal(*dec(f.Total)).
		SetNillableSubtotal(dec(f.Subtotal)).
		SetNillableTax(dec(f.Tax))

	if request.CategoryID != nil {
		builder = builder.SetCategoryID(*request.CategoryID)
	}
	if f.PaymentMethod != "" {
		builder = builder.SetPaymentMethod(f.PaymentMethod)
	}
	if f.PaymentLast4 != "" {
		builder = builder.SetPaymentLast4(f.PaymentLast4)
	}
	if f.Description != "" {
		builder = builder.SetDescription(f.Description)
	}

	rec, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}

	// Update the file to link to this receipt (removed SetJobID - shouldn't be needed for receipts)
	// TODO: Schema may need receipt_id field on receipt_files table
	// if err := r.client.ReceiptFile.UpdateOneID(file.ID).SetReceiptID(rec.ID).Exec(ctx); err != nil {
	// 	return nil, err
	// }

	return utils.ToReceipt(rec), nil
}
