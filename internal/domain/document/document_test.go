package document_test

import (
	"testing"
	"time"

	"fingo/internal/domain/document"
	"fingo/internal/money"
)

func on(day string) time.Time {
	parsed, err := time.Parse("2006-01-02", day)
	if err != nil {
		panic(err)
	}
	return parsed
}

func invoice(id, reference, day string, amount int64) document.Open {
	return document.Open{
		Document: document.Document{
			ID:           id,
			Kind:         document.KindSalesInvoice,
			Reference:    reference,
			DocumentDate: on(day),
		},
		Outstanding: money.FromInt(amount),
	}
}

// A payment settles the oldest debt first. That is what a customer expects and
// what an aging report assumes, so it is the rule rather than a choice.
func TestPlanAllocation_SettlesOldestFirst(t *testing.T) {
	open := []document.Open{
		invoice("d-mar", "HD003", "2026-03-01", 300),
		invoice("d-jan", "HD001", "2026-01-01", 100),
		invoice("d-feb", "HD002", "2026-02-01", 200),
	}

	plan, err := document.PlanAllocation(money.FromInt(150), open)
	if err != nil {
		t.Fatalf("PlanAllocation: %v", err)
	}

	if len(plan.Allocations) != 2 {
		t.Fatalf("allocations = %v, want the January invoice settled and February part-settled", plan.Allocations)
	}
	if plan.Allocations[0].DocumentID != "d-jan" || !plan.Allocations[0].Amount.Equal(money.FromInt(100)) {
		t.Errorf("first allocation = %v, want all of d-jan (100)", plan.Allocations[0])
	}
	if plan.Allocations[1].DocumentID != "d-feb" || !plan.Allocations[1].Amount.Equal(money.FromInt(50)) {
		t.Errorf("second allocation = %v, want 50 of d-feb", plan.Allocations[1])
	}
	if !plan.Unapplied.IsZero() {
		t.Errorf("Unapplied = %s, want 0", plan.Unapplied)
	}
}

// An overpayment is a real position — the customer is in credit — so the
// remainder is reported rather than absorbed into an invoice that does not owe it.
func TestPlanAllocation_LeavesAnOverpaymentUnapplied(t *testing.T) {
	open := []document.Open{
		invoice("d-jan", "HD001", "2026-01-01", 100),
		invoice("d-feb", "HD002", "2026-02-01", 200),
	}

	plan, err := document.PlanAllocation(money.FromInt(500), open)
	if err != nil {
		t.Fatalf("PlanAllocation: %v", err)
	}

	if len(plan.Allocations) != 2 {
		t.Fatalf("allocations = %v, want both invoices settled", plan.Allocations)
	}
	if !plan.Unapplied.Equal(money.FromInt(200)) {
		t.Errorf("Unapplied = %s, want 200", plan.Unapplied)
	}
}

// Never allocate more than a document owes. Paying a 100 invoice with 500 must
// not turn it into a 400 credit against that invoice.
func TestPlanAllocation_NeverExceedsWhatADocumentOwes(t *testing.T) {
	open := []document.Open{invoice("d-jan", "HD001", "2026-01-01", 100)}

	plan, err := document.PlanAllocation(money.FromInt(500), open)
	if err != nil {
		t.Fatalf("PlanAllocation: %v", err)
	}

	if len(plan.Allocations) != 1 {
		t.Fatalf("allocations = %v, want one", plan.Allocations)
	}
	if !plan.Allocations[0].Amount.Equal(money.FromInt(100)) {
		t.Errorf("allocation = %s, want exactly 100", plan.Allocations[0].Amount)
	}
}

// A document already settled is not a target. Allocating to it again would
// double-count the payment.
func TestPlanAllocation_SkipsSettledDocuments(t *testing.T) {
	open := []document.Open{
		invoice("d-settled", "HD001", "2026-01-01", 0),
		invoice("d-open", "HD002", "2026-02-01", 200),
	}

	plan, err := document.PlanAllocation(money.FromInt(50), open)
	if err != nil {
		t.Fatalf("PlanAllocation: %v", err)
	}

	if len(plan.Allocations) != 1 || plan.Allocations[0].DocumentID != "d-open" {
		t.Fatalf("allocations = %v, want only d-open", plan.Allocations)
	}
}

// The parts must sum to exactly what was applied. A đồng lost here is a
// receivable that never clears; a đồng invented is one that clears twice.
func TestPlanAllocation_AppliesExactlyWhatItAllocates(t *testing.T) {
	cases := []struct {
		payment int64
		amounts []int64
	}{
		{payment: 1, amounts: []int64{1}},
		{payment: 99, amounts: []int64{100}},
		{payment: 300, amounts: []int64{100, 200}},
		{payment: 7, amounts: []int64{3, 5, 11}},
		{payment: 1000, amounts: []int64{1, 2, 3}},
	}

	for _, c := range cases {
		var open []document.Open
		for i, amount := range c.amounts {
			open = append(open, invoice(
				"d-"+string(rune('a'+i)), "HD00"+string(rune('1'+i)), "2026-01-01", amount))
		}

		payment := money.FromInt(c.payment)

		plan, err := document.PlanAllocation(payment, open)
		if err != nil {
			t.Fatalf("payment %d: %v", c.payment, err)
		}

		applied := money.Zero
		for _, a := range plan.Allocations {
			applied = applied.Add(a.Amount)
		}
		applied = applied.Add(plan.Unapplied)

		if !applied.Equal(payment) {
			t.Errorf("payment %d: allocated %s plus unapplied %s = %s, want %s",
				c.payment, applied.Sub(plan.Unapplied), plan.Unapplied, applied, payment)
		}
	}
}

// A payment of nothing settles nothing, and a negative one is a mistake rather
// than a credit note.
func TestPlanAllocation_RefusesANonPositivePayment(t *testing.T) {
	open := []document.Open{invoice("d-jan", "HD001", "2026-01-01", 100)}

	for _, payment := range []money.Dong{money.Zero, money.FromInt(-50)} {
		if _, err := document.PlanAllocation(payment, open); err == nil {
			t.Errorf("a payment of %s must be refused, got nil", payment)
		}
	}
}

// Nothing open means nothing to settle, which is not an error: the payment
// simply stays unapplied and the customer sits in credit.
func TestPlanAllocation_WithNothingOpen(t *testing.T) {
	plan, err := document.PlanAllocation(money.FromInt(100), nil)
	if err != nil {
		t.Fatalf("PlanAllocation: %v", err)
	}
	if len(plan.Allocations) != 0 {
		t.Errorf("allocations = %v, want none", plan.Allocations)
	}
	if !plan.Unapplied.Equal(money.FromInt(100)) {
		t.Errorf("Unapplied = %s, want the whole payment", plan.Unapplied)
	}
}
