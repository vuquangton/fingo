package tax_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/tax"
)

func day(value string) time.Time {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		panic(err)
	}
	return parsed
}

// Standard and reduced rates are always available: they are the permanent
// schedule, not a temporary measure.
func TestSchedule_StandardAndReducedAreAlwaysAvailable(t *testing.T) {
	schedule := tax.DefaultSchedule()

	for _, when := range []string{"2020-01-01", "2025-07-01", "2026-12-31", "2030-06-15"} {
		standard, err := schedule.Percent(tax.KindStandard, day(when))
		if err != nil {
			t.Fatalf("standard rate on %s: %v", when, err)
		}
		if !standard.Equal(decimal.NewFromInt(10)) {
			t.Errorf("standard rate on %s = %s, want 10", when, standard)
		}

		reduced, err := schedule.Percent(tax.KindReduced, day(when))
		if err != nil {
			t.Fatalf("reduced rate on %s: %v", when, err)
		}
		if !reduced.Equal(decimal.NewFromInt(5)) {
			t.Errorf("reduced rate on %s = %s, want 5", when, reduced)
		}
	}
}

// The 8% reduction is a window, and both ends are inclusive. Outside it the
// treatment simply does not exist — a 2027 sale cannot be invoiced at 8%.
func TestSchedule_TemporaryReductionOnlyWithinItsWindow(t *testing.T) {
	schedule := tax.DefaultSchedule()

	inside := []string{"2025-07-01", "2025-12-31", "2026-01-01", "2026-12-31"}
	for _, when := range inside {
		got, err := schedule.Percent(tax.KindTemporarilyReduced, day(when))
		if err != nil {
			t.Fatalf("8%% on %s: %v", when, err)
		}
		if !got.Equal(decimal.NewFromInt(8)) {
			t.Errorf("8%% on %s = %s, want 8", when, got)
		}
	}

	outside := []string{"2025-06-30", "2027-01-01"}
	for _, when := range outside {
		if _, err := schedule.Percent(tax.KindTemporarilyReduced, day(when)); err == nil {
			t.Errorf("8%% on %s must be refused — the window does not cover it", when)
		}
	}
}

// A zero-rated sale and a non-taxable sale both carry no VAT, but they are
// different treatments and both must resolve to nothing.
func TestSchedule_ZeroAndNonTaxableCarryNoVAT(t *testing.T) {
	schedule := tax.DefaultSchedule()

	for _, kind := range []tax.Kind{tax.KindZero, tax.KindExempt, tax.KindNotDeclared} {
		got, err := schedule.Percent(kind, day("2026-06-01"))
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if !got.IsZero() {
			t.Errorf("%s = %s, want 0", kind, got)
		}
	}
}

// An unknown treatment is refused rather than defaulted — defaulting would
// silently tax a sale that should not be taxed.
func TestSchedule_RejectsAnUnknownTreatment(t *testing.T) {
	schedule := tax.DefaultSchedule()

	if _, err := schedule.Percent(tax.Kind("MYSTERY"), day("2026-06-01")); err == nil {
		t.Fatal("an unknown VAT treatment must be refused, got nil")
	}
	if _, err := schedule.Percent(tax.Kind(""), day("2026-06-01")); err == nil {
		t.Fatal("an empty VAT treatment must be refused, got nil")
	}
}

// The reduction's end date is configuration, not something the code knows. A
// deployment whose window moved must be able to say so without editing logic.
func TestSchedule_WindowDatesAreConfiguration(t *testing.T) {
	extended := tax.NewSchedule([]tax.Rate{
		{Kind: tax.KindStandard, Percent: decimal.NewFromInt(10)},
		{
			Kind:    tax.KindTemporarilyReduced,
			Percent: decimal.NewFromInt(8),
			From:    day("2025-07-01"),
			To:      day("2027-06-30"), // an extension the default does not know about
		},
	})

	if _, err := extended.Percent(tax.KindTemporarilyReduced, day("2027-01-01")); err != nil {
		t.Fatalf("the extended schedule must allow 8%% in 2027: %v", err)
	}

	// And the default, unchanged, still refuses it.
	if _, err := tax.DefaultSchedule().Percent(tax.KindTemporarilyReduced, day("2027-01-01")); err == nil {
		t.Fatal("the default schedule must still refuse 8% in 2027")
	}
}

// The schedule can report what it holds, so an operator can see the rates in
// force without reading the source.
func TestSchedule_ReportsTheRatesItHolds(t *testing.T) {
	rates := tax.DefaultSchedule().Rates()

	if len(rates) < 5 {
		t.Fatalf("rates = %d, want the five statutory treatments at least", len(rates))
	}

	seen := map[tax.Kind]bool{}
	for _, r := range rates {
		seen[r.Kind] = true
	}
	for _, kind := range []tax.Kind{
		tax.KindStandard, tax.KindReduced, tax.KindTemporarilyReduced,
		tax.KindZero, tax.KindExempt, tax.KindNotDeclared,
	} {
		if !seen[kind] {
			t.Errorf("the schedule does not hold %s", kind)
		}
	}
}

// A window is expressed in days, so the whole of its last day is inside it —
// not merely its first instant. Documents carry a time, and comparing instants
// would exclude every real document of the closing day.
func TestSchedule_WindowCoversTheWholeOfItsLastDay(t *testing.T) {
	schedule := tax.DefaultSchedule()

	inside := []string{"2026-12-31T00:00:00Z", "2026-12-31T09:30:00Z", "2026-12-31T23:59:59Z"}
	for _, at := range inside {
		when, err := time.Parse(time.RFC3339, at)
		if err != nil {
			t.Fatalf("parse %s: %v", at, err)
		}
		if _, err := schedule.Percent(tax.KindTemporarilyReduced, when); err != nil {
			t.Errorf("%s falls on the last day of the window: %v", at, err)
		}
	}

	after, err := time.Parse(time.RFC3339, "2027-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := schedule.Percent(tax.KindTemporarilyReduced, after); err == nil {
		t.Error("2027-01-01 is outside the window and must be refused")
	}
}

// A treatment in force on a date is reported, and one that is not is not.
func TestSchedule_OnReportsWhatApplies(t *testing.T) {
	schedule := tax.DefaultSchedule()

	during := schedule.On(day("2026-06-01"))
	if !containsKind(during, tax.KindTemporarilyReduced) {
		t.Error("the 8% treatment must be reported during the window")
	}

	after := schedule.On(day("2027-06-01"))
	if containsKind(after, tax.KindTemporarilyReduced) {
		t.Error("the 8% treatment must not be reported after the window")
	}
	if !containsKind(after, tax.KindStandard) {
		t.Error("the standard treatment must still be reported after the window")
	}
}

func containsKind(rates []tax.Rate, kind tax.Kind) bool {
	for _, r := range rates {
		if r.Kind == kind {
			return true
		}
	}
	return false
}
