package money

import (
	"fmt"
	"math/big"
	"sort"
)

// Allocate divides a total among recipients in proportion to their weights, in
// whole minor units, so that the parts add back to the total exactly.
//
// Largest remainder: each recipient first takes the floor of its exact share,
// and the units left over go one at a time to the recipients with the largest
// fractional claim. Ties go to the earlier recipient. The rule is arbitrary but
// fixed — the same input must always give the same split, or two runs of the
// same report disagree with each other.
//
// Weights that sum to zero carry no proportion, so the total is split equally.
// A negative weight panics: it is not a proportion, and accepting it would move
// money silently from one recipient to another.
func Allocate(total Money, weights []int) []Money {
	if len(weights) == 0 {
		return []Money{}
	}

	// Copied rather than adjusted in place: the caller's slice is theirs.
	effective := make([]int, len(weights))
	sum := 0

	for i, weight := range weights {
		if weight < 0 {
			panic(fmt.Sprintf("money: weight %d is %d, and a negative weight is not a proportion", i, weight))
		}
		effective[i] = weight
		sum += weight
	}

	if sum == 0 {
		for i := range effective {
			effective[i] = 1
		}
		sum = len(effective)
	}

	amount := total.amount
	negative := amount < 0
	if negative {
		amount = -amount
	}

	// The products and divisions run in big integers. amount × weight overflows
	// int64 long before the amounts a ledger deals with, and a wrapped product
	// would break the one guarantee this function makes.
	bigAmount := big.NewInt(amount)
	bigSum := big.NewInt(int64(sum))

	type share struct {
		index     int
		floor     int64
		remainder *big.Int
	}

	shares := make([]share, len(effective))
	floored := int64(0)

	for i, weight := range effective {
		product := new(big.Int).Mul(bigAmount, big.NewInt(int64(weight)))

		floor := new(big.Int).Quo(product, bigSum)
		remainder := new(big.Int).Rem(product, bigSum)

		shares[i] = share{index: i, floor: floor.Int64(), remainder: remainder}
		floored += floor.Int64()
	}

	// Distributing at most one unit per recipient, largest fractional claim
	// first. Σfloor is at most the amount and at least amount−n, so what is left
	// is fewer than n units.
	order := make([]share, len(shares))
	copy(order, shares)

	sort.Slice(order, func(a, b int) bool {
		if cmp := order[a].remainder.Cmp(order[b].remainder); cmp != 0 {
			return cmp > 0
		}

		return order[a].index < order[b].index
	})

	amounts := make([]int64, len(effective))
	for _, s := range shares {
		amounts[s.index] = s.floor
	}

	for i := int64(0); i < amount-floored; i++ {
		amounts[order[i].index]++
	}

	parts := make([]Money, len(effective))
	for i, a := range amounts {
		if negative {
			a = -a
		}
		parts[i] = Money{amount: a, currency: total.currency}
	}

	return parts
}
