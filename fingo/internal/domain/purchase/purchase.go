package purchase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PaymentStatus string

const (
	PaymentStatusUnpaid        PaymentStatus = "UNPAID"
	PaymentStatusPartiallyPaid PaymentStatus = "PARTIALLY_PAID"
	PaymentStatusPaid          PaymentStatus = "PAID"
)

var (
	ErrMissingVendorID         = errors.New("vendor ID is required")
	ErrMissingInvoiceNo        = errors.New("invoice number is required")
	ErrInvalidLineQuantity     = errors.New("line quantity must be greater than zero")
	ErrInvalidLineUnitPrice    = errors.New("line unit price cannot be negative")
	ErrInvalidVATRate          = errors.New("invalid VAT rate (must be 0, 5, 8, 10, or -1 for exempt)")
	ErrZeroInvoiceLines        = errors.New("purchase invoice must have at least one line item")
	ErrPurchaseInvoiceNotFound = errors.New("purchase invoice not found")
)

type PurchaseInvoiceLine struct {
	ID                string
	PurchaseInvoiceID string
	LineOrder         int
	ItemID            string
	WarehouseID       *string
	DebitAccountID    string
	CreditAccountID   string
	Quantity          decimal.Decimal
	UnitPriceVND      decimal.Decimal
	AmountVND         decimal.Decimal
	VATRate           decimal.Decimal
	VATAmountVND      decimal.Decimal
	Note              string
}

type PurchaseInvoice struct {
	ID                string
	VoucherID         string
	CompanyProfileID  string
	VendorID          string
	InvoiceTemplate   string
	InvoiceSeries     string
	InvoiceNo         string
	InvoiceDate       time.Time
	DueDate           time.Time
	PaymentStatus     PaymentStatus
	SubtotalVND       decimal.Decimal
	VATAmountVND      decimal.Decimal
	TotalAmountVND    decimal.Decimal
	PaidAmountVND     decimal.Decimal
	IsStockInwardAuto bool
	Lines             []PurchaseInvoiceLine
}

type CreatePurchaseInvoiceLineParams struct {
	LineOrder       int
	ItemID          string
	WarehouseID     *string
	DebitAccountID  string
	CreditAccountID string
	Quantity        decimal.Decimal
	UnitPriceVND    decimal.Decimal
	VATRate         decimal.Decimal
	Note            string
}

type CreatePurchaseInvoiceParams struct {
	ID                string
	VoucherID         string
	CompanyProfileID  string
	VendorID          string
	InvoiceTemplate   string
	InvoiceSeries     string
	InvoiceNo         string
	InvoiceDate       time.Time
	DueDate           time.Time
	IsStockInwardAuto bool
	Lines             []CreatePurchaseInvoiceLineParams
}

func isValidVATRate(rate decimal.Decimal) bool {
	// Statutory rates: -1 (exempt / không chịu thuế), 0%, 5%, 8% (Resolution 204/2025), 10%
	validRates := []int{-1, 0, 5, 8, 10}
	for _, r := range validRates {
		if rate.Equal(decimal.NewFromInt(int64(r))) {
			return true
		}
	}
	return false
}

func NewPurchaseInvoice(params CreatePurchaseInvoiceParams) (*PurchaseInvoice, error) {
	if strings.TrimSpace(params.VendorID) == "" {
		return nil, ErrMissingVendorID
	}
	if strings.TrimSpace(params.InvoiceNo) == "" {
		return nil, ErrMissingInvoiceNo
	}
	if len(params.Lines) == 0 {
		return nil, ErrZeroInvoiceLines
	}

	invoiceID := params.ID
	if invoiceID == "" {
		invoiceID = uuid.New().String()
	}

	subtotal := decimal.Zero
	totalVAT := decimal.Zero
	domainLines := make([]PurchaseInvoiceLine, len(params.Lines))

	for i, l := range params.Lines {
		if l.Quantity.LessThanOrEqual(decimal.Zero) {
			return nil, ErrInvalidLineQuantity
		}
		if l.UnitPriceVND.LessThan(decimal.Zero) {
			return nil, ErrInvalidLineUnitPrice
		}
		if !isValidVATRate(l.VATRate) {
			return nil, ErrInvalidVATRate
		}

		lineAmount := l.Quantity.Mul(l.UnitPriceVND).RoundBank(0)
		lineVAT := decimal.Zero
		if l.VATRate.GreaterThan(decimal.Zero) {
			lineVAT = lineAmount.Mul(l.VATRate).Div(decimal.NewFromInt(100)).RoundBank(0)
		}

		subtotal = subtotal.Add(lineAmount)
		totalVAT = totalVAT.Add(lineVAT)

		order := l.LineOrder
		if order == 0 {
			order = i + 1
		}

		domainLines[i] = PurchaseInvoiceLine{
			ID:                uuid.New().String(),
			PurchaseInvoiceID: invoiceID,
			LineOrder:         order,
			ItemID:            l.ItemID,
			WarehouseID:       l.WarehouseID,
			DebitAccountID:    l.DebitAccountID,
			CreditAccountID:   l.CreditAccountID,
			Quantity:          l.Quantity,
			UnitPriceVND:      l.UnitPriceVND.RoundBank(0),
			AmountVND:         lineAmount,
			VATRate:           l.VATRate,
			VATAmountVND:      lineVAT,
			Note:              l.Note,
		}
	}

	template := params.InvoiceTemplate
	if template == "" {
		template = "1"
	}

	return &PurchaseInvoice{
		ID:                invoiceID,
		VoucherID:         params.VoucherID,
		CompanyProfileID:  params.CompanyProfileID,
		VendorID:          params.VendorID,
		InvoiceTemplate:   template,
		InvoiceSeries:     params.InvoiceSeries,
		InvoiceNo:         params.InvoiceNo,
		InvoiceDate:       params.InvoiceDate,
		DueDate:           params.DueDate,
		PaymentStatus:     PaymentStatusUnpaid,
		SubtotalVND:       subtotal,
		VATAmountVND:      totalVAT,
		TotalAmountVND:    subtotal.Add(totalVAT),
		PaidAmountVND:     decimal.Zero,
		IsStockInwardAuto: params.IsStockInwardAuto,
		Lines:             domainLines,
	}, nil
}

// PurchaseRepository defines database operations for the purchase subledger.
type PurchaseRepository interface {
	SavePurchaseInvoice(ctx context.Context, pi *PurchaseInvoice) error
	GetPurchaseInvoiceByID(ctx context.Context, id string) (*PurchaseInvoice, error)
	GetPurchaseInvoiceByVoucherID(ctx context.Context, voucherID string) (*PurchaseInvoice, error)
	UpdatePaymentStatus(ctx context.Context, id string, status PaymentStatus, paidAmount decimal.Decimal) error
}

// PurchaseRepositoryStub provides an in-memory thread-safe pure Go stub.
type PurchaseRepositoryStub struct {
	mu        sync.RWMutex
	invoices  map[string]*PurchaseInvoice
	byVoucher map[string]*PurchaseInvoice
}

func NewPurchaseRepositoryStub() *PurchaseRepositoryStub {
	return &PurchaseRepositoryStub{
		invoices:  make(map[string]*PurchaseInvoice),
		byVoucher: make(map[string]*PurchaseInvoice),
	}
}

func (s *PurchaseRepositoryStub) SavePurchaseInvoice(ctx context.Context, pi *PurchaseInvoice) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invoices[pi.ID] = pi
	s.byVoucher[pi.VoucherID] = pi
	return nil
}

func (s *PurchaseRepositoryStub) GetPurchaseInvoiceByID(ctx context.Context, id string) (*PurchaseInvoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pi, ok := s.invoices[id]
	if !ok {
		return nil, ErrPurchaseInvoiceNotFound
	}
	return pi, nil
}

func (s *PurchaseRepositoryStub) GetPurchaseInvoiceByVoucherID(ctx context.Context, voucherID string) (*PurchaseInvoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pi, ok := s.byVoucher[voucherID]
	if !ok {
		return nil, ErrPurchaseInvoiceNotFound
	}
	return pi, nil
}

func (s *PurchaseRepositoryStub) UpdatePaymentStatus(ctx context.Context, id string, status PaymentStatus, paidAmount decimal.Decimal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pi, ok := s.invoices[id]
	if !ok {
		return ErrPurchaseInvoiceNotFound
	}
	pi.PaymentStatus = status
	pi.PaidAmountVND = paidAmount
	return nil
}
