package utils

import (
	"fmt"
	"time"

	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	receiptspb "github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"
	"github.com/joseph-ayodele/receipts-tracker/internal/entity"
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

func ToPBProfileFromEntity(p *entity.Profile) *receiptspb.Profile {
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

func ToPBReceiptFromEntity(r *entity.Receipt) *receiptspb.Receipt {
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

func ToProfile(e *ent.Profile) *entity.Profile {
	return &entity.Profile{
		ID:              e.ID,
		Name:            e.Name,
		JobTitle:        e.JobTitle,
		JobDescription:  e.JobDescription,
		DefaultCurrency: e.DefaultCurrency,
		CreatedAt:       e.CreatedAt,
		UpdatedAt:       e.UpdatedAt,
	}
}

func ToReceipt(e *ent.Receipt) *entity.Receipt {
	return &entity.Receipt{
		ID:            e.ID,
		ProfileID:     e.ProfileID,
		MerchantName:  e.MerchantName,
		TxDate:        e.TxDate,
		Subtotal:      e.Subtotal,
		Tax:           e.Tax,
		Total:         e.Total,
		CurrencyCode:  e.CurrencyCode,
		CategoryName:  e.CategoryName,
		PaymentMethod: e.PaymentMethod,
		PaymentLast4:  e.PaymentLast4,
		Description:   e.Description,
		CreatedAt:     e.CreatedAt,
		UpdatedAt:     e.UpdatedAt,
	}
}

func ToReceiptFile(e *ent.ReceiptFile) *entity.ReceiptFile {
	return &entity.ReceiptFile{
		ID:          e.ID,
		ProfileID:   e.ProfileID,
		SourcePath:  e.SourcePath,
		ContentHash: e.ContentHash,
		Filename:    e.Filename,
		FileExt:     e.FileExt,
		FileSize:    e.FileSize,
		UploadedAt:  e.UploadedAt,
	}
}

func ToExtractJob(e *ent.ExtractJob) *entity.ExtractJob {
	return &entity.ExtractJob{
		ID:                   e.ID,
		FileID:               e.FileID,
		ProfileID:            e.ProfileID,
		ReceiptID:            e.ReceiptID,
		Format:               e.Format,
		StartedAt:            e.StartedAt,
		FinishedAt:           e.FinishedAt,
		Status:               e.Status,
		ErrorMessage:         e.ErrorMessage,
		ExtractionConfidence: e.ExtractionConfidence,
		NeedsReview:          e.NeedsReview,
		OCRText:              e.OcrText,
		ExtractedJSON:        e.ExtractedJSON,
		ModelName:            e.ModelName,
		ModelParams:          e.ModelParams,
	}
}
