package tax

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Kind is the VAT treatment of a sale line.
//
// A treatment, not a percentage. The percentage follows from the treatment and
// the document date — a temporarily-reduced line is 8% only while the reduction
// is in force. Carrying a percentage on the line would let a caller type 7, and
// nothing downstream could tell that it was illegal.
type Kind string

const (
	// KindStandard is the ordinary rate, 10%.
	KindStandard Kind = "STANDARD"

	// KindReduced is the 5% list.
	KindReduced Kind = "REDUCED"

	// KindTemporarilyReduced is the 8% reduction, available only inside the
	// window the National Assembly resolution sets.
	KindTemporarilyReduced Kind = "TEMPORARILY_REDUCED"

	// KindZero covers exports and the services listed at 0%.
	KindZero Kind = "ZERO"

	// KindExempt is không chịu thuế — outside the scope of VAT.
	KindExempt Kind = "EXEMPT"

	// KindNotDeclared is không kê khai, không tính thuế — not declared and not
	// charged. Distinct from Exempt on the return, which is why they are
	// separate treatments rather than both being "zero".
	KindNotDeclared Kind = "NOT_DECLARED"
)

func (k Kind) Valid() bool {
	switch k {
	case KindStandard, KindReduced, KindTemporarilyReduced, KindZero, KindExempt, KindNotDeclared:
		return true
	}
	return false
}

// Rate is a VAT treatment in force over a period.
type Rate struct {
	Kind    Kind
	Percent decimal.Decimal
	From    time.Time // inclusive; zero means from the beginning
	To      time.Time // inclusive; zero means until replaced
}

// InForceOn reports whether the treatment applies to a document dated on this
// day. Both ends are inclusive, and the whole of each day counts: a window
// ending 31 December covers 31 December at 23:00, not just its first instant.
//
// The comparison is by calendar day because a rate window is expressed in days.
// Comparing instants would silently exclude everything after midnight on the
// closing date — which is every real document of that day.
func (r Rate) InForceOn(date time.Time) bool {
	d := truncateToDay(date)

	if !r.From.IsZero() && d.Before(truncateToDay(r.From)) {
		return false
	}
	if !r.To.IsZero() && d.After(truncateToDay(r.To)) {
		return false
	}

	return true
}

// truncateToDay drops the time of day, in UTC.
func truncateToDay(t time.Time) time.Time {
	u := t.UTC()

	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// Schedule is the set of VAT treatments that apply, by date.
type Schedule struct {
	rates []Rate
}

func NewSchedule(rates []Rate) *Schedule {
	held := make([]Rate, len(rates))
	copy(held, rates)

	return &Schedule{rates: held}
}

// DefaultSchedule is the statutory VAT schedule for the TT133 regime.
//
// Sources — see docs/legal-basis.md:
//   - the rates at Luật Thuế GTGT 48/2024/QH15, Điều 9
//   - the 0% list and its exclusions at Điều 9 khoản 1
//   - the non-taxable list at Điều 5
//   - the 8% reduction at Nghị quyết 204/2025/QH15, with Nghị định
//     174/2025/NĐ-CP implementing it
//
// THE 8% END DATE IS CONFIGURATION, NOT A FACT THIS CODE OWNS. Nghị quyết
// 204/2025/QH15 sets it at 31/12/2026, but no instrument extending the
// reduction past that date was found — and not finding one is not evidence that
// none was issued. Re-verify it against the gazette before relying on it; if it
// has moved, this table is the only thing that changes.
//
// The exclusions from the reduction — telecom, finance, banking, securities,
// insurance, real estate, metal products, mining other than coal, and excise
// goods other than petrol — are a property of the GOODS, not of the rate. They
// belong on the item, and are not modelled here.
func DefaultSchedule() *Schedule {
	return NewSchedule([]Rate{
		{Kind: KindStandard, Percent: decimal.NewFromInt(10)},
		{Kind: KindReduced, Percent: decimal.NewFromInt(5)},
		{Kind: KindZero, Percent: decimal.Zero},
		{Kind: KindExempt, Percent: decimal.Zero},
		{Kind: KindNotDeclared, Percent: decimal.Zero},
		{
			Kind:    KindTemporarilyReduced,
			Percent: decimal.NewFromInt(8),
			From:    time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
			To:      time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		},
	})
}

// Rates returns the treatments the schedule holds, so an operator can see what
// is configured without reading the source.
func (s *Schedule) Rates() []Rate {
	out := make([]Rate, len(s.rates))
	copy(out, s.rates)

	return out
}

// On returns the treatments available for a document dated on this day.
func (s *Schedule) On(date time.Time) []Rate {
	var out []Rate

	for _, r := range s.rates {
		if r.InForceOn(date) {
			out = append(out, r)
		}
	}

	return out
}

// Percent resolves a treatment to its rate on a given day.
//
// An unavailable treatment is refused rather than defaulted: defaulting would
// silently tax a sale that should not be taxed, or charge 8% after the
// reduction has lapsed.
func (s *Schedule) Percent(kind Kind, date time.Time) (decimal.Decimal, error) {
	if !kind.Valid() {
		return decimal.Zero, fmt.Errorf("VAT treatment %q is not one of the known kinds", kind)
	}

	for _, r := range s.rates {
		if r.Kind == kind && r.InForceOn(date) {
			return r.Percent, nil
		}
	}

	return decimal.Zero, fmt.Errorf("VAT treatment %s is not available for a document dated %s",
		kind, date.UTC().Format("2006-01-02"))
}
