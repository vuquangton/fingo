package books

import (
	"context"
	"fmt"
	"time"

	"fingo/internal/domain/gl"
	"fingo/internal/ledger"
	"fingo/internal/posting"
	"fingo/internal/trading"
	"fingo/internal/vat"
)

// Sales builds the sales flow over these books.
//
// The flow itself is internal/trading: it prices the lines, decides the VAT, and
// posts Dr 131 / Cr 5111 / Cr 33311. This function only supplies the four things
// it needs, because none of them is a decision trading may make for the client.
func (b *Books) Sales(ctx context.Context) (*trading.Service, error) {
	rates, err := salesVATRates(ctx)
	if err != nil {
		return nil, err
	}

	rules, err := posting.Load(posting.TT133Defaults())
	if err != nil {
		return nil, fmt.Errorf("books: the TT133 posting rules are contradictory: %w", err)
	}

	return trading.NewService(b.Documents(), poster{b: b}, rates, rules)
}

// salesVATRates is the rate table the operator may choose from.
//
// The rates are the statutory ones and are kept as an in-memory registry because
// a client may add a rate at a date without a schema change. The 8% window is
// recorded with its end date rather than being looked up at entry time.
func salesVATRates(_ context.Context) (*vat.Registry, error) {
	registry := vat.New()

	// The registry holds the rate in force on a date, not a menu: two active rates
	// may not cover the same day. Vietnamese VAT has several rates at once (0, 5,
	// 8, 10 by goods), and modelling that needs a rate dimension the engine does
	// not have yet. The standard rate is registered here; the others are recorded
	// as an open question rather than faked.
	// A catalog, not a single rate: the line names the rate it was taxed under.
	// 0% and 5% are statutory categories for particular goods; 10% is standard.
	// The 8% reduced window is deliberately absent until the practitioner confirms
	// its dates — a wrong date would tax a real invoice at the wrong rate, and an
	// absent rate refuses the entry instead (see docs/v2/review/logic/00-verdict.md).
	rates := []vat.VATRate{
		{ID: "vat0", Rate: 0, Active: true, EffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "vat5", Rate: 500, Active: true, EffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "vat10", Rate: 1000, Active: true, EffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	}

	for _, r := range rates {
		if err := registry.Create(r); err != nil {
			return nil, fmt.Errorf("books: register VAT rate %s: %w", r.ID, err)
		}
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
