package repository

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/gen/ent/receipt"
)

type ReceiptRepository interface {
	ListReceipts(ctx context.Context, profileID uuid.UUID, fromDate, toDate *time.Time) ([]*ent.Receipt, error)
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

func (r *receiptRepository) ListReceipts(ctx context.Context, profileID uuid.UUID, fromDate, toDate *time.Time) ([]*ent.Receipt, error) {
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
	return recs, nil
}
