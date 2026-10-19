package books

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/money"
)

// OpeningStockLine is one item's opening position.
type OpeningStockLine struct {
	SKU      string
	Name     string
	Unit     string
	Qty      int
	UnitCost money.Dong
}

// ErrOpeningStockExists is returned once the warehouse has any movement.
var ErrOpeningStockExists = errors.New("books: kho đã có phát sinh, không nhập lại số dư đầu kỳ")

// ImportOpeningStock seeds the warehouse from the opening 156 balance.
//
// The account already holds the value — opening balances posted it — so this does
// not post a voucher. It creates the items and the receipt movements, and it
// refuses unless the stock value it is about to create equals the 156 balance to
// the đồng. A quantity without a matching value would make the first sale's cost
// arbitrary, which is worse than refusing the import.
func (b *Books) ImportOpeningStock(ctx context.Context, date time.Time, lines []OpeningStockLine) (money.Dong, error) {
	if len(lines) == 0 {
		return money.Zero, errors.New("books: cần ít nhất một mặt hàng")
	}

	day := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	total := money.Zero

	for i := range lines {
		l := &lines[i]
		l.SKU = strings.TrimSpace(l.SKU)
		l.Name = strings.TrimSpace(l.Name)
		l.Unit = strings.TrimSpace(l.Unit)

		switch {
		case l.SKU == "":
			return money.Zero, fmt.Errorf("dòng %d: thiếu mã hàng", i+1)
		case l.Name == "":
			return money.Zero, fmt.Errorf("dòng %d: thiếu tên hàng", i+1)
		case l.Unit == "":
			return money.Zero, fmt.Errorf("dòng %d: thiếu đơn vị tính", i+1)
		case l.Qty <= 0:
			return money.Zero, fmt.Errorf("dòng %d: số lượng phải là số nguyên dương", i+1)
		case l.UnitCost <= money.Zero:
			return money.Zero, fmt.Errorf("dòng %d: đơn giá phải lớn hơn 0", i+1)
		}

		total = total.Add(l.UnitCost.Mul(money.FromInt(int64(l.Qty))))
	}

	err := b.store.InClient(ctx, b.scope, func(tx pgx.Tx) error {
		var movements int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM stock_movements`).Scan(&movements); err != nil {
			return err
		}
		if movements > 0 {
			return ErrOpeningStockExists
		}

		var balance156 int64
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(CASE WHEN side = 'DEBIT' THEN amount ELSE -amount END), 0)
			  FROM voucher_lines l
			  JOIN vouchers v ON v.id = l.voucher_id
			 WHERE v.posted_at IS NOT NULL
			   AND l.account_code = '156'
			   AND v.voucher_date <= $1`, day).Scan(&balance156); err != nil {
			return err
		}

		if money.FromInt(balance156) != total {
			return fmt.Errorf(
				"books: tổng giá trị hàng nhập %s không bằng số dư TK 156 %s (chênh %s)",
				total, money.FromInt(balance156), total.Sub(money.FromInt(balance156)))
		}

		for _, l := range lines {
			if _, err := tx.Exec(ctx, `
				INSERT INTO items (client_id, sku, name, unit)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (client_id, sku) DO UPDATE SET name = EXCLUDED.name, unit = EXCLUDED.unit`,
				b.scope.ID(), l.SKU, l.Name, l.Unit); err != nil {
				return err
			}

			var itemID string
			if err := tx.QueryRow(ctx,
				`SELECT id::text FROM items WHERE sku = $1`, l.SKU).Scan(&itemID); err != nil {
				return err
			}

			value := l.UnitCost.Mul(money.FromInt(int64(l.Qty)))

			if _, err := tx.Exec(ctx, `
				INSERT INTO stock_movements
					(client_id, item_code, warehouse_code, kind, quantity, cost, unit_cost,
					 source_doc_id, moved_at)
				VALUES ($1, $2, 'MAIN', 'RECEIPT', $3, $4, $5, $6, $7)`,
				b.scope.ID(), itemID, l.Qty, value.Int64(), l.UnitCost.Int64(),
				"OPENING-"+day.Format("2006-01-02"), day); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return money.Zero, err
	}

	return total, nil
}
