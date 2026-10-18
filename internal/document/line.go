package document

import (
	"fmt"
	"strings"

	"fingo/internal/money"
)

// Line is one account line of a document: which account the money is for, and
// how much of it.
//
// A line carries one side. Debit and Credit are both present because a stored
// line has a shape a report can read without branching, and exactly one of them
// is set — that is what makes a line a debit or a credit rather than a signed
// number somebody has to interpret.
//
// BaseAmount and VATAmount decompose the line for the VAT return: net, and the
// tax on it. They are stored as they were written, not recomputed here —
// internal/vat owns what tax an amount carries, and a second implementation of
// that rule is a second answer.
type Line struct {
	// DocumentID is the document the line belongs to.
	DocumentID string

	// AccountID names the account the money is posted to.
	AccountID string

	// Debit is the amount on the debit side. Zero on a credit line.
	Debit money.Money

	// Credit is the amount on the credit side. Zero on a debit line.
	Credit money.Money

	// VATAmount is the tax carried on this line, when it carries any.
	VATAmount money.Money

	// BaseAmount is the line value the tax was computed on, exclusive of it.
	BaseAmount money.Money
}

// Validate reports whether the line could belong to a document kept in a
// currency.
//
// currency is the document's. An amount names its own currency, and an amount
// naming a different one from the document's is not a line of that document —
// it is a conversion nobody performed.
func (l Line) Validate(currency money.Currency) error {
	if strings.TrimSpace(l.AccountID) == "" {
		return fmt.Errorf("%w: a line of document %s names no account", ErrInvalidLine, l.DocumentID)
	}
	if l.Debit.Amount() < 0 || l.Credit.Amount() < 0 {
		return fmt.Errorf("%w: line on account %s has a negative side (%s debit, %s credit)",
			ErrInvalidLine, l.AccountID, l.Debit, l.Credit)
	}
	if l.Debit.Amount() > 0 && l.Credit.Amount() > 0 {
		return fmt.Errorf("%w: line on account %s is both a debit of %s and a credit of %s",
			ErrInvalidLine, l.AccountID, l.Debit, l.Credit)
	}
	if l.Debit.Amount() == 0 && l.Credit.Amount() == 0 {
		return fmt.Errorf("%w: line on account %s moves nothing", ErrInvalidLine, l.AccountID)
	}

	for _, amount := range []money.Money{l.Debit, l.Credit, l.VATAmount, l.BaseAmount} {
		if amount.Currency() != "" && amount.Currency() != currency {
			return fmt.Errorf("%w: line on account %s carries %s, and the document is kept in %s",
				ErrCurrencyMismatch, l.AccountID, amount, currency)
		}
	}

	return nil
}

// CheckLines reports whether a set of lines could make up a document.
//
// A document needs at least one line, every line must belong to it, and the
// lines must balance: what is debited is credited. The voucher that posted the
// document balances too — this is the same rule one layer down, where a
// sub-ledger that did not balance would be a document whose parts do not add up
// to what it is owed.
func CheckLines(d Document, lines []Line) error {
	if !d.Total.Currency().Valid() {
		return fmt.Errorf("%w: document %s is kept in no currency", ErrInvalidDocument, d.ID)
	}
	if len(lines) == 0 {
		return fmt.Errorf("%w: document %s has no lines", ErrInvalidDocument, d.ID)
	}

	currency := d.Total.Currency()
	debits := money.New(0, currency)
	credits := money.New(0, currency)

	for i, line := range lines {
		if line.DocumentID != d.ID {
			return fmt.Errorf("%w: line %d belongs to document %q, not %s",
				ErrInvalidLine, i+1, line.DocumentID, d.ID)
		}
		if err := line.Validate(currency); err != nil {
			return fmt.Errorf("document %s line %d: %w", d.ID, i+1, err)
		}

		debits = debits.Add(line.Debit)
		credits = credits.Add(line.Credit)
	}

	if !sameAmount(debits, credits) {
		return fmt.Errorf("%w: document %s debits %s and credits %s",
			ErrUnbalanced, d.ID, debits, credits)
	}

	return nil
}

// sameAmount reports whether two amounts are the same money.
//
// Currency first: 500 in one currency and 500 in another are not equal, and
// treating them as equal is the one mistake a ledger cannot recover from.
func sameAmount(a, b money.Money) bool {
	return a.Currency() == b.Currency() && a.Amount() == b.Amount()
}
