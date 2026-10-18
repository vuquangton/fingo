package gl

import (
	"fmt"
	"sort"
	"time"
)

// YearMonth identifies an accounting period.
type YearMonth struct {
	Year  int
	Month time.Month
}

func (ym YearMonth) String() string {
	return fmt.Sprintf("%04d-%02d", ym.Year, int(ym.Month))
}

// MonthOf returns the accounting period a date falls in, read in UTC.
//
// UTC because the ledger records instants in UTC (see ADR-0002). Interpreting a
// date in local time would move entries between periods on a machine in a
// different timezone — and a period boundary is exactly where that matters.
func MonthOf(t time.Time) YearMonth {
	u := t.UTC()
	return YearMonth{Year: u.Year(), Month: u.Month()}
}

// Periods records which accounting periods are closed.
//
// Posting into a closed period is refused: a figure that has been reported must
// not change underneath. The closed set is a value rather than a stored flag, so
// the rule can be applied without the domain reaching into storage — and so a
// test can state exactly which periods are shut.
type Periods struct {
	closed map[YearMonth]struct{}
}

func NewPeriods() *Periods {
	return &Periods{closed: make(map[YearMonth]struct{})}
}

// Close shuts a period.
//
// Closing one that is already closed is refused: the attempt means the caller
// believed it was still open, which is worth knowing about.
func (p *Periods) Close(ym YearMonth) error {
	if _, already := p.closed[ym]; already {
		return fmt.Errorf("period %s is already closed", ym)
	}

	p.closed[ym] = struct{}{}

	return nil
}

// Reopen is the explicit, deliberate reversal of Close.
func (p *Periods) Reopen(ym YearMonth) error {
	if _, closed := p.closed[ym]; !closed {
		return fmt.Errorf("period %s is not closed", ym)
	}

	delete(p.closed, ym)

	return nil
}

// IsClosed reports whether a period is shut.
func (p *Periods) IsClosed(ym YearMonth) bool {
	_, closed := p.closed[ym]
	return closed
}

// IsOpen reports whether an entry may be dated at this instant.
//
// A nil receiver means no calendar was supplied. That reads as open here and is
// refused by Post, so the refusal happens where it can be explained rather than
// silently allowing the posting.
func (p *Periods) IsOpen(date time.Time) bool {
	if p == nil {
		return true
	}

	return !p.IsClosed(MonthOf(date))
}

// Closed returns the closed periods, oldest first, so callers and error messages
// are deterministic.
func (p *Periods) Closed() []YearMonth {
	out := make([]YearMonth, 0, len(p.closed))
	for period := range p.closed {
		out = append(out, period)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Year != out[j].Year {
			return out[i].Year < out[j].Year
		}
		return out[i].Month < out[j].Month
	})

	return out
}
