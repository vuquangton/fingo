package books

import (
	"context"
	"errors"
	"fmt"
	"time"

	"fingo/internal/domain/gl"
	"fingo/internal/ledger"
	"fingo/internal/posting"
	"fingo/internal/store"
	"fingo/internal/trading"
	"fingo/internal/vat"
)

// Sales builds the sales flow over these books.
//
// The flow itself is internal/trading: it prices the lines, decides the VAT, and
// posts Dr 131 / Cr 5111 / Cr 33311. This function only supplies the four things
// it needs, because none of them is a decision trading may make for the client.
func (b *Books) Sales(ctx context.Context) (*trading.Service, error) {
	rates, err := salesVATRates(ctx, b.store)
	if err != nil {
		return nil, err
	}

	rules, err := posting.Load(posting.TT133Defaults())
	if err != nil {
		return nil, fmt.Errorf("books: the TT133 posting rules are contradictory: %w", err)
	}

	return trading.NewService(b.Documents(), poster{b: b}, rates, rules)
}

// salesVATRates loads the rate catalog from the database.
//
// The rates are data, not code: adding the 8% window or retiring a rate is a row,
// not a deploy. A line names the row it was taxed under, so the tax on a stored
// line stays reproducible when the table gains rows later.
func salesVATRates(ctx context.Context, st *store.Store) (*vat.Registry, error) {
	rows, err := st.Pool().Query(ctx, `
		SELECT id, basis_points, effective_from, effective_to
		  FROM vat_rates
		 WHERE active
		 ORDER BY effective_from, id`)
	if err != nil {
		return nil, fmt.Errorf("books: read the VAT rates: %w", err)
	}
	defer rows.Close()

	registry := vat.New()

	for rows.Next() {
		var (
			rate vat.VATRate
			bp   int
			from time.Time
			to   *time.Time
		)
		if err := rows.Scan(&rate.ID, &bp, &from, &to); err != nil {
			return nil, err
		}
		rate.Rate = vat.BasisPoints(bp)
		rate.Active = true
		rate.EffectiveFrom = from
		rate.EffectiveTo = to

		if err := registry.Create(rate); err != nil {
			return nil, fmt.Errorf("books: register VAT rate %s: %w", rate.ID, err)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(registry.List()) == 0 {
		return nil, errors.New("books: the VAT rate table is empty")
	}

	return registry, nil
}

// poster adapts trading's posting to the ledger.
type poster struct {
	b *Books
}

// Post sends one voucher through the ordinary posting path.
func (p poster) Post(ctx context.Context, posting trading.Posting) error {
	if posting.Voucher == nil {
		return fmt.Errorf("books: a trade posting has no voucher")
	}
	_, err := ledger.PostVoucher(ctx, p.b.store, p.b.scope, posting.Voucher, posting.By)
	return err
}

var _ = gl.VoucherTypeGeneral
