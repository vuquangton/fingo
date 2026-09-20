package tax_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/tax"
)

func TestVATDeclaration_TaxPayableCalculation(t *testing.T) {
	decl := tax.NewVATDeclarationStub("tax-2026-Q1", 2026, 1, tax.PeriodQuarter)

	// Output tax (33311) = 50,000,000
	decl.OutputVAT = decimal.NewFromInt(50000000)
	// Deductible input tax (1331) = 30,000,000
	decl.DeductibleInputVAT = decimal.NewFromInt(30000000)

	payable, carryForward := decl.CalculateTaxPosition()

	if payable.Cmp(decimal.NewFromInt(20000000)) != 0 {
		t.Fatalf("expected tax payable 20,000,000, got %s", payable)
	}
	if !carryForward.IsZero() {
		t.Fatalf("expected zero carry forward, got %s", carryForward)
	}
}

func TestEInvoice_StubInvariants(t *testing.T) {
	einv := tax.NewEInvoiceStub("e-1", "1/001", "C26TAV", "00000123", time.Now())
	if einv.Series != "C26TAV" || einv.Status != tax.EInvoiceStatusDraft {
		t.Fatalf("unexpected einvoice stub: %+v", einv)
	}
}
