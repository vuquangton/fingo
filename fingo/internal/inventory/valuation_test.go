package inventory_test

import (
	"errors"
	"testing"
	"time"

	"fingo/internal/inventory"
	"fingo/internal/money"
)

var vnd = money.Currency("VND")

// lot is a quantity received at a cost per unit.
type lot struct {
	qty  int
	cost int64
}

func cents(amount int64) money.Money { return money.New(amount, vnd) }

func day(year int, month time.Month, dayOfMonth int) time.Time {
	return time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.UTC)
}

func item(id string) inventory.Item {
	return inventory.Item{
		ID:   id,
		SKU:  "SKU-" + id,
		Name: "Hàng hoá " + id,
		Unit: "cái",
	}
}

// receipt is an inbound movement at a cost per unit.
func receipt(id, itemID string, qty int, unitCost int64) inventory.StockMove {
	return inventory.StockMove{
		ID:          id,
		ItemID:      itemID,
		Direction:   inventory.DirectionIn,
		Qty:         qty,
		UnitCost:    cents(unitCost),
		Date:        day(2026, 3, 1),
		SourceDocID: "doc-" + id,
	}
}

// issue is an outbound movement. Its cost is not the caller's to say.
func issue(id, itemID string, qty int) inventory.StockMove {
	return inventory.StockMove{
		ID:          id,
		ItemID:      itemID,
		Direction:   inventory.DirectionOut,
		Qty:         qty,
		Date:        day(2026, 3, 2),
		SourceDocID: "doc-" + id,
	}
}

func ledger(t *testing.T) *inventory.Ledger {
	t.Helper()

	book, err := inventory.NewLedger(vnd)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}

	return book
}

// Two lots at different prices give the average of what was paid for all of it,
// weighted by how much of each there was.
func TestValuation_WeightedAverageAcrossInboundLots(t *testing.T) {
	valuation := inventory.Valuation{ItemID: "i-1", Currency: vnd}

	first, err := valuation.Receive(10, cents(100))
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if first.AvgCost != 100 || first.Quantity != 10 {
		t.Fatalf("after the first lot = %d units at %d, want 10 at 100", first.Quantity, first.AvgCost)
	}

	// Ten more at twice the price: 1000 + 2000 over 20 units.
	second, err := first.Receive(10, cents(200))
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if second.AvgCost != 150 {
		t.Errorf("average = %d, want 150", second.AvgCost)
	}
	if second.Quantity != 20 {
		t.Errorf("quantity = %d, want 20", second.Quantity)
	}
	if got := second.Value().Amount(); got != 3000 {
		t.Errorf("value = %d, want 3000", got)
	}

	// A third lot pulls the average towards its own price, weighted by size.
	third, err := second.Receive(20, cents(300))
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if third.AvgCost != 225 {
		t.Errorf("average = %d, want (3000 + 6000) / 40 = 225", third.AvgCost)
	}
}

// An issue leaves at the average that is standing, and what leaves is what that
// quantity was carried at.
func TestValuation_OutboundIsValuedAtTheAverageStanding(t *testing.T) {
	valuation := inventory.Valuation{ItemID: "i-1", Currency: vnd}

	valuation, err := valuation.Receive(10, cents(100))
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	valuation, err = valuation.Receive(10, cents(200))
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}

	after, cost, err := valuation.Issue(5)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if cost.Amount() != 750 {
		t.Errorf("the issue cost %s, want 5 × 150 = 750", cost)
	}
	if after.Quantity != 15 {
		t.Errorf("quantity = %d, want 15", after.Quantity)
	}
	if after.AvgCost != 150 {
		t.Errorf("average = %d, want it unmoved at 150: issuing does not change the average", after.AvgCost)
	}
	if got := after.Value().Amount(); got != 2250 {
		t.Errorf("value on hand = %d, want 3000 − 750 = 2250", got)
	}
}

// Taking stock away at the average leaves the rest at the same average, so a
// receipt after an issue folds into what is actually left.
func TestValuation_AnIssueDoesNotMoveTheAverageForWhatIsLeft(t *testing.T) {
	valuation := inventory.Valuation{ItemID: "i-1", Currency: vnd}

	valuation, _ = valuation.Receive(10, cents(100))
	valuation, _, _ = valuation.Issue(9)

	// One unit left at 100, and ten more at 200: (100 + 2000) / 11.
	next, err := valuation.Receive(10, cents(200))
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if want := int64(2100 / 11); next.AvgCost != want {
		t.Errorf("average = %d, want %d", next.AvgCost, want)
	}
}

// Stock cannot go below nothing. Goods that left a warehouse never held are
// goods nobody can find.
func TestValuation_RefusesToIssueMoreThanIsOnHand(t *testing.T) {
	valuation := inventory.Valuation{ItemID: "i-1", Currency: vnd}

	valuation, err := valuation.Receive(3, cents(100))
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}

	after, cost, err := valuation.Issue(4)
	if !errors.Is(err, inventory.ErrNegativeStock) {
		t.Fatalf("Issue of 4 from 3 = %v, want ErrNegativeStock", err)
	}
	if after.Quantity != 3 || after.AvgCost != 100 {
		t.Errorf("the refused issue changed the valuation: %+v", after)
	}
	if !cost.IsZero() {
		t.Errorf("a refused issue cost %s, want nothing", cost)
	}

	// Exactly what is on hand is not more than is on hand.
	after, cost, err = valuation.Issue(3)
	if err != nil {
		t.Fatalf("Issue of all of it: %v", err)
	}
	if after.Quantity != 0 || cost.Amount() != 300 {
		t.Errorf("issuing the whole holding = %d units at %s, want 0 having taken 300", after.Quantity, cost)
	}
}

// An empty holding is worth nothing, at no rate. What was paid for stock that is
// no longer there must not colour what is bought next.
func TestValuation_AnEmptyHoldingIsWorthNothingAtNoRate(t *testing.T) {
	valuation := inventory.Valuation{ItemID: "i-1", Currency: vnd}

	valuation, _ = valuation.Receive(10, cents(100))
	valuation, _, _ = valuation.Issue(10)

	if valuation.Quantity != 0 || valuation.AvgCost != 0 {
		t.Fatalf("after issuing everything = %d units at %d, want 0 at 0", valuation.Quantity, valuation.AvgCost)
	}
	if !valuation.Value().IsZero() {
		t.Errorf("an empty holding is worth %s, want nothing", valuation.Value())
	}

	// The next receipt stands on its own price.
	next, err := valuation.Receive(2, cents(700))
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if next.AvgCost != 700 {
		t.Errorf("average = %d, want the new price 700", next.AvgCost)
	}
}

// The average is an integer, and it is truncated toward zero rather than
// rounded: a value that cannot be represented in whole minor units is not
// invented.
func TestValuation_TruncatesTheAverageInsteadOfRoundingIt(t *testing.T) {
	cases := []struct {
		name string
		lots []lot
		want int64
	}{
		{
			name: "100.25 truncates to 100, it does not round to 100",
			lots: []lot{{3, 100}, {1, 101}},
			want: 100,
		},
		{
			name: "199.5 truncates to 199, it does not round to 200",
			lots: []lot{{1, 199}, {1, 200}},
			want: 199,
		},
		{
			name: "an exact average is exact",
			lots: []lot{{2, 150}, {2, 250}},
			want: 200,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			valuation := inventory.Valuation{ItemID: "i-1", Currency: vnd}

			for i, l := range c.lots {
				next, err := valuation.Receive(l.qty, cents(l.cost))
				if err != nil {
					t.Fatalf("Receive lot %d: %v", i+1, err)
				}
				valuation = next
			}

			if valuation.AvgCost != c.want {
				t.Errorf("average = %d, want %d", valuation.AvgCost, c.want)
			}
		})
	}
}

// An amount a float64 cannot hold survives the whole way through. 2^53 + 1 is
// the smallest integer a float64 rounds away, and a valuation that had been
// computed in one would come back as 2^53.
func TestValuation_KeepsAmountsFloatingPointCannotHold(t *testing.T) {
	const beyond = int64(1)<<53 + 1 // 9007199254740993

	valuation := inventory.Valuation{ItemID: "i-1", Currency: vnd}

	next, err := valuation.Receive(1, cents(beyond))
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if next.AvgCost != beyond {
		t.Fatalf("average = %d, want %d exactly — a float64 would have rounded it", next.AvgCost, beyond)
	}
	if got := next.Value().Amount(); got != beyond {
		t.Errorf("value = %d, want %d", got, beyond)
	}

	after, cost, err := next.Issue(1)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if cost.Amount() != beyond {
		t.Errorf("the issue cost %d, want %d", cost.Amount(), beyond)
	}
	if after.Quantity != 0 || after.AvgCost != 0 {
		t.Errorf("after the issue = %d units at %d, want empty", after.Quantity, after.AvgCost)
	}
}

// A large numerator is the case the average overflows on: 9e15 units at 9e15
// minor units is a product no int64 holds, and a wrapped one would produce an
// average that is wrong in the direction of more money.
func TestValuation_DoesNotOverflowOnALargeHolding(t *testing.T) {
	valuation := inventory.Valuation{ItemID: "i-1", Currency: vnd}

	valuation, err := valuation.Receive(9_000_000_000_000_000, cents(1_000_000_000_000_000))
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if valuation.AvgCost != 1_000_000_000_000_000 {
		t.Fatalf("average = %d, want the price paid", valuation.AvgCost)
	}

	// A second lot at a tenth the price: the average is weighted, not wrapped.
	next, err := valuation.Receive(9_000_000_000_000_000, cents(100_000_000_000_000))
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if want := int64(550_000_000_000_000); next.AvgCost != want {
		t.Errorf("average = %d, want %d", next.AvgCost, want)
	}
}

// One ledger, one currency. A receipt costed in another is a rate nobody
// applied and a date nobody agreed.
func TestValuation_RefusesAReceiptInAnotherCurrency(t *testing.T) {
	valuation := inventory.Valuation{ItemID: "i-1", Currency: vnd}

	_, err := valuation.Receive(1, money.New(100, money.Currency("USD")))
	if !errors.Is(err, inventory.ErrCurrencyMismatch) {
		t.Fatalf("Receive in USD = %v, want ErrCurrencyMismatch", err)
	}
}

// A movement of nothing is not a movement, in either direction.
func TestValuation_RefusesAQuantityThatIsNotPositive(t *testing.T) {
	valuation := inventory.Valuation{ItemID: "i-1", Currency: vnd}

	for _, qty := range []int{0, -1} {
		if _, err := valuation.Receive(qty, cents(100)); !errors.Is(err, inventory.ErrInvalidMove) {
			t.Errorf("Receive(%d) = %v, want ErrInvalidMove", qty, err)
		}
		if _, _, err := valuation.Issue(qty); !errors.Is(err, inventory.ErrInvalidMove) {
			t.Errorf("Issue(%d) = %v, want ErrInvalidMove", qty, err)
		}
	}
}

// A receipt that names no currency cannot be folded into an average.
func TestValuation_RefusesAReceiptThatNamesNoCurrency(t *testing.T) {
	valuation := inventory.Valuation{ItemID: "i-1", Currency: vnd}

	if _, err := valuation.Receive(1, money.Money{}); !errors.Is(err, inventory.ErrInvalidMove) {
		t.Fatalf("Receive of an amount with no currency = %v, want ErrInvalidMove", err)
	}
}
