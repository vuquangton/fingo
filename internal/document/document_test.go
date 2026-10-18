package document_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"fingo/internal/document"
	"fingo/internal/money"
)

// The interface the MariaDB adapter is written against is the one the in-memory
// book satisfies, so a rule proved against one holds for the other.
var _ document.Repository = (*document.MemoryRepository)(nil)

var vnd = money.Currency("VND")

// cents is an amount in the minor unit of the currency the tests use.
func cents(amount int64) money.Money { return money.New(amount, vnd) }

func day(year int, month time.Month, dayOfMonth int) time.Time {
	return time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.UTC)
}

func at(year int, month time.Month, dayOfMonth int) time.Time {
	return time.Date(year, month, dayOfMonth, 9, 0, 0, 0, time.UTC)
}

// invoice is an open document worth total, owed by partner.
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

// receipt is a payment document for money received.
func receipt(id, partnerID string, total int64) document.Document {
	d := invoice(id, partnerID, total)
	d.Type = document.TypeReceipt
	d.Date = day(2026, 3, 10)

	return d
}

// payments is a balanced two-line set for a document: what was debited, and what
// was credited.
func payments(d document.Document) []document.Line {
	return []document.Line{
		{DocumentID: d.ID, AccountID: "131", Debit: d.Total},
		{DocumentID: d.ID, AccountID: "511", Credit: d.Total},
	}
}

func store(t *testing.T, book document.Repository, d document.Document) {
	t.Helper()

	if err := book.Create(context.Background(), d, payments(d)); err != nil {
		t.Fatalf("Create %s: %v", d.ID, err)
	}
}

func TestDocument_ValidateAcceptsAnOpenDocument(t *testing.T) {
	if err := invoice("inv-1", "p-1", 100000).Validate(); err != nil {
		t.Fatalf("an open invoice must be valid: %v", err)
	}
}

// Status is not decoration: a document that says paid while still owed, or open
// while already settled, cannot be stored at all.
func TestDocument_ValidateRefusesAStatusThatContradictsTheAmounts(t *testing.T) {
	cases := map[string]document.Document{
		"open but already allocated": func() document.Document {
			d := invoice("inv-1", "p-1", 100000)
			d.Allocated = cents(40000)
			return d
		}(),
		"partially paid but nothing allocated": func() document.Document {
			d := invoice("inv-1", "p-1", 100000)
			d.Status = document.StatusPartiallyPaid
			return d
		}(),
		"paid but only half allocated": func() document.Document {
			d := invoice("inv-1", "p-1", 100000)
			d.Allocated = cents(50000)
			d.Status = document.StatusPaid
			return d
		}(),
		"open but fully allocated": func() document.Document {
			d := invoice("inv-1", "p-1", 100000)
			d.Allocated = cents(100000)
			return d
		}(),
	}

	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			if err := d.Validate(); !errors.Is(err, document.ErrInvalidDocument) {
				t.Fatalf("Validate() = %v, want ErrInvalidDocument", err)
			}
		})
	}
}

// The statuses that agree with the amounts are accepted.
func TestDocument_ValidateAcceptsEachConsistentStatus(t *testing.T) {
	full := invoice("inv-1", "p-1", 100000)
	full.Allocated = cents(100000)
	full.Status = document.StatusPaid

	half := invoice("inv-2", "p-1", 100000)
	half.Allocated = cents(40000)
	half.Status = document.StatusPartiallyPaid

	void := invoice("inv-3", "p-1", 100000)
	void.Status = document.StatusVoid

	for _, d := range []document.Document{invoice("inv-4", "p-1", 100000), full, half, void} {
		if err := d.Validate(); err != nil {
			t.Errorf("document %s with status %s: %v", d.ID, d.Status, err)
		}
	}
}

func TestDocument_ValidateRefusesADocumentThatCouldNotBeUsed(t *testing.T) {
	cases := map[string]document.Document{
		"no id":            {Type: document.TypeInvoice, PartnerID: "p-1", Date: day(2026, 3, 1), PeriodID: "fy2026", Total: cents(1000), Status: document.StatusOpen},
		"no partner":       {ID: "d", Type: document.TypeInvoice, Date: day(2026, 3, 1), PeriodID: "fy2026", Total: cents(1000), Status: document.StatusOpen},
		"no date":          {ID: "d", Type: document.TypeInvoice, PartnerID: "p-1", PeriodID: "fy2026", Total: cents(1000), Status: document.StatusOpen},
		"no period":        {ID: "d", Type: document.TypeInvoice, PartnerID: "p-1", Date: day(2026, 3, 1), Total: cents(1000), Status: document.StatusOpen},
		"unknown type":     {ID: "d", Type: "receipt_note", PartnerID: "p-1", Date: day(2026, 3, 1), PeriodID: "fy2026", Total: cents(1000), Status: document.StatusOpen},
		"unknown status":   {ID: "d", Type: document.TypeInvoice, PartnerID: "p-1", Date: day(2026, 3, 1), PeriodID: "fy2026", Total: cents(1000), Status: "settled"},
		"a total of zero":  {ID: "d", Type: document.TypeInvoice, PartnerID: "p-1", Date: day(2026, 3, 1), PeriodID: "fy2026", Total: cents(0), Status: document.StatusOpen},
		"a negative total": {ID: "d", Type: document.TypeInvoice, PartnerID: "p-1", Date: day(2026, 3, 1), PeriodID: "fy2026", Total: cents(-1000), Status: document.StatusOpen},
		"no currency":      {ID: "d", Type: document.TypeInvoice, PartnerID: "p-1", Date: day(2026, 3, 1), PeriodID: "fy2026", Total: money.Money{}},
		"over-allocated": func() document.Document {
			d := invoice("d", "p-1", 1000)
			d.Allocated = cents(2000)
			return d
		}(),
	}

	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			if err := d.Validate(); !errors.Is(err, document.ErrInvalidDocument) {
				t.Fatalf("Validate() = %v, want ErrInvalidDocument", err)
			}
		})
	}
}

// A voided document is owed to nobody, whatever its amounts say.
func TestDocument_OutstandingIsNothingWhenVoided(t *testing.T) {
	d := invoice("inv-1", "p-1", 100000)
	d.Status = document.StatusVoid

	if got := d.Outstanding(); !got.IsZero() {
		t.Errorf("a voided document owes %s, want nothing", got)
	}
}

func TestDocument_OutstandingFallsAsItIsSettled(t *testing.T) {
	d := invoice("inv-1", "p-1", 100000)
	if got := d.Outstanding().Amount(); got != 100000 {
		t.Errorf("outstanding = %d, want 100000", got)
	}

	d.Allocated = cents(40000)
	if got := d.Outstanding().Amount(); got != 60000 {
		t.Errorf("outstanding = %d, want 60000", got)
	}
}

// A line carries one side. Both, or neither, is not a line.
func TestLine_ValidateRefusesALineThatIsNotOneSide(t *testing.T) {
	cases := map[string]document.Line{
		"no account":        {DocumentID: "d", Debit: cents(100)},
		"both sides":        {DocumentID: "d", AccountID: "131", Debit: cents(100), Credit: cents(100)},
		"neither side":      {DocumentID: "d", AccountID: "131"},
		"a negative debit":  {DocumentID: "d", AccountID: "131", Debit: cents(-100)},
		"a negative credit": {DocumentID: "d", AccountID: "131", Credit: cents(-100)},
		"another currency":  {DocumentID: "d", AccountID: "131", Debit: money.New(100, money.Currency("USD"))},
	}

	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			if err := line.Validate(vnd); !errors.Is(err, document.ErrInvalidLine) && !errors.Is(err, document.ErrCurrencyMismatch) {
				t.Fatalf("Validate() = %v, want ErrInvalidLine or ErrCurrencyMismatch", err)
			}
		})
	}
}

// What is debited is credited. A document whose lines do not add up is not a
// document whose parts add up to what it is owed.
func TestCheckLines_RefusesLinesThatDoNotBalance(t *testing.T) {
	d := invoice("inv-1", "p-1", 100000)

	lines := []document.Line{
		{DocumentID: d.ID, AccountID: "131", Debit: cents(100000)},
		{DocumentID: d.ID, AccountID: "511", Credit: cents(60000)},
	}

	if err := document.CheckLines(d, lines); !errors.Is(err, document.ErrUnbalanced) {
		t.Fatalf("CheckLines() = %v, want ErrUnbalanced", err)
	}
}

func TestCheckLines_RefusesLinesOfAnotherDocument(t *testing.T) {
	d := invoice("inv-1", "p-1", 100000)

	lines := []document.Line{
		{DocumentID: "inv-2", AccountID: "131", Debit: cents(100000)},
		{DocumentID: "inv-2", AccountID: "511", Credit: cents(100000)},
	}

	if err := document.CheckLines(d, lines); !errors.Is(err, document.ErrInvalidLine) {
		t.Fatalf("CheckLines() = %v, want ErrInvalidLine", err)
	}
}

// The whole of the lifecycle a document has without money moving: it goes in,
// comes back with its lines, is listed, and can be cancelled.
func TestMemoryRepository_CreatesReadsListsAndVoids(t *testing.T) {
	ctx := context.Background()
	book := document.NewMemoryRepository()

	d := invoice("inv-1", "p-1", 100000)
	store(t, book, d)

	got, err := book.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != d.ID || got.Type != document.TypeInvoice || got.PartnerID != "p-1" || got.Status != document.StatusOpen {
		t.Fatalf("round trip = %+v", got)
	}
	if got.Total.Amount() != 100000 || got.Total.Currency() != vnd {
		t.Errorf("total = %s, want 100000 VND", got.Total)
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

	listed, err := book.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("documents = %d, want 1", len(listed))
	}

	if err := book.Void(ctx, "inv-1"); err != nil {
		t.Fatalf("Void: %v", err)
	}

	voided, err := book.Get(ctx, "inv-1")
	if err != nil {
		t.Fatalf("Get after void: %v", err)
	}
	if voided.Status != document.StatusVoid {
		t.Errorf("status = %s, want void", voided.Status)
	}
}

func TestMemoryRepository_RefusesToCreateTheSameIDTwice(t *testing.T) {
	ctx := context.Background()
	book := document.NewMemoryRepository()
	store(t, book, invoice("inv-1", "p-1", 100000))

	err := book.Create(ctx, invoice("inv-1", "p-1", 50000), nil)
	if !errors.Is(err, document.ErrDocumentExists) {
		t.Fatalf("Create of a taken id = %v, want ErrDocumentExists", err)
	}
}

// A document is created open and with nothing allocated. What has been settled
// is applied to it, not asserted about it when it is written.
func TestMemoryRepository_CreatesOnlyOpenUndallocatedDocuments(t *testing.T) {
	ctx := context.Background()
	book := document.NewMemoryRepository()

	settled := invoice("inv-1", "p-1", 100000)
	settled.Allocated = cents(100000)
	settled.Status = document.StatusPaid

	err := book.Create(ctx, settled, payments(settled))
	if !errors.Is(err, document.ErrInvalidDocument) {
		t.Fatalf("Create of a settled document = %v, want ErrInvalidDocument", err)
	}
}

func TestMemoryRepository_StoresNothingItCannotValidate(t *testing.T) {
	ctx := context.Background()
	book := document.NewMemoryRepository()

	d := invoice("inv-1", "p-1", 100000)
	unbalanced := []document.Line{
		{DocumentID: d.ID, AccountID: "131", Debit: cents(100000)},
		{DocumentID: d.ID, AccountID: "511", Credit: cents(99999)},
	}

	if err := book.Create(ctx, d, unbalanced); !errors.Is(err, document.ErrUnbalanced) {
		t.Fatalf("Create with unbalanced lines = %v, want ErrUnbalanced", err)
	}

	listed, err := book.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("documents = %d after a refused create, want 0", len(listed))
	}
}

// Acting on a document that is not there is reported, not silently ignored.
func TestMemoryRepository_ReportsUnknownDocuments(t *testing.T) {
	ctx := context.Background()
	book := document.NewMemoryRepository()

	if _, err := book.Get(ctx, "nope"); !errors.Is(err, document.ErrUnknownDocument) {
		t.Errorf("Get = %v, want ErrUnknownDocument", err)
	}
	if _, err := book.Lines(ctx, "nope"); !errors.Is(err, document.ErrUnknownDocument) {
		t.Errorf("Lines = %v, want ErrUnknownDocument", err)
	}
	if _, err := book.Allocations(ctx, "nope"); !errors.Is(err, document.ErrUnknownDocument) {
		t.Errorf("Allocations = %v, want ErrUnknownDocument", err)
	}
	if err := book.Void(ctx, "nope"); !errors.Is(err, document.ErrUnknownDocument) {
		t.Errorf("Void = %v, want ErrUnknownDocument", err)
	}
}
