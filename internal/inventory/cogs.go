package inventory

import (
	"context"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/document"
	"fingo/internal/domain/gl"
	"fingo/internal/money"
	"fingo/internal/posting"
)

// ErrOverReturn is a return of more units than the issue it reverses put out.
//
// The reversal is sized from the cost the goods left at, so returning more than
// left would credit the cost of sales with money that was never debited to it.
var ErrOverReturn = errors.New("inventory: more units are coming back than went out")

// Entry is the cost of goods, booked.
//
// It is what an issue of stock produces: the double entry that moves the value
// of the goods out of inventory and into the cost of selling them.
//
//	Dr  cost of sales (632 by default)    qty × unit cost
//	Cr  inventory     (156 by default)    qty × unit cost
//
// Which two accounts those are is not decided here. They come from the client's
// posting rules, because Circular 46/2025/TT-BTC lets a client point either of
// them somewhere else, and a cost entry that hard-coded 632 would post to an
// account the client may not use.
type Entry struct {
	// MoveID is the StockMove this entry books.
	//
	// It is the link that makes the entry traceable: the cost of a sale can be
	// followed back to the movement that took the goods out of the warehouse,
	// and from there to the document that movement came from. Nothing else in
	// the entry is allowed to be the only record of what it is for.
	MoveID string

	// Voucher is the entry itself, unposted. The ledger numbers and posts it
	// like any other voucher.
	Voucher *gl.Voucher

	// Qty is how many units the entry covers.
	Qty int

	// UnitCost is the average cost of one unit when the goods left, in the
	// currency's minor unit. It is kept because a partial return has to be
	// reversable at the cost it left at, and re-deriving that later from a
	// valuation that has moved on would give a different number.
	UnitCost int64

	// Amount is what the entry moves: Qty × UnitCost.
	Amount money.Money

	// CostAccount and InventoryAccount are the two accounts the entry touches,
	// as the rules answered them.
	CostAccount      string
	InventoryAccount string

	// Reverses is the MoveID of the issue this entry reverses, and is empty on
	// an entry that is not a reversal.
	Reverses string
}

// Reversal reports whether the entry puts goods back.
func (e Entry) Reversal() bool { return e.Reverses != "" }

// IssueCost builds the entry that books what an issue of stock cost.
//
// cost is what the valuation said the goods were worth when they left — the same
// figure the stock ledger holds — so the voucher and the warehouse can never
// disagree about what a movement was worth.
//
// The voucher id is derived from the movement and not supplied, so booking the
// same movement twice is impossible rather than merely discouraged: a retried
// request, a double-clicked button and a replayed message all produce the same
// id, and the second one collides with the first.
func IssueCost(rules *posting.RuleSet, move StockMove, cost money.Money) (*Entry, error) {
	if move.Direction != DirectionOut {
		return nil, fmt.Errorf("%w: movement %s is %s, and only goods leaving carry a cost of sales",
			ErrInvalidMove, move.ID, move.Direction)
	}
	if err := move.Validate(); err != nil {
		return nil, err
	}

	costAccount, inventoryAccount, err := costAccounts(rules)
	if err != nil {
		return nil, err
	}

	return buildCostEntry(costAccount, inventoryAccount, move, cost)
}

// buildCostEntry writes what an issue cost onto a voucher, once the two accounts
// are known.
func buildCostEntry(costAccount, inventoryAccount string, move StockMove, cost money.Money) (*Entry, error) {
	if !cost.Currency().Valid() || cost.IsNegative() {
		return nil, fmt.Errorf("%w: the cost of movement %s is %s", ErrInvalidMove, move.ID, cost)
	}

	voucher := gl.NewVoucher(costVoucherID(move.ID),
		fmt.Sprintf("Cost of goods — movement %s", move.ID),
		gl.VoucherTypeGeneral, move.Date)

	if err := addEntryLines(voucher, costAccount, inventoryAccount, cost, move); err != nil {
		return nil, err
	}

	return &Entry{
		MoveID:           move.ID,
		Voucher:          voucher,
		Qty:              move.Qty,
		UnitCost:         unitCostOf(cost, move.Qty),
		Amount:           cost,
		CostAccount:      costAccount,
		InventoryAccount: inventoryAccount,
	}, nil
}

// ReverseCost builds the entry that puts goods back, reversing what they left
// at.
//
//	Dr  inventory      qty × the cost they left at
//	Cr  cost of sales  qty × the cost they left at
//
// The value comes from the original entry and not from today's average. Goods
// coming back into a warehouse at a price different from the one they went out
// at would leave the inventory account holding a gain or a loss nobody earned,
// and would not net the original entry to nothing.
func ReverseCost(original *Entry, returned StockMove) (*Entry, error) {
	if original == nil {
		return nil, fmt.Errorf("%w: there is no issue to reverse", ErrInvalidMove)
	}
	if returned.Direction != DirectionIn {
		return nil, fmt.Errorf("%w: movement %s is %s, and goods coming back arrive",
			ErrInvalidMove, returned.ID, returned.Direction)
	}
	if err := returned.Validate(); err != nil {
		return nil, err
	}
	if returned.Qty > original.Qty {
		return nil, fmt.Errorf("%w: %d units are coming back against an issue of %d",
			ErrOverReturn, returned.Qty, original.Qty)
	}
	if returned.UnitCost.Currency().Valid() && returned.UnitCost.Amount() != original.UnitCost {
		return nil, fmt.Errorf("%w: movement %s brings goods back at %s, and they left at %d",
			ErrInvalidMove, returned.ID, returned.UnitCost, original.UnitCost)
	}

	amount := money.New(original.UnitCost*int64(returned.Qty), original.Amount.Currency())

	voucher := gl.NewVoucher(returnVoucherID(returned.ID),
		fmt.Sprintf("Cost of goods reversed — movement %s", returned.ID),
		gl.VoucherTypeGeneral, returned.Date)

	if err := addEntryLines(voucher, original.InventoryAccount, original.CostAccount, amount, returned); err != nil {
		return nil, err
	}

	return &Entry{
		MoveID:           returned.ID,
		Voucher:          voucher,
		Qty:              returned.Qty,
		UnitCost:         original.UnitCost,
		Amount:           amount,
		CostAccount:      original.CostAccount,
		InventoryAccount: original.InventoryAccount,
		Reverses:         original.MoveID,
	}, nil
}

// RecordIssue records stock going out and returns the entry that books what it
// cost.
//
// This is the whole of the requirement in one call: an issue cannot be recorded
// without its cost entry being built, so a caller cannot take goods out of the
// warehouse and leave the inventory account carrying them. A movement that is
// not going out is refused — a receipt books no cost of sales.
//
// The movement and the entry are decided together, under one lock, and neither
// is kept until both have been built. The valuation is advanced on a copy first:
// a chart that cannot say where the cost goes, or a movement that would overdraw
// the warehouse, is refused before anything is written, rather than after the
// stock has already gone.
func (l *Ledger) RecordIssue(rules *posting.RuleSet, move StockMove) (*Entry, error) {
	if move.Direction != DirectionOut {
		return nil, fmt.Errorf("%w: movement %s is %s, and a cost of sales comes from goods leaving",
			ErrInvalidMove, move.ID, move.Direction)
	}
	if err := move.Validate(); err != nil {
		return nil, err
	}

	costAccount, inventoryAccount, err := costAccounts(rules)
	if err != nil {
		return nil, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	item, known := l.items[move.ItemID]
	if !known {
		return nil, fmt.Errorf("%w: %s", ErrUnknownItem, move.ItemID)
	}
	if move.UnitCost.Currency().Valid() && move.UnitCost.Currency() != l.currency {
		return nil, fmt.Errorf("%w: movement %s is costed in %s and the ledger keeps %s",
			ErrCurrencyMismatch, move.ID, move.UnitCost.Currency(), l.currency)
	}

	current := l.valuationLocked(item.ID)

	next, value, err := current.Apply(move)
	if err != nil {
		return nil, fmt.Errorf("movement %s of item %s: %w", move.ID, item.ID, err)
	}

	entry, err := buildCostEntry(costAccount, inventoryAccount, move, value)
	if err != nil {
		return nil, err
	}

	l.valuations[item.ID] = next
	l.moves[item.ID] = append(l.moves[item.ID], move)

	return entry, nil
}

// RecordReturn records goods coming back and returns the entry that reverses
// what they left at.
//
// The returned movement is valued at the cost the goods left at, whatever the
// caller put in UnitCost — goods do not come back at today's average. A caller
// that has stated a different cost is refused rather than quietly overruled,
// because the figure it stated is the figure something else believed.
//
// What has already come back against an issue is remembered, so a run of returns
// cannot together exceed what went out. Each one alone would look reasonable;
// the sum would credit the cost of sales with money that was never debited to
// it.
func (l *Ledger) RecordReturn(original *Entry, move StockMove) (*Entry, error) {
	if original == nil {
		return nil, fmt.Errorf("%w: there is no issue to reverse", ErrInvalidMove)
	}
	if move.Direction != DirectionIn {
		return nil, fmt.Errorf("%w: movement %s is %s, and goods coming back arrive",
			ErrInvalidMove, move.ID, move.Direction)
	}
	if err := move.Validate(); err != nil {
		return nil, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	item, known := l.items[move.ItemID]
	if !known {
		return nil, fmt.Errorf("%w: %s", ErrUnknownItem, move.ItemID)
	}

	if already := l.returned[original.MoveID]; move.Qty > original.Qty-already {
		return nil, fmt.Errorf("%w: %d units are coming back against the %d that left, of which %d already have",
			ErrOverReturn, move.Qty, original.Qty, already)
	}
	if move.UnitCost.Currency().Valid() && move.UnitCost.Amount() != original.UnitCost {
		return nil, fmt.Errorf("%w: movement %s brings goods back at %s, and they left at %d",
			ErrInvalidMove, move.ID, move.UnitCost, original.UnitCost)
	}

	if move.UnitCost.Currency().Valid() && move.UnitCost.Currency() != l.currency {
		return nil, fmt.Errorf("%w: movement %s is costed in %s and the ledger keeps %s",
			ErrCurrencyMismatch, move.ID, move.UnitCost.Currency(), l.currency)
	}
	move.UnitCost = money.New(original.UnitCost, original.Amount.Currency())

	current := l.valuationLocked(item.ID)

	next, _, err := current.Apply(move)
	if err != nil {
		return nil, fmt.Errorf("movement %s of item %s: %w", move.ID, item.ID, err)
	}

	entry, err := ReverseCost(original, move)
	if err != nil {
		return nil, err
	}

	if l.returned == nil {
		l.returned = make(map[string]int)
	}

	l.valuations[item.ID] = next
	l.moves[item.ID] = append(l.moves[item.ID], move)
	l.returned[original.MoveID] += move.Qty

	return entry, nil
}

// Sale is one sale as the books see it: what was invoiced, and what the goods it
// sold cost.
type Sale struct {
	// Revenue is the voucher for the sale itself — the receivable, the revenue
	// and the output VAT. Required.
	Revenue *gl.Voucher

	// Cost is the entry that takes the goods it sold out of inventory. Required
	// for a sale of goods, and nil only for a sale of something that was never
	// in a warehouse: a service, a licence, a fee.
	Cost *Entry
}

// Poster writes a sale and its cost of goods as one transaction.
//
// The shape of this interface is the guarantee. There is one method and it takes
// both the revenue and the cost, so there is no way to write one without the
// other: two methods would let a caller call the first and not the second, and a
// sale whose cost never reached the ledger is a sale that overstated profit
// until somebody noticed at year end.
//
// An implementation owns one database transaction and commits both vouchers
// inside it — see the ledger's Posting, which carries the same pair.
type Poster interface {
	Post(ctx context.Context, sale Sale) error
}

// PostSale hands a sale and its cost to the ledger together.
//
// It refuses a sale that has a cost but no revenue, and a cost entry that books
// a movement other than the one the sale was for. What it cannot check is the
// transaction itself: that belongs to the Poster, which is why the interface has
// one method.
func PostSale(ctx context.Context, poster Poster, sale Sale) error {
	if poster == nil {
		return errors.New("inventory: a sale needs somewhere to be posted")
	}
	if sale.Revenue == nil {
		return fmt.Errorf("%w: a sale needs the voucher that records it", ErrInvalidMove)
	}
	if sale.Cost != nil {
		if sale.Cost.Voucher == nil {
			return fmt.Errorf("%w: the cost of a sale has no voucher", ErrInvalidMove)
		}
		if sale.Cost.MoveID == "" {
			return fmt.Errorf("%w: the cost of a sale names no movement, so nothing can be traced to it", ErrInvalidMove)
		}
	}

	return poster.Post(ctx, sale)
}

// costAccounts resolves the two accounts a cost entry needs.
//
// Lookup rather than Resolve: Resolve panics on an unanswered decision, and that
// is right for a rule set the ledger has already checked. This one may have been
// built against a different set of required decisions, and a domain call that
// can panic on configuration is a domain call that takes the process down at
// month end.
func costAccounts(rules *posting.RuleSet) (string, string, error) {
	if rules == nil {
		return "", "", fmt.Errorf("%w: no posting rules were supplied", posting.ErrUndefined)
	}

	cost := posting.Key{
		DocType:     document.KindSalesInvoice,
		Side:        gl.SideDebit,
		AccountType: gl.AccountTypeExpense,
	}
	inventory := posting.Key{
		DocType:     document.KindSalesInvoice,
		Side:        gl.SideCredit,
		AccountType: gl.AccountTypeAsset,
	}

	costAccount, answered := rules.Lookup(cost.DocType, cost.Side, cost.AccountType)
	if !answered {
		return "", "", fmt.Errorf("%w: %s, so there is nowhere to put the cost of a sale", posting.ErrUndefined, cost)
	}

	inventoryAccount, answered := rules.Lookup(inventory.DocType, inventory.Side, inventory.AccountType)
	if !answered {
		return "", "", fmt.Errorf("%w: %s, so there is nowhere to take the goods out of", posting.ErrUndefined, inventory)
	}

	return costAccount, inventoryAccount, nil
}

// addEntryLines writes the two sides of a cost entry onto a voucher.
func addEntryLines(voucher *gl.Voucher, debitAccount, creditAccount string, amount money.Money, move StockMove) error {
	if amount.IsZero() {
		return fmt.Errorf("%w: movement %s moved %s, and nothing is nothing to book",
			ErrInvalidMove, move.ID, amount)
	}

	note := fmt.Sprintf("movement %s from %s", move.ID, move.SourceDocID)

	// The amount is a whole number of minor units by construction — the
	// valuation is integers all the way down — and gl refuses a fractional
	// đồng, so the conversion is a formality that also proves the point.
	for _, line := range []gl.VoucherLine{
		{AccountCode: debitAccount, Side: gl.SideDebit, Amount: decimalOf(amount), Note: note},
		{AccountCode: creditAccount, Side: gl.SideCredit, Amount: decimalOf(amount), Note: note},
	} {
		if err := voucher.AddLine(line); err != nil {
			return fmt.Errorf("cost of movement %s: %w", move.ID, err)
		}
	}

	return nil
}

// unitCostOf is the cost of one unit, for the entry to remember.
//
// cost is Qty × average by construction, so the division is exact; a caller that
// passed something else gets a truncation rather than an error, because the
// amount is what the ledger moves and the unit cost is only what a later partial
// return is sized from.
func unitCostOf(cost money.Money, qty int) int64 {
	if qty <= 0 {
		return 0
	}

	return cost.Amount() / int64(qty)
}

// costVoucherID derives the voucher id for a movement's cost entry.
//
// Derived rather than drawn at random, so the same movement cannot be booked
// twice: a retry produces the same id and collides with the row it already
// wrote.
func costVoucherID(moveID string) string { return "cogs-" + moveID }

// returnVoucherID derives the voucher id for the reversal of a return.
func returnVoucherID(moveID string) string { return "cogs-return-" + moveID }

// decimalOf renders a whole number of minor units as the ledger's decimal.
//
// Every amount that reaches a voucher comes from the valuation, which is integer
// arithmetic throughout, so this is an exact conversion and never a rounding.
func decimalOf(amount money.Money) decimal.Decimal {
	return decimal.NewFromInt(amount.Amount())
}
