package vat_test

import (
	"testing"
	"time"

	"fingo/internal/vat"
)

// mustDay parses a date, or fails the test that asked for it. Windows are
// written in days everywhere in this package, so almost every test needs one.
func mustDay(value string) time.Time {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		panic(err)
	}

	return parsed
}

// spot returns a pointer to a parsed day, for the rows that declare an end.
func spot(value string) *time.Time {
	d := mustDay(value)

	return &d
}

// 10500 minor units at 20 per cent is 2100.
func TestTax_ChargesTheNetAmountAtTheRate(t *testing.T) {
	rate := vat.VATRate{ID: "r-20", Rate: 2000, Active: true}

	if got := rate.Tax(10500); got != 2100 {
		t.Errorf("tax on 10500 at 2000bp = %d, want 2100", got)
	}
}

// The division is the only place a fraction appears, and it rounds half away
// from zero — up on a debit, down on a credit — so a document's VAT is decided
// by a stated rule rather than by whichever way truncation happened to fall.
func TestTax_RoundsHalfAwayFromZero(t *testing.T) {
	cases := []struct {
		name string
		rate vat.BasisPoints
		net  int64
		want int64
	}{
		{"the stated case, 20% of 10050", 2000, 10050, 2010},
		{"exactly half a unit goes up", 2500, 2, 1},
		{"exactly half a unit on a credit goes down", 2500, -2, -1},
		{"exactly half a unit at one basis point goes up", 1, 5000, 1},
		{"just under half goes down", 1, 4999, 0},
		{"just over half goes up", 1, 5001, 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rate := vat.VATRate{ID: "r", Rate: c.rate}

			if got := rate.Tax(c.net); got != c.want {
				t.Errorf("%d at %dbp = %d, want %d", c.net, int(c.rate), got, c.want)
			}
		})
	}
}

// The edges of the range are legal rates: nothing charged, and everything
// charged.
func TestTax_HandlesTheEndsOfTheRange(t *testing.T) {
	zero := vat.VATRate{ID: "r-zero", Rate: 0}
	if got := zero.Tax(10500); got != 0 {
		t.Errorf("a zero rate charged %d, want 0", got)
	}

	empty := vat.VATRate{ID: "r-10", Rate: 1000}
	if got := empty.Tax(0); got != 0 {
		t.Errorf("an empty line was charged %d, want 0", got)
	}

	all := vat.VATRate{ID: "r-100", Rate: vat.Whole}
	if got := all.Tax(1234); got != 1234 {
		t.Errorf("100%% of 1234 = %d, want 1234", got)
	}
}

// net × rate overflows an int64 for a line of a few thousand trillion minor
// units at a high rate. The tax is still the right number rather than the low
// bits of a wrapped product.
func TestTax_DoesNotOverflowOnALargeLine(t *testing.T) {
	const net = int64(9_300_000_000_000_000) // 9.3e15; × 10000 is 9.3e19

	all := vat.VATRate{ID: "r-100", Rate: vat.Whole}
	if got := all.Tax(net); got != net {
		t.Errorf("100%% of %d = %d, want %d (the product exceeds int64)", net, got, net)
	}

	tenth := vat.VATRate{ID: "r-10", Rate: 1000}
	if got, want := tenth.Tax(net+1), int64(930_000_000_000_000); got != want {
		t.Errorf("10%% of %d = %d, want %d", net+1, got, want)
	}
}

// A rate outside the range is a programming error, not a number to compute
// with: it can only arrive from a row that Registry refused at creation.
func TestTax_PanicsOnAnIllegalRate(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a rate above 100% must not be computed with silently")
		}
	}()

	vat.VATRate{ID: "r-bad", Rate: 10001}.Tax(100)
}

// A window is expressed in days, so the whole of its last day is inside it.
func TestRate_InForceOnCoversTheWholeOfItsLastDay(t *testing.T) {
	rate := vat.VATRate{
		ID:            "r-window",
		Rate:          800,
		EffectiveFrom: mustDay("2025-07-01"),
		EffectiveTo:   spot("2026-12-31"),
		Active:        true,
	}

	inside := []string{"2025-07-01T00:00:00Z", "2025-07-01T23:59:59Z", "2026-12-31T09:30:00Z"}
	for _, at := range inside {
		when, err := time.Parse(time.RFC3339, at)
		if err != nil {
			t.Fatalf("parse %s: %v", at, err)
		}
		if !rate.InForceOn(when) {
			t.Errorf("%s is inside the window", at)
		}
	}

	outside := []string{"2025-06-30T23:59:59Z", "2027-01-01T00:00:00Z"}
	for _, at := range outside {
		when, err := time.Parse(time.RFC3339, at)
		if err != nil {
			t.Fatalf("parse %s: %v", at, err)
		}
		if rate.InForceOn(when) {
			t.Errorf("%s is outside the window", at)
		}
	}
}

// A row with no declared end has no declared end — but it does have a start.
func TestRate_InForceOnWithNoDeclaredEnd(t *testing.T) {
	rate := vat.VATRate{ID: "r-open", Rate: 1000, EffectiveFrom: mustDay("2025-01-01"), Active: true}

	if !rate.InForceOn(mustDay("2035-01-01")) {
		t.Error("an open-ended row covers a day long after its start")
	}
	if rate.InForceOn(mustDay("2024-12-31")) {
		t.Error("no row covers a day before its own start")
	}
}

// A line carries the ID of the row it was taxed under. The two facts the charge
// keeps are the row and the tax, so a caller that stored only the line can
// still report what it was charged.
func TestCharge_GrossIsTheLinePlusItsTax(t *testing.T) {
	charge := vat.Charge{
		Line: vat.Line{NetAmount: 10500, VATRateID: "r-20"},
		Rate: vat.VATRate{ID: "r-20", Rate: 2000},
		Tax:  2100,
	}

	if got := charge.Gross(); got != 12600 {
		t.Errorf("gross = %d, want 12600", got)
	}
}
