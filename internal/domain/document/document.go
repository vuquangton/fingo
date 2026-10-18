// Package document holds the business documents a voucher records, and the
// allocations that settle them.
//
// A voucher says money moved. A document says what the money is for: which
// invoice, dated when, due when, and how much of it is still owed. Without that
// the ledger can total TK 131 but cannot age it, and "how much is overdue?" has
// no answer — which is the question a trading SME actually asks.
package document

import (
	"errors"
	"fingo/internal/money"
	"fmt"
	"sort"
	"time"
)

// Kind is what a document is.
type Kind string

const (
	KindSalesInvoice    Kind = "SALES_INVOICE"
	KindPurchaseInvoice Kind = "PURCHASE_INVOICE"
	KindCreditNote      Kind = "CREDIT_NOTE"
)

func (k Kind) Valid() bool {
	switch k {
	case KindSalesInvoice, KindPurchaseInvoice, KindCreditNote:
		return true
	}
	return false
}

// Document is a business document, recorded by the voucher that posted it.
type Document struct {
	ID           string
	PartnerID    string
	VoucherID    string
	Kind         Kind
	Reference    string // số hóa đơn — the number written on the paper
	DocumentDate time.Time
	DueDate      time.Time
	Amount       money.Dong // gross, what the counterparty owes
}

// Validate reports whether a document could be chased.
func (d Document) Validate() error {
	if d.ID == "" {
		return errors.New("a document needs an id")
	}
	if d.PartnerID == "" {
		return errors.New("a document is owed by or to somebody")
	}
	if !d.Kind.Valid() {
		return fmt.Errorf("document kind %q is not one of the known kinds", d.Kind)
	}
	if d.Reference == "" {
		return errors.New("a document needs the reference written on it")
	}
	if d.Amount.IsNegative() {
		return errors.New("a document must not have a negative amount")
	}

	return nil
}

// Open is a document together with what is still owed on it.
type Open struct {
	Document    Document
	Outstanding money.Dong
}

// Allocation is part of a payment applied to one document.
type Allocation struct {
	DocumentID string
	Amount     money.Dong
}

// Plan is how a payment spreads across a partner's open documents.
type Plan struct {
	Allocations []Allocation

	// Unapplied is what no document could absorb. It is reported rather than
	// absorbed: an overpayment means the customer is in credit, which is a real
	// position somebody has to deal with.
	Unapplied money.Dong
}

// PlanAllocation settles open documents oldest first.
//
// Oldest first is what a customer expects and what an aging report assumes, so
// it is the rule rather than a preference. The order is made stable by reference
// so that two documents dated the same day settle predictably instead of in
// whatever order storage happened to return them.
//
// Nothing is ever allocated beyond what a document owes: paying a 100 invoice
// with 500 settles that invoice and leaves 400 unapplied, rather than turning
// the invoice into a credit nobody asked for.
func PlanAllocation(payment money.Dong, open []Open) (Plan, error) {
	if !payment.IsPositive() {
		return Plan{}, errors.New("an allocation needs a payment greater than zero")
	}

	ordered := make([]Open, len(open))
	copy(ordered, open)
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := ordered[i].Document, ordered[j].Document
		if !left.DocumentDate.Equal(right.DocumentDate) {
			return left.DocumentDate.Before(right.DocumentDate)
		}
		return left.Reference < right.Reference
	})

	remaining := payment
	plan := Plan{Unapplied: money.Zero}

	for _, o := range ordered {
		if !remaining.IsPositive() {
			break
		}
		if !o.Outstanding.IsPositive() {
			continue
		}

		applied := o.Outstanding
		if remaining.LessThan(applied) {
			applied = remaining
		}

		plan.Allocations = append(plan.Allocations, Allocation{
			DocumentID: o.Document.ID,
			Amount:     applied,
		})
		remaining = remaining.Sub(applied)
	}

	plan.Unapplied = remaining

	return plan, nil
}
