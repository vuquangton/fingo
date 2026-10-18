package inventory_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/inventory"
)

func at(day int) time.Time {
	return time.Date(2026, 3, day, 9, 0, 0, 0, time.UTC)
}

func receipt(quantity, cost int64, day int) inventory.Movement {
	return inventory.Movement{
		ItemCode:      "VT01",
		WarehouseCode: "KHO1",
		Kind:          inventory.KindReceipt,
		Quantity:      d(quantity),
		Cost:          d(cost),
		MovedAt:       at(day),
	}
}

func issue(quantity int64, day int) inventory.Movement {
	return inventory.Movement{
		ItemCode:      "VT01",
		WarehouseCode: "KHO1",
		Kind:          inventory.KindIssue,
		Quantity:      d(quantity),
		MovedAt:       at(day),
	}
}

// The balance is what the history says it is.
func TestReplay_RebuildsThePositionFromHistory(t *testing.T) {
	position, err := inventory.Replay([]inventory.Movement{
		receipt(10, 100_000, 1),
		receipt(10, 140_000, 5),
		issue(5, 8),
	})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if !position.Quantity.Equal(d(15)) {
		t.Errorf("quantity = %s, want 15", position.Quantity)
	}
	if !position.Value.Equal(d(180_000)) {
		t.Errorf("value = %s, want 180000", position.Value)
	}
}

// The same history always produces the same balance, so an audit that re-derives
// it cannot reach a different answer from the day it was written.
func TestReplay_IsDeterministic(t *testing.T) {
	history := []inventory.Movement{
		receipt(7, 1000, 1),
		issue(3, 2),
		receipt(11, 1777, 4),
		issue(5, 6),
	}

	first, err := inventory.Replay(history)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	second, err := inventory.Replay(history)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if !first.Quantity.Equal(second.Quantity) || !first.Value.Equal(second.Value) {
		t.Errorf("two replays disagree: %s/%s and %s/%s",
			first.Quantity, first.Value, second.Quantity, second.Value)
	}
}

// A recorded issue cost that disagrees with the replay means the log and the
// balance have parted company — worth refusing rather than silently trusting.
func TestReplay_RefusesACostThatDisagreesWithTheReplay(t *testing.T) {
	wrong := issue(5, 8)
	wrong.Cost = decimal.NewFromInt(999)

	if _, err := inventory.Replay([]inventory.Movement{
		receipt(10, 100_000, 1),
		wrong,
	}); err == nil {
		t.Fatal("a recorded cost that disagrees with the replay must be refused, got nil")
	}
}

// The right cost passes the same check, so the guard does not reject honest logs.
func TestReplay_AcceptsARecordedCostThatMatches(t *testing.T) {
	honest := issue(5, 8)
	honest.Cost = decimal.NewFromInt(50_000) // 10 units at 100000, then 5 at the average

	if _, err := inventory.Replay([]inventory.Movement{
		receipt(10, 100_000, 1),
		honest,
	}); err != nil {
		t.Fatalf("an honest log must replay: %v", err)
	}
}

// History that cannot have happened is refused: stock that was never there
// cannot have gone out.
func TestReplay_RefusesAnIssueBeyondWhatWasEverReceived(t *testing.T) {
	if _, err := inventory.Replay([]inventory.Movement{
		receipt(10, 100_000, 1),
		issue(11, 2),
	}); err == nil {
		t.Fatal("issuing more than was received must be refused, got nil")
	}
}

// One balance cannot span two items or two warehouses, so a mixed history is a
// caller error rather than something to average together.
func TestReplay_RefusesAMixedHistory(t *testing.T) {
	other := receipt(5, 500, 2)
	other.ItemCode = "VT02"

	if _, err := inventory.Replay([]inventory.Movement{receipt(10, 1000, 1), other}); err == nil {
		t.Fatal("movements for two items must be refused, got nil")
	}

	otherWarehouse := receipt(5, 500, 2)
	otherWarehouse.WarehouseCode = "KHO2"

	if _, err := inventory.Replay([]inventory.Movement{receipt(10, 1000, 1), otherWarehouse}); err == nil {
		t.Fatal("movements for two warehouses must be refused, got nil")
	}
}

func TestReplay_RefusesAnEmptyOrUnknownHistory(t *testing.T) {
	if _, err := inventory.Replay(nil); err == nil {
		t.Error("an empty history must be refused, got nil")
	}

	mystery := receipt(10, 1000, 1)
	mystery.Kind = inventory.Kind("MYSTERY")

	if _, err := inventory.Replay([]inventory.Movement{mystery}); err == nil {
		t.Error("an unknown movement kind must be refused, got nil")
	}
}

// A run that empties the item must leave nothing behind, however awkward the
// numbers were on the way.
func TestReplay_AHistoryThatEmptiesTheItemEndsAtNothing(t *testing.T) {
	position, err := inventory.Replay([]inventory.Movement{
		receipt(3, 100, 1),
		receipt(7, 250, 2),
		issue(10, 3),
	})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if !position.Quantity.IsZero() || !position.Value.IsZero() {
		t.Errorf("position = %s units worth %s, want nothing", position.Quantity, position.Value)
	}
}
