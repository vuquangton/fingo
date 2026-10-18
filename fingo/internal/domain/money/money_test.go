package money_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/money"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// VND has no subunit. Every amount that reaches the ledger is a whole number of
// đồng, rounded half away from zero.
func TestDong_RoundsToWholeDong(t *testing.T) {
	cases := map[string]string{
		"110":    "110",
		"110.4":  "110",
		"110.5":  "111", // half rounds away from zero
		"110.6":  "111",
		"0.5":    "1",
		"0.4999": "0",
	}

	for in, want := range cases {
		if got := money.Dong(d(in)); !got.Equal(d(want)) {
			t.Errorf("Dong(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestLineNet_MultipliesAndRounds(t *testing.T) {
	// 3 × 1,234,567 = 3,703,701 exactly.
	if got := money.LineNet(d("3"), d("1234567")); !got.Equal(d("3703701")) {
		t.Errorf("LineNet = %s, want 3703701", got)
	}

	// A fractional quantity still yields whole đồng in the ledger.
	if got := money.LineNet(d("0.333"), d("100")); !got.Equal(d("33")) {
		t.Errorf("LineNet = %s, want 33", got)
	}
}

// VAT is computed on the already-rounded line net, so the tax agrees with the
// figure it is charged on.
func TestLineVAT_IsComputedOnTheRoundedNet(t *testing.T) {
	cases := []struct {
		net, rate, want string
	}{
		{"100", "10", "10"},
		{"333", "5", "17"}, // 16.65 → 17
		{"3703701", "8", "296296"},
		{"1000", "0", "0"},
	}

	for _, c := range cases {
		if got := money.LineVAT(d(c.net), d(c.rate)); !got.Equal(d(c.want)) {
			t.Errorf("LineVAT(%s, %s%%) = %s, want %s", c.net, c.rate, got, c.want)
		}
	}
}

// The invariant that makes an invoice total agree with its own lines: the parts
// must sum to exactly the total, with no đồng lost or invented.
func TestAllocate_PartsSumExactlyToTheTotal(t *testing.T) {
	cases := []struct {
		total   string
		weights []string
		want    []string
	}{
		// Equal thirds: 33.33… each, one đồng left over to the first.
		{"100", []string{"1", "1", "1"}, []string{"34", "33", "33"}},
		// 10 * 1/6 = 1.667, 10 * 2/6 = 3.333, 10 * 3/6 = 5 exactly.
		{"10", []string{"1", "2", "3"}, []string{"2", "3", "5"}},
		// The larger fraction takes the remaining đồng.
		{"1000", []string{"1", "2"}, []string{"333", "667"}},
		// A single weight absorbs everything.
		{"777", []string{"5"}, []string{"777"}},
		// Zero total allocates zero.
		{"0", []string{"1", "2"}, []string{"0", "0"}},
	}

	for _, c := range cases {
		weights := make([]decimal.Decimal, 0, len(c.weights))
		for _, w := range c.weights {
			weights = append(weights, d(w))
		}

		got, err := money.Allocate(d(c.total), weights)
		if err != nil {
			t.Fatalf("Allocate(%s, %v): %v", c.total, c.weights, err)
		}
		if len(got) != len(c.want) {
			t.Fatalf("Allocate(%s, %v) returned %d parts, want %d", c.total, c.weights, len(got), len(c.want))
		}

		for i := range got {
			if !got[i].Equal(d(c.want[i])) {
				t.Errorf("Allocate(%s, %v) = %v, want %v", c.total, c.weights, got, c.want)
				break
			}
		}
	}
}

// All weights zero is a degenerate but real input — landed cost allocated by a
// value that happens to be zero. An equal split keeps the summation invariant
// rather than silently losing the total.
func TestAllocate_ZeroWeightsSplitEqually(t *testing.T) {
	got, err := money.Allocate(d("100"), []decimal.Decimal{d("0"), d("0"), d("0")})
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	want := []string{"34", "33", "33"}
	for i := range got {
		if !got[i].Equal(d(want[i])) {
			t.Fatalf("Allocate with zero weights = %v, want %v", got, want)
		}
	}
}

// Whatever the split, the parts must sum to the total. This is the property the
// whole function exists for.
func TestAllocate_AlwaysSumsToTheTotal(t *testing.T) {
	totals := []string{"1", "7", "999", "100000", "1234567"}
	weightSets := [][]string{
		{"1", "1", "1"},
		{"1", "2", "3", "4"},
		{"7"},
		{"1", "0", "1"},
		{"13", "17", "19", "23", "29"},
	}

	for _, total := range totals {
		for _, ws := range weightSets {
			weights := make([]decimal.Decimal, 0, len(ws))
			for _, w := range ws {
				weights = append(weights, d(w))
			}

			parts, err := money.Allocate(d(total), weights)
			if err != nil {
				t.Fatalf("Allocate(%s, %v): %v", total, ws, err)
			}

			if sum := money.Sum(parts...); !sum.Equal(d(total)) {
				t.Errorf("Allocate(%s, %v) summed to %s, want %s", total, ws, sum, total)
			}
		}
	}
}

func TestAllocate_RejectsImpossibleInput(t *testing.T) {
	cases := map[string]struct {
		total   string
		weights []string
	}{
		"no weights":      {"100", nil},
		"negative weight": {"100", []string{"1", "-1"}},
		"negative total":  {"-100", []string{"1"}},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			weights := make([]decimal.Decimal, 0, len(c.weights))
			for _, w := range c.weights {
				weights = append(weights, d(w))
			}

			if _, err := money.Allocate(d(c.total), weights); err == nil {
				t.Fatalf("%s must be refused, got nil", name)
			}
		})
	}
}

func TestSum_AddsAmounts(t *testing.T) {
	if got := money.Sum(d("10"), d("20"), d("30")); !got.Equal(d("60")) {
		t.Fatalf("Sum = %s, want 60", got)
	}
	if got := money.Sum(); !got.IsZero() {
		t.Fatalf("Sum of nothing = %s, want 0", got)
	}
}
