package voucher

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"fingo/internal/money"
)

var (
	// ErrUnbalanced reports a voucher whose debits do not equal its credits.
	ErrUnbalanced = errors.New("the voucher's debits do not equal its credits")

	// ErrNoLines reports a voucher with no movements in it.
	ErrNoLines = errors.New("a voucher needs at least one line")

	// ErrLineWithoutSide reports a line that is neither a debit nor a credit.
	ErrLineWithoutSide = errors.New("a line must be a debit or a credit, not neither")

	// ErrLineWithBothSides reports a line that is both at once.
	ErrLineWithBothSides = errors.New("a line must be a debit or a credit, not both")

	// ErrNegativeAmount reports a negative amount, which is the other side
	// written in the wrong place.
	ErrNegativeAmount = errors.New("a line amount must not be negative")

	// ErrMixedCurrency reports lines in more than one currency.
	ErrMixedCurrency = errors.New("a voucher's lines must all be in one currency")
)

// Line is one account movement: one account, one side, one amount.
//
// The side is whichever of Debit and Credit is set; both, or neither, is
// refused. This is deliberately not a debit/credit pair — a pair cannot express
// an entry with two credits, and every VAT sale is one. See
// docs/adr/0001-journal-line-model.md.
type Line struct {
	AccountID string
	Debit     money.Money
	Credit    money.Money
}

// ValidateLine reports whether the line is a single movement.
func (l Line) ValidateLine() error {
	if strings.TrimSpace(l.AccountID) == "" {
		return errors.New("a line needs an account")
	}

	debit, credit := l.Debit.Amount(), l.Credit.Amount()

	if debit < 0 || credit < 0 {
		return fmt.Errorf("%w: line on account %s", ErrNegativeAmount, l.AccountID)
	}

	switch {
	case debit == 0 && credit == 0:
		return fmt.Errorf("%w: line on account %s", ErrLineWithoutSide, l.AccountID)
	case debit > 0 && credit > 0:
		return fmt.Errorf("%w: line on account %s", ErrLineWithBothSides, l.AccountID)
	}

	return nil
}

// Voucher is a numbered, dated set of balanced account movements.
type Voucher struct {
	ID          string
	Number      string
	Date        time.Time
	Description string

	// PeriodID names the accounting period the voucher falls in, when the client
	// has set periods up. Empty is allowed: a client with no calendar must still
	// be able to keep books.
	PeriodID string

	Lines []Line
}

// Validate reports whether the voucher may be persisted.
func (v Voucher) Validate() error {
	if strings.TrimSpace(v.ID) == "" {
		return errors.New("a voucher needs an id")
	}
	if strings.TrimSpace(v.Number) == "" {
		return errors.New("a voucher needs a number")
	}
	if v.Date.IsZero() {
		return errors.New("a voucher needs a date")
	}
	if strings.TrimSpace(v.Description) == "" {
		return errors.New("a voucher needs a description: a line with no explanation cannot be defended")
	}

	return v.ValidateBalance()
}

// ValidateBalance reports whether the debits equal the credits.
//
// A voucher that cannot be balanced at all — no lines, a line with no side, a
// negative amount, two currencies — is refused for that reason rather than as
// "unbalanced". Saying a voucher is out by some amount when it is really
// malformed sends the reader looking for an arithmetic mistake that is not there.
func (v Voucher) ValidateBalance() error {
	if len(v.Lines) == 0 {
		return ErrNoLines
	}

	for i, l := range v.Lines {
		if err := l.ValidateLine(); err != nil {
			return fmt.Errorf("line %d: %w", i+1, err)
		}
	}

	debits, err := v.TotalDebits()
	if err != nil {
		return err
	}

	credits, err := v.TotalCredits()
	if err != nil {
		return err
	}

	if debits.Amount() != credits.Amount() {
		return fmt.Errorf("%w: debits are %d and credits are %d, out by %d",
			ErrUnbalanced, debits.Amount(), credits.Amount(), debits.Amount()-credits.Amount())
	}

	return nil
}

// TotalDebits is the sum of the debit side.
func (v Voucher) TotalDebits() (money.Money, error) { return v.total(true) }

// TotalCredits is the sum of the credit side.
func (v Voucher) TotalCredits() (money.Money, error) { return v.total(false) }

// total sums one side.
//
// The currency is checked first, before anything is added: money.Money panics on
// a mismatch, and a panic is not how a caller should find out their voucher is
// wrong.
func (v Voucher) total(debit bool) (money.Money, error) {
	currency, err := v.currency()
	if err != nil {
		return money.Money{}, err
	}

	sum := money.Money{}
	if currency != "" {
		sum = money.New(0, currency)
	}

	for _, l := range v.Lines {
		amount := l.Credit
		if debit {
			amount = l.Debit
		}

		if amount.IsZero() {
			continue
		}

		sum = sum.Add(amount)
	}

	return sum, nil
}

// currency returns the one currency the voucher's lines are in.
//
// An unset side holds money.Money's zero value, which names no currency at all,
// so it is skipped — otherwise every line would look like a mixture of VND and
// nothing.
func (v Voucher) currency() (money.Currency, error) {
	found := money.Currency("")

	for _, l := range v.Lines {
		for _, amount := range []money.Money{l.Debit, l.Credit} {
			if amount.IsZero() || amount.Currency() == "" {
				continue
			}

			if found == "" {
				found = amount.Currency()
				continue
			}

			if found != amount.Currency() {
				return "", fmt.Errorf("%w: %s and %s", ErrMixedCurrency, found, amount.Currency())
			}
		}
	}

	return found, nil
}
