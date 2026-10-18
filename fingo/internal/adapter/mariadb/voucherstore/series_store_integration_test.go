package voucherstore_test

import (
	"context"
	"database/sql"
	"sort"
	"sync"
	"testing"

	"fingo/internal/adapter/mariadb/testsupport"
	"fingo/internal/adapter/mariadb/voucherstore"
	"fingo/internal/voucher"
)

func newStore(t *testing.T) (*voucherstore.Store, *sql.DB) {
	t.Helper()

	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, _ := testsupport.MigratedDatabase(t, cfg, admin)

	return voucherstore.NewStore(db), db
}

// 100 concurrent Next() calls → 100 unique sequential numbers.
//
// The allocations are sorted before they are checked, so this asserts the set is
// exactly 1..100: neither a duplicate (two documents sharing a number) nor a gap
// (a number nobody can account for).
func TestStore_NextIsSequentialUnderConcurrency(t *testing.T) {
	store, db := newStore(t)

	// Far fewer connections than callers, so the allocations genuinely contend
	// for the series row instead of being handed one connection each.
	db.SetMaxOpenConns(16)

	ctx := context.Background()

	const callers = 100

	var (
		wait    sync.WaitGroup
		mutex   sync.Mutex
		numbers = make([]int, 0, callers)
		failed  []error
	)

	wait.Add(callers)

	for i := 0; i < callers; i++ {
		go func() {
			defer wait.Done()

			number, err := store.Next(ctx, "INV", 2026)

			mutex.Lock()
			defer mutex.Unlock()

			if err != nil {
				failed = append(failed, err)
				return
			}

			numbers = append(numbers, number)
		}()
	}

	wait.Wait()

	if len(failed) != 0 {
		t.Fatalf("%d of %d allocations failed; first: %v", len(failed), callers, failed[0])
	}
	if len(numbers) != callers {
		t.Fatalf("got %d numbers, want %d", len(numbers), callers)
	}

	sort.Ints(numbers)

	for i, number := range numbers {
		if number != i+1 {
			t.Fatalf("the numbers are not 1..%d: position %d holds %d", callers, i, number)
		}
	}
}

// Year rollover resets the counter: JV-2026-00042 and JV-2027-00042 are two
// different documents, so the run restarts in January.
func TestStore_YearRolloverResetsTheCounter(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		got, err := store.Next(ctx, "JV", 2026)
		if err != nil {
			t.Fatalf("2026 allocation %d: %v", i, err)
		}
		if got != i {
			t.Fatalf("2026 allocation %d = %d", i, got)
		}
	}

	got, err := store.Next(ctx, "JV", 2027)
	if err != nil {
		t.Fatalf("2027 allocation: %v", err)
	}
	if got != 1 {
		t.Errorf("the first 2027 number = %d, want 1", got)
	}

	// And 2026 carries on where it left off.
	got, err = store.Next(ctx, "JV", 2026)
	if err != nil {
		t.Fatalf("2026 allocation: %v", err)
	}
	if got != 4 {
		t.Errorf("the next 2026 number = %d, want 4", got)
	}
}

// A prefix is a run of its own: invoices and purchase orders do not share a
// counter.
func TestStore_SeparatePrefixesAreSeparateRuns(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := store.Next(ctx, "INV", 2026); err != nil {
			t.Fatalf("INV: %v", err)
		}
	}

	got, err := store.Next(ctx, "PO", 2026)
	if err != nil {
		t.Fatalf("PO: %v", err)
	}
	if got != 1 {
		t.Errorf("the first PO number = %d, want 1", got)
	}
}

// What comes out of the store and what Format renders agree — the two are only
// useful together.
func TestStore_NumberRendersWithTheSeriesPrefix(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	seen := make(map[string]bool)

	for i := 0; i < 5; i++ {
		number, err := store.Next(ctx, "INV", 2026)
		if err != nil {
			t.Fatalf("Next: %v", err)
		}

		rendered := voucher.Format("INV", 2026, number)
		if seen[rendered] {
			t.Fatalf("the number %s was issued twice", rendered)
		}
		seen[rendered] = true
	}

	if !seen["INV-2026-00001"] || !seen["INV-2026-00005"] {
		t.Errorf("rendered numbers = %v, want INV-2026-00001 through INV-2026-00005", seen)
	}
}

// A prefix the format could not render, or a year that is not one, is refused
// before anything is written.
func TestStore_RefusesAnUnusablePrefixOrYear(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	for _, prefix := range []voucher.Prefix{"", "inv", "IN-V", "INV/1"} {
		if _, err := store.Next(ctx, prefix, 2026); err == nil {
			t.Errorf("prefix %q must be refused, got nil", prefix)
		}
	}

	if _, err := store.Next(ctx, "INV", 0); err == nil {
		t.Error("year 0 must be refused, got nil")
	}
}
