package trading_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"fingo/internal/money"
	"fingo/internal/trading"
)

// Nothing in this package may touch binary floating point.
//
// The behaviour tests can only check the arithmetic they happen to exercise. A
// float is a property of the source: one `float64` anywhere on the pricing path
// puts a representation error into every figure that passes through it, and the
// test that would have caught it is the one nobody thought to write. So the
// package is read and asked directly.
func TestPackage_UsesNoFloatingPoint(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list the package source: %v", err)
	}

	scanned := 0

	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		scanned++

		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.Ident:
				if n.Name == "float32" || n.Name == "float64" {
					t.Errorf("%s uses %s; every amount here is a whole number of minor units", path, n.Name)
				}
			case *ast.BasicLit:
				if n.Kind == token.FLOAT {
					t.Errorf("%s holds the floating-point literal %s", path, n.Value)
				}
			}

			return true
		})
	}

	if scanned == 0 {
		t.Fatal("no source files were scanned, so this test proves nothing")
	}
}

// An amount a float64 cannot hold survives the whole flow, from the price on the
// line to the figure on the voucher.
//
// 2^53 + 1 is the smallest integer a float64 rounds away. A flow that had priced
// a line in one would invoice 9007199254740992 and never know.
func TestFlow_AmountsAreIntegerMinorUnitsAllTheWayThrough(t *testing.T) {
	const beyond = int64(1)<<53 + 1 // 9007199254740993

	flow, poster, receivables := service(t)
	ctx := context.Background()

	invoice, err := flow.Invoice(ctx, invoiceCmd("inv-1", line("item-1", 1, beyond, "vat-0")))
	if err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	if invoice.Base.Amount() != beyond {
		t.Fatalf("base = %d, want %d exactly — a float64 would have rounded it", invoice.Base.Amount(), beyond)
	}
	if invoice.Total.Amount() != beyond {
		t.Errorf("total = %d, want %d", invoice.Total.Amount(), beyond)
	}

	// And the ledger is handed the same figure, not an approximation of it.
	receivable := lineFor(t, poster.voucher(t, "invoice-inv-1"), "131")
	if receivable.Amount.String() != "9007199254740993" {
		t.Errorf("the voucher was written for %s, want the exact amount", receivable.Amount)
	}

	recorded, err := receivables.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if recorded.Total.Amount() != beyond {
		t.Errorf("the receivable is for %d, want %d", recorded.Total.Amount(), beyond)
	}
}

// Quantities and prices multiply exactly, and the VAT is an integer share of the
// product.
func TestFlow_TheTotalsAddUpToTheMinorUnit(t *testing.T) {
	flow, _, _ := service(t)
	ctx := context.Background()

	// 3 × 1,000,001 at 10%: 3,000,003 and 300,000 (the tax truncates from
	// 300,000.3 rather than rounding to 300,000).
	invoice, err := flow.Invoice(ctx, invoiceCmd("inv-1", line("item-1", 3, 1_000_001, "vat-10")))
	if err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	if invoice.Base.Amount() != 3_000_003 {
		t.Errorf("base = %d, want 3 × 1,000,001", invoice.Base.Amount())
	}
	if invoice.VAT.Amount() != 300_000 {
		t.Errorf("VAT = %d, want 300,000 — the tax truncates, it does not round up", invoice.VAT.Amount())
	}
	if invoice.Total.Amount() != 3_300_003 {
		t.Errorf("total = %d, want 3,300,003", invoice.Total.Amount())
	}
	if invoice.Base.Amount()+invoice.VAT.Amount() != invoice.Total.Amount() {
		t.Error("the invoice total is not its base plus its VAT")
	}
}

// The flow's own seam is exercised through the money type, not a bare int: the
// payment carries a currency, and a payment in the wrong one is refused before
// anything is posted.
func TestFlow_RefusesAPaymentInAnotherCurrency(t *testing.T) {
	flow, poster, _ := service(t)
	ctx := context.Background()

	if _, err := flow.Invoice(ctx, invoiceCmd("inv-1", line("item-1", 1, 100, "vat-10"))); err != nil {
		t.Fatalf("Invoice: %v", err)
	}

	before := len(poster.posted())

	_, err := flow.Pay(ctx, trading.PaymentCommand{
		PaymentID: "pay-1", InvoiceID: "inv-1",
		Amount:  money.New(110, money.Currency("USD")),
		Account: "112",
		Date:    paymentDate, At: postedAt, By: "operator-1",
	})
	if err == nil {
		t.Fatal("a payment in another currency must be refused")
	}

	if got := len(poster.posted()); got != before {
		t.Errorf("a refused payment posted a voucher: %d, want %d", got, before)
	}
}
