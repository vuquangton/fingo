package sales_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/sales"
)

func TestSalesInvoice_TotalCalculation(t *testing.T) {
	inv := sales.NewSalesInvoiceStub("inv-1", "HD0001", "KH01", time.Now())
	inv.AddLine(sales.SalesInvoiceLine{
		ID:        "l-1",
		ItemID:    "VT01",
		Quantity:  decimal.NewFromInt(2),
		UnitPrice: decimal.NewFromInt(100000),
		VATRate:   decimal.NewFromInt(10),
	})

	total := inv.CalculateTotal()
	expected := decimal.NewFromInt(220000) // 200,000 + 10% VAT = 220,000

	if total.Cmp(expected) != 0 {
		t.Fatalf("expected total %s, got %s", expected, total)
	}
}
