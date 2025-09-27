package repository

import (
	"context"
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
}

func NewReceiptRepository(client *ent.Client) ReceiptRepository {
	return &receiptRepository{client: client}
}

func (r *receiptRepository) ListReceipts(ctx context.Context, profileID uuid.UUID, fromDate, toDate *time.Time) ([]*ent.Receipt, error) {
	q := r.client.Receipt.Query().Where(receipt.ProfileID(profileID))
	if fromDate != nil {
		q = q.Where(receipt.TxDateGTE(*fromDate))
	}
	if toDate != nil {
		q = q.Where(receipt.TxDateLTE(*toDate))
	}
	return q.Order(receipt.ByTxDate()).All(ctx)
}
