// Package vat is the VAT rate table: which rate applies, from when, and what it
// charges on a line.
//
// A rate here is a row with an ID and a number of basis points — never a
// percentage typed onto a transaction line. A percentage on a line is a claim
// about the law rather than a reference to it: nothing downstream can tell a
// legal 8% from a lapsed one, or from a typo, and the same stored line silently
// recomputes to a different tax the moment the schedule moves. A line carries
// the ID of the row it was taxed under, and the rate is read back from that row.
// See Line and Registry.Charge.
//
// Rows are append-only and immutable. A rate change is a NEW row with a new
// EffectiveFrom, never an edit: the tax on an invoice posted last year must
// still compute today to what it computed then. There is deliberately no way to
// rewrite a row — see Registry.Create.
package vat

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// Whole is one hundred per cent expressed in basis points.
const Whole BasisPoints = 10000

// BasisPoints is a VAT rate in hundredths of a per cent: 10000 is 100%, 2000 is
// 20%, 800 is 8%.
//
// A named integer, not a float percentage. The legal rates are 0, 5, 8 and 10
// per cent, and a tax return in which 8 + 2 is 9.999999 is a tax return that
// does not add up.
type BasisPoints int

// Errors a caller may need to tell apart. The overlap and duplicate errors are
// the ones the table's rules are stated in.
var (
	// ErrInvalidRate is a rate that is not a legal rate at all: no ID, outside
	// 0..Whole, no start date, or ending on or before it starts.
	ErrInvalidRate = errors.New("vat: invalid rate")

	// ErrDuplicateRateID is an attempt to create a second row with an ID
	// already in the table. A row is never replaced; a rate change is a new row.
	ErrDuplicateRateID = errors.New("vat: this rate id already exists, and a rate is never rewritten — create a new row with a new effective-from date")

	// ErrOverlappingRate is an attempt to create an active row that would
	// cover a day an active row already covers. One day, one rate.
	ErrOverlappingRate = errors.New("vat: two active rates cannot cover the same date")

	// ErrUnknownRateID is a lookup of an ID the table does not hold.
	ErrUnknownRateID = errors.New("vat: no such rate")

	// ErrNoRateInForce is a date no active row covers — before the table's
	// first row, or in a gap left by a row that declared an end.
	ErrNoRateInForce = errors.New("vat: no rate is in force")

	// ErrUnnamedRate is a VAT line that names no rate row.
	ErrUnnamedRate = errors.New("vat: a line that carries VAT must name the rate it is taxed under")
)

// VATRate is one row of the rate table: a rate, in force from a date.
//
// A row does not change. To change the rate, create a new row with a later
// EffectiveFrom — the new row takes over on its own start date, and nothing is
// ever written back to this one.
type VATRate struct {
	// ID names this row. It is what a transaction line stores, so it must stay
	// stable for the life of the books — usually a UUID.
	ID string

	// Rate is the rate in basis points. 2000 is 20 per cent.
	Rate BasisPoints

	// EffectiveFrom is the first day the row applies. The whole of that day is
	// inside the window, whatever time of day a document carries.
	EffectiveFrom time.Time

	// EffectiveTo is the last day the row applies, inclusive, or nil when the
	// row declares no end of its own.
	//
	// A nil end does not mean the rate runs forever. The row in force on a day
	// is the latest active row that started on or before it, so a later row
	// takes over on its own EffectiveFrom without this row being touched. nil
	// means "until replaced", which is the ordinary case. A non-nil
	// EffectiveTo is the exceptional one: a window that lapses on a date
	// already known, such as the 8 per cent reduction.
	EffectiveTo *time.Time

	// Active reports whether the row governs any date at all. An inactive row
	// is kept for the record — an import, a draft, a superseded schedule — and
	// takes no part in resolving a date or in the overlap checks.
	//
	// Active is set at creation like every other field. Retiring a rate means
	// appending a row that takes over, not editing this one.
	Active bool
}

// Validate reports whether the row could be a legal row of the table, ignoring
// what the table already holds. Registry.Create applies it to everything it is
// asked to store.
func (r VATRate) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("%w: a rate needs an id", ErrInvalidRate)
	}
	if r.Rate < 0 || r.Rate > Whole {
		return fmt.Errorf("%w: rate %s is %d basis points, outside 0..%d",
			ErrInvalidRate, r.ID, int(r.Rate), int(Whole))
	}
	if r.EffectiveFrom.IsZero() {
		return fmt.Errorf("%w: rate %s has no effective-from date", ErrInvalidRate, r.ID)
	}
	if r.EffectiveTo != nil && !truncateToDay(*r.EffectiveTo).After(truncateToDay(r.EffectiveFrom)) {
		return fmt.Errorf("%w: rate %s ends on %s, on or before it starts on %s",
			ErrInvalidRate, r.ID, day(*r.EffectiveTo), day(r.EffectiveFrom))
	}

	return nil
}

// InForceOn reports whether this row's own declared window covers a date. Both
// ends are inclusive and the whole of each day counts, so a window ending 31
// December covers 31 December at 23:00 and not merely its first instant.
//
// The declared window is not the whole story: a row with no EffectiveTo is
// overtaken by any later row, and this method does not know about later rows.
// To ask the table which rate applies on a day, ask Registry.RateOn.
func (r VATRate) InForceOn(date time.Time) bool {
	d := truncateToDay(date)

	if d.Before(truncateToDay(r.EffectiveFrom)) {
		return false
	}
	if r.EffectiveTo != nil && d.After(truncateToDay(*r.EffectiveTo)) {
		return false
	}

	return true
}

// Tax returns the VAT a net amount carries at this rate, in the same minor unit
// as the amount.
//
//	tax = netAmount × Rate ÷ 10000
//
// All of it is integer arithmetic. The division rounds half away from zero —
// the same direction as money.Dong — so a remainder of exactly half a minor
// unit goes up on a debit and down on a credit: 2 at 2500bp is 1, and -2 is -1.
// Rounding is per line, so a document total is the sum of its rounded lines and
// never disagrees with its own lines.
//
// The product is wider than an int64 for a large enough line (10^15 minor units
// at 10 per cent), so it is formed in arbitrary precision rather than wrapping
// round into a wrong tax figure.
//
// Panics if Rate is outside 0..Whole. That is a programming error rather than a
// runtime condition: a rate reaches a caller through Registry, which refuses it
// at creation.
func (r VATRate) Tax(netAmount int64) int64 {
	if r.Rate < 0 || r.Rate > Whole {
		panic(fmt.Sprintf("vat: rate %s is %d basis points, outside 0..%d",
			r.ID, int(r.Rate), int(Whole)))
	}

	return taxOn(netAmount, r.Rate)
}

// taxOn computes netAmount × rate ÷ 10000, rounded half away from zero, without
// overflowing on the way.
func taxOn(netAmount int64, rate BasisPoints) int64 {
	product := new(big.Int).Mul(big.NewInt(netAmount), big.NewInt(int64(rate)))
	divisor := big.NewInt(int64(Whole))

	// QuoRem truncates toward zero, so the remainder carries the sign of the
	// product and "half away from zero" is a comparison of magnitudes.
	quotient, remainder := new(big.Int).QuoRem(product, divisor, new(big.Int))

	if new(big.Int).Abs(remainder).Int64()*2 >= int64(Whole) {
		if product.Sign() < 0 {
			quotient.Sub(quotient, big.NewInt(1))
		} else {
			quotient.Add(quotient, big.NewInt(1))
		}
	}

	return quotient.Int64()
}

// Line is one transaction line, as far as VAT is concerned.
//
// There is no percentage on it, and that is the point. A line states which rate
// ROW it was taxed under; the rate itself is read back from that row, so the
// tax on a stored line is reproducible years later whatever the table has
// gained since.
type Line struct {
	// NetAmount is the line value exclusive of VAT, in minor units.
	NetAmount int64

	// VATRateID names the row this line is taxed under. Required on a line that
	// touches VAT: a line taxed at nothing and a line nobody named a rate for
	// are different facts, and the second is refused rather than treated as the
	// first.
	VATRateID string
}

// Charge is the VAT one line carries: the row it was read from, and the tax.
type Charge struct {
	Line Line
	Rate VATRate
	Tax  int64
}

// Gross returns the line total, VAT included.
func (c Charge) Gross() int64 { return c.Line.NetAmount + c.Tax }

// truncateToDay drops the time of day, in UTC. A rate window is expressed in
// days, so comparing instants would silently exclude everything after midnight
// on a window's closing date — which is every real document of that day.
func truncateToDay(t time.Time) time.Time {
	u := t.UTC()

	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// day renders a date for an error message. Dates in messages are read by the
// operator who has to fix the table, not by the machine.
func day(t time.Time) string {
	return truncateToDay(t).Format("2006-01-02")
}
