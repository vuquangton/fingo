package purchase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"fingo/internal/domain/gl"
	"fingo/internal/domain/purchase"
	glUseCase "fingo/internal/usecase/gl"
	"fingo/pkg/logger"
)

var (
	ErrVATAccountRequired = errors.New("deductible VAT account ID (13311) is required when invoice has VAT amount")
)

type PurchaseUseCase struct {
	purchaseRepo purchase.PurchaseRepository
	glUseCase    *glUseCase.GLUseCase
	logger       *logger.Logger
}

func NewPurchaseUseCase(
	purchaseRepo purchase.PurchaseRepository,
	glUC *glUseCase.GLUseCase,
	log *logger.Logger,
) *PurchaseUseCase {
	return &PurchaseUseCase{
		purchaseRepo: purchaseRepo,
		glUseCase:    glUC,
		logger:       log,
	}
}

type CreatePurchaseInvoiceLineCommand struct {
	LineOrder       int
	ItemID          string
	WarehouseID     *string
	DebitAccountID  string // e.g. 1561, 152, 642
	CreditAccountID string // e.g. 331
	Quantity        decimal.Decimal
	UnitPriceVND    decimal.Decimal
	VATRate         decimal.Decimal
	Note            string
}

type CreatePurchaseInvoiceCommand struct {
	CompanyProfileID  string
	BranchID          *string
	VoucherNo         string
	VoucherDate       time.Time
	PostedDate        time.Time
	VendorID          string
	InvoiceTemplate   string
	InvoiceSeries     string
	InvoiceNo         string
	InvoiceDate       time.Time
	DueDate           time.Time
	VATAccountID      string // e.g. 13311 for input VAT
	IsStockInwardAuto bool
	Lines             []CreatePurchaseInvoiceLineCommand
	IdempotencyKey    *string
	CreatedBy         string
}

func (u *PurchaseUseCase) CreatePurchaseInvoice(ctx context.Context, cmd CreatePurchaseInvoiceCommand) (*purchase.PurchaseInvoice, *gl.Voucher, error) {
	// 1. Construct and validate domain PurchaseInvoice
	domainLineParams := make([]purchase.CreatePurchaseInvoiceLineParams, len(cmd.Lines))
	for i, l := range cmd.Lines {
		domainLineParams[i] = purchase.CreatePurchaseInvoiceLineParams{
			LineOrder:       l.LineOrder,
			ItemID:          l.ItemID,
			WarehouseID:     l.WarehouseID,
			DebitAccountID:  l.DebitAccountID,
			CreditAccountID: l.CreditAccountID,
			Quantity:        l.Quantity,
			UnitPriceVND:    l.UnitPriceVND,
			VATRate:         l.VATRate,
			Note:            l.Note,
		}
	}

	piID := uuid.New().String()
	pi, err := purchase.NewPurchaseInvoice(purchase.CreatePurchaseInvoiceParams{
		ID:                piID,
		VoucherID:         "pre-validation",
		CompanyProfileID:  cmd.CompanyProfileID,
		VendorID:          cmd.VendorID,
		InvoiceTemplate:   cmd.InvoiceTemplate,
		InvoiceSeries:     cmd.InvoiceSeries,
		InvoiceNo:         cmd.InvoiceNo,
		InvoiceDate:       cmd.InvoiceDate,
		DueDate:           cmd.DueDate,
		IsStockInwardAuto: cmd.IsStockInwardAuto,
		Lines:             domainLineParams,
	})
	if err != nil {
		return nil, nil, err
	}

	// 2. Validate VAT account if invoice has VAT
	if pi.VATAmountVND.GreaterThan(decimal.Zero) && strings.TrimSpace(cmd.VATAccountID) == "" {
		return nil, nil, ErrVATAccountRequired
	}

	// 3. Construct Voucher Lines
	var vLines []glUseCase.CreateVoucherLineCommand

	// (a) Principal lines: Nợ 1561/152/642 / Có 331
	for _, l := range pi.Lines {
		vLines = append(vLines, glUseCase.CreateVoucherLineCommand{
			DebitAccountID:  l.DebitAccountID,
			CreditAccountID: l.CreditAccountID,
			AmountVND:       l.AmountVND,
			VendorID:        &cmd.VendorID,
			WarehouseID:     l.WarehouseID,
			ItemID:          &l.ItemID,
			InvoiceNo:       &cmd.InvoiceNo,
			InvoiceDate:     &cmd.InvoiceDate,
			Note:            l.Note,
		})
	}

	// (b) Deductible VAT line: Nợ 13311 / Có 331
	if pi.VATAmountVND.GreaterThan(decimal.Zero) {
		vatNote := fmt.Sprintf("Thuế GTGT đầu vào HĐ %s", cmd.InvoiceNo)
		vLines = append(vLines, glUseCase.CreateVoucherLineCommand{
			DebitAccountID:  cmd.VATAccountID,
			CreditAccountID: pi.Lines[0].CreditAccountID,
			AmountVND:       pi.VATAmountVND,
			VendorID:        &cmd.VendorID,
			InvoiceNo:       &cmd.InvoiceNo,
			InvoiceDate:     &cmd.InvoiceDate,
			Note:            vatNote,
		})
	}

	// 4. Delegate Master GL Voucher Creation
	vCmd := glUseCase.CreateVoucherCommand{
		CompanyProfileID: cmd.CompanyProfileID,
		BranchID:         cmd.BranchID,
		VoucherNo:        cmd.VoucherNo,
		VoucherDate:      cmd.VoucherDate,
		PostedDate:       cmd.PostedDate,
		VoucherType:      gl.VoucherTypePurchase,
		Description:      fmt.Sprintf("Mua hàng hóa theo HĐ số %s", cmd.InvoiceNo),
		IdempotencyKey:   cmd.IdempotencyKey,
		CreatedBy:        cmd.CreatedBy,
		Lines:            vLines,
	}

	voucher, err := u.glUseCase.CreateVoucher(ctx, vCmd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create master purchase voucher: %w", err)
	}

	// 5. Idempotent check
	existing, err := u.purchaseRepo.GetPurchaseInvoiceByVoucherID(ctx, voucher.ID)
	if err == nil && existing != nil {
		return existing, voucher, nil
	}

	// 6. Link voucher and persist PurchaseInvoice
	pi.VoucherID = voucher.ID
	if err := u.purchaseRepo.SavePurchaseInvoice(ctx, pi); err != nil {
		return nil, nil, fmt.Errorf("failed to persist purchase invoice: %w", err)
	}

	u.logger.Info(ctx, "purchase invoice created successfully",
		slog.String("entity_id", pi.ID),
		slog.String("voucher_id", voucher.ID),
		slog.String("voucher_no", voucher.VoucherNo),
		slog.String("amount", pi.TotalAmountVND.String()),
	)

	return pi, voucher, nil
}

func (u *PurchaseUseCase) GetPurchaseInvoice(ctx context.Context, id string) (*purchase.PurchaseInvoice, error) {
	pi, err := u.purchaseRepo.GetPurchaseInvoiceByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", purchase.ErrPurchaseInvoiceNotFound, id)
	}
	return pi, nil
}
