package inventory_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/inventory"
)

func d(value int64) decimal.Decimal { return decimal.NewFromInt(value) }

func mustReceive(t *testing.T, s inventory.Stock, quantity, cost int64) inventory.Stock {
	t.Helper()

	next, err := s.Receive(d(quantity), d(cost))
	if err != nil {
		t.Fatalf("Receive(%d, %d): %v", quantity, cost, err)
	}

	return next
}

func mustIssue(t *testing.T, s inventory.Stock, quantity int64) (inventory.Stock, decimal.Decimal) {
	t.Helper()

	next, cost, err := s.Issue(d(quantity))
	if err != nil {
		t.Fatalf("Issue(%d): %v", quantity, err)
	}

	return next, cost
}

// Receiving stock adds what was paid for it, and the average cost follows.
//
// The receipt carries a TOTAL cost, not a unit cost: giá gốc includes transport,
// handling and insurance, and forcing those through a per-unit figure on the way
// in would round a fraction of a đồng per unit into the inventory value.
func TestStock_ReceivingAddsQuantityAndValue(t *testing.T) {
	s := inventory.NewStock("VT01", "KHO1")

	s = mustReceive(t, s, 10, 100_000)

	if !s.Quantity.Equal(d(10)) {
		t.Errorf("quantity = %s, want 10", s.Quantity)
	}
	if !s.Value.Equal(d(100_000)) {
		t.Errorf("value = %s, want 100000", s.Value)
	}
	if !s.UnitCost().Equal(d(10_000)) {
		t.Errorf("unit cost = %s, want 10000", s.UnitCost())
	}
}

// The moving average is recomputed on every receipt — that is what distinguishes
// "bình quân gia quyền sau mỗi lần nhập" from the period-end average.
func TestStock_MovingAverageFollowsEachReceipt(t *testing.T) {
	s := inventory.NewStock("VT01", "KHO1")
	s = mustReceive(t, s, 10, 100_000)
	s = mustReceive(t, s, 10, 140_000)

	if !s.Quantity.Equal(d(20)) || !s.Value.Equal(d(240_000)) {
		t.Fatalf("after two receipts: %s units worth %s, want 20 / 240000", s.Quantity, s.Value)
	}
	if !s.UnitCost().Equal(d(12_000)) {
		t.Errorf("unit cost = %s, want 12000", s.UnitCost())
	}

	s, cost := mustIssue(t, s, 5)

	if !cost.Equal(d(60_000)) {
		t.Errorf("cost of 5 at 12000 = %s, want 60000", cost)
	}
	if !s.Quantity.Equal(d(15)) || !s.Value.Equal(d(180_000)) {
		t.Errorf("after issue: %s units worth %s, want 15 / 180000", s.Quantity, s.Value)
	}
}

// Issuing everything must take the whole remaining value, not quantity × average.
// Otherwise a fraction of a đồng stays behind in an item that has nothing left in
// it, and the balance sheet carries stock that is not there.
func TestStock_IssuingEverythingClearsTheValueExactly(t *testing.T) {
	// 3 units for 100 đồng: an average of 33.333…, which cannot be represented.
	s := inventory.NewStock("VT01", "KHO1")
	s = mustReceive(t, s, 3, 100)

	s, cost := mustIssue(t, s, 3)

	if !cost.Equal(d(100)) {
		t.Errorf("cost of the whole holding = %s, want the whole 100", cost)
	}
	if !s.Quantity.IsZero() {
		t.Errorf("quantity = %s, want 0", s.Quantity)
	}
	if !s.Value.IsZero() {
		t.Errorf("value = %s, want exactly 0 — a stranded đồng is stock that is not there", s.Value)
	}
}

// The same, reached through a run of awkward numbers rather than one step.
func TestStock_ValueNeverStrandsAcrossARunOfMovements(t *testing.T) {
	s := inventory.NewStock("VT01", "KHO1")
	issued := decimal.Zero

	var cost decimal.Decimal

	s = mustReceive(t, s, 7, 1000)
	s, cost = mustIssue(t, s, 3)
	issued = issued.Add(cost)

	s = mustReceive(t, s, 11, 1777)
	s, cost = mustIssue(t, s, 5)
	issued = issued.Add(cost)

	s = mustReceive(t, s, 2, 333)
	s, cost = mustIssue(t, s, 12)
	issued = issued.Add(cost)

	if !s.Quantity.IsZero() {
		t.Fatalf("quantity = %s, want 0", s.Quantity)
	}
	if !s.Value.IsZero() {
		t.Errorf("value = %s, want exactly 0", s.Value)
	}

	// Everything that came in went out, at exactly what it came in for:
	// 1000 + 1777 + 333. No đồng invented, none stranded.
	if got, want := issued.String(), "3110"; got != want {
		t.Errorf("total cost issued = %s, want %s", got, want)
	}
}

// Stock cannot go negative. An issue beyond what is on hand is a stocktake
// problem, not a posting the ledger should accept.
func TestStock_RefusesToIssueMoreThanIsOnHand(t *testing.T) {
	s := inventory.NewStock("VT01", "KHO1")
	s = mustReceive(t, s, 10, 100_000)

	if _, _, err := s.Issue(d(11)); err == nil {
		t.Fatal("issuing 11 of 10 must be refused, got nil")
	}
	if _, _, err := s.Issue(d(0)); err == nil {
		t.Fatal("issuing nothing must be refused, got nil")
	}
	if _, _, err := s.Issue(d(-1)); err == nil {
		t.Fatal("issuing a negative quantity must be refused, got nil")
	}
}

// Receiving nothing is not a receipt, and a negative cost is a credit note
// against a purchase rather than a way to reduce stock value silently.
func TestStock_RefusesAnUnusableReceipt(t *testing.T) {
	s := inventory.NewStock("VT01", "KHO1")

	for _, c := range []struct {
		quantity, cost int64
	}{
		{0, 100},
		{-1, 100},
		{10, -1},
	} {
		if _, err := s.Receive(d(c.quantity), d(c.cost)); err == nil {
			t.Errorf("Receive(%d, %d) must be refused, got nil", c.quantity, c.cost)
		}
	}
}

// A new item holds nothing, and an item cannot be created already holding stock.
func TestNewStock_StartsEmpty(t *testing.T) {
	s := inventory.NewStock("VT01", "KHO1")

	if !s.Quantity.IsZero() || !s.Value.IsZero() {
		t.Fatalf("a new stock = %s / %s, want empty", s.Quantity, s.Value)
	}
	if !s.UnitCost().IsZero() {
		t.Errorf("unit cost of nothing = %s, want 0", s.UnitCost())
	}
}
