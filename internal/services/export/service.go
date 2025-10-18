package export

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/joseph-ayodele/receipts-tracker/gen/ent"
	"github.com/joseph-ayodele/receipts-tracker/internal/repository"
)

// Service is a tiny façade over repositories that produces XLSX bytes for exports.
type Service struct {
	ent          *ent.Client
	receiptsRepo repository.ReceiptRepository
	filesRepo    repository.ReceiptFileRepository
	logger       *slog.Logger
}

func NewService(entc *ent.Client, repo repository.ReceiptRepository, filesRepo repository.ReceiptFileRepository, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{ent: entc, receiptsRepo: repo, filesRepo: filesRepo, logger: logger}
}

// ExportReceiptsXLSX returns an XLSX workbook (as bytes) for the given profile and date window.
// If only from is provided -> from..today (inclusive).
// If only to is provided   -> beginning..to (inclusive).
// If neither is provided   -> all receipts for profile.
func (s *Service) ExportReceiptsXLSX(ctx context.Context, profileID uuid.UUID, from, to *time.Time) ([]byte, error) {
	start := time.Now()

	// Normalize dates (date-only, UTC)
	var fromDate, toDate *time.Time
	if from != nil {
		f := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
		fromDate = &f
	}
	if to != nil {
		t := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
		toDate = &t
	}
	if fromDate != nil && toDate == nil {
		today := time.Now().UTC()
		t := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
		toDate = &t
	}

	recs, err := s.receiptsRepo.ListReceipts(ctx, profileID, fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("query receipts: %w", err)
	}

	f := excelize.NewFile()
	const sheet = "Receipts"
	if index, _ := f.GetSheetIndex(sheet); index == -1 {
		_, err := f.NewSheet(sheet)
		if err != nil {
			return nil, err
		}
	}
	activeIndex, _ := f.GetSheetIndex(sheet)
	f.SetActiveSheet(activeIndex)

	headers := []string{
		"Transaction Date",
		"Expense Category",
		"Item/Service",
		"Amount",
		"Deduction Amount",
		"Purpose/Notes",
		"Receipt/File Path",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}

	row := 2
	for _, r := range recs {
		// Resolve file path if we have a link
		filePath := ""
		// Prefer a direct field (e.g., r.FileID) if your schema has it; otherwise adapt this block.
		if r.FileID != nil && *r.FileID != uuid.Nil {
			fileRow, err := s.filesRepo.GetByID(ctx, *r.FileID)
			if err == nil && fileRow != nil {
				filePath = fileRow.SourcePath
			}
		}

		// Choose Item/Service (for now merchant name). If you later add ItemName, prefer it.
		item := r.MerchantName
		if item == "" {
			item = "—"
		}

		// Amount & Deduction Amount (for now fully deductible)
		amount := fmt.Sprintf("%v", r.Total)
		deduction := amount

		write := func(col int, v any) {
			cell, _ := excelize.CoordinatesToCellName(col, row)
			_ = f.SetCellValue(sheet, cell, v)
		}

		// 1) Transaction Date
		if !r.TxDate.IsZero() {
			write(1, r.TxDate.Format("2006-01-02"))
		} else {
			write(1, "")
		}
		// 2) Expense Category (enum string)
		write(2, r.CategoryName)

		// 3) Item/Service
		write(3, item)

		// 4) Amount
		write(4, amount)

		// 5) Deduction Amount (same as Amount for now)
		write(5, deduction)

		// 6) Purpose/Notes
		write(6, truncate(fmt.Sprintf("%v", r.Description), 140))

		// 7) Receipt/File Path
		write(7, filePath)

		row++
	}

	// Widen a few columns
	_ = f.SetColWidth(sheet, "A", "A", 14) // date
	_ = f.SetColWidth(sheet, "B", "B", 22) // category
	_ = f.SetColWidth(sheet, "C", "C", 28) // item
	_ = f.SetColWidth(sheet, "D", "E", 14) // amounts
	_ = f.SetColWidth(sheet, "F", "F", 48) // notes
	_ = f.SetColWidth(sheet, "G", "G", 60) // path

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("xlsx write: %w", err)
	}

	s.logger.Info("export.xlsx.ok",
		"profile_id", profileID.String(),
		"rows", len(recs),
		"elapsed_ms", time.Since(start).Milliseconds(),
	)
	return buf.Bytes(), nil
}

func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
