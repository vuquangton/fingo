package documentstore_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"fingo/internal/adapter/mariadb/documentstore"
	"fingo/internal/adapter/mariadb/testsupport"
	"fingo/internal/document"
	"fingo/internal/money"
)

var vnd = money.Currency("VND")

func cents(amount int64) money.Money { return money.New(amount, vnd) }

func day(year int, month time.Month, dayOfMonth int) time.Time {
	return time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.UTC)
}

func at(year int, month time.Month, dayOfMonth int) time.Time {
	return time.Date(year, month, dayOfMonth, 9, 0, 0, 0, time.UTC)
}

func invoice(id, partnerID string, total int64) document.Document {
	return document.Document{
		ID:        id,
		Type:      document.TypeInvoice,
		PartnerID: partnerID,
		Date:      day(2026, 3, 1),
		PeriodID:  "fy2026",
		Total:     cents(total),
		Status:    document.StatusOpen,
	}
}

func payments(d document.Document) []document.Line {
	return []document.Line{
		{DocumentID: d.ID, AccountID: "131", Debit: d.Total, BaseAmount: d.Total},
		{DocumentID: d.ID, AccountID: "511", Credit: d.Total, BaseAmount: d.Total},
	}
}

// newStore opens a migrated scratch database. It skips when no .env or no server
// is reachable, so a green run without a database proves nothing about this
// file.
func newStore(t *testing.T) *documentstore.Store {
	t.Helper()

	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, _ := testsupport.MigratedDatabase(t, cfg, admin)

	return documentstore.NewStore(db)
}

func store(t *testing.T, book document.Repository, d document.Document) {
	t.Helper()

	if err := book.Create(context.Background(), d, payments(d)); err != nil {
		t.Fatalf("Create %s: %v", d.ID, err)
	}
}

// A document and its lines go in and come back, and the lines keep their order
// and their sides.
func TestStore_RoundTripsADocumentAndItsLines(t *testing.T) {
	book := newStore(t)
	ctx := context.Background()

	d := invoice("inv-1", "p-1", 100000)
	store(t, book, d)

	got, err := book.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Type != document.TypeInvoice || got.PartnerID != "p-1" || got.PeriodID != "fy2026" {
		t.Fatalf("round trip = %+v", got)
	}
	if got.Total.Amount() != 100000 || got.Total.Currency() != vnd {
		t.Errorf("total = %s, want 100000 VND", got.Total)
	}
	if got.Status != document.StatusOpen || !got.Allocated.IsZero() {
		t.Errorf("a fresh document = %+v, want it open and unallocated", got)
	}
	if !got.Date.Equal(day(2026, 3, 1)) {
		t.Errorf("date = %s, want 2026-03-01", got.Date)
	}

	lines, err := book.Lines(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Lines: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	if lines[0].AccountID != "131" || lines[0].Debit.Amount() != 100000 || !lines[0].Credit.IsZero() {
		t.Errorf("first line = %+v", lines[0])
	}
	if lines[0].Debit.Currency() != vnd {
		t.Errorf("line currency = %s, want the document's VND", lines[0].Debit.Currency())
	}
	if lines[1].AccountID != "511" || lines[1].Credit.Amount() != 100000 {
		t.Errorf("second line = %+v", lines[1])
	}

	listed, err := book.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("documents = %d, want 1", len(listed))
	}
}

func TestStore_FullPaymentMarksTheDocumentPaid(t *testing.T) {
	book := newStore(t)
	ctx := context.Background()
	d := invoice("inv-1", "p-1", 100000)
	store(t, book, d)

	result, err := book.Apply(ctx, document.Application{
		ID: "app-1", DocumentID: d.ID, PaymentID: "pay-1", Amount: cents(100000), At: at(2026, 3, 10),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Document.Status != document.StatusPaid {
		t.Errorf("status = %s, want paid", result.Document.Status)
	}

	stored, err := book.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Status != document.StatusPaid || stored.Allocated.Amount() != 100000 {
		t.Errorf("stored document = %+v, want it paid in full", stored)
	}

	allocations, err := book.Allocations(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Allocations: %v", err)
	}
	if len(allocations) != 1 || allocations[0].Amount.Amount() != 100000 {
		t.Fatalf("allocations = %+v, want one of 100000", allocations)
	}
}

func TestStore_PartialPaymentMarksTheDocumentPartiallyPaid(t *testing.T) {
	book := newStore(t)
	ctx := context.Background()
	d := invoice("inv-1", "p-1", 100000)
	store(t, book, d)

	result, err := book.Apply(ctx, document.Application{
		ID: "app-1", DocumentID: d.ID, PaymentID: "pay-1", Amount: cents(40000), At: at(2026, 3, 10),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Document.Status != document.StatusPartiallyPaid {
		t.Errorf("status = %s, want partially_paid", result.Document.Status)
	}

	stored, err := book.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Status != document.StatusPartiallyPaid || stored.Allocated.Amount() != 40000 {
		t.Errorf("stored document = %+v, want 40000 of 100000 allocated", stored)
	}
	if got := stored.Outstanding().Amount(); got != 60000 {
		t.Errorf("outstanding = %d, want 60000", got)
	}
}

func TestStore_OverpaymentCreatesACreditBalance(t *testing.T) {
	book := newStore(t)
	ctx := context.Background()
	d := invoice("inv-1", "p-1", 100000)
	store(t, book, d)

	result, err := book.Apply(ctx, document.Application{
		ID: "app-1", DocumentID: d.ID, PaymentID: "pay-1", Amount: cents(150000), At: at(2026, 3, 10),
	})
	if err != nil {
		t.Fatalf("an overpayment must not be refused: %v", err)
	}
	if result.Document.Status != document.StatusPaid {
		t.Errorf("status = %s, want paid", result.Document.Status)
	}
	if result.Allocation == nil || result.Allocation.Amount.Amount() != 100000 {
		t.Fatalf("allocation = %+v, want 100000 and no more", result.Allocation)
	}
	if result.Credit == nil || result.Credit.Amount.Amount() != 50000 {
		t.Fatalf("credit = %+v, want 50000", result.Credit)
	}

	credits, err := book.Credits(ctx, "p-1")
	if err != nil {
		t.Fatalf("Credits: %v", err)
	}
	if len(credits) != 1 {
		t.Fatalf("credits = %d, want 1", len(credits))
	}
	if credits[0].Amount.Amount() != 50000 || credits[0].Amount.Currency() != vnd {
		t.Errorf("credit balance = %s, want 50000 VND", credits[0].Amount)
	}
	if credits[0].PartnerID != "p-1" || credits[0].DocumentID != d.ID || credits[0].PaymentID != "pay-1" {
		t.Errorf("credit = %+v, want it to name the partner, the document and the payment", credits[0])
	}

	// Nothing else took the money: the document holds 100000 and no more.
	stored, err := book.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Allocated.Amount() != 100000 {
		t.Errorf("allocated = %s, want 100000", stored.Allocated)
	}
}

// Two payments arriving at once must not both spend the same outstanding
// balance. This is the test the row lock exists for: without SELECT ... FOR
// UPDATE inside the transaction, both transactions would read the same
// allocated amount and both would write over the other.
func TestStore_ConcurrentPaymentsDoNotOverAllocate(t *testing.T) {
	book := newStore(t)
	ctx := context.Background()
	d := invoice("inv-1", "p-1", 100000)
	store(t, book, d)

	const payments = 8
	const each = 25000

	results := make([]document.Result, payments)
	errs := make([]error, payments)

	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < payments; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = book.Apply(ctx, document.Application{
				ID:         fmt.Sprintf("app-%d", i),
				DocumentID: d.ID,
				PaymentID:  fmt.Sprintf("pay-%d", i),
				Amount:     cents(each),
				At:         at(2026, 3, 10),
			})
		}(i)
	}

	close(start)
	wg.Wait()

	var allocated, credited int64

	for i, err := range errs {
		if err != nil {
			t.Fatalf("payment %d: %v", i, err)
		}
		if r := results[i].Allocation; r != nil {
			allocated += r.Amount.Amount()
		}
		if r := results[i].Credit; r != nil {
			credited += r.Amount.Amount()
		}
	}

	if allocated != 100000 {
		t.Errorf("allocated in total = %d, want exactly the 100000 the document was worth", allocated)
	}
	if credited != 100000 {
		t.Errorf("credited in total = %d, want the 100000 no document could absorb", credited)
	}

	stored, err := book.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Allocated.Amount() != 100000 {
		t.Errorf("stored allocated = %s, want 100000 — the document was over-allocated", stored.Allocated)
	}
	if stored.Status != document.StatusPaid {
		t.Errorf("stored status = %s, want paid", stored.Status)
	}

	allocations, err := book.Allocations(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Allocations: %v", err)
	}
	if len(allocations) != 4 {
		t.Errorf("allocation rows = %d, want 4 of 25000 before the document was full", len(allocations))
	}

	credits, err := book.Credits(ctx, "p-1")
	if err != nil {
		t.Fatalf("Credits: %v", err)
	}
	if len(credits) != 4 {
		t.Errorf("credit rows = %d, want 4 of 25000", len(credits))
	}
}

// A payment applies to a document once. The unique key on
// (document_id, payment_id) is behind the check, so a repeat is refused even if
// it somehow reached the insert.
func TestStore_RefusesTheSamePaymentTwice(t *testing.T) {
	book := newStore(t)
	ctx := context.Background()
	d := invoice("inv-1", "p-1", 100000)
	store(t, book, d)

	app := document.Application{
		ID: "app-1", DocumentID: d.ID, PaymentID: "pay-1", Amount: cents(40000), At: at(2026, 3, 10),
	}
	if _, err := book.Apply(ctx, app); err != nil {
		t.Fatalf("first Apply: %v", err)
	}

	app.ID = "app-1-retry"
	if _, err := book.Apply(ctx, app); !errors.Is(err, document.ErrAlreadyApplied) {
		t.Fatalf("second Apply = %v, want ErrAlreadyApplied", err)
	}

	stored, err := book.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Allocated.Amount() != 40000 {
		t.Errorf("allocated = %s after a refused retry, want 40000", stored.Allocated)
	}
}

// A voided document is owed to nobody, and one that has been settled may not be
// voided at all.
func TestStore_VoidRules(t *testing.T) {
	book := newStore(t)
	ctx := context.Background()

	open := invoice("inv-1", "p-1", 100000)
	store(t, book, open)

	if err := book.Void(ctx, "inv-1"); err != nil {
		t.Fatalf("Void: %v", err)
	}

	voided, err := book.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if voided.Status != document.StatusVoid {
		t.Errorf("status = %s, want void", voided.Status)
	}

	// A voided document may not be settled.
	if _, err := book.Apply(ctx, document.Application{
		ID: "app-1", DocumentID: "inv-1", PaymentID: "pay-1", Amount: cents(1000), At: at(2026, 3, 10),
	}); !errors.Is(err, document.ErrVoided) {
		t.Fatalf("Apply against a voided document = %v, want ErrVoided", err)
	}

	// A document that has taken money may not be voided.
	settled := invoice("inv-2", "p-1", 100000)
	store(t, book, settled)

	if _, err := book.Apply(ctx, document.Application{
		ID: "app-2", DocumentID: "inv-2", PaymentID: "pay-2", Amount: cents(40000), At: at(2026, 3, 10),
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := book.Void(ctx, "inv-2"); !errors.Is(err, document.ErrVoidNotAllowed) {
		t.Fatalf("Void of an allocated document = %v, want ErrVoidNotAllowed", err)
	}
}

// A document that could not be stored at all never reaches the table.
func TestStore_RefusesADocumentItCannotValidate(t *testing.T) {
	book := newStore(t)
	ctx := context.Background()

	settled := invoice("inv-1", "p-1", 100000)
	settled.Allocated = cents(100000)
	settled.Status = document.StatusPaid

	if err := book.Create(ctx, settled, payments(settled)); !errors.Is(err, document.ErrInvalidDocument) {
		t.Fatalf("Create of a settled document = %v, want ErrInvalidDocument", err)
	}

	d := invoice("inv-2", "p-1", 100000)
	unbalanced := []document.Line{
		{DocumentID: d.ID, AccountID: "131", Debit: cents(100000)},
		{DocumentID: d.ID, AccountID: "511", Credit: cents(99999)},
	}
	if err := book.Create(ctx, d, unbalanced); !errors.Is(err, document.ErrUnbalanced) {
		t.Fatalf("Create with unbalanced lines = %v, want ErrUnbalanced", err)
	}

	if _, err := book.Get(ctx, "inv-2"); !errors.Is(err, document.ErrUnknownDocument) {
		t.Fatalf("Get after a refused create = %v, want ErrUnknownDocument", err)
	}
}

// Acting on a document that is not there is reported, not silently ignored.
func TestStore_ReportsUnknownDocuments(t *testing.T) {
	book := newStore(t)
	ctx := context.Background()

	if _, err := book.Get(ctx, "nope"); !errors.Is(err, document.ErrUnknownDocument) {
		t.Errorf("Get = %v, want ErrUnknownDocument", err)
	}
	if _, err := book.Lines(ctx, "nope"); !errors.Is(err, document.ErrUnknownDocument) {
		t.Errorf("Lines = %v, want ErrUnknownDocument", err)
	}
	if err := book.Void(ctx, "nope"); !errors.Is(err, document.ErrUnknownDocument) {
		t.Errorf("Void = %v, want ErrUnknownDocument", err)
	}
	if _, err := book.Apply(ctx, document.Application{
		ID: "app-1", DocumentID: "nope", PaymentID: "pay-1", Amount: cents(1000), At: at(2026, 3, 10),
	}); !errors.Is(err, document.ErrUnknownDocument) {
		t.Errorf("Apply = %v, want ErrUnknownDocument", err)
	}
}

// The same id twice is refused, and the first document is left alone.
func TestStore_RefusesToCreateTheSameIDTwice(t *testing.T) {
	book := newStore(t)
	ctx := context.Background()
	store(t, book, invoice("inv-1", "p-1", 100000))

	err := book.Create(ctx, invoice("inv-1", "p-1", 50000), payments(invoice("inv-1", "p-1", 50000)))
	if !errors.Is(err, document.ErrDocumentExists) {
		t.Fatalf("Create of a taken id = %v, want ErrDocumentExists", err)
	}

	stored, err := book.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Total.Amount() != 100000 {
		t.Errorf("total = %s after a refused create, want the original 100000", stored.Total)
	}
}
