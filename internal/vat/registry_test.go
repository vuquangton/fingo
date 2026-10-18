package vat_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"fingo/internal/vat"
)

// active is an open-ended active row starting on a day.
func active(id string, rate vat.BasisPoints, from string) vat.VATRate {
	return vat.VATRate{
		ID:            id,
		Rate:          rate,
		EffectiveFrom: mustDay(from),
		Active:        true,
	}
}

// The table must answer "which rate applies on this date" with one row. Two
// active rows that would cover the same day are refused at creation, whichever
// order they arrive in.
func TestRegistry_RefusesTwoActiveRatesCoveringTheSameDate(t *testing.T) {
	table := vat.New()

	if err := table.Create(active("r-2025", 1000, "2025-01-01")); err != nil {
		t.Fatalf("the first active rate must be accepted: %v", err)
	}

	clashes := []struct {
		name string
		rate vat.VATRate
	}{
		{"a second rate starting on the same day", active("r-tie", 800, "2025-01-01")},
		{"a rate backdated before it", active("r-backfill", 800, "2024-06-01")},
	}

	for _, c := range clashes {
		t.Run(c.name, func(t *testing.T) {
			err := table.Create(c.rate)
			if !errors.Is(err, vat.ErrOverlappingRate) {
				t.Fatalf("Create() = %v, want ErrOverlappingRate", err)
			}
		})
	}

	if got := len(table.List()); got != 1 {
		t.Fatalf("the table holds %d rows, want only the one that was accepted", got)
	}
}

// A row that declared an end is still in force until that end. A new row
// starting inside the window would contradict it, so it is refused.
func TestRegistry_RefusesARateStartingInsideAClosedWindow(t *testing.T) {
	table := vat.New()

	window := active("r-8pct", 800, "2025-07-01")
	window.EffectiveTo = spot("2025-12-31")
	if err := table.Create(window); err != nil {
		t.Fatalf("the 8%% window must be accepted: %v", err)
	}

	if err := table.Create(active("r-inside", 1000, "2025-09-01")); !errors.Is(err, vat.ErrOverlappingRate) {
		t.Fatalf("Create() = %v, want ErrOverlappingRate", err)
	}

	// The day after the window closes is a different date, and is free.
	if err := table.Create(active("r-after", 1000, "2026-01-01")); err != nil {
		t.Fatalf("a rate starting after the window closed must be accepted: %v", err)
	}
}

// A rate change is a new row after the old one, and it takes over from its own
// start date. The row it replaces is not edited, and is not consulted again.
func TestRegistry_ARateChangeIsANewRowAppendedAfterTheOld(t *testing.T) {
	table := vat.New()

	if err := table.Create(active("r-10pct", 1000, "2025-01-01")); err != nil {
		t.Fatal(err)
	}
	if err := table.Create(active("r-8pct", 800, "2025-07-01")); err != nil {
		t.Fatalf("a later row is a rate change, not an overlap: %v", err)
	}

	before, err := table.RateOn(mustDay("2025-06-30"))
	if err != nil {
		t.Fatalf("2025-06-30: %v", err)
	}
	if before.ID != "r-10pct" || before.Rate != 1000 {
		t.Errorf("the day before the change = %s at %dbp, want r-10pct at 1000bp", before.ID, int(before.Rate))
	}

	after, err := table.RateOn(mustDay("2025-07-01"))
	if err != nil {
		t.Fatalf("2025-07-01: %v", err)
	}
	if after.ID != "r-8pct" || after.Rate != 800 {
		t.Errorf("the day of the change = %s at %dbp, want r-8pct at 800bp", after.ID, int(after.Rate))
	}

	// The replaced row still answers for the days it governed.
	old, err := table.Get("r-10pct")
	if err != nil {
		t.Fatal(err)
	}
	if old.Rate != 1000 || !old.EffectiveFrom.Equal(mustDay("2025-01-01")) {
		t.Errorf("the replaced row was rewritten: %+v", old)
	}
}

// A row that declares an end leaves a gap behind it. A date in that gap has no
// rate, and is refused rather than falling back to an older one.
func TestRegistry_RefusesADateInAGapBetweenWindows(t *testing.T) {
	table := vat.New()

	first := active("r-first", 1000, "2025-01-01")
	first.EffectiveTo = spot("2025-06-01")
	if err := table.Create(first); err != nil {
		t.Fatal(err)
	}
	if err := table.Create(active("r-second", 800, "2025-09-01")); err != nil {
		t.Fatal(err)
	}

	if _, err := table.RateOn(mustDay("2025-07-01")); !errors.Is(err, vat.ErrNoRateInForce) {
		t.Fatalf("RateOn(2025-07-01) = %v, want ErrNoRateInForce", err)
	}
	if _, err := table.RateOn(mustDay("2024-12-31")); !errors.Is(err, vat.ErrNoRateInForce) {
		t.Fatalf("RateOn(2024-12-31) = %v, want ErrNoRateInForce", err)
	}

	covered, err := table.RateOn(mustDay("2025-10-01"))
	if err != nil {
		t.Fatalf("2025-10-01: %v", err)
	}
	if covered.ID != "r-second" {
		t.Errorf("2025-10-01 is governed by %s, want r-second", covered.ID)
	}
}

// An inactive row is kept for the record and governs nothing: it neither
// answers for a date nor blocks one.
func TestRegistry_AnInactiveRowGovernsNoDate(t *testing.T) {
	table := vat.New()

	draft := active("r-draft", 800, "2025-01-01")
	draft.Active = false
	if err := table.Create(draft); err != nil {
		t.Fatalf("an inactive row must be storable: %v", err)
	}

	if _, err := table.RateOn(mustDay("2025-06-01")); !errors.Is(err, vat.ErrNoRateInForce) {
		t.Fatalf("RateOn() = %v, want ErrNoRateInForce", err)
	}

	// And it does not stand in the way of the rate that really applies.
	if err := table.Create(active("r-10pct", 1000, "2025-01-01")); err != nil {
		t.Fatalf("an inactive row must not block an active one: %v", err)
	}
}

// A rate is never rewritten, so an ID that is already in the table cannot be
// used again — not even to correct it.
func TestRegistry_RefusesToReplaceARowWithTheSameID(t *testing.T) {
	table := vat.New()

	if err := table.Create(active("r-1", 1000, "2025-01-01")); err != nil {
		t.Fatal(err)
	}

	if err := table.Create(active("r-1", 800, "2026-01-01")); !errors.Is(err, vat.ErrDuplicateRateID) {
		t.Fatalf("Create() = %v, want ErrDuplicateRateID", err)
	}

	stored, err := table.Get("r-1")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Rate != 1000 || !stored.EffectiveFrom.Equal(mustDay("2025-01-01")) {
		t.Errorf("the stored row changed under a refused create: %+v", stored)
	}
}

// Immutability is structural as well as behavioural: the table exposes no
// operation that writes to a row already stored. A rate change is Create, and
// there is nothing else.
func TestRegistry_HasNoWayToUpdateAStoredRate(t *testing.T) {
	writes := []string{"update", "delete", "remove", "replace", "edit", "set", "save", "write", "mutate", "correct"}

	typ := reflect.TypeOf(&vat.Registry{})
	for i := 0; i < typ.NumMethod(); i++ {
		name := strings.ToLower(typ.Method(i).Name)
		for _, verb := range writes {
			if strings.Contains(name, verb) {
				t.Errorf("vat.Registry.%s is a write path onto rows that must stay immutable", typ.Method(i).Name)
			}
		}
	}

	// The check is only meaningful if the methods it looks at exist at all.
	if typ.NumMethod() == 0 {
		t.Fatal("the table exposes no methods, so this test proves nothing")
	}
}

// A caller is handed copies. Writing to what it was given — including through
// the EffectiveTo pointer — cannot reach back into the table.
func TestRegistry_HandsOutCopiesOfItsRows(t *testing.T) {
	table := vat.New()

	row := active("r-8pct", 800, "2025-07-01")
	row.EffectiveTo = spot("2025-12-31")
	if err := table.Create(row); err != nil {
		t.Fatal(err)
	}

	// The caller's own copy, edited after the fact, is not the stored row.
	row.Rate = 9999
	*row.EffectiveTo = mustDay("2030-01-01")

	stored, err := table.Get("r-8pct")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Rate != 800 {
		t.Errorf("the table's rate = %d, want 800: a caller edited the stored row", int(stored.Rate))
	}
	if !stored.EffectiveTo.Equal(mustDay("2025-12-31")) {
		t.Errorf("the table's end date = %s, want 2025-12-31", stored.EffectiveTo)
	}

	// Nor can a caller write through what Get handed it.
	handed, err := table.Get("r-8pct")
	if err != nil {
		t.Fatal(err)
	}
	*handed.EffectiveTo = mustDay("2031-01-01")

	again, err := table.Get("r-8pct")
	if err != nil {
		t.Fatal(err)
	}
	if !again.EffectiveTo.Equal(mustDay("2025-12-31")) {
		t.Errorf("the table's end date = %s after a caller wrote through a copy", again.EffectiveTo)
	}

	// And the list is a copy too.
	rows := table.List()
	rows[0].Rate = 0
	fresh, err := table.Get("r-8pct")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Rate != 800 {
		t.Errorf("the table's rate = %d after a caller edited the list", int(fresh.Rate))
	}
}

// A row that could never be legal is refused before it reaches the table.
func TestRegistry_RefusesAnIllegalRate(t *testing.T) {
	cases := []struct {
		name string
		rate vat.VATRate
	}{
		{"no id", vat.VATRate{Rate: 1000, EffectiveFrom: mustDay("2025-01-01"), Active: true}},
		{"a blank id", vat.VATRate{ID: "  ", Rate: 1000, EffectiveFrom: mustDay("2025-01-01"), Active: true}},
		{"a negative rate", active("r-neg", -1, "2025-01-01")},
		{"a rate above 100%", active("r-high", vat.Whole+1, "2025-01-01")},
		{"no start date", vat.VATRate{ID: "r-undated", Rate: 1000, Active: true}},
		{"an end before its start", endOn(active("r-backwards", 1000, "2025-06-01"), "2025-01-01")},
		{"an end on its start", endOn(active("r-empty-window", 1000, "2025-06-01"), "2025-06-01")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := vat.New().Create(c.rate); !errors.Is(err, vat.ErrInvalidRate) {
				t.Fatalf("Create() = %v, want ErrInvalidRate", err)
			}
		})
	}
}

// endOn gives a row an end date, so a table of cases can stay readable.
func endOn(rate vat.VATRate, to string) vat.VATRate {
	rate.EffectiveTo = spot(to)

	return rate
}

// A line is taxed at the row it names, whatever the table has done since.
func TestRegistry_ChargeReadsTheRowTheLineNames(t *testing.T) {
	table := vat.New()

	row := active("r-20pct", 2000, "2025-01-01")
	row.EffectiveTo = spot("2025-12-31")
	if err := table.Create(row); err != nil {
		t.Fatal(err)
	}

	charge, err := table.Charge(vat.Line{NetAmount: 10500, VATRateID: "r-20pct"})
	if err != nil {
		t.Fatalf("Charge() = %v", err)
	}
	if charge.Tax != 2100 {
		t.Errorf("tax = %d, want 2100", charge.Tax)
	}
	if charge.Rate.ID != "r-20pct" {
		t.Errorf("the charge kept %s, want r-20pct", charge.Rate.ID)
	}
	if charge.Gross() != 12600 {
		t.Errorf("gross = %d, want 12600", charge.Gross())
	}

	// The rate has since lapsed, and a new one is in force. The stored line
	// still computes to what it computed to on the day: the ID is the fact.
	if err := table.Create(active("r-10pct", 1000, "2026-01-01")); err != nil {
		t.Fatal(err)
	}

	again, err := table.Charge(vat.Line{NetAmount: 10500, VATRateID: "r-20pct"})
	if err != nil {
		t.Fatalf("Charge() after the rate lapsed = %v", err)
	}
	if again.Tax != 2100 {
		t.Errorf("tax after the rate lapsed = %d, want 2100", again.Tax)
	}
}

// A line that touches VAT must name the row it was taxed under. A line that
// names none is refused rather than treated as zero-rated — zero and unnamed
// are different facts on a return.
func TestRegistry_ChargeRefusesALineThatNamesNoRate(t *testing.T) {
	table := vat.New()

	if err := table.Create(active("r-10pct", 1000, "2025-01-01")); err != nil {
		t.Fatal(err)
	}

	_, err := table.Charge(vat.Line{NetAmount: 10500})
	if !errors.Is(err, vat.ErrUnnamedRate) {
		t.Fatalf("Charge() = %v, want ErrUnnamedRate", err)
	}

	_, err = table.Charge(vat.Line{NetAmount: 10500, VATRateID: "r-imaginary"})
	if !errors.Is(err, vat.ErrUnknownRateID) {
		t.Fatalf("Charge() = %v, want ErrUnknownRateID", err)
	}
}
