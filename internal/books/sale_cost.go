package books

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fingo/internal/inventory"
	"fingo/internal/ledger"
	"fingo/internal/money"
	"fingo/internal/posting"
	"fingo/internal/trading"

	"github.com/jackc/pgx/v5"
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
		cost money.Dong
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

	// The sale's own lines are persisted for the credit note: the ledger knows the
	// amounts, not what was sold, and a return cannot be computed from amounts alone.
	costs := map[string]int64{}
	for _, is := range issues {
		costs[is.item.SKU] = is.cost.Int64()
	}
	if err := b.saveSaleLines(ctx, invoice.ID, cmd.Lines, costs); err != nil {
		return invoice, fmt.Errorf("sale %s posted, but its lines were not recorded: %w", invoice.ID, err)
	}

	if len(issues) == 0 {
		return invoice, nil
	}

	day := time.Date(cmd.Date.Year(), cmd.Date.Month(), cmd.Date.Day(), 0, 0, 0, 0, time.UTC)

	for idx := range issues {
		is := &issues[idx]
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

		is.cost = money.FromInt(entry.UnitCost)
		if err := b.saveMove(ctx, move); err != nil {
			return invoice, fmt.Errorf("cost posted, but the movement for %s was not saved: %w", is.item.SKU, err)
		}
	}

	return invoice, nil
}

// saveSaleLines records what each invoice line sold, and at what cost.
func (b *Books) saveSaleLines(ctx context.Context, invoiceID string, lines []trading.InvoiceLine, costs map[string]int64) error {
	return b.store.InClient(ctx, b.scope, func(tx pgx.Tx) error {
		var documentID string
		if err := tx.QueryRow(ctx, `
			SELECT id::text FROM documents
			 WHERE kind = 'SALES_INVOICE'
			 ORDER BY created_at DESC LIMIT 1`).Scan(&documentID); err != nil {
			return err
		}

		for i, line := range lines {
			itemID := ""
			cost := int64(0)
			if c, stocked := costs[strings.TrimSpace(line.ItemID)]; stocked {
				itemID = strings.TrimSpace(line.ItemID)
				cost = c
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO sale_lines
					(client_id, document_id, line_no, item_id, quantity, unit_price, vat_rate_id, cost_at_sale)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				ON CONFLICT (document_id, line_no) DO NOTHING`,
				b.scope.ID(), documentID, i+1, itemID, line.Quantity,
				line.UnitPrice.Amount(), line.VATRateID, cost); err != nil {
				return err
			}
		}
		return nil
	})
}
