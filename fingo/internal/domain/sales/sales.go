package sales

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type EInvoiceStatus string

const (
	EInvoiceStatusDraft       EInvoiceStatus = "DRAFT"
	EInvoiceStatusSigned      EInvoiceStatus = "SIGNED"
	EInvoiceStatusCQTSent     EInvoiceStatus = "CQT_SENT"
	EInvoiceStatusCQTAccepted EInvoiceStatus = "CQT_ACCEPTED"
	EInvoiceStatusCQTRejected EInvoiceStatus = "CQT_REJECTED"
)

var (
	ErrMissingCustomerID   = errors.New("customer ID is required")
	ErrMissingInvoiceNo    = errors.New("invoice number is required")
	ErrInvalidLineQuantity = errors.New("line quantity must be greater than zero")
	ErrInvalidLineUnitPrice = errors.New("line unit price cannot be negative")
	ErrInvalidDiscountRate = errors.New("discount rate must be between 0% and 100%")
	ErrInvalidVATRate      = errors.New("invalid VAT rate (must be 0, 5, 8, 10, or -1 for exempt)")
	ErrZeroInvoiceLines    = errors.New("sales invoice must have at least one line item")
	ErrSalesInvoiceNotFound = errors.New("sales invoice not found")
)

type SalesInvoiceLine struct {
	ID                string
	SalesInvoiceID    string
	LineOrder         int
	ItemID            string
	WarehouseID       *string
	DebitAccountID    string
	CreditAccountID   string
	Quantity          decimal.Decimal
	UnitPriceVND      decimal.Decimal
	AmountVND         decimal.Decimal // Gross: Quantity * UnitPrice
	DiscountRate      decimal.Decimal
	DiscountAmountVND decimal.Decimal // Discount: AmountVND * DiscountRate / 100
	VATRate           decimal.Decimal
	VATAmountVND      decimal.Decimal // VAT: (AmountVND - DiscountAmountVND) * VATRate / 100
	Note              string
}

// NetAmount returns line gross amount minus discount amount.
func (l SalesInvoiceLine) NetAmount() decimal.Decimal {
	return l.AmountVND.Sub(l.DiscountAmountVND)
}

type SalesInvoice struct {
	ID                 string
	VoucherID          string
	CompanyProfileID   string
	CustomerID         string
	InvoiceTemplate    string
	InvoiceSeries      string
	InvoiceNo          string
	InvoiceDate        time.Time
	DueDate            time.Time
	PaymentMethod      string
	EInvoiceStatus     EInvoiceStatus
	EInvoiceCodeCQT    *string
	SubtotalVND        decimal.Decimal // Gross Subtotal
	DiscountVND        decimal.Decimal // Total Discount
	VATAmountVND       decimal.Decimal // Total Output VAT
	TotalAmountVND     decimal.Decimal // Net Subtotal + Total VAT
	IsStockOutwardAuto bool
	Lines              []SalesInvoiceLine
}

type CreateSalesInvoiceLineParams struct {
	LineOrder       int
	ItemID          string
	WarehouseID     *string
	DebitAccountID  string
	CreditAccountID string
	Quantity        decimal.Decimal
	UnitPriceVND    decimal.Decimal
	DiscountRate    decimal.Decimal
	VATRate         decimal.Decimal
	Note            string
}

type CreateSalesInvoiceParams struct {
	ID                 string
	VoucherID          string
	CompanyProfileID   string
	CustomerID         string
	InvoiceTemplate    string
	InvoiceSeries      string
	InvoiceNo          string
	InvoiceDate        time.Time
	DueDate            time.Time
	PaymentMethod      string
	IsStockOutwardAuto bool
	Lines              []CreateSalesInvoiceLineParams
}

func isValidVATRate(rate decimal.Decimal) bool {
	validRates := []int{-1, 0, 5, 8, 10}
	for _, r := range validRates {
		if rate.Equal(decimal.NewFromInt(int64(r))) {
			return true
		}
	}
	return false
}

func NewSalesInvoice(params CreateSalesInvoiceParams) (*SalesInvoice, error) {
	if strings.TrimSpace(params.CustomerID) == "" {
		return nil, ErrMissingCustomerID
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

	subtotalGross := decimal.Zero
	totalDiscount := decimal.Zero
	totalVAT := decimal.Zero
	domainLines := make([]SalesInvoiceLine, len(params.Lines))

	for i, l := range params.Lines {
		if l.Quantity.LessThanOrEqual(decimal.Zero) {
			return nil, ErrInvalidLineQuantity
		}
		if l.UnitPriceVND.LessThan(decimal.Zero) {
			return nil, ErrInvalidLineUnitPrice
		}
		if l.DiscountRate.LessThan(decimal.Zero) || l.DiscountRate.GreaterThan(decimal.NewFromInt(100)) {
			return nil, ErrInvalidDiscountRate
		}
		if !isValidVATRate(l.VATRate) {
			return nil, ErrInvalidVATRate
		}

		grossAmount := l.Quantity.Mul(l.UnitPriceVND).RoundBank(0)
		discountAmount := decimal.Zero
		if l.DiscountRate.GreaterThan(decimal.Zero) {
			discountAmount = grossAmount.Mul(l.DiscountRate).Div(decimal.NewFromInt(100)).RoundBank(0)
		}

		netAmount := grossAmount.Sub(discountAmount)
		lineVAT := decimal.Zero
		if l.VATRate.GreaterThan(decimal.Zero) {
			lineVAT = netAmount.Mul(l.VATRate).Div(decimal.NewFromInt(100)).RoundBank(0)
		}

		subtotalGross = subtotalGross.Add(grossAmount)
		totalDiscount = totalDiscount.Add(discountAmount)
		totalVAT = totalVAT.Add(lineVAT)

		order := l.LineOrder
		if order == 0 {
			order = i + 1
		}

		domainLines[i] = SalesInvoiceLine{
			ID:                uuid.New().String(),
			SalesInvoiceID:    invoiceID,
			LineOrder:         order,
			ItemID:            l.ItemID,
			WarehouseID:       l.WarehouseID,
			DebitAccountID:    l.DebitAccountID,
			CreditAccountID:   l.CreditAccountID,
			Quantity:          l.Quantity,
			UnitPriceVND:      l.UnitPriceVND.RoundBank(0),
			AmountVND:         grossAmount,
			DiscountRate:      l.DiscountRate,
			DiscountAmountVND: discountAmount,
			VATRate:           l.VATRate,
			VATAmountVND:      lineVAT,
			Note:              l.Note,
		}
	}

	template := params.InvoiceTemplate
	if template == "" {
		template = "1"
	}

	payMethod := params.PaymentMethod
	if payMethod == "" {
		payMethod = "CK"
	}

	netSubtotal := subtotalGross.Sub(totalDiscount)
	totalAmount := netSubtotal.Add(totalVAT)

	return &SalesInvoice{
		ID:                 invoiceID,
		VoucherID:          params.VoucherID,
		CompanyProfileID:   params.CompanyProfileID,
		CustomerID:         params.CustomerID,
		InvoiceTemplate:    template,
		InvoiceSeries:      params.InvoiceSeries,
		InvoiceNo:          params.InvoiceNo,
		InvoiceDate:        params.InvoiceDate,
		DueDate:            params.DueDate,
		PaymentMethod:      payMethod,
		EInvoiceStatus:     EInvoiceStatusDraft,
		SubtotalVND:        subtotalGross,
		DiscountVND:        totalDiscount,
		VATAmountVND:       totalVAT,
		TotalAmountVND:     totalAmount,
		IsStockOutwardAuto: params.IsStockOutwardAuto,
		Lines:              domainLines,
	}, nil
}

// SalesRepository defines database operations for the sales subledger.
type SalesRepository interface {
	SaveSalesInvoice(ctx context.Context, si *SalesInvoice) error
	GetSalesInvoiceByID(ctx context.Context, id string) (*SalesInvoice, error)
	GetSalesInvoiceByVoucherID(ctx context.Context, voucherID string) (*SalesInvoice, error)
	UpdateEInvoiceStatus(ctx context.Context, id string, status EInvoiceStatus, cqtCode *string) error
}

// SalesRepositoryStub provides an in-memory pure Go stub.
type SalesRepositoryStub struct {
	mu        sync.RWMutex
	invoices  map[string]*SalesInvoice
	byVoucher map[string]*SalesInvoice
}

func NewSalesRepositoryStub() *SalesRepositoryStub {
	return &SalesRepositoryStub{
		invoices:  make(map[string]*SalesInvoice),
		byVoucher: make(map[string]*SalesInvoice),
	}
}

func (s *SalesRepositoryStub) SaveSalesInvoice(ctx context.Context, si *SalesInvoice) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invoices[si.ID] = si
	s.byVoucher[si.VoucherID] = si
	return nil
}

func (s *SalesRepositoryStub) GetSalesInvoiceByID(ctx context.Context, id string) (*SalesInvoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	si, ok := s.invoices[id]
	if !ok {
		return nil, ErrSalesInvoiceNotFound
	}
	return si, nil
}

func (s *SalesRepositoryStub) GetSalesInvoiceByVoucherID(ctx context.Context, voucherID string) (*SalesInvoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	si, ok := s.byVoucher[voucherID]
	if !ok {
		return nil, ErrSalesInvoiceNotFound
	}
	return si, nil
}

func (s *SalesRepositoryStub) UpdateEInvoiceStatus(ctx context.Context, id string, status EInvoiceStatus, cqtCode *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	si, ok := s.invoices[id]
	if !ok {
		return ErrSalesInvoiceNotFound
	}
	si.EInvoiceStatus = status
	si.EInvoiceCodeCQT = cqtCode
	return nil
}
