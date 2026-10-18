// Package money is an exact currency amount.
//
// Amounts are integers in a currency's MINOR UNIT — the smallest unit that
// currency has. For USD that is the cent. For VND, which has no subunit, it is
// the đồng itself. This package deliberately carries no table saying how many
// minor units make a major one: that is a display concern, and guessing it here
// would put a silent factor of a hundred into every Vietnamese amount.
//
// Integer, never float. 0.1 + 0.2 is not 0.3 in binary floating point, and a
// ledger that cannot add up is not a ledger.
package money

import "fmt"

// Money is an amount of one currency.
//
// The zero value is 0 with no currency yet. That is deliberate and useful: it is
// the identity an accumulator starts from, so
//
//	var total Money
//	total = total.Add(line)
//
// works without anyone having to name a currency up front. A non-zero Money
// always has a currency, because New is the only way to make one.
type Money struct {
	amount   int64
	currency Currency
}

// New returns an amount in a currency's minor unit.
//
// Panics if the currency is not a three-letter code. That is a programming
// error rather than a runtime condition — a code comes from the program, not
// from input — and failing here beats carrying an amount that cannot be totalled.
func New(amount int64, currency Currency) Money {
	if !currency.Valid() {
		panic(fmt.Sprintf("money: %q is not a three-letter currency code", currency))
	}

	return Money{amount: amount, currency: currency}
}

// FromDB rebuilds a Money from the two columns it is stored in: an int64 amount
// and a varchar(3) code.
//
// An amount with no currency is refused, because an amount nobody can name is an
// amount nobody can total. The one exception is zero, which is the zero value
// and carries no currency by design.
func FromDB(amount int64, currency string) (Money, error) {
	if currency == "" {
		if amount != 0 {
			return Money{}, fmt.Errorf("money: an amount of %d has no currency", amount)
		}

		return Money{}, nil
	}

	if !Currency(currency).Valid() {
		return Money{}, fmt.Errorf("money: %q is not a three-letter currency code", currency)
	}

	return Money{amount: amount, currency: Currency(currency)}, nil
}

func (m Money) Amount() int64      { return m.amount }
func (m Money) Currency() Currency { return m.currency }
func (m Money) IsZero() bool       { return m.amount == 0 }
func (m Money) IsNegative() bool   { return m.amount < 0 }

// Add returns the sum.
//
// Panics if the two amounts are in different currencies. The zero value names no
// currency and adopts the other side's, which is what lets an accumulator start
// from nothing.
func (m Money) Add(other Money) Money {
	m.requireSameCurrency(other)

	return Money{amount: m.amount + other.amount, currency: m.currencyWith(other)}
}

// Sub returns the difference. It may go negative, which is a position rather
// than an error.
func (m Money) Sub(other Money) Money {
	m.requireSameCurrency(other)

	return Money{amount: m.amount - other.amount, currency: m.currencyWith(other)}
}

// Mul scales by a whole number, exactly.
func (m Money) Mul(factor int64) Money {
	return Money{amount: m.amount * factor, currency: m.currency}
}

// Div divides by a whole number, returning the quotient and the remainder.
//
// The remainder is exposed rather than rounded away: whether to floor, round or
// carry what is left is the caller's decision, and arithmetic that silently
// discards it loses up to a minor unit every time it runs.
//
// Both are truncated toward zero, so q×divisor + r == the original.
//
// Panics if the divisor is zero.
func (m Money) Div(divisor int64) (Money, Money) {
	if divisor == 0 {
		panic("money: division by zero")
	}

	return Money{amount: m.amount / divisor, currency: m.currency},
		Money{amount: m.amount % divisor, currency: m.currency}
}

// String renders the amount in its minor unit, e.g. "1234 VND". It is not a
// display format: there are no separators and no decimal point, because this
// package does not know how many minor units make a major one.
func (m Money) String() string {
	if m.currency == "" {
		return fmt.Sprintf("%d", m.amount)
	}

	return fmt.Sprintf("%d %s", m.amount, m.currency)
}

// requireSameCurrency panics when both sides name a currency and they differ.
func (m Money) requireSameCurrency(other Money) {
	if m.currency == "" || other.currency == "" {
		return
	}
	if m.currency != other.currency {
		panic(fmt.Sprintf("money: currency mismatch: %s and %s", m.currency, other.currency))
	}
}

// currencyWith returns whichever side names a currency, preferring this one.
func (m Money) currencyWith(other Money) Currency {
	if m.currency != "" {
		return m.currency
	}

	return other.currency
}
