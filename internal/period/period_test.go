package period_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"fingo/internal/period"
)

func day(year int, month time.Month, dayOfMonth int) time.Time {
	return time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.UTC)
}

func at(hour int) time.Time {
	return time.Date(2026, 3, 14, hour, 30, 0, 0, time.UTC)
}

func fy2026() period.Period {
	return period.Period{ID: "fy2026", StartDate: day(2026, 1, 1), EndDate: day(2026, 12, 31)}
}

func fy2027() period.Period {
	return period.Period{ID: "fy2027", StartDate: day(2027, 1, 1), EndDate: day(2027, 12, 31)}
}

// ---------------------------------------------------------------------------
// Contiguity and overlap, enforced on create
// ---------------------------------------------------------------------------

// Two periods that share a day are ambiguous: a transaction on that day belongs
// to both, and the ledger cannot say which lock applies to it.
func TestCalendar_RejectsOverlappingPeriods(t *testing.T) {
	january := period.Period{ID: "jan", StartDate: day(2026, 1, 1), EndDate: day(2026, 1, 31)}
	sharedStart := period.Period{ID: "overlap", StartDate: day(2026, 1, 31), EndDate: day(2026, 2, 28)}

	if _, err := period.NewCalendar([]period.Period{january, sharedStart}); err == nil {
		t.Fatal("overlapping periods must be refused, got nil")
	}

	// Fully contained is an overlap too.
	contained := period.Period{ID: "inside", StartDate: day(2026, 1, 10), EndDate: day(2026, 1, 20)}
	if _, err := period.NewCalendar([]period.Period{january, contained}); err == nil {
		t.Fatal("a period contained in another must be refused, got nil")
	}
}

// A gap is a day that belongs to no period, so nothing could say whether posting
// to it is allowed.
func TestCalendar_RejectsGaps(t *testing.T) {
	january := period.Period{ID: "jan", StartDate: day(2026, 1, 1), EndDate: day(2026, 1, 31)}
	// February starts on the 2nd, leaving the 1st uncovered.
	march := period.Period{ID: "mar", StartDate: day(2026, 2, 2), EndDate: day(2026, 2, 28)}

	if _, err := period.NewCalendar([]period.Period{january, march}); err == nil {
		t.Fatal("a gap between periods must be refused, got nil")
	}
}

func TestCalendar_AcceptsContiguousPeriods(t *testing.T) {
	if _, err := period.NewCalendar([]period.Period{fy2026(), fy2027()}); err != nil {
		t.Fatalf("contiguous periods must be accepted: %v", err)
	}

	// A single period is trivially contiguous.
	if _, err := period.NewCalendar([]period.Period{fy2026()}); err != nil {
		t.Fatalf("a single period must be accepted: %v", err)
	}

	// So is an empty calendar: a client who has not set one up yet.
	if _, err := period.NewCalendar(nil); err != nil {
		t.Fatalf("an empty calendar must be accepted: %v", err)
	}
}

// Order in the input must not matter — the calendar sorts before it checks.
func TestCalendar_AcceptsContiguousPeriodsInAnyOrder(t *testing.T) {
	if _, err := period.NewCalendar([]period.Period{fy2027(), fy2026()}); err != nil {
		t.Fatalf("contiguous periods given out of order must be accepted: %v", err)
	}
}

func TestCalendar_Add_ExtendsAContiguousCalendar(t *testing.T) {
	calendar, err := period.NewCalendar([]period.Period{fy2026()})
	if err != nil {
		t.Fatalf("NewCalendar: %v", err)
	}

	if err := calendar.Add(fy2027()); err != nil {
		t.Fatalf("adding the adjacent year must be accepted: %v", err)
	}

	if got := len(calendar.Periods()); got != 2 {
		t.Errorf("periods = %d, want 2", got)
	}
}

func TestCalendar_Add_RejectsOverlapAndGap(t *testing.T) {
	calendar, err := period.NewCalendar([]period.Period{fy2026()})
	if err != nil {
		t.Fatalf("NewCalendar: %v", err)
	}

	overlap := period.Period{ID: "overlap", StartDate: day(2026, 6, 1), EndDate: day(2027, 6, 30)}
	if err := calendar.Add(overlap); err == nil {
		t.Error("adding an overlapping period must be refused, got nil")
	}

	gap := period.Period{ID: "gap", StartDate: day(2028, 1, 1), EndDate: day(2028, 12, 31)}
	if err := calendar.Add(gap); err == nil {
		t.Error("adding a period that leaves a gap must be refused, got nil")
	}

	// Neither attempt changed anything.
	if got := len(calendar.Periods()); got != 1 {
		t.Errorf("periods = %d after refused adds, want 1", got)
	}
}

func TestPeriod_ValidateRequiresAnIDAndAnOrderedRange(t *testing.T) {
	cases := map[string]period.Period{
		"no id":     {StartDate: day(2026, 1, 1), EndDate: day(2026, 12, 31)},
		"backwards": {ID: "x", StartDate: day(2026, 12, 31), EndDate: day(2026, 1, 1)},
		"no start":  {ID: "x", EndDate: day(2026, 12, 31)},
		"no end":    {ID: "x", StartDate: day(2026, 1, 1)},
	}

	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			if err := p.Validate(); err == nil {
				t.Fatalf("%s must be refused, got nil", name)
			}
		})
	}

	if err := fy2026().Validate(); err != nil {
		t.Fatalf("a well-formed period must be accepted: %v", err)
	}
}

// ---------------------------------------------------------------------------
// The posting gate
// ---------------------------------------------------------------------------

// Posting to a locked period → ErrPeriodLocked.
func TestCheckPosting_RefusesALockedPeriod(t *testing.T) {
	locked := fy2026()
	locked.Locked = true
	lockedAt := day(2026, 3, 1)
	locked.LockedAt = &lockedAt

	guard := period.NewGuard(period.StaticRepository{Periods: []period.Period{locked, fy2027()}})

	err := guard.CheckPosting(context.Background(), day(2026, 6, 15))
	if err == nil {
		t.Fatal("posting into a locked period must be refused, got nil")
	}
	if !errors.Is(err, period.ErrPeriodLocked) {
		t.Fatalf("error = %v, want ErrPeriodLocked", err)
	}
}

// Posting to an open period succeeds.
func TestCheckPosting_AllowsAnOpenPeriod(t *testing.T) {
	guard := period.NewGuard(period.StaticRepository{Periods: []period.Period{fy2026(), fy2027()}})

	if err := guard.CheckPosting(context.Background(), day(2026, 6, 15)); err != nil {
		t.Fatalf("posting into an open period must be allowed: %v", err)
	}
}

// A date no period covers is refused rather than waved through: if nothing says
// which period it belongs to, nothing can say whether that period is locked.
func TestCheckPosting_RefusesADateNoPeriodCovers(t *testing.T) {
	guard := period.NewGuard(period.StaticRepository{Periods: []period.Period{fy2026()}})

	err := guard.CheckPosting(context.Background(), day(2030, 6, 15))
	if !errors.Is(err, period.ErrNoPeriod) {
		t.Fatalf("error = %v, want ErrNoPeriod", err)
	}
}

// The first and last day of a period belong to it, whatever time of day the
// transaction carries. Comparing instants would exclude everything after
// midnight on the closing date — which is every real transaction of that day.
func TestCheckPosting_CoversWholeDays(t *testing.T) {
	guard := period.NewGuard(period.StaticRepository{Periods: []period.Period{fy2026()}})

	for _, moment := range []time.Time{
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 23, 59, 59, 0, time.UTC),
		time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
	} {
		if err := guard.CheckPosting(context.Background(), moment); err != nil {
			t.Errorf("%s is inside the period and must be allowed: %v", moment, err)
		}
	}

	// And the day either side is not covered.
	for _, moment := range []time.Time{
		time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
		time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		if err := guard.CheckPosting(context.Background(), moment); !errors.Is(err, period.ErrNoPeriod) {
			t.Errorf("%s is outside every period, want ErrNoPeriod, got %v", moment, err)
		}
	}
}

// A locked period among open ones only blocks its own dates.
func TestCheckPosting_OnlyBlocksTheLockedPeriod(t *testing.T) {
	locked := fy2026()
	locked.Locked = true

	guard := period.NewGuard(period.StaticRepository{Periods: []period.Period{locked, fy2027()}})

	if err := guard.CheckPosting(context.Background(), day(2026, 6, 15)); !errors.Is(err, period.ErrPeriodLocked) {
		t.Errorf("2026 must be locked, got %v", err)
	}
	if err := guard.CheckPosting(context.Background(), day(2027, 6, 15)); err != nil {
		t.Errorf("2027 is open and must be allowed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Locking, and its irreversibility
// ---------------------------------------------------------------------------

func TestCalendar_Lock_SetsTheFlagAndTheTimestamp(t *testing.T) {
	calendar, err := period.NewCalendar([]period.Period{fy2026()})
	if err != nil {
		t.Fatalf("NewCalendar: %v", err)
	}

	when := at(14)

	if err := calendar.Lock("fy2026", when); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	found, err := calendar.Find(day(2026, 6, 15))
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if !found.Locked {
		t.Error("the period must be locked")
	}
	if found.LockedAt == nil || !found.LockedAt.Equal(when) {
		t.Errorf("LockedAt = %v, want %v", found.LockedAt, when)
	}
}

// Locking an already locked period is a mistake, not a no-op: the second attempt
// means the caller believed it was open.
func TestCalendar_Lock_RefusesASecondLock(t *testing.T) {
	calendar, err := period.NewCalendar([]period.Period{fy2026()})
	if err != nil {
		t.Fatalf("NewCalendar: %v", err)
	}

	if err := calendar.Lock("fy2026", at(9)); err != nil {
		t.Fatalf("first Lock: %v", err)
	}

	if err := calendar.Lock("fy2026", at(15)); !errors.Is(err, period.ErrAlreadyLocked) {
		t.Fatalf("second Lock = %v, want ErrAlreadyLocked", err)
	}

	// The original timestamp stands: a failed re-lock must not move it.
	found, err := calendar.Find(day(2026, 6, 15))
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if !found.LockedAt.Equal(at(9)) {
		t.Errorf("LockedAt = %v, want the first lock's %v", found.LockedAt, at(9))
	}
}

func TestCalendar_Lock_RefusesAnUnknownPeriod(t *testing.T) {
	calendar, err := period.NewCalendar([]period.Period{fy2026()})
	if err != nil {
		t.Fatalf("NewCalendar: %v", err)
	}

	if err := calendar.Lock("nope", at(9)); !errors.Is(err, period.ErrUnknownPeriod) {
		t.Fatalf("Lock of an unknown period = %v, want ErrUnknownPeriod", err)
	}
}

// Locking is irreversible (no unlock method exists).
//
// Irreversibility is a property of the API, so it is checked as one: there must
// be no way to clear a lock. A test that merely called Lock twice would not
// notice an Unlock being added later.
func TestLocking_IsIrreversible(t *testing.T) {
	forbidden := []string{"unlock", "reopen", "clear", "remove", "delete"}

	for _, typ := range []reflect.Type{
		reflect.TypeOf(period.Calendar{}),
		reflect.TypeOf(period.Period{}),
		reflect.TypeOf(&period.Calendar{}),
	} {
		for i := 0; i < typ.NumMethod(); i++ {
			name := typ.Method(i).Name
			lower := strings.ToLower(name)

			for _, word := range forbidden {
				if strings.Contains(lower, word) {
					t.Errorf("%s has method %s; locking must be irreversible in v1",
						typ, name)
				}
			}
		}
	}

	// The repository is the other way in, and it must not offer one either.
	repo := reflect.TypeOf((*period.Repository)(nil)).Elem()
	for i := 0; i < repo.NumMethod(); i++ {
		name := strings.ToLower(repo.Method(i).Name)
		for _, word := range forbidden {
			if strings.Contains(name, word) {
				t.Errorf("period.Repository has method %s; locking must be irreversible in v1",
					repo.Method(i).Name)
			}
		}
	}
}

// Once locked, a period stays locked — through the guard, and through a
// re-read of the same calendar.
func TestLock_SurvivesFurtherUse(t *testing.T) {
	calendar, err := period.NewCalendar([]period.Period{fy2026()})
	if err != nil {
		t.Fatalf("NewCalendar: %v", err)
	}
	if err := calendar.Lock("fy2026", at(9)); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Nothing the calendar offers can put it back.
	if err := calendar.Add(fy2027()); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := calendar.Find(day(2026, 6, 15)); err != nil {
		t.Fatalf("Find: %v", err)
	}

	found, err := calendar.Find(day(2026, 6, 15))
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if !found.Locked {
		t.Error("the period came back unlocked")
	}
}

// ---------------------------------------------------------------------------
// CheckLocked, the check a posting makes
// ---------------------------------------------------------------------------

// A locked period is refused. This is the same answer CheckPosting gives, and
// the reason the ledger can use this one.
func TestCheckLocked_RefusesALockedPeriod(t *testing.T) {
	locked := fy2026()
	locked.Locked = true

	guard := period.NewGuard(period.StaticRepository{Periods: []period.Period{locked, fy2027()}})

	if err := guard.CheckLocked(context.Background(), day(2026, 6, 15)); !errors.Is(err, period.ErrPeriodLocked) {
		t.Fatalf("a locked period = %v, want ErrPeriodLocked", err)
	}

	// And the open year beside it is unaffected.
	if err := guard.CheckLocked(context.Background(), day(2027, 6, 15)); err != nil {
		t.Errorf("the open year must be allowed: %v", err)
	}
}

// A date no period covers is allowed through. This is the one place CheckLocked
// and CheckPosting disagree, and it is deliberate.
//
// A client who has not set their calendar up yet must still be able to keep
// books. Refusing here would mean an empty calendar blocked every posting, and
// adopting periods would require back-filling one for every client first.
func TestCheckLocked_AllowsADateNoPeriodCovers(t *testing.T) {
	guard := period.NewGuard(period.StaticRepository{Periods: []period.Period{fy2026()}})

	if err := guard.CheckLocked(context.Background(), day(2030, 6, 15)); err != nil {
		t.Errorf("a date outside every period must be allowed by CheckLocked: %v", err)
	}

	// The stricter check still refuses it, which is the difference between them.
	if err := guard.CheckPosting(context.Background(), day(2030, 6, 15)); !errors.Is(err, period.ErrNoPeriod) {
		t.Errorf("CheckPosting must still insist on a period, got %v", err)
	}
}

// An empty calendar locks nothing, so nothing is refused.
func TestCheckLocked_WithNoPeriodsAtAll(t *testing.T) {
	guard := period.NewGuard(period.StaticRepository{})

	if err := guard.CheckLocked(context.Background(), day(2026, 6, 15)); err != nil {
		t.Errorf("an empty calendar must not block posting: %v", err)
	}
}

// The whole of the closing day counts, the same as everywhere else.
func TestCheckLocked_CoversWholeDays(t *testing.T) {
	locked := period.Period{
		ID: "jan", StartDate: day(2026, 1, 1), EndDate: day(2026, 1, 31), Locked: true,
	}
	guard := period.NewGuard(period.StaticRepository{Periods: []period.Period{locked}})

	for _, moment := range []time.Time{
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC),
	} {
		if err := guard.CheckLocked(context.Background(), moment); !errors.Is(err, period.ErrPeriodLocked) {
			t.Errorf("%s is inside the locked period and must be refused, got %v", moment, err)
		}
	}

	if err := guard.CheckLocked(context.Background(), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Errorf("the day after must be allowed: %v", err)
	}
}

// A repository that cannot be read is an error, not a silent permission. Failing
// open here would let a database problem unlock every period at once.
func TestCheckLocked_PropagatesAFailureToRead(t *testing.T) {
	guard := period.NewGuard(failingRepository{})

	if err := guard.CheckLocked(context.Background(), day(2026, 6, 15)); err == nil {
		t.Fatal("a repository failure must be reported, got nil")
	}
	if err := guard.CheckPosting(context.Background(), day(2026, 6, 15)); err == nil {
		t.Fatal("a repository failure must be reported by CheckPosting too, got nil")
	}
}

// failingRepository cannot be read.
type failingRepository struct{}

func (failingRepository) List(context.Context) ([]period.Period, error) {
	return nil, errors.New("the database is unreachable")
}

func (failingRepository) Create(context.Context, period.Period) error { return nil }
func (failingRepository) Lock(context.Context, string, time.Time) error {
	return nil
}

// ---------------------------------------------------------------------------
// Which period the books are read for
// ---------------------------------------------------------------------------

// The period a posting today would land in is the one a page leads with.
//
// The fallback is the part worth pinning. A calendar that has not been carried
// past its own end still names the period the books were left in, so a reader
// sees where things stand instead of an empty screen — and that is the whole of
// the difference from Calendar.Find, which refuses a date no period covers.
func TestCurrent_NamesThePeriodPostingTodayWouldUse(t *testing.T) {
	periods := []period.Period{fy2026(), fy2027()}

	cases := []struct {
		name  string
		today time.Time
		want  string
	}{
		{"inside the first", at(9), "fy2026"},
		{"the first day of the second", day(2027, 1, 1), "fy2027"},
		{"the last day of the second", day(2027, 12, 31), "fy2027"},
		{"past every period", day(2028, 3, 1), "fy2027"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := period.Current(periods, c.today)
			if !ok {
				t.Fatalf("no period was named for %s", c.today.Format(time.DateOnly))
			}
			if got.ID != c.want {
				t.Errorf("%s falls in %s, want %s", c.today.Format(time.DateOnly), got.ID, c.want)
			}
		})
	}

	// A calendar with nothing in it names nothing, and says so rather than
	// returning an empty period that a caller would render as a real one.
	if got, ok := period.Current(nil, day(2026, 3, 14)); ok || got.ID != "" {
		t.Errorf("an empty calendar named %q/%v, want no period at all", got.ID, ok)
	}
}
