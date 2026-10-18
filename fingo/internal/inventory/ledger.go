package inventory

import (
	"fmt"
	"sort"
	"sync"

	"fingo/internal/money"
)

// Ledger is the stock of every item, derived from the movements recorded against
// it.
//
// Only movements are held. The level is the sum of them and the valuation is
// folded from them, so there is no running total that can come apart from the
// history that produced it — re-deriving what an item was worth in March gives
// the same answer in December.
//
// A Ledger is safe for concurrent use. Record holds its lock across the whole of
// the valuation, so two issues arriving together cannot both find the same stock
// on hand and both take it.
type Ledger struct {
	mu sync.Mutex

	currency   money.Currency
	items      map[string]Item
	moves      map[string][]StockMove
	valuations map[string]Valuation

	// returned counts what has come back against each issue, so that a run of
	// returns cannot together exceed what went out. See RecordReturn.
	returned map[string]int
}

// NewLedger returns an empty ledger for one currency.
//
// A ledger is one client's books, and a client's books are kept in one currency:
// valuing the same warehouse in đồng and in dollars at once would need a rate,
// and a rate has a date, which is a different problem on every movement.
func NewLedger(currency money.Currency) (*Ledger, error) {
	if !currency.Valid() {
		return nil, fmt.Errorf("inventory: %q is not a three-letter currency code", currency)
	}

	return &Ledger{
		currency:   currency,
		items:      make(map[string]Item),
		moves:      make(map[string][]StockMove),
		valuations: make(map[string]Valuation),
		returned:   make(map[string]int),
	}, nil
}

// Currency is what the ledger is kept in.
func (l *Ledger) Currency() money.Currency { return l.currency }

// AddItem stocks a new item.
func (l *Ledger) AddItem(item Item) error {
	if err := item.Validate(); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.items[item.ID]; exists {
		return fmt.Errorf("%w: %s", ErrItemExists, item.ID)
	}
	l.items[item.ID] = item

	return nil
}

// Item returns one item.
func (l *Ledger) Item(itemID string) (Item, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	item, ok := l.items[itemID]
	if !ok {
		return Item{}, fmt.Errorf("%w: %s", ErrUnknownItem, itemID)
	}

	return item, nil
}

// Items returns every item, by SKU and then id.
func (l *Ledger) Items() []Item {
	l.mu.Lock()
	defer l.mu.Unlock()

	out := make([]Item, 0, len(l.items))
	for _, item := range l.items {
		out = append(out, item)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].SKU != out[j].SKU {
			return out[i].SKU < out[j].SKU
		}

		return out[i].ID < out[j].ID
	})

	return out
}

// Record folds one movement into an item's stock, and reports what the movement
// was worth.
//
// A receipt is worth what was paid for it and moves the average; an issue is
// worth the quantity at the average standing when it left, and does not. An
// issue larger than the stock on hand is refused — see ErrNegativeStock.
//
// The decision and the write happen under one lock, so a level is never read by
// two issues at once.
func (l *Ledger) Record(move StockMove) (money.Money, error) {
	if err := move.Validate(); err != nil {
		return money.Money{}, err
	}

	if move.UnitCost.Currency().Valid() && move.UnitCost.Currency() != l.currency {
		return money.Money{}, fmt.Errorf("%w: movement %s is costed in %s and the ledger keeps %s",
			ErrCurrencyMismatch, move.ID, move.UnitCost.Currency(), l.currency)
	}
	if move.Direction == DirectionIn && !move.UnitCost.Currency().Valid() {
		return money.Money{}, fmt.Errorf("%w: receipt %s does not say what a unit cost",
			ErrInvalidMove, move.ID)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	item, known := l.items[move.ItemID]
	if !known {
		return money.Money{}, fmt.Errorf("%w: %s", ErrUnknownItem, move.ItemID)
	}

	current := l.valuationLocked(item.ID)

	next, value, err := current.Apply(move)
	if err != nil {
		return money.Money{}, fmt.Errorf("movement %s of item %s: %w", move.ID, item.ID, err)
	}

	// An issue is recorded at the average it was actually costed at. Storing
	// zero and leaving every reader to work the average out again is how a
	// movement log stops being a record and becomes an input.
	if move.Direction == DirectionOut {
		move.UnitCost = money.New(next.AvgCost, l.currency)
	}

	l.valuations[item.ID] = next
	l.moves[item.ID] = append(l.moves[item.ID], move)

	return value, nil
}

// Level is how many units of an item are on hand: the sum of every movement
// recorded, in less every movement out.
//
// It is summed rather than read from the valuation, because the movements are
// what happened and the valuation is what they are worth. The two agree by
// construction, and a test says so.
func (l *Ledger) Level(itemID string) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, known := l.items[itemID]; !known {
		return 0, fmt.Errorf("%w: %s", ErrUnknownItem, itemID)
	}

	level := 0
	for _, move := range l.moves[itemID] {
		switch move.Direction {
		case DirectionIn:
			level += move.Qty
		case DirectionOut:
			level -= move.Qty
		}
	}

	return level, nil
}

// Valuation returns what an item's stock on hand is worth.
func (l *Ledger) Valuation(itemID string) (Valuation, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, known := l.items[itemID]; !known {
		return Valuation{}, fmt.Errorf("%w: %s", ErrUnknownItem, itemID)
	}

	return l.valuationLocked(itemID), nil
}

// Moves returns an item's movements, in the order they were recorded.
//
// The order is the ledger's insertion order and not the business date, because
// insertion order is the only order a moving average can be replayed in: two
// movements can share a date, and settling them in a different order gives a
// different average. See the note on the stock_movements table.
func (l *Ledger) Moves(itemID string) ([]StockMove, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, known := l.items[itemID]; !known {
		return nil, fmt.Errorf("%w: %s", ErrUnknownItem, itemID)
	}

	return append([]StockMove(nil), l.moves[itemID]...), nil
}

// BelowReorderPoint returns the items at or below the level they should be
// reordered at, by SKU.
//
// An item with no reorder point set is never reported: zero means nobody has
// said what the level is, which is not the same as a level of zero.
func (l *Ledger) BelowReorderPoint() []Item {
	l.mu.Lock()
	defer l.mu.Unlock()

	var low []Item

	for id, item := range l.items {
		if item.ReorderPoint <= 0 {
			continue
		}

		level := 0
		for _, move := range l.moves[id] {
			switch move.Direction {
			case DirectionIn:
				level += move.Qty
			case DirectionOut:
				level -= move.Qty
			}
		}

		if level <= item.ReorderPoint {
			low = append(low, item)
		}
	}

	sort.Slice(low, func(i, j int) bool {
		if low[i].SKU != low[j].SKU {
			return low[i].SKU < low[j].SKU
		}

		return low[i].ID < low[j].ID
	})

	return low
}

// valuationLocked returns an item's valuation, named and in the ledger's
// currency even before anything has moved.
func (l *Ledger) valuationLocked(itemID string) Valuation {
	valuation, known := l.valuations[itemID]
	if !known {
		return Valuation{ItemID: itemID, Currency: l.currency}
	}

	valuation.ItemID = itemID
	valuation.Currency = l.currency

	return valuation
}
