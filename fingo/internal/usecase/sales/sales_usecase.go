package sales

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
	"fingo/internal/domain/sales"
	glUseCase "fingo/internal/usecase/gl"
	"fingo/pkg/logger"
)

var (
	ErrVATAccountRequired = errors.New("output VAT account ID (33311) is required when invoice has VAT amount")
)

type SalesUseCase struct {
	salesRepo sales.SalesRepository
	glUseCase *glUseCase.GLUseCase
	logger    *logger.Logger
}

func NewSalesUseCase(
	salesRepo sales.SalesRepository,
	glUC *glUseCase.GLUseCase,
	log *logger.Logger,
) *SalesUseCase {
	return &SalesUseCase{
		salesRepo: salesRepo,
		glUseCase: glUC,
		logger:    log,
	}
}

type CreateSalesInvoiceLineCommand struct {
	LineOrder       int
	ItemID          string
	WarehouseID     *string
	DebitAccountID  string // e.g. 131
	CreditAccountID string // e.g. 5111, 5112
	Quantity        decimal.Decimal
	UnitPriceVND    decimal.Decimal
	DiscountRate    decimal.Decimal
	VATRate         decimal.Decimal
	Note            string
}

type CreateSalesInvoiceCommand struct {
	CompanyProfileID   string
	BranchID           *string
	VoucherNo          string
	VoucherDate        time.Time
	PostedDate         time.Time
	CustomerID         string
	InvoiceTemplate    string
	InvoiceSeries      string
	InvoiceNo          string
	InvoiceDate        time.Time
	DueDate            time.Time
	PaymentMethod      string
	VATAccountID       string // e.g. 33311 for output VAT
	IsStockOutwardAuto bool
	Lines              []CreateSalesInvoiceLineCommand
	IdempotencyKey     *string
	CreatedBy          string
}

func (u *SalesUseCase) CreateSalesInvoice(ctx context.Context, cmd CreateSalesInvoiceCommand) (*sales.SalesInvoice, *gl.Voucher, error) {
	// 1. Construct and validate domain SalesInvoice
	domainLineParams := make([]sales.CreateSalesInvoiceLineParams, len(cmd.Lines))
	for i, l := range cmd.Lines {
		domainLineParams[i] = sales.CreateSalesInvoiceLineParams{
			LineOrder:       l.LineOrder,
			ItemID:          l.ItemID,
			WarehouseID:     l.WarehouseID,
			DebitAccountID:  l.DebitAccountID,
			CreditAccountID: l.CreditAccountID,
			Quantity:        l.Quantity,
			UnitPriceVND:    l.UnitPriceVND,
			DiscountRate:    l.DiscountRate,
			VATRate:         l.VATRate,
			Note:            l.Note,
		}
	}

	siID := uuid.New().String()
	si, err := sales.NewSalesInvoice(sales.CreateSalesInvoiceParams{
		ID:                 siID,
		VoucherID:          "pre-validation",
		CompanyProfileID:   cmd.CompanyProfileID,
		CustomerID:         cmd.CustomerID,
		InvoiceTemplate:    cmd.InvoiceTemplate,
		InvoiceSeries:      cmd.InvoiceSeries,
		InvoiceNo:          cmd.InvoiceNo,
		InvoiceDate:        cmd.InvoiceDate,
		DueDate:            cmd.DueDate,
		PaymentMethod:      cmd.PaymentMethod,
		IsStockOutwardAuto: cmd.IsStockOutwardAuto,
		Lines:              domainLineParams,
	})
	if err != nil {
		return nil, nil, err
	}

	// 2. Validate VAT account if invoice has output VAT
	if si.VATAmountVND.GreaterThan(decimal.Zero) && strings.TrimSpace(cmd.VATAccountID) == "" {
		return nil, nil, ErrVATAccountRequired
	}

	// 3. Construct Voucher Lines
	var vLines []glUseCase.CreateVoucherLineCommand

	// (a) Revenue lines: Nợ 131 / Có 511x (ghi nhận doanh thu thuần sau chiết khấu)
	for _, l := range si.Lines {
		vLines = append(vLines, glUseCase.CreateVoucherLineCommand{
			DebitAccountID:  l.DebitAccountID,
			CreditAccountID: l.CreditAccountID,
			AmountVND:       l.NetAmount(),
			CustomerID:      &cmd.CustomerID,
			WarehouseID:     l.WarehouseID,
			ItemID:          &l.ItemID,
			InvoiceNo:       &cmd.InvoiceNo,
			InvoiceDate:     &cmd.InvoiceDate,
			Note:            l.Note,
		})
	}

	// (b) Output VAT line: Nợ 131 / Có 33311
	if si.VATAmountVND.GreaterThan(decimal.Zero) {
		vatNote := fmt.Sprintf("Thuế GTGT đầu ra HĐ %s", cmd.InvoiceNo)
		vLines = append(vLines, glUseCase.CreateVoucherLineCommand{
			DebitAccountID:  si.Lines[0].DebitAccountID,
			CreditAccountID: cmd.VATAccountID,
			AmountVND:       si.VATAmountVND,
			CustomerID:      &cmd.CustomerID,
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
		VoucherType:      gl.VoucherTypeSales,
		Description:      fmt.Sprintf("Bán hàng theo HĐ số %s", cmd.InvoiceNo),
		IdempotencyKey:   cmd.IdempotencyKey,
		CreatedBy:        cmd.CreatedBy,
		Lines:            vLines,
	}

	voucher, err := u.glUseCase.CreateVoucher(ctx, vCmd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create master sales voucher: %w", err)
	}

	// 5. Idempotent check
	existing, err := u.salesRepo.GetSalesInvoiceByVoucherID(ctx, voucher.ID)
	if err == nil && existing != nil {
		return existing, voucher, nil
	}

	// 6. Link voucher and persist SalesInvoice
	si.VoucherID = voucher.ID
	if err := u.salesRepo.SaveSalesInvoice(ctx, si); err != nil {
		return nil, nil, fmt.Errorf("failed to persist sales invoice: %w", err)
	}

	u.logger.Info(ctx, "sales invoice created successfully",
		slog.String("entity_id", si.ID),
		slog.String("voucher_id", voucher.ID),
		slog.String("voucher_no", voucher.VoucherNo),
		slog.String("amount", si.TotalAmountVND.String()),
	)

	return si, voucher, nil
}

func (u *SalesUseCase) GetSalesInvoice(ctx context.Context, id string) (*sales.SalesInvoice, error) {
	si, err := u.salesRepo.GetSalesInvoiceByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", sales.ErrSalesInvoiceNotFound, id)
	}
	return si, nil
}
