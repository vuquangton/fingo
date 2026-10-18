package close

import (
	"context"
	"testing"
	"time"

	"fingo/internal/document"
	"fingo/internal/money"
)

// What holds a period open is an invoice somebody is still waiting to be paid
// for. A document in another period, a settled one, a voided one and a document
// the customer has only part-paid do not hold it open — the last of those still
// being owed.
func TestRepositorySource_ReturnsOnlyWhatIsStillOwedInThePeriod(t *testing.T) {
	ctx := context.Background()
	book := document.NewMemoryRepository()

	createInvoice(ctx, t, book, "inv-1", "2026-01", 5_000_000)
	createInvoice(ctx, t, book, "inv-2", "2026-02", 7_000_000)
	createInvoice(ctx, t, book, "inv-3", "2026-01", 3_000_000)
	createInvoice(ctx, t, book, "inv-4", "2026-01", 1_000_000)
	createInvoice(ctx, t, book, "inv-5", "2026-01", 2_000_000)

	// inv-3 is settled in full, inv-4 is voided: neither is still owed. inv-5 is
	// part-paid, so it still is.
	if _, err := book.Apply(ctx, document.Application{
		ID:         "pay-1",
		DocumentID: "inv-3",
		PaymentID:  "receipt-1",
		Amount:     money.New(3_000_000, money.Currency("VND")),
		At:         time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("settle inv-3: %v", err)
	}
	if _, err := book.Apply(ctx, document.Application{
		ID:         "pay-2",
		DocumentID: "inv-5",
		PaymentID:  "receipt-2",
		Amount:     money.New(500_000, money.Currency("VND")),
		At:         time.Date(2026, 2, 4, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("part-pay inv-5: %v", err)
	}
	if err := book.Void(ctx, "inv-4"); err != nil {
		t.Fatalf("void inv-4: %v", err)
	}

	open, err := NewRepositorySource(book).OpenInPeriod(ctx, "2026-01")
	if err != nil {
		t.Fatalf("OpenInPeriod: %v", err)
	}

	if len(open) != 2 {
		t.Fatalf("open documents = %+v, want inv-1 and the part-paid inv-5", open)
	}
	for i, want := range []string{"inv-1", "inv-5"} {
		if open[i].ID != want {
			t.Errorf("open document %d = %s, want %s", i+1, open[i].ID, want)
		}
	}
}

// createInvoice writes one open invoice into an AR book: what the customer owes,
// and the revenue it is owed for.
func createInvoice(ctx context.Context, t *testing.T, book *document.MemoryRepository, id, periodID string, total int64) {
	t.Helper()

	vnd := money.Currency("VND")
	doc := document.Document{
		ID:        id,
		Type:      document.TypeInvoice,
		PartnerID: "cust-1",
		Date:      januaryStart,
		PeriodID:  periodID,
		Total:     money.New(total, vnd),
		Allocated: money.New(0, vnd),
		Status:    document.StatusOpen,
	}

	lines := []document.Line{
		{DocumentID: id, AccountID: "131", Debit: money.New(total, vnd)},
		{DocumentID: id, AccountID: "5111", Credit: money.New(total, vnd)},
	}

	if err := book.Create(ctx, doc, lines); err != nil {
		t.Fatalf("create %s: %v", id, err)
	}
}
