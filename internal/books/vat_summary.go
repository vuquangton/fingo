package books

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/money"
)

// VATSummary is the month-end VAT position, read from the ledger.
//
// Output VAT is the movement on 33311, input VAT the movement on 1331. It is a
// ledger view on purpose: it must agree with the trial balance, and a return that
// disagrees with the books is worse than no return.
type VATSummary struct {
	StartDate, EndDate time.Time
	OutputVAT          money.Dong
	InputVAT           money.Dong
	NetPayable         money.Dong
	Revenue            money.Dong
}

// VATSummary reads the VAT position of one period.
func (b *Books) VATSummary(ctx context.Context, periodID string) (VATSummary, error) {
	var out VATSummary

	err := b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			SELECT start_date, end_date FROM accounting_periods WHERE id = $1`, periodID).
			Scan(&out.StartDate, &out.EndDate); err != nil {
			return err
		}

		return tx.QueryRow(ctx, `
			SELECT
			  COALESCE(SUM(CASE WHEN l.account_code = '33311' THEN
			      CASE WHEN l.side = 'CREDIT' THEN l.amount ELSE -l.amount END END), 0),
			  COALESCE(SUM(CASE WHEN l.account_code = '1331' THEN
			      CASE WHEN l.side = 'DEBIT' THEN l.amount ELSE -l.amount END END), 0),
			  COALESCE(SUM(CASE WHEN l.account_code = '5111' AND l.side = 'CREDIT' THEN l.amount END), 0)
			  FROM voucher_lines l
			  JOIN vouchers v ON v.id = l.voucher_id
			 WHERE v.posted_at IS NOT NULL
			   AND v.voucher_date BETWEEN $1 AND $2`,
			out.StartDate, out.EndDate).Scan(&out.OutputVAT, &out.InputVAT, &out.Revenue)
	})
	if err != nil {
		return VATSummary{}, fmt.Errorf("books: VAT summary: %w", err)
	}

	out.NetPayable = out.OutputVAT.Sub(out.InputVAT)
	return out, nil
}
