// Package period holds a client's accounting periods and the lock that stops
// entries being posted into them.
//
// A period is a stretch of dates, not a month: a client's calendar is whatever
// they set up. It must be contiguous and free of overlap, so that every date
// belongs to exactly one period — without that, a transaction's date could fall
// in two periods, or in none, and nothing could say whether posting to it is
// allowed.
package period

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

var (
	// ErrPeriodLocked is returned when a transaction's date falls in a period
	// that has been locked.
	ErrPeriodLocked = errors.New("the accounting period is locked")

	// ErrNoPeriod is returned when no period covers a date.
	//
	// Refused rather than waved through: if nothing says which period a date
	// belongs to, nothing can say whether that period is locked.
	ErrNoPeriod = errors.New("no accounting period covers this date")

	// ErrOverlap reports two periods that share a day.
	ErrOverlap = errors.New("accounting periods must not overlap")

	// ErrGap reports periods with uncovered days between them.
	ErrGap = errors.New("accounting periods must be contiguous")

	// ErrAlreadyLocked reports a second attempt to lock a period.
	ErrAlreadyLocked = errors.New("the accounting period is already locked")

	// ErrUnknownPeriod reports an id the calendar does not hold.
	ErrUnknownPeriod = errors.New("no accounting period has that id")
)

// Period is a stretch of accounting time, and whether it may still be posted to.
type Period struct {
	ID        string
	StartDate time.Time
	EndDate   time.Time
	Locked    bool
	LockedAt  *time.Time
}

// Validate reports whether the period describes a usable range.
func (p Period) Validate() error {
	if p.ID == "" {
		return errors.New("an accounting period needs an id")
	}
	if p.StartDate.IsZero() {
		return fmt.Errorf("accounting period %s has no start date", p.ID)
	}
	if p.EndDate.IsZero() {
		return fmt.Errorf("accounting period %s has no end date", p.ID)
	}
	if day(p.EndDate).Before(day(p.StartDate)) {
		return fmt.Errorf("accounting period %s ends %s, before it starts %s",
			p.ID, day(p.EndDate).Format(time.DateOnly), day(p.StartDate).Format(time.DateOnly))
	}

	return nil
}

// Covers reports whether a date falls in the period.
//
// Both ends are inclusive and the whole of each day counts, so a transaction at
// 23:00 on the closing date is inside it.
func (p Period) Covers(date time.Time) bool {
	d := day(date)

	return !d.Before(day(p.StartDate)) && !d.After(day(p.EndDate))
}

// day drops the time of day, in UTC.
//
// A period is a range of DATES, so every comparison here is by calendar day.
// Comparing instants would put a transaction at 09:00 on the closing date
// outside its own period — which is every real transaction of that day.
func day(t time.Time) time.Time {
	u := t.UTC()

	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// Calendar is a client's periods, kept contiguous and free of overlap.
type Calendar struct {
	periods []Period
}

// NewCalendar builds a calendar, refusing a set that is not a valid sequence.
func NewCalendar(periods []Period) (*Calendar, error) {
	c := &Calendar{}

	for _, p := range periods {
		if err := c.Add(p); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// Periods returns the calendar, oldest first.
func (c *Calendar) Periods() []Period {
	out := make([]Period, len(c.periods))
	copy(out, c.periods)

	return out
}

// Add inserts a period, refusing one that would overlap or leave a gap.
//
// The whole set is re-checked rather than only the new period's neighbours, so
// a period can be added anywhere in the sequence and the rule holds either way.
// A refused add changes nothing.
func (c *Calendar) Add(p Period) error {
	if err := p.Validate(); err != nil {
		return err
	}

	next := make([]Period, 0, len(c.periods)+1)
	next = append(next, c.periods...)
	next = append(next, p)

	sortByStart(next)

	if err := checkSequence(next); err != nil {
		return err
	}

	c.periods = next

	return nil
}

// Find returns the period covering a date.
func (c *Calendar) Find(date time.Time) (Period, error) {
	for _, p := range c.periods {
		if p.Covers(date) {
			return p, nil
		}
	}

	return Period{}, fmt.Errorf("%w: %s", ErrNoPeriod, day(date).Format(time.DateOnly))
}

// Current returns the period a posting dated today would land in.
//
// When no period covers today it falls back to the latest period there is, and
// reports true. A client whose calendar has not been extended into the new year
// still holds last year, and that is where the books were left — a caller asking
// "where do things stand" is better served by that period than by nothing. A
// calendar with no periods at all reports false.
//
// The fallback is the whole of the difference from Calendar.Find, which refuses
// a date no period covers. Find is right to refuse: a posting must not be filed
// into a period that does not describe it. Reading is the other question, and it
// has a different answer.
func Current(periods []Period, today time.Time) (Period, bool) {
	for _, p := range periods {
		if p.Covers(today) {
			return p, true
		}
	}

	if len(periods) == 0 {
		return Period{}, false
	}

	latest := periods[0]
	for _, p := range periods {
		if day(p.StartDate).After(day(latest.StartDate)) {
			latest = p
		}
	}

	return latest, true
}

// Lock marks a period locked, recording when.
//
// There is no counterpart. Locking is irreversible in v1: a period that could be
// unlocked is not a lock, it is a preference, and a reported figure could then
// change underneath the report that relied on it.
func (c *Calendar) Lock(id string, at time.Time) error {
	for i := range c.periods {
		if c.periods[i].ID != id {
			continue
		}

		if c.periods[i].Locked {
			return fmt.Errorf("%w: %s", ErrAlreadyLocked, id)
		}

		lockedAt := at.UTC()
		c.periods[i].Locked = true
		c.periods[i].LockedAt = &lockedAt

		return nil
	}

	return fmt.Errorf("%w: %s", ErrUnknownPeriod, id)
}

// sortByStart orders periods oldest first.
func sortByStart(periods []Period) {
	sort.SliceStable(periods, func(i, j int) bool {
		return day(periods[i].StartDate).Before(day(periods[j].StartDate))
	})
}

// checkSequence verifies that a sorted run of periods covers its span with no
// overlap, no gap and no repeated id.
func checkSequence(periods []Period) error {
	seen := make(map[string]bool, len(periods))

	for i, p := range periods {
		if seen[p.ID] {
			return fmt.Errorf("two accounting periods share the id %s", p.ID)
		}
		seen[p.ID] = true

		if i == 0 {
			continue
		}

		previous := periods[i-1]

		if !day(p.StartDate).After(day(previous.EndDate)) {
			return fmt.Errorf("%w: %s ends %s but %s starts %s",
				ErrOverlap,
				previous.ID, day(previous.EndDate).Format(time.DateOnly),
				p.ID, day(p.StartDate).Format(time.DateOnly))
		}

		expected := day(previous.EndDate).AddDate(0, 0, 1)
		if !day(p.StartDate).Equal(expected) {
			return fmt.Errorf("%w: %s ends %s but %s starts %s, leaving %s uncovered",
				ErrGap,
				previous.ID, day(previous.EndDate).Format(time.DateOnly),
				p.ID, day(p.StartDate).Format(time.DateOnly),
				expected.Format(time.DateOnly))
		}
	}

	return nil
}
