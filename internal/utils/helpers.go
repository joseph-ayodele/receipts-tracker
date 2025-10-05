package utils

import (
	"fmt"
	"time"

	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	receiptspb "github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
)

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ToPBProfile(p *ent.Profile) *receiptspb.Profile {
	return &receiptspb.Profile{
		Id:              p.ID.String(),
		Name:            p.Name,
		JobTitle:        strOrEmpty(p.JobTitle),
		JobDescription:  strOrEmpty(p.JobDescription),
		DefaultCurrency: p.DefaultCurrency,
		CreatedAt:       p.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       p.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func ToPBReceipt(r *ent.Receipt) *receiptspb.Receipt {
	return &receiptspb.Receipt{
		Id:           r.ID.String(),
		ProfileId:    r.ProfileID.String(),
		MerchantName: r.MerchantName,
		TxDate:       r.TxDate.Format("2006-01-02"),
		Total:        fmt.Sprintf("%.2f", r.Total),
		CurrencyCode: r.CurrencyCode,
		CreatedAt:    r.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    r.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func ParseYMD(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return time.Time{}, err
	}
	// strip time to midnight UTC to match DATE semantics
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
}
