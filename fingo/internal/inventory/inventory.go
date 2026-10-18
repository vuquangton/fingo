// Package inventory values stock at the weighted moving average.
//
// Every movement in and out of a warehouse is recorded, and what the stock on
// hand is worth is a function of that history. Nothing is stored as a running
// total that could drift from the movements that produced it: the level is the
// sum of the movements, and the average is recomputed on every receipt.
//
// The average is held per unit, as an integer in the currency's minor unit — a
// whole đồng for VND, cents for a currency that has them. Integer arithmetic
// throughout: the average of two prices is very often not a price, and an
// implementation that computed it in binary floating point would carry a
// fraction of a unit into every later valuation. See Valuation.AvgCost for what
// the integer average costs, which is not nothing, and internal/domain/inventory
// for the other way of doing it.
package inventory

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"fingo/internal/money"
)

// Errors a caller may need to tell apart.
var (
	// ErrInvalidItem is an item that could not be stocked: no id, no SKU, no
	// name, no unit, or a reorder point below zero.
	ErrInvalidItem = errors.New("inventory: invalid item")

	// ErrInvalidMove is a movement that could not be recorded: no id, no item,
	// an unknown direction, a quantity that is not positive, no date, or no
	// document behind it.
	ErrInvalidMove = errors.New("inventory: invalid stock move")

	// ErrUnknownItem is a movement or a query for an item the ledger does not
	// hold.
	ErrUnknownItem = errors.New("inventory: no such item")

	// ErrItemExists is a second item under an id already in the ledger.
	ErrItemExists = errors.New("inventory: an item with this id already exists")

	// ErrCurrencyMismatch is a movement costed in another currency than the one
	// the ledger keeps.
	ErrCurrencyMismatch = errors.New("inventory: the movement is in a different currency")

	// ErrNegativeStock is an issue of more than is on hand.
	//
	// It is an error rather than a negative balance because the alternative is
	// stock that does not exist: goods that left a warehouse never held, costed
	// at an average nobody paid. Where the ledger genuinely is behind, the
	// missing receipt is recorded, and the issue then has something to take.
	ErrNegativeStock = errors.New("inventory: an issue would take the stock below zero")
)

// Direction is which way stock moved.
type Direction string

const (
	// DirectionIn is a receipt: goods arriving, at what they cost.
	DirectionIn Direction = "in"

	// DirectionOut is an issue: goods leaving, valued at the average.
	DirectionOut Direction = "out"
)

// Valid reports whether the direction is one the ledger knows.
func (d Direction) Valid() bool {
	switch d {
	case DirectionIn, DirectionOut:
		return true
	}

	return false
}

// Item is something the business keeps in stock.
type Item struct {
	// ID identifies the item. Stable for the life of the books, because it is
	// what every movement refers to.
	ID string

	// SKU is the code the item is picked by (mã hàng hoá).
	SKU string

	// Name is what the item is called.
	Name string

	// Unit is what it is counted in: cái, kg, lít, thùng.
	Unit string

	// ReorderPoint is the level at or below which the item should be reordered.
	// Zero means no level has been set, and the item is never reported as low.
	ReorderPoint int
}

// Validate reports whether the item could be stocked and found again.
func (i Item) Validate() error {
	if strings.TrimSpace(i.ID) == "" {
		return fmt.Errorf("%w: an item needs an id", ErrInvalidItem)
	}
	if strings.TrimSpace(i.SKU) == "" {
		return fmt.Errorf("%w: item %s has no SKU", ErrInvalidItem, i.ID)
	}
	if strings.TrimSpace(i.Name) == "" {
		return fmt.Errorf("%w: item %s has no name", ErrInvalidItem, i.ID)
	}
	if strings.TrimSpace(i.Unit) == "" {
		return fmt.Errorf("%w: item %s is counted in nothing", ErrInvalidItem, i.ID)
	}
	if i.ReorderPoint < 0 {
		return fmt.Errorf("%w: item %s has a reorder point of %d", ErrInvalidItem, i.ID, i.ReorderPoint)
	}

	return nil
}

// StockMove is one movement of one item.
//
// A movement is never edited. A mistake is corrected by a movement the other
// way, dated when it was noticed: the history is what the valuation is derived
// from, and a history that can be rewritten afterwards is not one an audit can
// reproduce.
type StockMove struct {
	// ID identifies the movement.
	ID string

	// ItemID is the item that moved.
	ItemID string

	// Direction is which way it moved.
	Direction Direction

	// Qty is how many units moved. Always positive: the direction says which
	// way, and a negative quantity would say it twice.
	Qty int

	// UnitCost is what one unit cost.
	//
	// On a receipt it is what was paid, and is required. On an issue the
	// valuation decides it — the current average — and the ledger records what
	// it decided here, so that a movement log read years later says what each
	// issue was costed at rather than leaving it to be recomputed.
	UnitCost money.Money

	// Date is the business date the movement belongs to.
	Date time.Time

	// SourceDocID is the document the movement came from: the goods receipt,
	// the delivery note, the invoice.
	//
	// Required. A movement nobody can trace back to a document is a movement
	// nobody can explain to an auditor, and stock that changes without paper is
	// how a warehouse and a ledger come apart.
	SourceDocID string
}

// Validate reports whether the movement could be recorded, without reference to
// what is on hand.
func (m StockMove) Validate() error {
	if strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("%w: a movement needs an id", ErrInvalidMove)
	}
	if strings.TrimSpace(m.ItemID) == "" {
		return fmt.Errorf("%w: movement %s names no item", ErrInvalidMove, m.ID)
	}
	if !m.Direction.Valid() {
		return fmt.Errorf("%w: movement %s has direction %q, which is not one of in, out",
			ErrInvalidMove, m.ID, m.Direction)
	}
	if m.Qty <= 0 {
		return fmt.Errorf("%w: movement %s moves %d units, and a movement moves something",
			ErrInvalidMove, m.ID, m.Qty)
	}
	if m.Date.IsZero() {
		return fmt.Errorf("%w: movement %s has no date", ErrInvalidMove, m.ID)
	}
	if strings.TrimSpace(m.SourceDocID) == "" {
		return fmt.Errorf("%w: movement %s comes from no document", ErrInvalidMove, m.ID)
	}
	if m.UnitCost.IsNegative() {
		return fmt.Errorf("%w: movement %s costs %s a unit, which is less than nothing",
			ErrInvalidMove, m.ID, m.UnitCost)
	}

	return nil
}
