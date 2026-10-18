package document_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"fingo/internal/document"
	"fingo/internal/money"
)

// A document paid in full is paid, in the stored row and not merely in the
// result the caller was handed.
func TestApply_FullPaymentMarksTheDocumentPaid(t *testing.T) {
	ctx := context.Background()
	book := document.NewMemoryRepository()
	d := invoice("inv-1", "p-1", 100000)
	store(t, book, d)

	result, err := book.Apply(ctx, document.Application{
		ID: "app-1", DocumentID: d.ID, PaymentID: "pay-1",
		Amount: cents(100000), At: at(2026, 3, 10),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if result.Document.Status != document.StatusPaid {
		t.Errorf("status = %s, want paid", result.Document.Status)
	}
	if result.Document.Allocated.Amount() != 100000 {
		t.Errorf("allocated = %s, want 100000 VND", result.Document.Allocated)
	}
	if !result.Document.Outstanding().IsZero() {
		t.Errorf("outstanding = %s, want nothing", result.Document.Outstanding())
	}
	if result.Allocation == nil || result.Allocation.Amount.Amount() != 100000 {
		t.Errorf("allocation = %+v, want 100000 VND", result.Allocation)
	}
	if result.Credit != nil {
		t.Errorf("a payment that exactly settles a document leaves no credit, got %+v", result.Credit)
	}

	stored, err := book.Get(ctx, d.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Status != document.StatusPaid || stored.Allocated.Amount() != 100000 {
		t.Errorf("stored document = %+v, want it paid in full", stored)
	}

	credits, err := book.Credits(ctx, "p-1")
	if err != nil {
		t.Fatalf("Credits: %v", err)
	}
	if len(credits) != 0 {
		t.Errorf("credits = %d, want none", len(credits))
	}
}

// Part of the money leaves the document partly paid, and says how much is still
// owed.
func TestApply_PartialPaymentMarksTheDocumentPartiallyPaid(t *testing.T) {
	ctx := context.Background()
	book := document.NewMemoryRepository()
	d := invoice("inv-1", "p-1", 100000)
	store(t, book, d)

	result, err := book.Apply(ctx, document.Application{
		ID: "app-1", DocumentID: d.ID, PaymentID: "pay-1",
		Amount: cents(40000), At: at(2026, 3, 10),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if result.Document.Status != document.StatusPartiallyPaid {
		t.Errorf("status = %s, want partially_paid", result.Document.Status)
	}
	if got := result.Document.Allocated.Amount(); got != 40000 {
		t.Errorf("allocated = %d, want 40000", got)
	}
	if got := result.Document.Outstanding().Amount(); got != 60000 {
		t.Errorf("outstanding = %d, want 60000", got)
	}
	if result.Credit != nil {
		t.Errorf("a partial payment leaves no credit, got %+v", result.Credit)
	}

	// A second payment finishes it.
	result, err = book.Apply(ctx, document.Application{
		ID: "app-2", DocumentID: d.ID, PaymentID: "pay-2",
		Amount: cents(60000), At: at(2026, 4, 10),
	})
	if err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if result.Document.Status != document.StatusPaid {
		t.Errorf("status = %s, want paid once the rest arrives", result.Document.Status)
	}

	allocations, err := book.Allocations(ctx, d.ID)
	if err != nil {
		t.Fatalf("Allocations: %v", err)
	}
	if len(allocations) != 2 {
		t.Fatalf("allocations = %d, want 2", len(allocations))
	}
}

// Paying more than a document is worth settles it and leaves the difference as a
// credit balance. It is not an error, and it does not turn the document into a
// credit for the difference.
func TestApply_OverpaymentCreatesACreditBalance(t *testing.T) {
	ctx := context.Background()
	book := document.NewMemoryRepository()

	d := invoice("inv-1", "p-1", 100000)
	store(t, book, d)

	payment := receipt("pay-1", "p-1", 150000)
	store(t, book, payment)

	result, err := book.Apply(ctx, document.Application{
		ID: "app-1", DocumentID: d.ID, PaymentID: payment.ID,
		Amount: cents(150000), At: at(2026, 3, 10),
	})
	if err != nil {
		t.Fatalf("an overpayment must not be refused: %v", err)
	}

	if result.Document.Status != document.StatusPaid {
		t.Errorf("status = %s, want paid", result.Document.Status)
	}
	if result.Document.Allocated.Amount() != 100000 {
		t.Errorf("allocated = %s, want the 100000 the document was worth", result.Document.Allocated)
	}
	if result.Allocation == nil || result.Allocation.Amount.Amount() != 100000 {
		t.Errorf("allocation = %+v, want 100000 VND and no more", result.Allocation)
	}
	if result.Credit == nil {
		t.Fatal("the 50000 the document could not absorb must become a credit balance")
	}
	if result.Credit.Amount.Amount() != 50000 {
		t.Errorf("credit = %s, want 50000 VND", result.Credit.Amount)
	}
	if result.Credit.PartnerID != "p-1" || result.Credit.DocumentID != d.ID || result.Credit.PaymentID != payment.ID {
		t.Errorf("credit = %+v, want it to name the partner, the document and the payment", result.Credit)
	}

	// The credit is a row the caller can read back, not just a number in a
	// result they might drop.
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
	if credits[0].ID != result.Credit.ID {
		t.Errorf("credit id = %q, want the one the result named (%q)", credits[0].ID, result.Credit.ID)
	}

	// And the document never allocated more than it was worth.
	stored, err := book.Get(ctx, d.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Allocated.Amount() != 100000 {
		t.Errorf("stored allocated = %s, want 100000", stored.Allocated)
	}
}

// A payment offered against a document that is already settled becomes a credit
// balance in full — the ordinary case of a customer paying a round sum against
// an invoice they had already cleared.
func TestApply_PaymentAgainstASettledDocumentBecomesACredit(t *testing.T) {
	ctx := context.Background()
	book := document.NewMemoryRepository()
	d := invoice("inv-1", "p-1", 100000)
	store(t, book, d)

	if _, err := book.Apply(ctx, document.Application{
		ID: "app-1", DocumentID: d.ID, PaymentID: "pay-1", Amount: cents(100000), At: at(2026, 3, 10),
	}); err != nil {
		t.Fatalf("first Apply: %v", err)
	}

	result, err := book.Apply(ctx, document.Application{
		ID: "app-2", DocumentID: d.ID, PaymentID: "pay-2", Amount: cents(25000), At: at(2026, 4, 10),
	})
	if err != nil {
		t.Fatalf("Apply against a settled document: %v", err)
	}

	if result.Allocation != nil {
		t.Errorf("a settled document absorbed %+v, want nothing", result.Allocation)
	}
	if result.Credit == nil || result.Credit.Amount.Amount() != 25000 {
		t.Fatalf("credit = %+v, want the whole 25000", result.Credit)
	}
	if result.Document.Status != document.StatusPaid {
		t.Errorf("status = %s, want it to stay paid", result.Document.Status)
	}
}

// Two payments arriving at once must not both spend the same outstanding
// balance.
//
// Eight payments of 25000 are offered against a document worth 100000, all at
// once. Whatever order they land in, exactly 100000 may be allocated and exactly
// 100000 must become credit: not a minor unit less, and not one more.
func TestApply_ConcurrentPaymentsDoNotOverAllocate(t *testing.T) {
	ctx := context.Background()
	book := document.NewMemoryRepository()
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
			<-start // every goroutine is inside Apply at the same moment
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

	stored, err := book.Get(ctx, d.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Allocated.Amount() != 100000 {
		t.Errorf("stored allocated = %s, want 100000 — the document was over-allocated", stored.Allocated)
	}
	if stored.Status != document.StatusPaid {
		t.Errorf("stored status = %s, want paid", stored.Status)
	}

	allocations, err := book.Allocations(ctx, d.ID)
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

// A payment applies to a document once. A retried request must not settle it
// twice.
func TestApply_RefusesTheSamePaymentTwice(t *testing.T) {
	ctx := context.Background()
	book := document.NewMemoryRepository()
	d := invoice("inv-1", "p-1", 100000)
	store(t, book, d)

	app := document.Application{
		ID: "app-1", DocumentID: d.ID, PaymentID: "pay-1", Amount: cents(40000), At: at(2026, 3, 10),
	}

	if _, err := book.Apply(ctx, app); err != nil {
		t.Fatalf("first Apply: %v", err)
	}

	// The same payment again, even under a fresh application id.
	app.ID = "app-1-retry"
	if _, err := book.Apply(ctx, app); !errors.Is(err, document.ErrAlreadyApplied) {
		t.Fatalf("second Apply = %v, want ErrAlreadyApplied", err)
	}

	stored, err := book.Get(ctx, d.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Allocated.Amount() != 40000 {
		t.Errorf("allocated = %s after a refused retry, want 40000", stored.Allocated)
	}
}

// A voided document is owed to nobody and may not be settled.
func TestApply_RefusesAVoidedDocument(t *testing.T) {
	ctx := context.Background()
	book := document.NewMemoryRepository()
	d := invoice("inv-1", "p-1", 100000)
	store(t, book, d)

	if err := book.Void(ctx, d.ID); err != nil {
		t.Fatalf("Void: %v", err)
	}

	_, err := book.Apply(ctx, document.Application{
		ID: "app-1", DocumentID: d.ID, PaymentID: "pay-1", Amount: cents(1000), At: at(2026, 3, 10),
	})
	if !errors.Is(err, document.ErrVoided) {
		t.Fatalf("Apply against a voided document = %v, want ErrVoided", err)
	}
}

// A document that has been settled may not be voided: the payment that settled
// it would be left pointing at nothing.
func TestVoid_RefusesADocumentThatHasBeenAllocated(t *testing.T) {
	ctx := context.Background()
	book := document.NewMemoryRepository()
	d := invoice("inv-1", "p-1", 100000)
	store(t, book, d)

	if _, err := book.Apply(ctx, document.Application{
		ID: "app-1", DocumentID: d.ID, PaymentID: "pay-1", Amount: cents(40000), At: at(2026, 3, 10),
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if err := book.Void(ctx, d.ID); !errors.Is(err, document.ErrVoidNotAllowed) {
		t.Fatalf("Void = %v, want ErrVoidNotAllowed", err)
	}
}

// The rule itself, with nothing stored around it.
func TestSettle_AppliesNoMoreThanTheDocumentOwes(t *testing.T) {
	d := invoice("inv-1", "p-1", 100000)

	result, err := document.Settle(d, document.Application{
		ID: "app-1", DocumentID: d.ID, PaymentID: "pay-1", Amount: cents(150000), At: at(2026, 3, 10),
	})
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}

	if result.Allocation.Amount.Amount() != 100000 {
		t.Errorf("applied = %s, want the document's 100000", result.Allocation.Amount)
	}
	if result.Credit.Amount.Amount() != 50000 {
		t.Errorf("credited = %s, want 50000", result.Credit.Amount)
	}
	if got := result.Allocation.Amount.Amount() + result.Credit.Amount.Amount(); got != 150000 {
		t.Errorf("applied + credited = %d, want the 150000 that was offered", got)
	}
}

func TestSettle_RefusesAPaymentThatIsNotOne(t *testing.T) {
	d := invoice("inv-1", "p-1", 100000)

	cases := map[string]document.Application{
		"no application id": {DocumentID: d.ID, PaymentID: "pay-1", Amount: cents(1000)},
		"no payment id":     {ID: "app-1", DocumentID: d.ID, Amount: cents(1000)},
		"another document":  {ID: "app-1", DocumentID: "inv-2", PaymentID: "pay-1", Amount: cents(1000)},
		"nothing at all":    {ID: "app-1", DocumentID: d.ID, PaymentID: "pay-1", Amount: cents(0)},
		"a negative amount": {ID: "app-1", DocumentID: d.ID, PaymentID: "pay-1", Amount: cents(-1000)},
		"no currency":       {ID: "app-1", DocumentID: d.ID, PaymentID: "pay-1", Amount: money.Money{}},
	}

	for name, app := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := document.Settle(d, app); !errors.Is(err, document.ErrInvalidApplication) {
				t.Fatalf("Settle() = %v, want ErrInvalidApplication", err)
			}
		})
	}
}

// 1000 VND is not 1000 USD, and a ledger that treats them as the same money
// cannot be recovered.
func TestSettle_RefusesAPaymentInAnotherCurrency(t *testing.T) {
	d := invoice("inv-1", "p-1", 100000)

	_, err := document.Settle(d, document.Application{
		ID: "app-1", DocumentID: d.ID, PaymentID: "pay-1",
		Amount: money.New(100000, money.Currency("USD")), At: at(2026, 3, 10),
	})
	if !errors.Is(err, document.ErrCurrencyMismatch) {
		t.Fatalf("Settle() = %v, want ErrCurrencyMismatch", err)
	}
}
