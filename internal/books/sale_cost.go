package books

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fingo/internal/inventory"
	"fingo/internal/ledger"
	"fingo/internal/posting"
	"fingo/internal/trading"
)

// ErrOutOfStock is returned before a sale is posted when the goods are not there.
var ErrOutOfStock = errors.New("books: not enough stock for the sale")

// SaleWithCost posts a sale and, for every line whose item code is a stocked
// item, issues the goods and posts the cost of sales.
//
// Stock is checked **before** the sale is posted: a sale that cannot be costed
// must not be admitted, or the ledger would carry revenue with no cost and the
// warehouse would go negative quietly. The two vouchers are posted separately —
// the sale, then its cost — because each is numbered in its own book; a failure
// between them leaves a sale whose cost can be posted by hand, which is visible,
// rather than an invisible mismatch.
func (b *Books) SaleWithCost(ctx context.Context, cmd trading.InvoiceCommand) (trading.Invoice, error) {
	stock, err := b.stockLedger(ctx)
	if err != nil {
		return trading.Invoice{}, err
	}

	items, err := b.ListItems(ctx)
	if err != nil {
		return trading.Invoice{}, err
	}
	bySKU := make(map[string]Item, len(items))
	for _, it := range items {
		bySKU[it.SKU] = it
	}

	rules, err := posting.Load(posting.TT133Defaults())
	if err != nil {
		return trading.Invoice{}, fmt.Errorf("books: the TT133 posting rules are contradictory: %w", err)
	}

	// Pre-flight: every stocked line must be covered by what is on hand.
	type issue struct {
		item Item
		qty  int
	}
	var issues []issue

	for _, line := range cmd.Lines {
		item, stocked := bySKU[strings.TrimSpace(line.ItemID)]
		if !stocked {
			continue // a service, or a code the warehouse does not track
		}

		valuation, err := stock.Valuation(item.ID)
		if err != nil {
			return trading.Invoice{}, fmt.Errorf("books: value %s: %w", item.SKU, err)
		}
		if valuation.Quantity < line.Quantity {
			return trading.Invoice{}, fmt.Errorf("%w: %s has %d on hand and the sale takes %d",
				ErrOutOfStock, item.SKU, valuation.Quantity, line.Quantity)
		}

		issues = append(issues, issue{item: item, qty: line.Quantity})
	}

	sales, err := b.Sales(ctx)
	if err != nil {
		return trading.Invoice{}, err
	}

	invoice, err := sales.Invoice(ctx, cmd)
	if err != nil {
		return trading.Invoice{}, err
	}

	if len(issues) == 0 {
		return invoice, nil
	}

	day := time.Date(cmd.Date.Year(), cmd.Date.Month(), cmd.Date.Day(), 0, 0, 0, 0, time.UTC)

	for _, is := range issues {
		move := inventory.StockMove{
			ID:          newUUID(),
			ItemID:      is.item.ID,
			Direction:   inventory.DirectionOut,
			Qty:         is.qty,
			Date:        day,
			SourceDocID: invoice.ID,
		}

		entry, err := stock.RecordIssue(rules, move)
		if err != nil {
			return invoice, fmt.Errorf("sale %s posted, but costing %s failed: %w",
				invoice.ID, is.item.SKU, err)
		}

		if entry.Voucher == nil {
			return invoice, fmt.Errorf("books: the cost entry for %s has no voucher", is.item.SKU)
		}

		if _, err := ledger.PostVoucher(ctx, b.store, b.scope, entry.Voucher, cmd.By); err != nil {
			return invoice, fmt.Errorf("sale %s posted, but the cost of %s did not: %w",
				invoice.ID, is.item.SKU, err)
		}

		if err := b.saveMove(ctx, move); err != nil {
			return invoice, fmt.Errorf("cost posted, but the movement for %s was not saved: %w", is.item.SKU, err)
		}
	}

	return invoice, nil
}
