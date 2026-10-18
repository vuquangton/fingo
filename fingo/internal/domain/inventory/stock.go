package inventory

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/money"
)

// Stock is what one item in one warehouse is worth.
//
// Value is carried as a TOTAL, not as a unit cost. VND has no subunit, so a unit
// cost is often not representable — 3 units for 100 đồng is 33.333… each.
// Keeping the total exact and dividing only when a unit cost is asked for is what
// stops fractions of a đồng accumulating into inventory that is not in the
// warehouse.
type Stock struct {
	ItemCode    string
	WarehouseID string
	Quantity    decimal.Decimal
	Value       decimal.Decimal
}

// NewStock is an item with nothing on hand.
func NewStock(itemCode, warehouseID string) Stock {
	return Stock{
		ItemCode:    itemCode,
		WarehouseID: warehouseID,
		Quantity:    decimal.Zero,
		Value:       decimal.Zero,
	}
}

// UnitCost is the moving average cost of one unit.
//
// It is reported for display and for pricing, and is never the basis of an
// issue: a cost of goods sold derived from a rounded unit cost does not
// reconcile with the value it was taken from.
func (s Stock) UnitCost() decimal.Decimal {
	if !s.Quantity.IsPositive() {
		return decimal.Zero
	}

	return s.Value.Div(s.Quantity)
}

// Receive adds stock at what it cost in total.
//
// A total rather than a unit cost because giá gốc includes transport, handling
// and insurance (TT133, TK 152 khoản 1.b): pushing those through a per-unit
// figure on the way in would round a fraction of a đồng per unit straight into
// the inventory value.
func (s Stock) Receive(quantity, totalCost decimal.Decimal) (Stock, error) {
	if !quantity.IsPositive() {
		return s, errors.New("a receipt needs a quantity greater than zero")
	}
	if totalCost.IsNegative() {
		return s, errors.New("a receipt must not have a negative cost")
	}

	s.Quantity = s.Quantity.Add(quantity)
	s.Value = s.Value.Add(totalCost)

	return s, nil
}

// Issue removes stock and reports what it cost.
func (s Stock) Issue(quantity decimal.Decimal) (Stock, decimal.Decimal, error) {
	if !quantity.IsPositive() {
		return s, decimal.Zero, errors.New("an issue needs a quantity greater than zero")
	}
	if quantity.GreaterThan(s.Quantity) {
		return s, decimal.Zero, fmt.Errorf("cannot issue %s of %s: only %s is on hand",
			quantity, s.ItemCode, s.Quantity)
	}

	var cost decimal.Decimal

	if quantity.Equal(s.Quantity) {
		// Issuing the whole holding takes the whole remaining value. Taking
		// quantity × average instead would leave a fraction of a đồng behind in
		// an item with nothing left in it — stock on the balance sheet that is
		// not in the warehouse.
		cost = s.Value
	} else {
		cost = money.Dong(s.Value.Mul(quantity).Div(s.Quantity))
	}

	s.Quantity = s.Quantity.Sub(quantity)
	s.Value = s.Value.Sub(cost)

	// Exact by construction; the guard keeps the invariant true even if that
	// ever stops being so. Empty must mean worth nothing.
	if s.Quantity.IsZero() {
		s.Value = decimal.Zero
	}

	return s, cost, nil
}
