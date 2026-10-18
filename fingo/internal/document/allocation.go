package document

import (
	"fmt"
	"strings"
	"time"

	"fingo/internal/money"
)

// Application is a payment offered against one document.
type Application struct {
	// ID identifies this application — this payment against this document —
	// and not the attempt. A retry carries the same one, and the rows it writes
	// are derived from it, so a retry collides with itself instead of settling
	// the document twice.
	ID string

	// DocumentID is the document being settled.
	DocumentID string

	// PaymentID is the payment the money came from: the receipt or payment
	// document, or the voucher that recorded it.
	PaymentID string

	// Amount is what is being offered. It may be more than the document is
	// worth — that is an overpayment, and the difference becomes a credit
	// balance rather than an error.
	Amount money.Money

	// At is when the application was made.
	At time.Time
}

// Allocation is one payment applied to one document.
type Allocation struct {
	ID         string
	DocumentID string
	PaymentID  string
	Amount     money.Money
	At         time.Time
}

// Credit is money a payment left over that the document could not absorb.
//
// It is a real position, not a rounding: the partner has paid more than they owe
// and the difference is theirs to spend on the next document, or to be refunded.
// Creating it rather than refusing the payment is what lets the ordinary case —
// a customer paying a round sum against an untidy invoice — work at all.
type Credit struct {
	ID string

	// PartnerID is who the credit belongs to: the document's counterparty.
	PartnerID string

	// DocumentID is the document the payment was offered against.
	DocumentID string

	// PaymentID is the payment the money came from.
	PaymentID string

	// Amount is what the document could not absorb.
	Amount money.Money

	// At is when the credit arose.
	At time.Time
}

// Result is what applying a payment did.
type Result struct {
	// Document is the document as it now stands: Allocated and Status moved
	// together.
	Document Document

	// Allocation is the row recording what was applied, and is nil when the
	// document could absorb nothing.
	Allocation *Allocation

	// Credit is the credit balance the payment left behind, and is nil when the
	// document absorbed all of it.
	Credit *Credit
}

// Settle works out what applying a payment to a document does, without writing
// anything.
//
// The rule is one line: a document can absorb what it still owes, and no more.
// Paying a 100 document with 500 settles the document and leaves 400 as a credit
// balance — the document does not become a credit for 400, and the payment is
// not refused. That is what makes an overpayment an ordinary event rather than a
// mistake the system punishes.
//
// Allocated and Status are computed here, together, from the document and the
// payment. Nothing else in the package sets either of them, so they cannot come
// apart.
func Settle(d Document, app Application) (Result, error) {
	if err := d.Validate(); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(app.ID) == "" {
		return Result{}, fmt.Errorf("%w: an application needs an id", ErrInvalidApplication)
	}
	if strings.TrimSpace(app.PaymentID) == "" {
		return Result{}, fmt.Errorf("%w: application %s does not say which payment it came from",
			ErrInvalidApplication, app.ID)
	}
	if app.DocumentID != d.ID {
		return Result{}, fmt.Errorf("%w: application %s is for document %q, and document %s was given",
			ErrInvalidApplication, app.ID, app.DocumentID, d.ID)
	}
	if !app.Amount.Currency().Valid() {
		return Result{}, fmt.Errorf("%w: application %s applies %s, which names no currency",
			ErrInvalidApplication, app.ID, app.Amount)
	}
	if app.Amount.Amount() <= 0 {
		return Result{}, fmt.Errorf("%w: application %s applies %s, and a payment is greater than nothing",
			ErrInvalidApplication, app.ID, app.Amount)
	}
	if app.Amount.Currency() != d.Total.Currency() {
		return Result{}, fmt.Errorf("%w: document %s is kept in %s and application %s pays %s",
			ErrCurrencyMismatch, d.ID, d.Total.Currency(), app.ID, app.Amount.Currency())
	}
	if d.Status == StatusVoid {
		return Result{}, fmt.Errorf("%w: %s", ErrVoided, d.ID)
	}

	// What the document can absorb, and what is left over. Never the other way
	// round: a document that absorbed more than it owes would be a document with
	// a negative balance, which nobody can act on.
	applied := app.Amount
	if outstanding := d.Outstanding(); outstanding.Amount() < applied.Amount() {
		applied = outstanding
	}
	credited := app.Amount.Sub(applied)

	settled := d
	settled.Allocated = d.Allocated.Add(applied)
	settled.Status = statusFor(settled.Total, settled.Allocated)

	result := Result{Document: settled}

	if applied.Amount() > 0 {
		result.Allocation = &Allocation{
			ID:         allocationID(app.ID),
			DocumentID: d.ID,
			PaymentID:  app.PaymentID,
			Amount:     applied,
			At:         app.At,
		}
	}
	if credited.Amount() > 0 {
		result.Credit = &Credit{
			ID:         creditID(app.ID),
			PartnerID:  d.PartnerID,
			DocumentID: d.ID,
			PaymentID:  app.PaymentID,
			Amount:     credited,
			At:         app.At,
		}
	}

	return result, nil
}

// allocationID derives the row id for a settled application.
//
// Derived rather than drawn at random, so that the same application produces the
// same row: a retry collides with what it already wrote instead of writing a
// second allocation for one payment.
func allocationID(applicationID string) string { return "allocation-" + applicationID }

// creditID derives the row id for the credit an application left behind. Derived
// for the same reason, and from the same application.
func creditID(applicationID string) string { return "credit-" + applicationID }
