package inventory

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Kind is whether a movement brought stock in or took it out.
type Kind string

const (
	KindReceipt Kind = "RECEIPT"
	KindIssue   Kind = "ISSUE"
)

func (k Kind) Valid() bool {
	switch k {
	case KindReceipt, KindIssue:
		return true
	}
	return false
}

// Movement is one change to an item's stock.
//
// Quantity is fractional — kilograms, litres, metres — while Cost is whole đồng.
// The two are different kinds of number and are stored as such.
type Movement struct {
	ID            string
	ItemCode      string
	WarehouseCode string
	Kind          Kind
	Quantity      decimal.Decimal
	Cost          decimal.Decimal // total cost of this movement, in đồng
	VoucherID     string
	MovedAt       time.Time
}

// Replay rebuilds a stock position from its movements, in order.
//
// The movements are the record and the balance is a function of them. Nothing
// stores a running total that could drift from the history that produced it, so
// re-deriving the balance during an audit, or after a restore, gives the same
// answer as the day it was written.
//
// It also re-checks the arithmetic rather than trusting it: an issue's recorded
// cost must equal what the replay produces. If they disagree, the log and the
// balance have parted company and neither can be relied on, so the caller is
// told rather than handed a number that is quietly wrong.
func Replay(movements []Movement) (Stock, error) {
	if len(movements) == 0 {
		return Stock{}, errors.New("a stock balance needs at least one movement")
	}

	first := movements[0]
	if first.ItemCode == "" {
		return Stock{}, errors.New("a stock movement needs an item")
	}

	position := NewStock(first.ItemCode, first.WarehouseCode)

	for i, m := range movements {
		if m.ItemCode != first.ItemCode || m.WarehouseCode != first.WarehouseCode {
			return Stock{}, fmt.Errorf(
				"movement %d is for %s in %s, but the balance is for %s in %s",
				i+1, m.ItemCode, m.WarehouseCode, first.ItemCode, first.WarehouseCode)
		}

		var err error

		switch m.Kind {
		case KindReceipt:
			position, err = position.Receive(m.Quantity, m.Cost)
			if err != nil {
				return Stock{}, fmt.Errorf("movement %d: %w", i+1, err)
			}

		case KindIssue:
			var cost decimal.Decimal

			position, cost, err = position.Issue(m.Quantity)
			if err != nil {
				return Stock{}, fmt.Errorf("movement %d: %w", i+1, err)
			}

			if m.Cost.IsPositive() && !m.Cost.Equal(cost) {
				return Stock{}, fmt.Errorf(
					"movement %d recorded a cost of %s đồng but replays as %s",
					i+1, m.Cost, cost)
			}

		default:
			return Stock{}, fmt.Errorf("movement %d has kind %q, which is neither a receipt nor an issue",
				i+1, m.Kind)
		}
	}

	return position, nil
}
