// Package document holds the business documents a client's books are kept
// against, the lines that make them up, and the allocations that settle them.
//
// A voucher says money moved. A document says what the money is for: which
// invoice, to whom, dated when, in which period, and how much of it is still
// owed. Without that the ledger can total TK 131 but cannot age it, and "how
// much is overdue?" has no answer — which is the question a trading SME actually
// asks.
//
// The rule this package exists to hold is that a document's Allocated and Status
// are one fact, not two. They move together, inside one locked transaction, so
// there is never a document that has been paid and still says open, or one that
// says paid and still has money outstanding. See Repository.Apply.
//
// NOTE: this is the AR/AP model. It coexists with internal/domain/document,
// which carries a voucher_id, a reference number and a due date and is what the
// posting path currently uses. They are two models of the same idea and one has
// to go; that consolidation is a separate decision and is not made here. See the
// note at the top of migration 00014.
package document

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"fingo/internal/money"
)

// Errors a caller may need to tell apart.
var (
	// ErrInvalidDocument is a document that could not be stored: no id, no
	// partner, no period, an unknown type or status, or amounts that contradict
	// its status.
	ErrInvalidDocument = errors.New("document: invalid document")

	// ErrInvalidLine is a line that could not belong to a document: no account,
	// both sides or neither, or an amount in another currency.
	ErrInvalidLine = errors.New("document: invalid line")

	// ErrUnbalanced is a document whose lines do not sum to zero.
	ErrUnbalanced = errors.New("document: the lines do not balance")

	// ErrUnknownDocument is a lookup, allocation or void of an id the book does
	// not hold.
	ErrUnknownDocument = errors.New("document: no such document")

	// ErrDocumentExists is a create of an id already in the book.
	ErrDocumentExists = errors.New("document: a document with this id already exists")

	// ErrVoided is an allocation against a document that has been voided. A
	// voided document is not owed and must not be settled.
	ErrVoided = errors.New("document: a voided document cannot be settled")

	// ErrCurrencyMismatch is an amount in one currency offered against a
	// document kept in another.
	ErrCurrencyMismatch = errors.New("document: amounts are in different currencies")

	// ErrInvalidApplication is an application that could not be made at all: no
	// id, no payment, no amount, or an amount that names no currency.
	ErrInvalidApplication = errors.New("document: invalid application")

	// ErrAlreadyApplied is the same payment applied to the same document twice.
	ErrAlreadyApplied = errors.New("document: this payment has already been applied to this document")

	// ErrVoidNotAllowed is an attempt to void a document that has already been
	// settled in part or in full.
	ErrVoidNotAllowed = errors.New("document: a document that has been allocated cannot be voided, only reversed")
)

// Type is what a document is.
type Type string

const (
	// TypeInvoice is a bill owed: a sales invoice (AR) or a purchase invoice
	// (AP).
	TypeInvoice Type = "invoice"

	// TypeCreditNote is a bill reversed — goods returned, an overcharge
	// corrected. It reduces what the partner owes.
	TypeCreditNote Type = "credit_note"

	// TypeReceipt is money received from a customer.
	TypeReceipt Type = "receipt"

	// TypePayment is money paid to a supplier.
	TypePayment Type = "payment"

	// TypeJournal is a document with no counterparty flow of its own: an
	// adjustment, an opening entry, a reclassification.
	TypeJournal Type = "journal"
)

// Valid reports whether the type is one the book knows.
func (t Type) Valid() bool {
	switch t {
	case TypeInvoice, TypeCreditNote, TypeReceipt, TypePayment, TypeJournal:
		return true
	}

	return false
}

// Status is how much of a document has been settled.
type Status string

const (
	// StatusOpen is nothing allocated yet.
	StatusOpen Status = "open"

	// StatusPartiallyPaid is some of the document allocated, and some still
	// owed.
	StatusPartiallyPaid Status = "partially_paid"

	// StatusPaid is the whole document allocated.
	StatusPaid Status = "paid"

	// StatusVoid is a document that has been cancelled. It is owed to nobody
	// and may not be settled.
	StatusVoid Status = "void"
)

// Valid reports whether the status is one the book knows.
func (s Status) Valid() bool {
	switch s {
	case StatusOpen, StatusPartiallyPaid, StatusPaid, StatusVoid:
		return true
	}

	return false
}

// Document is a business document the books are kept against.
type Document struct {
	// ID identifies the document. Stable for the life of the books — usually a
	// UUID — because it is what an allocation refers to.
	ID string

	// Type is what the document is.
	Type Type

	// PartnerID names the counterparty: the customer who owes an invoice, the
	// supplier who is owed a bill.
	PartnerID string

	// Date is the business date the document belongs to.
	Date time.Time

	// PeriodID names the accounting period the document falls in. Required: a
	// document nobody can place in a period cannot be reported.
	PeriodID string

	// VATRateID names the VAT rate row the document was taxed under, as an ID
	// rather than a percentage — see internal/vat. Empty on the documents that
	// carry no VAT, which is most receipts and payments.
	VATRateID string

	// Total is what the document is worth.
	Total money.Money

	// Allocated is how much of Total has been settled. It moves only through
	// Repository.Apply, and only together with Status.
	Allocated money.Money

	// Status is how much of the document has been settled. It is a function of
	// Total and Allocated, and is stored beside them rather than derived on
	// every read so that a query can filter on it — Status is checked against
	// them on every write.
	Status Status
}

// Validate reports whether the document could be stored and used.
//
// It checks Status against Total and Allocated, so a document that says paid
// while still owed, or open while already settled, cannot be stored at all.
func (d Document) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return fmt.Errorf("%w: a document needs an id", ErrInvalidDocument)
	}
	if !d.Type.Valid() {
		return fmt.Errorf("%w: document %s has type %q, which is not one of invoice, credit_note, receipt, payment, journal",
			ErrInvalidDocument, d.ID, d.Type)
	}
	if strings.TrimSpace(d.PartnerID) == "" {
		return fmt.Errorf("%w: document %s has no partner", ErrInvalidDocument, d.ID)
	}
	if d.Date.IsZero() {
		return fmt.Errorf("%w: document %s has no date", ErrInvalidDocument, d.ID)
	}
	if strings.TrimSpace(d.PeriodID) == "" {
		return fmt.Errorf("%w: document %s is in no period", ErrInvalidDocument, d.ID)
	}
	if !d.Status.Valid() {
		return fmt.Errorf("%w: document %s has status %q, which is not one of open, partially_paid, paid, void",
			ErrInvalidDocument, d.ID, d.Status)
	}
	if !d.Total.Currency().Valid() {
		return fmt.Errorf("%w: document %s has a total of %s, which names no currency",
			ErrInvalidDocument, d.ID, d.Total)
	}
	if d.Total.Amount() <= 0 {
		return fmt.Errorf("%w: document %s totals %s, and a document is worth more than nothing",
			ErrInvalidDocument, d.ID, d.Total)
	}
	if d.Allocated.Currency() != "" && d.Allocated.Currency() != d.Total.Currency() {
		return fmt.Errorf("%w: document %s totals %s but has %s allocated",
			ErrCurrencyMismatch, d.ID, d.Total, d.Allocated)
	}
	if d.Allocated.Amount() < 0 {
		return fmt.Errorf("%w: document %s has %s allocated, which is less than nothing",
			ErrInvalidDocument, d.ID, d.Allocated)
	}
	if d.Allocated.Amount() > d.Total.Amount() {
		return fmt.Errorf("%w: document %s totals %s but has %s allocated",
			ErrInvalidDocument, d.ID, d.Total, d.Allocated)
	}

	// A voided document is owed to nobody, whatever its amounts say.
	if d.Status == StatusVoid {
		return nil
	}
	if want := statusFor(d.Total, d.Allocated); d.Status != want {
		return fmt.Errorf("%w: document %s totals %s with %s allocated, so its status is %s, not %s",
			ErrInvalidDocument, d.ID, d.Total, d.Allocated, want, d.Status)
	}

	return nil
}

// CheckNew reports whether a document together with its lines may be created.
//
// A document is created open and with nothing allocated: what has been settled
// is applied to it, never asserted about it when it is written. Its lines must
// balance, which is what makes a document's parts add up to what it is owed.
//
// Both repositories check this before they write, so the rule holds whichever
// one a caller is holding.
func CheckNew(d Document, lines []Line) error {
	if err := d.Validate(); err != nil {
		return err
	}
	if d.Status != StatusOpen || !d.Allocated.IsZero() {
		return fmt.Errorf("%w: document %s is created open and with nothing allocated, and %s/%s was given",
			ErrInvalidDocument, d.ID, d.Allocated, d.Status)
	}

	return CheckLines(d, lines)
}

// Outstanding is what is still owed on the document.
//
// A voided document is owed to nobody, so it owes nothing. A document whose
// allocated amount has somehow reached its total owes nothing either, rather
// than a negative amount that a caller would have to remember to floor.
func (d Document) Outstanding() money.Money {
	if d.Status == StatusVoid || !d.Total.Currency().Valid() {
		return money.Money{}
	}

	outstanding := d.Total.Sub(d.Allocated)
	if outstanding.IsNegative() {
		return money.New(0, d.Total.Currency())
	}

	return outstanding
}

// Settled reports whether the document has been paid in full.
func (d Document) Settled() bool { return d.Status == StatusPaid }

// statusFor is the one place that decides what a document's status is, given
// what it is worth and what has been allocated to it.
//
// Everything that writes Allocated goes through it, so the two can never
// disagree; Validate checks incoming documents against it.
func statusFor(total, allocated money.Money) Status {
	switch {
	case allocated.Amount() <= 0:
		return StatusOpen
	case allocated.Amount() >= total.Amount():
		return StatusPaid
	default:
		return StatusPartiallyPaid
	}
}
