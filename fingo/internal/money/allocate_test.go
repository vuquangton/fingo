package money_test

import (
	"testing"

	"fingo/internal/money"
)

// sum is the property that matters: the parts must add back to the whole, to the
// last minor unit. A allocation that loses or invents one is a balance that never
// reconciles.
func sum(parts []money.Money) int64 {
	var total int64
	for _, p := range parts {
		total += p.Amount()
	}
	return total
}

func TestAllocate_SumsExactlyToTheTotal(t *testing.T) {
	cases := []struct {
		name    string
		total   int64
		weights []int
	}{
		{"divides evenly", 100, []int{1, 1, 1, 1}},
		{"leaves a remainder", 100, []int{1, 1, 1}},
		{"remainder of one", 10, []int{1, 1, 1}},
		{"uneven weights", 1000, []int{3, 5, 7}},
		{"a zero weight", 100, []int{0, 1, 1}},
		{"tiny total", 1, []int{1, 1, 1}},
		{"single recipient", 7, []int{5}},
		{"large weights", 999_999, []int{123_456, 654_321}},
		{"zero total", 0, []int{1, 2, 3}},
		{"negative total", -100, []int{1, 1, 1}},
		{"large total", 9_000_000_000, []int{1, 1, 1}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			parts := money.Allocate(money.New(c.total, "VND"), c.weights)

			if len(parts) != len(c.weights) {
				t.Fatalf("got %d parts, want %d", len(parts), len(c.weights))
			}
			if got := sum(parts); got != c.total {
				t.Errorf("parts sum to %d, want %d — parts: %v", got, c.total, parts)
			}
		})
	}
}

func TestAllocate_SharesOutProportionally(t *testing.T) {
	// 1000 split 2:3 is 400 and 600, exactly.
	parts := money.Allocate(money.New(1000, "VND"), []int{2, 3})

	if parts[0].Amount() != 400 || parts[1].Amount() != 600 {
		t.Errorf("Allocate(1000, [2 3]) = [%d %d], want [400 600]",
			parts[0].Amount(), parts[1].Amount())
	}
}

func TestAllocate_KeepsTheCurrency(t *testing.T) {
	for _, p := range money.Allocate(money.New(100, "USD"), []int{1, 1}) {
		if p.Currency() != "USD" {
			t.Errorf("a part has currency %q, want USD", p.Currency())
		}
	}
}

// A single recipient takes everything. This is the degenerate case a proportional
// formula can still get wrong by leaving the rounding remainder behind.
func TestAllocate_SingleRecipientTakesEverything(t *testing.T) {
	parts := money.Allocate(money.New(7, "VND"), []int{5})

	if len(parts) != 1 || parts[0].Amount() != 7 {
		t.Errorf("Allocate(7, [5]) = %v, want [7 VND]", parts)
	}
}

// Weights that sum to zero carry no proportion, so the amount is split equally.
// The alternative — returning nothing — would lose the whole total.
func TestAllocate_WeightsSummingToZeroSplitEqually(t *testing.T) {
	parts := money.Allocate(money.New(12, "VND"), []int{0, 0, 0})

	want := []int64{4, 4, 4}
	for i, w := range want {
		if parts[i].Amount() != w {
			t.Fatalf("Allocate(12, [0 0 0]) = %v, want %v", parts, want)
		}
	}
	if got := sum(parts); got != 12 {
		t.Errorf("parts sum to %d, want 12", got)
	}

	// An equal split that does not divide evenly still sums exactly, and the
	// earlier recipients take the remainder.
	uneven := money.Allocate(money.New(10, "VND"), []int{0, 0, 0})
	if uneven[0].Amount() != 4 || uneven[1].Amount() != 3 || uneven[2].Amount() != 3 {
		t.Errorf("Allocate(10, [0 0 0]) = [%d %d %d], want [4 3 3]",
			uneven[0].Amount(), uneven[1].Amount(), uneven[2].Amount())
	}
}

// When several recipients have the same claim on the remainder, the earlier one
// takes it. Arbitrary but fixed: the same input must always give the same answer,
// or two runs of the same report disagree.
func TestAllocate_TieBreaksTowardTheEarlierRecipient(t *testing.T) {
	// Each share is 3⅓, so one unit is left over and all three are tied.
	parts := money.Allocate(money.New(10, "VND"), []int{1, 1, 1})

	want := []int64{4, 3, 3}
	for i, w := range want {
		if parts[i].Amount() != w {
			t.Fatalf("Allocate(10, [1 1 1]) = [%d %d %d], want %v",
				parts[0].Amount(), parts[1].Amount(), parts[2].Amount(), want)
		}
	}

	// Two tied recipients, one leftover unit: the first takes it.
	two := money.Allocate(money.New(5, "VND"), []int{1, 1})
	if two[0].Amount() != 3 || two[1].Amount() != 2 {
		t.Errorf("Allocate(5, [1 1]) = [%d %d], want [3 2]", two[0].Amount(), two[1].Amount())
	}

	// And it is stable: the same call twice gives the same split.
	again := money.Allocate(money.New(5, "VND"), []int{1, 1})
	if again[0].Amount() != two[0].Amount() || again[1].Amount() != two[1].Amount() {
		t.Error("Allocate is not deterministic")
	}
}

// A larger fractional claim comes before a smaller one — that is what makes this
// largest-remainder rather than first-come-first.
func TestAllocate_GivesTheRemainderToTheLargestFraction(t *testing.T) {
	// 100 split 1:2:3 is 16.6̄, 33.3̄ and 50.0. The floors are 16, 33 and 50,
	// which is 99 — one unit left over, and it belongs to the largest fraction,
	// which is the first recipient's two thirds.
	parts := money.Allocate(money.New(100, "VND"), []int{1, 2, 3})

	want := []int64{17, 33, 50}
	for i, w := range want {
		if parts[i].Amount() != w {
			t.Fatalf("Allocate(100, [1 2 3]) = [%d %d %d], want %v",
				parts[0].Amount(), parts[1].Amount(), parts[2].Amount(), want)
		}
	}

	// With two units to give away, the two largest fractions take one each.
	// 110 split 1:2:3 is 18.3̄, 36.6̄, 55.0 → floors 18, 36, 55 = 109, one left.
	// 200 split 1:2:3 is 33.3̄, 66.6̄, 100.0 → floors 33, 66, 100 = 199, one left.
	// So use 200 with a weight set that leaves two.
	twoLeft := money.Allocate(money.New(200, "VND"), []int{1, 1, 1})
	if twoLeft[0].Amount() != 67 || twoLeft[1].Amount() != 67 || twoLeft[2].Amount() != 66 {
		t.Errorf("Allocate(200, [1 1 1]) = [%d %d %d], want [67 67 66]",
			twoLeft[0].Amount(), twoLeft[1].Amount(), twoLeft[2].Amount())
	}
}

func TestAllocate_ZeroTotalGivesZeroToEverybody(t *testing.T) {
	parts := money.Allocate(money.New(0, "VND"), []int{3, 5, 7})

	for i, p := range parts {
		if !p.IsZero() {
			t.Errorf("part %d = %d, want 0", i, p.Amount())
		}
	}
}

// A negative total allocates negatively and still sums exactly — a credit note
// split across lines is as real as an invoice.
func TestAllocate_NegativeTotalSumsExactly(t *testing.T) {
	parts := money.Allocate(money.New(-100, "VND"), []int{1, 2, 3})

	if got := sum(parts); got != -100 {
		t.Errorf("parts sum to %d, want -100", got)
	}
	// Proportional in the other direction: the biggest weight takes the most.
	if parts[2].Amount() >= parts[0].Amount() {
		t.Errorf("parts = %v, want the largest weight to hold the largest negative share", parts)
	}
}

// A negative weight is not a proportion, it is a mistake — and one that would
// silently move money between recipients.
func TestAllocate_PanicsOnANegativeWeight(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a negative weight must panic")
		}
	}()

	money.Allocate(money.New(100, "VND"), []int{1, -1, 3})
}

// No recipients and no total is not an error, it is an empty split.
func TestAllocate_WithNoWeights(t *testing.T) {
	parts := money.Allocate(money.New(100, "VND"), nil)

	if len(parts) != 0 {
		t.Errorf("Allocate with no weights = %v, want empty", parts)
	}
}
