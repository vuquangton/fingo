package books

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/document"
	"fingo/internal/domain/gl"
	"fingo/internal/inventory"
	"fingo/internal/ledger"
	"fingo/internal/money"
)

// Item is something the business keeps in stock.
type Item struct {
	ID           string
	SKU          string
	Name         string
	Unit         string
	ReorderPoint int
	OnHand       int
	Value        money.Dong
	UnitCost     money.Dong
}

// CreateItem stocks a new item.
func (b *Books) CreateItem(ctx context.Context, item Item) (Item, error) {
	item.SKU = strings.TrimSpace(item.SKU)
	item.Name = strings.TrimSpace(item.Name)
	item.Unit = strings.TrimSpace(item.Unit)

	switch {
	case item.SKU == "":
		return Item{}, errors.New("books: an item needs a SKU")
	case item.Name == "":
		return Item{}, errors.New("books: an item needs a name")
	case item.Unit == "":
		return Item{}, errors.New("books: an item needs a unit")
	case item.ReorderPoint < 0:
		return Item{}, errors.New("books: a reorder point cannot be negative")
	}

	if item.ID == "" {
		item.ID = newUUID()
	}

	err := b.store.InClient(ctx, b.scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO items (id, client_id, sku, name, unit, reorder_point)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			item.ID, b.scope.ID(), item.SKU, item.Name, item.Unit, item.ReorderPoint)
		return err
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Item{}, fmt.Errorf("books: an item with SKU %q already exists", item.SKU)
		}
		return Item{}, err
	}

	return item, nil
}

// ListItems returns the items with what is on hand and what it is worth.
func (b *Books) ListItems(ctx context.Context) ([]Item, error) {
	var out []Item

	err := b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, sku, name, unit, reorder_point FROM items WHERE active ORDER BY sku`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var it Item
			if err := rows.Scan(&it.ID, &it.SKU, &it.Name, &it.Unit, &it.ReorderPoint); err != nil {
				return err
			}
			out = append(out, it)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}

	book, err := b.stockLedger(ctx)
	if err != nil {
		return nil, err
	}

	for i := range out {
		valuation, err := book.Valuation(out[i].ID)
		if err != nil {
			continue
		}
		out[i].OnHand = valuation.Quantity
		out[i].Value = money.FromInt(valuation.Value().Amount())
		out[i].UnitCost = money.FromInt(valuation.AvgCost)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].SKU < out[j].SKU })
	return out, nil
}

// ReceiptCommand books goods into stock from one supplier document.
type StockReceiptCommand struct {
	ItemID      string
	PartnerID   string
	Qty         int
	UnitCost    money.Dong
	Date        time.Time
	SourceDocID string
	Actor       string
}

// ReceiveStock records a receipt and posts Dr 156 / Cr 331 for what it cost.
func (b *Books) ReceiveStock(ctx context.Context, cmd StockReceiptCommand) (ledger.Posted, error) {
	if cmd.ItemID == "" {
		return ledger.Posted{}, errors.New("books: a receipt needs an item")
	}
	if cmd.Qty <= 0 {
		return ledger.Posted{}, errors.New("books: a receipt moves at least one unit")
	}
	if cmd.UnitCost <= money.Zero {
		return ledger.Posted{}, errors.New("books: a receipt must say what a unit cost")
	}
	if cmd.SourceDocID == "" {
		return ledger.Posted{}, errors.New("books: a stock movement must name the document it came from")
	}

	book, err := b.stockLedger(ctx)
	if err != nil {
		return ledger.Posted{}, err
	}

	day := time.Date(cmd.Date.Year(), cmd.Date.Month(), cmd.Date.Day(), 0, 0, 0, 0, time.UTC)
	value := cmd.UnitCost.Mul(money.FromInt(int64(cmd.Qty)))

	// The movement is recorded in the engine first: it refuses a negative stock
	// and decides the valuation. If the voucher then fails, the movement is not
	// persisted and the books are unchanged.
	move := inventory.StockMove{
		ID:          newUUID(),
		ItemID:      cmd.ItemID,
		Direction:   inventory.DirectionIn,
		Qty:         cmd.Qty,
		UnitCost:    money.New(cmd.UnitCost.Int64(), "VND"),
		Date:        day,
		SourceDocID: cmd.SourceDocID,
	}

	if _, err := book.Record(move); err != nil {
		return ledger.Posted{}, fmt.Errorf("books: record the receipt: %w", err)
	}

	posted, err := ledger.Post(ctx, b.store, b.scope, ledger.Request{
		ID:          move.ID,
		Description: "Nhập kho",
		Actor:       cmd.Actor,
		Date:        day,
		Type:        gl.VoucherTypePurchase,
		Lines: []ledger.Line{
			{Account: "156", Side: gl.SideDebit, Amount: value},
			{Account: "331", Side: gl.SideCredit, Amount: value},
		},
	})
	if err != nil {
		return ledger.Posted{}, err
	}

	if err := b.saveMove(ctx, move); err != nil {
		return posted, fmt.Errorf("stock receipt %s posted, but the movement was not saved: %w", posted.VoucherNo, err)
	}

	// C1: the 331 credit is a payable, so the AP subledger must know about it.
	// Without this document the control account and the aging report disagree by
	// the whole receipt, and every later reconciliation has to explain it.
	period, err := b.EnsureCurrentYear(ctx, day)
	if err != nil {
		return posted, err
	}

	purchase := document.Document{
		ID:        newUUID(),
		Type:      document.TypeInvoice,
		PartnerID: cmd.PartnerID,
		Date:      day,
		PeriodID:  period.ID,
		Total:     money.New(value.Int64(), "VND"),
		Allocated: money.New(0, "VND"),
		Status:    document.StatusOpen,
	}

	purchaseLines := []document.Line{
		{DocumentID: purchase.ID, AccountID: "156", Debit: money.New(value.Int64(), "VND"), BaseAmount: money.New(value.Int64(), "VND")},
		{DocumentID: purchase.ID, AccountID: "331", Credit: money.New(value.Int64(), "VND")},
	}

	if err := b.Documents().CreateAs(ctx, "PURCHASE_BILL", purchase, purchaseLines); err != nil {
		return posted, fmt.Errorf("stock receipt %s posted, but recording the payable failed: %w", posted.VoucherNo, err)
	}

	return posted, nil
}

// stockLedger replays the client's movements into the inventory engine.
//
// The engine is in memory and derives every valuation from the movement history;
// replaying it from the stored log means the on-hand figure is never a number
// somebody wrote down and could not defend.
func (b *Books) stockLedger(ctx context.Context) (*inventory.Ledger, error) {
	book, err := inventory.NewLedger("VND")
	if err != nil {
		return nil, err
	}

	err = b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
		items, err := tx.Query(ctx, `
			SELECT id, sku, name, unit, reorder_point FROM items WHERE active ORDER BY sku`)
		if err != nil {
			return err
		}
		for items.Next() {
			var item inventory.Item
			if err := items.Scan(&item.ID, &item.SKU, &item.Name, &item.Unit, &item.ReorderPoint); err != nil {
				items.Close()
				return err
			}
			if err := book.AddItem(item); err != nil {
				items.Close()
				return err
			}
		}
		items.Close()
		if err := items.Err(); err != nil {
			return err
		}

		rows, err := tx.Query(ctx, `
			SELECT id, item_code, kind, quantity::bigint, unit_cost, moved_at, source_doc_id
			  FROM stock_movements
			 ORDER BY seq`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var (
				m        inventory.StockMove
				kind     string
				qty      int64
				unitCost int64
			)
			if err := rows.Scan(&m.ID, &m.ItemID, &kind, &qty, &unitCost, &m.Date, &m.SourceDocID); err != nil {
				return err
			}
			m.Direction = inventory.DirectionOut
			if kind == "RECEIPT" {
				m.Direction = inventory.DirectionIn
			}
			m.Qty = int(qty)
			m.UnitCost = money.New(unitCost, "VND")
			if _, err := book.Record(m); err != nil {
				return err
			}
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}

	return book, nil
}

// saveMove persists one movement.
func (b *Books) saveMove(ctx context.Context, m inventory.StockMove) error {
	kind := "ISSUE"
	if m.Direction == inventory.DirectionIn {
		kind = "RECEIPT"
	}

	return b.store.InClient(ctx, b.scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO stock_movements
				(client_id, item_code, warehouse_code, kind, quantity, cost, unit_cost,
				 source_doc_id, moved_at)
			VALUES ($1, $2, 'MAIN', $3, $4, $5, $6, $7, $8)`,
			b.scope.ID(), m.ItemID, kind, m.Qty,
			m.UnitCost.Amount()*int64(m.Qty), m.UnitCost.Amount(), m.SourceDocID, m.Date)
		return err
	})
}
