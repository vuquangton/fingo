package purchase_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/purchase"
)

func TestPurchaseInvoice_StubInvariants(t *testing.T) {
	pi := purchase.NewPurchaseInvoiceStub("pi-01", "HDM001", "NCC01", time.Now())
	pi.AddLine(purchase.PurchaseInvoiceLine{
		ID:        "pl-1",
		ItemID:    "VT01",
		Quantity:  decimal.NewFromInt(10),
		UnitPrice: decimal.NewFromInt(50000),
	})

	if pi.CalculateSubtotal().Cmp(decimal.NewFromInt(500000)) != 0 {
		t.Fatalf("expected subtotal 500,000, got %s", pi.CalculateSubtotal())
	}
}
