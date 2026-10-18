package period

import (
	"context"
	"fmt"
	"time"
)

// Repository stores a client's accounting periods.
type Repository interface {
	// List returns every period, so a caller can build a Calendar.
	List(ctx context.Context) ([]Period, error)

	// Create stores a period, refusing one that would overlap or leave a gap.
	Create(ctx context.Context, p Period) error

	// Lock marks a period locked.
	//
	// There is deliberately no counterpart: locking is irreversible in v1, and a
	// repository that could undo it would make the guarantee a convention rather
	// than a rule.
	Lock(ctx context.Context, id string, at time.Time) error
}

// Guard checks a posting date against a client's periods.
type Guard struct {
	periods Repository
}

func NewGuard(periods Repository) *Guard {
	return &Guard{periods: periods}
}

// covering returns the period covering a date, if any.
func (g *Guard) covering(ctx context.Context, date time.Time) (Period, bool, error) {
	periods, err := g.periods.List(ctx)
	if err != nil {
		return Period{}, false, fmt.Errorf("load accounting periods: %w", err)
	}

	for _, p := range periods {
		if p.Covers(date) {
			return p, true, nil
		}
	}

	return Period{}, false, nil
}

// CheckPosting reports whether a transaction dated then may be posted, and
// insists that the date belongs to a period at all.
//
// It returns ErrPeriodLocked if the period covering the date is locked, and
// ErrNoPeriod if no period covers it. Use it where the calendar must be
// complete — a boot check, a report, a reconciliation.
func (g *Guard) CheckPosting(ctx context.Context, transactionDate time.Time) error {
	covering, found, err := g.covering(ctx, transactionDate)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("%w: %s", ErrNoPeriod, day(transactionDate).Format(time.DateOnly))
	}

	if covering.Locked {
		return fmt.Errorf("%w: %s covers %s",
			ErrPeriodLocked, covering.ID, day(transactionDate).Format(time.DateOnly))
	}

	return nil
}

// CheckLocked refuses a posting date that falls in a locked period, and lets
// everything else through.
//
// This is the check a posting makes. It is deliberately weaker than
// CheckPosting: a date no period covers is allowed. A client who has not set
// their calendar up yet must still be able to keep books, and what this guards
// against is an entry into a period somebody has deliberately shut — not an
// entry into one nobody has described.
//
// Refusing the latter would mean an empty calendar blocked every posting, which
// would make adopting periods impossible without first back-filling one for
// every client.
func (g *Guard) CheckLocked(ctx context.Context, transactionDate time.Time) error {
	covering, found, err := g.covering(ctx, transactionDate)
	if err != nil {
		return err
	}

	if found && covering.Locked {
		return fmt.Errorf("%w: %s covers %s",
			ErrPeriodLocked, covering.ID, day(transactionDate).Format(time.DateOnly))
	}

	return nil
}

// StaticRepository is a fixed set of periods that applies the same rules as a
// real repository and then discards the result.
//
// It exists so that a caller already holding a calendar — a test, or a guard
// reading periods it has just loaded — does not need a database to check a rule.
// Applying the rules without storing them is enough to test a decision and not
// enough to keep one.
type StaticRepository struct {
	Periods []Period
}

func (r StaticRepository) List(context.Context) ([]Period, error) {
	out := make([]Period, len(r.Periods))
	copy(out, r.Periods)

	return out, nil
}

func (r StaticRepository) Create(_ context.Context, p Period) error {
	calendar, err := NewCalendar(r.Periods)
	if err != nil {
		return err
	}

	return calendar.Add(p)
}

func (r StaticRepository) Lock(_ context.Context, id string, at time.Time) error {
	calendar, err := NewCalendar(r.Periods)
	if err != nil {
		return err
	}

	return calendar.Lock(id, at)
}
