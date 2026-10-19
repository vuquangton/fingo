package books

import (
	"context"
	"errors"
	"fmt"
	"time"

	"fingo/internal/audit"
	"fingo/internal/domain/gl"
	"fingo/internal/ledger"
	"fingo/internal/posting"
	"fingo/internal/store"
	"fingo/internal/trading"
	"fingo/internal/vat"

	"github.com/jackc/pgx/v5"
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

// HasVATRate reports whether the catalog holds a rate id. It is how the sales
// screen knows that a statutory rate is missing rather than merely unselected.
func (b *Books) HasVATRate(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM vat_rates WHERE id = $1 AND active)`, id).Scan(&exists)
	})
	return exists, err
}

// RecordVATAcknowledgement writes the operator's confirmation that the invoice was
// reviewed against the missing 8% rate. It is an audit fact, not a posting.
func (b *Books) RecordVATAcknowledgement(ctx context.Context, invoiceID string, actor string, at time.Time) error {
	return b.store.InClient(ctx, b.scope, func(tx pgx.Tx) error {
		return audit.Record(ctx, tx, audit.Event{
			ClientID: b.scope.ID(),
			Actor:    actor,
			Action:   audit.Action("VAT_RATE_ACK"),
			Entity:   "document",
			EntityID: invoiceID,
			After:    []byte(`{"missing_rate":"vat8","invoice_date":"` + at.Format("2006-01-02") + `"}`),
		})
	})
}

// UnresolvedVATWindow reports whether a flagged statutory window covers the date.
// While it does, a sale needs an explicit acknowledgement before it posts.
func (b *Books) UnresolvedVATWindow(ctx context.Context, date time.Time) (bool, error) {
	var covered bool
	err := b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT EXISTS (
			  SELECT 1 FROM vat_unresolved_windows
			   WHERE active AND $1::date BETWEEN from_date AND to_date)`, date).Scan(&covered)
	})
	return covered, err
}
