package purchase

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

type PurchaseInvoiceLine struct {
	ID        string
	InvoiceID string
	ItemID    string
	Quantity  decimal.Decimal
	UnitPrice decimal.Decimal
	VATRate   decimal.Decimal
}

func (l PurchaseInvoiceLine) Subtotal() decimal.Decimal {
	return l.Quantity.Mul(l.UnitPrice)
}

type PurchaseInvoice struct {
	ID          string
	InvoiceNo   string
	VendorID    string
	InvoiceDate time.Time
	Lines       []PurchaseInvoiceLine
	IsPosted    bool
}

func NewPurchaseInvoiceStub(id, invoiceNo, vendorID string, date time.Time) *PurchaseInvoice {
	return &PurchaseInvoice{
		ID:          id,
		InvoiceNo:   invoiceNo,
		VendorID:    vendorID,
		InvoiceDate: date,
		Lines:       make([]PurchaseInvoiceLine, 0),
		IsPosted:    false,
	}
}

func (p *PurchaseInvoice) AddLine(line PurchaseInvoiceLine) {
	p.Lines = append(p.Lines, line)
}

func (p *PurchaseInvoice) CalculateSubtotal() decimal.Decimal {
	total := decimal.Zero
	for _, l := range p.Lines {
		total = total.Add(l.Subtotal())
	}
	return total
}

type PurchaseRepositoryStub interface {
	SavePurchaseInvoice(ctx context.Context, inv *PurchaseInvoice) error
}
