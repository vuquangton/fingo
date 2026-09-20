package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"

	sqlc "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/purchase"
)

type PurchaseRepo struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func NewPurchaseRepo(db *sql.DB) *PurchaseRepo {
	return &PurchaseRepo{
		db:      db,
		queries: sqlc.New(db),
	}
}

func (r *PurchaseRepo) SavePurchaseInvoice(ctx context.Context, pi *purchase.PurchaseInvoice) error {
	if pi == nil {
		return errors.New("purchase invoice cannot be nil")
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return fmt.Errorf("failed to begin tx for purchase invoice: %w", err)
	}
	defer tx.Rollback()

	qtx := r.queries.WithTx(tx)

	err = qtx.CreatePurchaseInvoice(ctx, sqlc.CreatePurchaseInvoiceParams{
		ID:                 pi.ID,
		VoucherID:          pi.VoucherID,
		CompanyProfileID:   pi.CompanyProfileID,
		VendorID:           pi.VendorID,
		InvoiceTemplate:    pi.InvoiceTemplate,
		InvoiceSeries:      pi.InvoiceSeries,
		InvoiceNo:          pi.InvoiceNo,
		InvoiceDate:        pi.InvoiceDate,
		DueDate:            pi.DueDate,
		PaymentStatus:      sqlc.PurchaseInvoicesPaymentStatus(pi.PaymentStatus),
		SubtotalVnd:        pi.SubtotalVND.String(),
		VatAmountVnd:       pi.VATAmountVND.String(),
		TotalAmountVnd:     pi.TotalAmountVND.String(),
		PaidAmountVnd:      pi.PaidAmountVND.String(),
		IsStockInwardAuto:  pi.IsStockInwardAuto,
	})
	if err != nil {
		return fmt.Errorf("failed to insert purchase invoice header: %w", err)
	}

	for _, l := range pi.Lines {
		whID := sql.NullString{Valid: false}
		if l.WarehouseID != nil && *l.WarehouseID != "" {
			whID = sql.NullString{String: *l.WarehouseID, Valid: true}
		}

		note := sql.NullString{Valid: false}
		if l.Note != "" {
			note = sql.NullString{String: l.Note, Valid: true}
		}

		err = qtx.CreatePurchaseInvoiceLine(ctx, sqlc.CreatePurchaseInvoiceLineParams{
			ID:                  l.ID,
			PurchaseInvoiceID:   pi.ID,
			LineOrder:           int32(l.LineOrder),
			ItemID:              l.ItemID,
			WarehouseID:         whID,
			DebitAccountID:      l.DebitAccountID,
			CreditAccountID:     l.CreditAccountID,
			Quantity:            l.Quantity.String(),
			UnitPriceVnd:        l.UnitPriceVND.String(),
			AmountVnd:           l.AmountVND.String(),
			VatRate:             l.VATRate.String(),
			VatAmountVnd:        l.VATAmountVND.String(),
			Note:                note,
		})
		if err != nil {
			return fmt.Errorf("failed to insert purchase invoice line %d: %w", l.LineOrder, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit purchase invoice tx: %w", err)
	}

	return nil
}

func (r *PurchaseRepo) GetPurchaseInvoiceByID(ctx context.Context, id string) (*purchase.PurchaseInvoice, error) {
	row, err := r.queries.GetPurchaseInvoiceByID(ctx, id)
	if err != nil {
		return nil, err
	}

	lines, err := r.queries.ListPurchaseInvoiceLinesByInvoiceID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to list lines for purchase invoice %s: %w", id, err)
	}

	return r.mapToDomainPurchaseInvoice(row, lines)
}

func (r *PurchaseRepo) GetPurchaseInvoiceByVoucherID(ctx context.Context, voucherID string) (*purchase.PurchaseInvoice, error) {
	row, err := r.queries.GetPurchaseInvoiceByVoucherID(ctx, voucherID)
	if err != nil {
		return nil, err
	}

	lines, err := r.queries.ListPurchaseInvoiceLinesByInvoiceID(ctx, row.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list lines for purchase invoice %s: %w", row.ID, err)
	}

	return r.mapToDomainPurchaseInvoice(row, lines)
}

func (r *PurchaseRepo) UpdatePaymentStatus(ctx context.Context, id string, status purchase.PaymentStatus, paidAmount decimal.Decimal) error {
	return r.queries.UpdatePurchaseInvoicePaymentStatus(ctx, sqlc.UpdatePurchaseInvoicePaymentStatusParams{
		PaymentStatus: sqlc.PurchaseInvoicesPaymentStatus(status),
		PaidAmountVnd: paidAmount.String(),
		ID:            id,
	})
}

func (r *PurchaseRepo) mapToDomainPurchaseInvoice(row sqlc.PurchaseInvoice, lines []sqlc.PurchaseInvoiceLine) (*purchase.PurchaseInvoice, error) {
	subtotal, err := decimal.NewFromString(row.SubtotalVnd)
	if err != nil {
		return nil, fmt.Errorf("invalid subtotal '%s': %w", row.SubtotalVnd, err)
	}

	vat, err := decimal.NewFromString(row.VatAmountVnd)
	if err != nil {
		return nil, fmt.Errorf("invalid vat amount '%s': %w", row.VatAmountVnd, err)
	}

	total, err := decimal.NewFromString(row.TotalAmountVnd)
	if err != nil {
		return nil, fmt.Errorf("invalid total amount '%s': %w", row.TotalAmountVnd, err)
	}

	paid, err := decimal.NewFromString(row.PaidAmountVnd)
	if err != nil {
		return nil, fmt.Errorf("invalid paid amount '%s': %w", row.PaidAmountVnd, err)
	}

	domainLines := make([]purchase.PurchaseInvoiceLine, len(lines))
	for i, l := range lines {
		qty, _ := decimal.NewFromString(l.Quantity)
		unitPrice, _ := decimal.NewFromString(l.UnitPriceVnd)
		amount, _ := decimal.NewFromString(l.AmountVnd)
		vatRate, _ := decimal.NewFromString(l.VatRate)
		vatAmount, _ := decimal.NewFromString(l.VatAmountVnd)

		var whID *string
		if l.WarehouseID.Valid {
			whID = &l.WarehouseID.String
		}

		domainLines[i] = purchase.PurchaseInvoiceLine{
			ID:                l.ID,
			PurchaseInvoiceID: l.PurchaseInvoiceID,
			LineOrder:         int(l.LineOrder),
			ItemID:            l.ItemID,
			WarehouseID:       whID,
			DebitAccountID:    l.DebitAccountID,
			CreditAccountID:   l.CreditAccountID,
			Quantity:          qty,
			UnitPriceVND:      unitPrice,
			AmountVND:         amount,
			VATRate:           vatRate,
			VATAmountVND:      vatAmount,
			Note:              l.Note.String,
		}
	}

	return &purchase.PurchaseInvoice{
		ID:                row.ID,
		VoucherID:         row.VoucherID,
		CompanyProfileID:  row.CompanyProfileID,
		VendorID:          row.VendorID,
		InvoiceTemplate:   row.InvoiceTemplate,
		InvoiceSeries:     row.InvoiceSeries,
		InvoiceNo:         row.InvoiceNo,
		InvoiceDate:       row.InvoiceDate,
		DueDate:           row.DueDate,
		PaymentStatus:     purchase.PaymentStatus(row.PaymentStatus),
		SubtotalVND:       subtotal,
		VATAmountVND:      vat,
		TotalAmountVND:    total,
		PaidAmountVND:     paid,
		IsStockInwardAuto: row.IsStockInwardAuto,
		Lines:             domainLines,
	}, nil
}
