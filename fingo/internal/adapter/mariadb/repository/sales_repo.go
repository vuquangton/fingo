package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"

	sqlc "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/sales"
)

type SalesRepo struct {
	db      *sql.DB
	queries *sqlc.Queries
}

func NewSalesRepo(db *sql.DB) *SalesRepo {
	return &SalesRepo{
		db:      db,
		queries: sqlc.New(db),
	}
}

func (r *SalesRepo) SaveSalesInvoice(ctx context.Context, si *sales.SalesInvoice) error {
	if si == nil {
		return errors.New("sales invoice cannot be nil")
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return fmt.Errorf("failed to begin tx for sales invoice: %w", err)
	}
	defer tx.Rollback()

	qtx := r.queries.WithTx(tx)

	cqtCode := sql.NullString{Valid: false}
	if si.EInvoiceCodeCQT != nil && *si.EInvoiceCodeCQT != "" {
		cqtCode = sql.NullString{String: *si.EInvoiceCodeCQT, Valid: true}
	}

	err = qtx.CreateSalesInvoice(ctx, sqlc.CreateSalesInvoiceParams{
		ID:                 si.ID,
		VoucherID:          si.VoucherID,
		CompanyProfileID:   si.CompanyProfileID,
		CustomerID:         si.CustomerID,
		InvoiceTemplate:    si.InvoiceTemplate,
		InvoiceSeries:      si.InvoiceSeries,
		InvoiceNo:          si.InvoiceNo,
		InvoiceDate:        si.InvoiceDate,
		DueDate:            si.DueDate,
		PaymentMethod:      si.PaymentMethod,
		EinvoiceStatus:     sqlc.SalesInvoicesEinvoiceStatus(si.EInvoiceStatus),
		EinvoiceCodeCqt:    cqtCode,
		SubtotalVnd:        si.SubtotalVND.String(),
		DiscountVnd:        si.DiscountVND.String(),
		VatAmountVnd:       si.VATAmountVND.String(),
		TotalAmountVnd:     si.TotalAmountVND.String(),
		IsStockOutwardAuto: si.IsStockOutwardAuto,
	})
	if err != nil {
		return fmt.Errorf("failed to insert sales invoice header: %w", err)
	}

	for _, l := range si.Lines {
		whID := sql.NullString{Valid: false}
		if l.WarehouseID != nil && *l.WarehouseID != "" {
			whID = sql.NullString{String: *l.WarehouseID, Valid: true}
		}

		note := sql.NullString{Valid: false}
		if l.Note != "" {
			note = sql.NullString{String: l.Note, Valid: true}
		}

		err = qtx.CreateSalesInvoiceLine(ctx, sqlc.CreateSalesInvoiceLineParams{
			ID:                l.ID,
			SalesInvoiceID:    si.ID,
			LineOrder:         int32(l.LineOrder),
			ItemID:            l.ItemID,
			WarehouseID:       whID,
			DebitAccountID:    l.DebitAccountID,
			CreditAccountID:   l.CreditAccountID,
			Quantity:          l.Quantity.String(),
			UnitPriceVnd:      l.UnitPriceVND.String(),
			AmountVnd:         l.AmountVND.String(),
			DiscountRate:      l.DiscountRate.String(),
			DiscountAmountVnd: l.DiscountAmountVND.String(),
			VatRate:           l.VATRate.String(),
			VatAmountVnd:      l.VATAmountVND.String(),
			Note:              note,
		})
		if err != nil {
			return fmt.Errorf("failed to insert sales invoice line %d: %w", l.LineOrder, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit sales invoice tx: %w", err)
	}

	return nil
}

func (r *SalesRepo) GetSalesInvoiceByID(ctx context.Context, id string) (*sales.SalesInvoice, error) {
	row, err := r.queries.GetSalesInvoiceByID(ctx, id)
	if err != nil {
		return nil, err
	}

	lines, err := r.queries.ListSalesInvoiceLinesByInvoiceID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to list lines for sales invoice %s: %w", id, err)
	}

	return r.mapToDomainSalesInvoice(row, lines)
}

func (r *SalesRepo) GetSalesInvoiceByVoucherID(ctx context.Context, voucherID string) (*sales.SalesInvoice, error) {
	row, err := r.queries.GetSalesInvoiceByVoucherID(ctx, voucherID)
	if err != nil {
		return nil, err
	}

	lines, err := r.queries.ListSalesInvoiceLinesByInvoiceID(ctx, row.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list lines for sales invoice %s: %w", row.ID, err)
	}

	return r.mapToDomainSalesInvoice(row, lines)
}

func (r *SalesRepo) UpdateEInvoiceStatus(ctx context.Context, id string, status sales.EInvoiceStatus, cqtCode *string) error {
	code := sql.NullString{Valid: false}
	if cqtCode != nil && *cqtCode != "" {
		code = sql.NullString{String: *cqtCode, Valid: true}
	}
	return r.queries.UpdateSalesInvoiceEInvoiceStatus(ctx, sqlc.UpdateSalesInvoiceEInvoiceStatusParams{
		EinvoiceStatus:  sqlc.SalesInvoicesEinvoiceStatus(status),
		EinvoiceCodeCqt: code,
		ID:              id,
	})
}

func (r *SalesRepo) mapToDomainSalesInvoice(row sqlc.SalesInvoice, lines []sqlc.SalesInvoiceLine) (*sales.SalesInvoice, error) {
	subtotal, err := decimal.NewFromString(row.SubtotalVnd)
	if err != nil {
		return nil, fmt.Errorf("invalid subtotal '%s': %w", row.SubtotalVnd, err)
	}

	discount, err := decimal.NewFromString(row.DiscountVnd)
	if err != nil {
		return nil, fmt.Errorf("invalid discount '%s': %w", row.DiscountVnd, err)
	}

	vat, err := decimal.NewFromString(row.VatAmountVnd)
	if err != nil {
		return nil, fmt.Errorf("invalid vat amount '%s': %w", row.VatAmountVnd, err)
	}

	total, err := decimal.NewFromString(row.TotalAmountVnd)
	if err != nil {
		return nil, fmt.Errorf("invalid total amount '%s': %w", row.TotalAmountVnd, err)
	}

	var cqtCode *string
	if row.EinvoiceCodeCqt.Valid {
		cqtCode = &row.EinvoiceCodeCqt.String
	}

	domainLines := make([]sales.SalesInvoiceLine, len(lines))
	for i, l := range lines {
		qty, _ := decimal.NewFromString(l.Quantity)
		unitPrice, _ := decimal.NewFromString(l.UnitPriceVnd)
		amount, _ := decimal.NewFromString(l.AmountVnd)
		discRate, _ := decimal.NewFromString(l.DiscountRate)
		discAmount, _ := decimal.NewFromString(l.DiscountAmountVnd)
		vatRate, _ := decimal.NewFromString(l.VatRate)
		vatAmount, _ := decimal.NewFromString(l.VatAmountVnd)

		var whID *string
		if l.WarehouseID.Valid {
			whID = &l.WarehouseID.String
		}

		domainLines[i] = sales.SalesInvoiceLine{
			ID:                l.ID,
			SalesInvoiceID:    l.SalesInvoiceID,
			LineOrder:         int(l.LineOrder),
			ItemID:            l.ItemID,
			WarehouseID:       whID,
			DebitAccountID:    l.DebitAccountID,
			CreditAccountID:   l.CreditAccountID,
			Quantity:          qty,
			UnitPriceVND:      unitPrice,
			AmountVND:         amount,
			DiscountRate:      discRate,
			DiscountAmountVND: discAmount,
			VATRate:           vatRate,
			VATAmountVND:      vatAmount,
			Note:              l.Note.String,
		}
	}

	return &sales.SalesInvoice{
		ID:                 row.ID,
		VoucherID:          row.VoucherID,
		CompanyProfileID:   row.CompanyProfileID,
		CustomerID:         row.CustomerID,
		InvoiceTemplate:    row.InvoiceTemplate,
		InvoiceSeries:      row.InvoiceSeries,
		InvoiceNo:          row.InvoiceNo,
		InvoiceDate:        row.InvoiceDate,
		DueDate:            row.DueDate,
		PaymentMethod:      row.PaymentMethod,
		EInvoiceStatus:     sales.EInvoiceStatus(row.EinvoiceStatus),
		EInvoiceCodeCQT:    cqtCode,
		SubtotalVND:        subtotal,
		DiscountVND:        discount,
		VATAmountVND:       vat,
		TotalAmountVND:     total,
		IsStockOutwardAuto: row.IsStockOutwardAuto,
		Lines:              domainLines,
	}, nil
}
