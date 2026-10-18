package periodstore_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"fingo/internal/adapter/mariadb/periodstore"
	"fingo/internal/adapter/mariadb/testsupport"
	"fingo/internal/period"
)

func day(year int, month time.Month, dayOfMonth int) time.Time {
	return time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.UTC)
}

// sameDay compares by calendar day. The database stores a DATE and the driver
// reads it back in the connection's location, so comparing instants would make
// this test depend on the machine's timezone.
func sameDay(a, b time.Time) bool {
	return a.Format(time.DateOnly) == b.Format(time.DateOnly)
}

func newStore(t *testing.T) (*periodstore.Store, *sql.DB) {
	t.Helper()

	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, _ := testsupport.MigratedDatabase(t, cfg, admin)

	return periodstore.NewStore(db), db
}

func TestStore_RoundTripsPeriods(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	want := period.Period{ID: "fy2026", StartDate: day(2026, 1, 1), EndDate: day(2026, 12, 31)}

	if err := store.Create(ctx, want); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("periods = %d, want 1", len(got))
	}
	if got[0].ID != want.ID || !sameDay(got[0].StartDate, want.StartDate) || !sameDay(got[0].EndDate, want.EndDate) {
		t.Errorf("round trip = %+v, want %+v", got[0], want)
	}
	if got[0].Locked {
		t.Error("a freshly created period must not be locked")
	}
	if got[0].LockedAt != nil {
		t.Errorf("LockedAt = %v, want nil", got[0].LockedAt)
	}
}

// Creating overlapping periods → error, and the offending row is not stored.
func TestStore_CreateRejectsOverlappingPeriods(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	if err := store.Create(ctx, period.Period{
		ID: "jan", StartDate: day(2026, 1, 1), EndDate: day(2026, 1, 31),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	err := store.Create(ctx, period.Period{
		ID: "overlap", StartDate: day(2026, 1, 31), EndDate: day(2026, 2, 28),
	})
	if !errors.Is(err, period.ErrOverlap) {
		t.Fatalf("error = %v, want ErrOverlap", err)
	}

	got, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("periods = %d after a refused create, want 1", len(got))
	}
}

func TestStore_CreateRejectsAGap(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	if err := store.Create(ctx, period.Period{
		ID: "jan", StartDate: day(2026, 1, 1), EndDate: day(2026, 1, 31),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	err := store.Create(ctx, period.Period{
		ID: "gap", StartDate: day(2026, 2, 2), EndDate: day(2026, 2, 28),
	})
	if !errors.Is(err, period.ErrGap) {
		t.Fatalf("error = %v, want ErrGap", err)
	}
}

func TestStore_CreateAcceptsAnAdjacentPeriod(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	for _, p := range []period.Period{
		{ID: "fy2026", StartDate: day(2026, 1, 1), EndDate: day(2026, 12, 31)},
		{ID: "fy2027", StartDate: day(2027, 1, 1), EndDate: day(2027, 12, 31)},
	} {
		if err := store.Create(ctx, p); err != nil {
			t.Fatalf("Create %s: %v", p.ID, err)
		}
	}

	got, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("periods = %d, want 2", len(got))
	}
	// Oldest first, so a caller can build a calendar without sorting.
	if got[0].ID != "fy2026" || got[1].ID != "fy2027" {
		t.Errorf("order = %s, %s, want fy2026 then fy2027", got[0].ID, got[1].ID)
	}
}

func TestStore_LockPersists(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	if err := store.Create(ctx, period.Period{
		ID: "fy2026", StartDate: day(2026, 1, 1), EndDate: day(2026, 12, 31),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	when := time.Date(2026, 3, 14, 14, 30, 0, 0, time.UTC)
	if err := store.Lock(ctx, "fy2026", when); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	got, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !got[0].Locked {
		t.Fatal("the period came back unlocked")
	}
	if got[0].LockedAt == nil {
		t.Fatal("LockedAt was not recorded")
	}
	if !got[0].LockedAt.UTC().Truncate(time.Second).Equal(when) {
		t.Errorf("LockedAt = %v, want %v", got[0].LockedAt, when)
	}
}

func TestStore_LockRefusesASecondLock(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	if err := store.Create(ctx, period.Period{
		ID: "fy2026", StartDate: day(2026, 1, 1), EndDate: day(2026, 12, 31),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.Lock(ctx, "fy2026", day(2026, 3, 1)); err != nil {
		t.Fatalf("first Lock: %v", err)
	}

	if err := store.Lock(ctx, "fy2026", day(2026, 6, 1)); !errors.Is(err, period.ErrAlreadyLocked) {
		t.Fatalf("second Lock = %v, want ErrAlreadyLocked", err)
	}
}

func TestStore_LockRefusesAnUnknownPeriod(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	if err := store.Lock(ctx, "nope", day(2026, 3, 1)); !errors.Is(err, period.ErrUnknownPeriod) {
		t.Fatalf("Lock of an unknown period = %v, want ErrUnknownPeriod", err)
	}
}

// Posting to a locked period → ErrPeriodLocked, through the database and the
// guard a posting would pass through.
func TestStore_GuardRefusesPostingToALockedPeriod(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	for _, p := range []period.Period{
		{ID: "fy2026", StartDate: day(2026, 1, 1), EndDate: day(2026, 12, 31)},
		{ID: "fy2027", StartDate: day(2027, 1, 1), EndDate: day(2027, 12, 31)},
	} {
		if err := store.Create(ctx, p); err != nil {
			t.Fatalf("Create %s: %v", p.ID, err)
		}
	}

	if err := store.Lock(ctx, "fy2026", day(2026, 3, 1)); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	guard := period.NewGuard(store)

	// Posting to a locked period → ErrPeriodLocked.
	err := guard.CheckPosting(ctx, day(2026, 6, 15))
	if !errors.Is(err, period.ErrPeriodLocked) {
		t.Fatalf("posting into the locked year = %v, want ErrPeriodLocked", err)
	}

	// Posting to an open period succeeds.
	if err := guard.CheckPosting(ctx, day(2027, 6, 15)); err != nil {
		t.Fatalf("posting into the open year must be allowed: %v", err)
	}

	// And a date no period covers is refused.
	if err := guard.CheckPosting(ctx, day(2030, 6, 15)); !errors.Is(err, period.ErrNoPeriod) {
		t.Fatalf("posting outside every period = %v, want ErrNoPeriod", err)
	}
}

// The database refuses to unlock, whatever reaches it. The domain has no unlock
// method; this is the guarantee that survives a repair script.
func TestStore_DatabaseRefusesUnlocking(t *testing.T) {
	store, db := newStore(t)
	ctx := context.Background()

	if err := store.Create(ctx, period.Period{
		ID: "fy2026", StartDate: day(2026, 1, 1), EndDate: day(2026, 12, 31),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Lock(ctx, "fy2026", day(2026, 3, 1)); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	if _, err := db.Exec("UPDATE accounting_period_locks SET is_locked = FALSE"); err == nil {
		t.Error("the database must refuse to unlock a period, got nil")
	}
	if _, err := db.Exec("UPDATE accounting_period_locks SET end_date = '2027-12-31'"); err == nil {
		t.Error("the database must refuse to move a locked period's end date, got nil")
	}
	if _, err := db.Exec("DELETE FROM accounting_period_locks"); err == nil {
		t.Error("the database must refuse to delete a locked period, got nil")
	}

	// And the period is still locked and still where it was.
	got, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !got[0].Locked || !sameDay(got[0].EndDate, day(2026, 12, 31)) {
		t.Errorf("the locked period changed: %+v", got[0])
	}
}
