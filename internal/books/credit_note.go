package books

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/domain/gl"
	"fingo/internal/ledger"
	"fingo/internal/money"
)

// CreditNoteCommand returns part or all of an invoice.
type CreditNoteCommand struct {
	InvoiceID string
	Date      time.Time
	Reason    string
	Actor     string
	// Quantities maps a sale line number to the quantity returned.
	Quantities map[int]int
}

// ErrNothingToReturn is returned when no quantity was asked for.
var ErrNothingToReturn = errors.New("books: chưa chọn số lượng hàng trả lại")

// saleLine is what an invoice line sold, read back for a return.
type saleLine struct {
	LineNo     int
	ItemID     string
	Quantity   int
	UnitPrice  int64
	RateID     string
	CostAtSale int64
}

// CreditNote posts a sales return against an invoice.
//
// Dr 5212 + Dr 33311 + Cr 131, and for stocked lines Dr 156 / Cr 632 at the cost the
// goods were issued at. The credit is applied to the invoice, so aging and 131 keep
// agreeing; a credit on an already-paid invoice leaves the customer with a balance.
func (b *Books) CreditNote(ctx context.Context, cmd CreditNoteCommand) (ledger.Posted, error) {
	if cmd.InvoiceID == "" {
		return ledger.Posted{}, errors.New("books: hóa đơn trả lại chưa được chọn")
	}
	if len(cmd.Quantities) == 0 {
		return ledger.Posted{}, ErrNothingToReturn
	}

	var invoiceTotal int64
	var partnerID string

	err := b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT total, COALESCE(partner_id::text, '')
			  FROM documents WHERE id = $1 AND kind = 'SALES_INVOICE'`, cmd.InvoiceID).
			Scan(&invoiceTotal, &partnerID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ledger.Posted{}, fmt.Errorf("books: không tìm thấy hóa đơn %s", cmd.InvoiceID)
	}
	if err != nil {
		return ledger.Posted{}, err
	}

	rates, err := salesVATRates(ctx, b.store)
	if err != nil {
		return ledger.Posted{}, err
	}

	sold, returned, err := b.saleLinesOf(ctx, cmd.InvoiceID)
	if err != nil {
		return ledger.Posted{}, err
	}

	byNo := map[int]saleLine{}
	for _, l := range sold {
		byNo[l.LineNo] = l
	}

	type returnedLine struct {
		line saleLine
		qty  int
	}

	var picked []returnedLine
	netTotal, vatTotal, costTotal := money.Zero, money.Zero, money.Zero

	for lineNo, qty := range cmd.Quantities {
		if qty <= 0 {
			continue
		}
		l, ok := byNo[lineNo]
		if !ok {
			return ledger.Posted{}, fmt.Errorf("books: dòng %d không có trên hóa đơn", lineNo)
		}
		if qty > l.Quantity-returned[lineNo] {
			return ledger.Posted{}, fmt.Errorf(
				"books: dòng %d chỉ còn %d có thể trả (đã bán %d, đã trả %d)",
				lineNo, l.Quantity-returned[lineNo], l.Quantity, returned[lineNo])
		}

		net := money.FromInt(l.UnitPrice).Mul(money.FromInt(int64(qty)))
		tax := money.Zero
		if l.RateID != "" {
			rate, err := rates.Get(l.RateID)
			if err != nil {
				return ledger.Posted{}, fmt.Errorf("books: thuế suất %s: %w", l.RateID, err)
			}
			tax = money.FromInt(rate.Tax(net.Int64()))
		}

		netTotal = netTotal.Add(net)
		vatTotal = vatTotal.Add(tax)
		costTotal = costTotal.Add(money.FromInt(l.CostAtSale).Mul(money.FromInt(int64(qty))))
		picked = append(picked, returnedLine{line: l, qty: qty})
	}

	if netTotal.IsZero() && vatTotal.IsZero() {
		return ledger.Posted{}, ErrNothingToReturn
	}

	total := netTotal.Add(vatTotal)
	sellReturn := []ledger.Line{
		{Account: "5213", Side: gl.SideDebit, Amount: netTotal},
	}
	if vatTotal.Int64() > 0 {
		sellReturn = append(sellReturn, ledger.Line{Account: "33311", Side: gl.SideDebit, Amount: vatTotal})
	}
	sellReturn = append(sellReturn, ledger.Line{Account: "131", Side: gl.SideCredit, Amount: total})
	if costTotal.Int64() > 0 {
		sellReturn = append(sellReturn,
			ledger.Line{Account: "156", Side: gl.SideDebit, Amount: costTotal},
			ledger.Line{Account: "632", Side: gl.SideCredit, Amount: costTotal})
	}

	day := time.Date(cmd.Date.Year(), cmd.Date.Month(), cmd.Date.Day(), 0, 0, 0, 0, time.UTC)
	creditID := newUUID()

	posted, err := ledger.Post(ctx, b.store, b.scope, ledger.Request{
		ID:          creditID,
		Description: "Hàng bán trả lại: " + defaultText(cmd.Reason, "khách trả lại"),
		Actor:       cmd.Actor,
		Date:        day,
		Type:        gl.VoucherTypeGeneral,
		Lines:       sellReturn,
	})
	if err != nil {
		return ledger.Posted{}, err
	}

	period, err := b.EnsureCurrentYear(ctx, day)
	if err != nil {
		return posted, err
	}

	if err := b.store.InClient(ctx, b.scope, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO documents
				(id, client_id, kind, partner_id, document_date, period_id, total, allocated, status, source_document_id)
			VALUES ($1, $2, 'CREDIT_NOTE', $3, $4, $5, $6, 0, 'SETTLED', $7)`,
			creditID, b.scope.ID(), nullUUID(partnerID), day, period.ID, total.Int64(), cmd.InvoiceID); err != nil {
			return err
		}

		order := 0
		addLine := func(account string, debit, credit int64, vatAmount int64) error {
			order++
			_, err := tx.Exec(ctx, `
				INSERT INTO document_lines
					(document_id, client_id, line_order, account_code, debit, credit, vat_amount, base_amount)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				creditID, b.scope.ID(), order, account, debit, credit, vatAmount, debit)
			return err
		}

		if err := addLine("5213", netTotal.Int64(), 0, 0); err != nil {
			return err
		}
		if vatTotal.Int64() > 0 {
			if err := addLine("33311", vatTotal.Int64(), 0, vatTotal.Int64()); err != nil {
				return err
			}
		}
		if err := addLine("131", 0, total.Int64(), 0); err != nil {
			return err
		}

		for _, p := range picked {
			if _, err := tx.Exec(ctx, `
				INSERT INTO sale_lines
					(client_id, document_id, line_no, item_id, quantity, unit_price, vat_rate_id, cost_at_sale)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				b.scope.ID(), creditID, p.line.LineNo, p.line.ItemID, p.qty,
				p.line.UnitPrice, p.line.RateID, p.line.CostAtSale); err != nil {
				return err
			}
		}

		for _, p := range picked {
			if p.line.ItemID == "" || p.line.CostAtSale == 0 {
				continue
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO stock_movements
					(client_id, item_code, warehouse_code, kind, quantity, cost, unit_cost, source_doc_id, moved_at)
				VALUES ($1, $2, 'MAIN', 'RECEIPT', $3, $4, $5, $6, $7)`,
				b.scope.ID(), p.line.ItemID, p.qty,
				int64(p.qty)*p.line.CostAtSale, p.line.CostAtSale, creditID, day); err != nil {
				return err
			}
		}

		// Apply the credit to the invoice: the receivable falls by what was returned.
		if _, err := tx.Exec(ctx, `
			INSERT INTO allocations (client_id, document_id, voucher_id, amount)
			VALUES ($1, $2, $3, $4)`,
			b.scope.ID(), cmd.InvoiceID, posted.ID, total.Int64()); err != nil {
			return err
		}

		_, err := tx.Exec(ctx, `
			UPDATE documents
			   SET allocated = LEAST(total, allocated + $2),
			       status = CASE WHEN total <= LEAST(total, allocated + $2) THEN 'SETTLED' ELSE 'PARTIAL' END
			 WHERE id = $1`, cmd.InvoiceID, total.Int64())
		return err
	}); err != nil {
		return posted, fmt.Errorf("credit note %s posted, but recording it failed: %w", posted.VoucherNo, err)
	}

	return posted, nil
}

// saleLinesOf reads an invoice's lines and how much of each was already returned.
func (b *Books) saleLinesOf(ctx context.Context, invoiceID string) ([]saleLine, map[int]int, error) {
	var (
		sold     []saleLine
		returned = map[int]int{}
	)

	err := b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT line_no, item_id, quantity, unit_price, COALESCE(vat_rate_id, ''), cost_at_sale
			  FROM sale_lines WHERE document_id = $1 ORDER BY line_no`, invoiceID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var l saleLine
			if err := rows.Scan(&l.LineNo, &l.ItemID, &l.Quantity, &l.UnitPrice, &l.RateID, &l.CostAtSale); err != nil {
				rows.Close()
				return err
			}
			sold = append(sold, l)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		rows, err = tx.Query(ctx, `
			SELECT sl.line_no, COALESCE(SUM(sl.quantity), 0)
			  FROM sale_lines sl
			  JOIN documents d ON d.id = sl.document_id
			 WHERE d.kind = 'CREDIT_NOTE' AND d.source_document_id = $1
			 GROUP BY sl.line_no`, invoiceID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var lineNo, qty int
			if err := rows.Scan(&lineNo, &qty); err != nil {
				return err
			}
			returned[lineNo] = qty
		}
		return rows.Err()
	})

	return sold, returned, err
}

// SaleLineView is one line of an invoice, for the return screen.
type SaleLineView struct {
	LineNo    int
	ItemID    string
	Quantity  int
	UnitPrice money.Dong
	RateID    string
	Returned  int
}

// SaleLines lists an invoice's lines and how much of each was already returned.
func (b *Books) SaleLines(ctx context.Context, invoiceID string) ([]SaleLineView, error) {
	sold, returned, err := b.saleLinesOf(ctx, invoiceID)
	if err != nil {
		return nil, err
	}

	out := make([]SaleLineView, 0, len(sold))
	for _, l := range sold {
		out = append(out, SaleLineView{
			LineNo: l.LineNo, ItemID: l.ItemID, Quantity: l.Quantity,
			UnitPrice: money.FromInt(l.UnitPrice), RateID: l.RateID, Returned: returned[l.LineNo],
		})
	}
	return out, nil
}
