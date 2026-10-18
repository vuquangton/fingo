package books

import (
	"context"
	"errors"
	"fmt"
	"time"

	"fingo/internal/domain/gl"
	"fingo/internal/inventory"
	"fingo/internal/ledger"
	"fingo/internal/money"
	"fingo/internal/posting"
)

// StocktakeCommand records what a physical count found for one item.
type StocktakeCommand struct {
	ItemID   string
	Counted  int
	Date     time.Time
	Reason   string
	Actor    string
	SourceID string
}

// AdjustStock brings the ledger to what was counted.
//
// A shortage is an issue and is costed like one (Dr 632 / Cr 156) at the moving
// average. A surplus is a receipt valued at the current average, credited to
// other income (Dr 156 / Cr 711) — the goods exist, nobody paid for them, and
// calling that revenue would be a fiction.
//
// Both write a movement whose source is the count sheet, because stock that
// changes without paper is how a warehouse and a ledger come apart.
func (b *Books) AdjustStock(ctx context.Context, cmd StocktakeCommand) (ledger.Posted, error) {
	switch {
	case cmd.ItemID == "":
		return ledger.Posted{}, errors.New("books: a stocktake needs an item")
	case cmd.Counted < 0:
		return ledger.Posted{}, errors.New("books: a counted quantity cannot be negative")
	case cmd.Actor == "":
		return ledger.Posted{}, errors.New("books: a stocktake needs an operator")
	}

	source := cmd.SourceID
	if source == "" {
		source = "Kiểm kê " + cmd.Date.Format("02/01/2006")
	}

	stock, err := b.stockLedger(ctx)
	if err != nil {
		return ledger.Posted{}, err
	}

	valuation, err := stock.Valuation(cmd.ItemID)
	if err != nil {
		return ledger.Posted{}, err
	}

	diff := cmd.Counted - valuation.Quantity
	if diff == 0 {
		return ledger.Posted{}, fmt.Errorf("%w: %s already stands at %d", ErrNoAdjustment, cmd.ItemID, cmd.Counted)
	}

	day := time.Date(cmd.Date.Year(), cmd.Date.Month(), cmd.Date.Day(), 0, 0, 0, 0, time.UTC)

	rules, err := posting.Load(posting.TT133Defaults())
	if err != nil {
		return ledger.Posted{}, fmt.Errorf("books: the TT133 posting rules are contradictory: %w", err)
	}

	if diff < 0 {
		move := inventory.StockMove{
			ID:          newUUID(),
			ItemID:      cmd.ItemID,
			Direction:   inventory.DirectionOut,
			Qty:         -diff,
			Date:        day,
			SourceDocID: source,
		}

		entry, err := stock.RecordIssue(rules, move)
		if err != nil {
			return ledger.Posted{}, fmt.Errorf("books: cost the shortage: %w", err)
		}
		if entry.Voucher == nil {
			return ledger.Posted{}, errors.New("books: the shortage entry has no voucher")
		}

		posted, err := ledger.PostVoucher(ctx, b.store, b.scope, entry.Voucher, cmd.Actor)
		if err != nil {
			return ledger.Posted{}, err
		}
		if err := b.saveMove(ctx, move); err != nil {
			return posted, fmt.Errorf("adjustment posted, but the movement was not saved: %w", err)
		}
		return posted, nil
	}

	// A surplus: value it at the average the books already carry, credited to
	// other income rather than to revenue.
	unitCost := money.FromInt(valuation.AvgCost)
	if unitCost.IsZero() {
		return ledger.Posted{}, fmt.Errorf("%w: %s has no cost to value the surplus at",
			ErrNoAdjustment, cmd.ItemID)
	}

	move := inventory.StockMove{
		ID:          newUUID(),
		ItemID:      cmd.ItemID,
		Direction:   inventory.DirectionIn,
		Qty:         diff,
		UnitCost:    money.New(unitCost.Int64(), "VND"),
		Date:        day,
		SourceDocID: source,
	}

	if _, err := stock.Record(move); err != nil {
		return ledger.Posted{}, fmt.Errorf("books: record the surplus: %w", err)
	}

	value := unitCost.Mul(money.FromInt(int64(diff)))

	posted, err := ledger.Post(ctx, b.store, b.scope, ledger.Request{
		ID:          move.ID,
		Description: "Kiểm kê thừa: " + source,
		Actor:       cmd.Actor,
		Date:        day,
		Type:        gl.VoucherTypeGeneral,
		Lines: []ledger.Line{
			{Account: "156", Side: gl.SideDebit, Amount: value},
			{Account: "711", Side: gl.SideCredit, Amount: value},
		},
	})
	if err != nil {
		return ledger.Posted{}, err
	}

	if err := b.saveMove(ctx, move); err != nil {
		return posted, fmt.Errorf("adjustment posted, but the movement was not saved: %w", err)
	}

	return posted, nil
}

// ErrNoAdjustment is returned when a count matches the ledger, so there is
// nothing to post.
var ErrNoAdjustment = errors.New("books: the count matches the ledger")
