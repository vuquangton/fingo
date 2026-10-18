package inventory

import (
	"fmt"
	"math/big"

	"fingo/internal/money"
)

// Valuation is what one item's stock on hand is worth, at the weighted moving
// average.
//
// Inbound and outbound are the only two things that change it. A receipt folds
// what was paid into the average; an issue takes stock away at the average
// already standing, and leaves the average alone — which is what "moving
// average" means, and why issuing the whole holding takes the whole remaining
// value with it.
type Valuation struct {
	// ItemID is the item this is the valuation of.
	ItemID string

	// Quantity is how many units are on hand.
	Quantity int

	// AvgCost is the moving average cost of ONE unit, in the currency's minor
	// unit: whole đồng for VND, cents for a currency with cents.
	//
	// An integer, and truncated toward zero when it is recomputed. That
	// truncation is the price of holding a per-unit average at all: 3 units
	// bought for 100 and 1 bought for 101 cost 100.25 each, which is not a
	// number of minor units, and the average is kept at 100 rather than at a
	// fraction. The effect is that the carrying value on hand is at most one
	// minor unit per unit below what was paid for it, and that the same
	// movements always produce the same figures — which is what an audit needs.
	//
	// internal/domain/inventory takes the other view and carries the exact
	// total instead, so that nothing is lost at all. The two are not
	// reconcilable by arithmetic; they are a choice, and this one is the choice
	// asked for here.
	AvgCost int64

	// Currency is the currency the item is costed in.
	Currency money.Currency
}

// Value is what the stock on hand is carried at: the average cost of a unit,
// times the units.
//
// The product must fit an int64 of minor units. That is the same bound every
// amount in this codebase already has — a holding worth more than the largest
// representable Money could not be expressed as one.
func (v Valuation) Value() money.Money {
	if !v.Currency.Valid() || v.Quantity == 0 || v.AvgCost == 0 {
		return money.Money{}
	}

	return money.New(v.AvgCost*int64(v.Quantity), v.Currency)
}

// Receive adds stock at what it cost per unit, and recomputes the average.
//
// The new average is
//
//	(prev_qty × prev_avg + in_qty × in_cost) ÷ (prev_qty + in_qty)
//
// in integer arithmetic, truncated toward zero. Every receipt goes through it,
// so the average is always a function of the whole history and never of the
// last price paid.
func (v Valuation) Receive(qty int, unitCost money.Money) (Valuation, error) {
	if qty <= 0 {
		return v, fmt.Errorf("%w: a receipt of %d units is not a receipt", ErrInvalidMove, qty)
	}
	if !unitCost.Currency().Valid() {
		return v, fmt.Errorf("%w: a receipt costing %s names no currency", ErrInvalidMove, unitCost)
	}
	if unitCost.IsNegative() {
		return v, fmt.Errorf("%w: a receipt costing %s a unit is less than nothing", ErrInvalidMove, unitCost)
	}
	if v.Currency.Valid() && v.Currency != unitCost.Currency() {
		return v, fmt.Errorf("%w: the item is costed in %s and the receipt is in %s",
			ErrCurrencyMismatch, v.Currency, unitCost.Currency())
	}

	next := v
	next.Currency = unitCost.Currency()
	next.Quantity = v.Quantity + qty
	next.AvgCost = weightedAverage(v.Quantity, v.AvgCost, qty, unitCost.Amount(), next.Quantity)
	next.ItemID = v.ItemID

	return next, nil
}

// Issue takes stock away at the average standing now, and reports what it cost.
//
// The average does not move: issuing at the average is what makes an issue
// cost-neutral to the remaining stock. An empty holding is left worth nothing at
// no rate, rather than at whatever the last average happened to be.
func (v Valuation) Issue(qty int) (Valuation, money.Money, error) {
	if qty <= 0 {
		return v, money.Money{}, fmt.Errorf("%w: an issue of %d units is not an issue", ErrInvalidMove, qty)
	}
	if qty > v.Quantity {
		return v, money.Money{}, fmt.Errorf("%w: cannot issue %d of %s: only %d is on hand",
			ErrNegativeStock, qty, v.ItemID, v.Quantity)
	}

	next := v
	next.Quantity = v.Quantity - qty
	if next.Quantity == 0 {
		next.AvgCost = 0
	}

	return next, money.New(v.AvgCost*int64(qty), v.Currency), nil
}

// Apply folds one movement into the valuation, and reports what the movement was
// worth.
//
// A receipt is worth what was paid for it. An issue is worth the quantity at the
// average standing when it left.
func (v Valuation) Apply(move StockMove) (Valuation, money.Money, error) {
	switch move.Direction {
	case DirectionIn:
		next, err := v.Receive(move.Qty, move.UnitCost)
		if err != nil {
			return v, money.Money{}, err
		}

		return next, money.New(move.UnitCost.Amount()*int64(move.Qty), move.UnitCost.Currency()), nil

	case DirectionOut:
		return v.Issue(move.Qty)
	}

	return v, money.Money{}, fmt.Errorf("%w: movement %s has direction %q", ErrInvalidMove, move.ID, move.Direction)
}

// weightedAverage is the moving average, in integer arithmetic.
//
// The numerator is formed in arbitrary precision. prev_qty × prev_avg overflows
// an int64 long before the quantities and costs a real warehouse deals with
// reach their limits, and a wrapped product would produce an average that is not
// merely imprecise but wrong in the direction of more money.
//
// The quotient is truncated toward zero, which for the non-negative inputs
// validation guarantees is the same as rounding down.
func weightedAverage(prevQty int, prevAvg int64, inQty int, inCost int64, totalQty int) int64 {
	numerator := new(big.Int).Add(
		new(big.Int).Mul(big.NewInt(int64(prevQty)), big.NewInt(prevAvg)),
		new(big.Int).Mul(big.NewInt(int64(inQty)), big.NewInt(inCost)),
	)

	return new(big.Int).Quo(numerator, big.NewInt(int64(totalQty))).Int64()
}
