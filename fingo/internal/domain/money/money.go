// Package money holds the arithmetic rules for Vietnamese đồng.
//
// VND has no subunit: every amount that reaches the ledger is a whole number of
// đồng. Rounding therefore happens at defined points rather than implicitly, and
// every computed figure passes through this package before it becomes a ledger
// line. See docs/adr/0001-journal-line-model.md.
package money

import (
	"errors"
	"fmt"
	"sort"

	"github.com/shopspring/decimal"
)

var hundred = decimal.NewFromInt(100)

// Dong rounds an amount to whole đồng, half away from zero.
func Dong(d decimal.Decimal) decimal.Decimal {
	return d.Round(0)
}

// LineNet returns the value of a line — quantity × unit price — in whole đồng.
func LineNet(quantity, unitPrice decimal.Decimal) decimal.Decimal {
	return Dong(quantity.Mul(unitPrice))
}

// LineVAT returns the VAT on an already-rounded net amount.
//
// ratePercent is a percentage: 10 means ten per cent, matching how the rates are
// written in the circulars. Rounding is per line, so an invoice total is always
// the sum of its rounded lines and never disagrees with them.
func LineVAT(net, ratePercent decimal.Decimal) decimal.Decimal {
	if ratePercent.IsZero() {
		return decimal.Zero
	}

	return Dong(net.Mul(ratePercent).Div(hundred))
}

// Sum adds amounts.
func Sum(amounts ...decimal.Decimal) decimal.Decimal {
	total := decimal.Zero
	for _, a := range amounts {
		total = total.Add(a)
	}
	return total
}

// Allocate splits total across weights so the parts sum to exactly total.
//
// This is largest-remainder apportionment: each part receives its exact share
// truncated to whole đồng, and the leftover đồng go to the parts with the
// largest fractional remainders, ties going to the earlier part.
//
// It exists because whole đồng do not divide evenly. A landed cost of 1,000,000
// spread over three lines must neither lose nor invent a đồng, and a payment
// applied across several invoices must consume the debt exactly.
//
// total must be a whole number of đồng, weights must not be negative. When every
// weight is zero the total is split equally, which preserves the summation
// invariant rather than silently dropping it.
func Allocate(total decimal.Decimal, weights []decimal.Decimal) ([]decimal.Decimal, error) {
	if len(weights) == 0 {
		return nil, errors.New("allocate requires at least one weight")
	}
	if total.IsNegative() {
		return nil, fmt.Errorf("allocate: total %s must not be negative", total)
	}
	if !total.Equal(total.Truncate(0)) {
		return nil, fmt.Errorf("allocate: total %s is not a whole number of đồng", total)
	}

	totalWeight := decimal.Zero
	for i, w := range weights {
		if w.IsNegative() {
			return nil, fmt.Errorf("allocate: weight %d is negative (%s)", i, w)
		}
		totalWeight = totalWeight.Add(w)
	}

	// Degenerate input: split equally so the parts still sum to the total.
	if totalWeight.IsZero() {
		equal := make([]decimal.Decimal, len(weights))
		for i := range equal {
			equal[i] = decimal.NewFromInt(1)
		}
		return apportion(total, equal, decimal.NewFromInt(int64(len(equal)))), nil
	}

	return apportion(total, weights, totalWeight), nil
}

// apportion performs the largest-remainder split. totalWeight is passed in
// because the equal-split fallback does not use the caller's weights.
func apportion(total decimal.Decimal, weights []decimal.Decimal, totalWeight decimal.Decimal) []decimal.Decimal {
	parts := make([]decimal.Decimal, len(weights))
	fractions := make([]decimal.Decimal, len(weights))

	allocated := decimal.Zero

	for i, w := range weights {
		exact := total.Mul(w).Div(totalWeight)
		floor := exact.Truncate(0)

		parts[i] = floor
		fractions[i] = exact.Sub(floor)
		allocated = allocated.Add(floor)
	}

	leftover := total.Sub(allocated).IntPart()

	order := make([]int, len(weights))
	for i := range order {
		order[i] = i
	}
	// Stable, so equal remainders keep their original order and the earlier
	// part takes the odd đồng.
	sort.SliceStable(order, func(a, b int) bool {
		return fractions[order[a]].GreaterThan(fractions[order[b]])
	})

	for i := int64(0); i < leftover; i++ {
		index := order[i%int64(len(order))]
		parts[index] = parts[index].Add(decimal.NewFromInt(1))
	}

	return parts
}
