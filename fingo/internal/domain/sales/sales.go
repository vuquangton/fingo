package sales

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

type SalesInvoiceLine struct {
	ID        string
	InvoiceID string
	ItemID    string
	Quantity  decimal.Decimal
	UnitPrice decimal.Decimal
	VATRate   decimal.Decimal // 0, 5, 8, 10
}

func (l SalesInvoiceLine) Subtotal() decimal.Decimal {
	return l.Quantity.Mul(l.UnitPrice)
}

func (l SalesInvoiceLine) VATAmount() decimal.Decimal {
	rate := l.VATRate.Div(decimal.NewFromInt(100))
	return l.Subtotal().Mul(rate)
}

func (l SalesInvoiceLine) Total() decimal.Decimal {
	return l.Subtotal().Add(l.VATAmount())
}

type SalesInvoice struct {
	ID          string
	InvoiceNo   string
	CustomerID  string
	InvoiceDate time.Time
	DueDate     time.Time
	Lines       []SalesInvoiceLine
	IsPosted    bool
}

func NewSalesInvoiceStub(id, invoiceNo, customerID string, date time.Time) *SalesInvoice {
	return &SalesInvoice{
		ID:          id,
		InvoiceNo:   invoiceNo,
		CustomerID:  customerID,
		InvoiceDate: date,
		DueDate:     date.AddDate(0, 0, 30),
		Lines:       make([]SalesInvoiceLine, 0),
		IsPosted:    false,
	}
}

func (s *SalesInvoice) AddLine(line SalesInvoiceLine) {
	s.Lines = append(s.Lines, line)
}

func (s *SalesInvoice) CalculateTotal() decimal.Decimal {
	total := decimal.Zero
	for _, l := range s.Lines {
		total = total.Add(l.Total())
	}
	return total
}

type SalesRepositoryStub interface {
	SaveInvoice(ctx context.Context, inv *SalesInvoice) error
	GetInvoice(ctx context.Context, id string) (*SalesInvoice, error)
}
