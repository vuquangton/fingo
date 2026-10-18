// Package books composes the engine into what the screens read.
//
// practice.Client holds a client's ledger; internal/reports reads it as a balance
// sheet or a profit and loss. Neither knows about the other, and putting them
// together is not something either should learn: it is assembly, and assembly
// belongs where the application is assembled.
//
// Nothing here decides anything. Every method is a delegation, and the reason the
// package exists at all is that the reports service needs three ports and the
// ledger repository happens to satisfy all three — a fact worth stating once,
// in one place, rather than in every screen that wants a figure.
package books

import (
	"context"
	"fmt"
	"time"

	"fingo/internal/adapter/mariadb/conn"
	"fingo/internal/adapter/mariadb/ledgerrepo"
	"fingo/internal/close"
	"fingo/internal/domain/gl"
	"fingo/internal/domain/tax"
	"fingo/internal/period"
	"fingo/internal/reports"
	"fingo/internal/usecase/ledger"
	"fingo/internal/usecase/practice"
	"fingo/internal/usecase/sales"
)

// Books is one client's books: the ledger, and the reports over it.
type Books struct {
	client  *practice.Client
	reports *reports.Service

	// source is the same ledger repository the reports read, kept so the screens
	// can read the lines themselves. The drill-through needs the movements, not
	// a total somebody else summed.
	source *ledgerrepo.Source
}

// Open opens a client's books and the reports over them.
func Open(ctx context.Context, cfg conn.Config, database string) (*Books, error) {
	client, err := practice.Open(ctx, cfg, database)
	if err != nil {
		return nil, err
	}

	// One source, three ports.
	//
	// ledgerrepo.Source reads posted lines, lists the periods and supplies the
	// chart — which is exactly Journal, Periods and ChartSource. A reports
	// service built from it is reading the same rows the ledger wrote, rather
	// than a summary somebody else computed.
	source := ledgerrepo.New(client.DB(), client, client)

	service, err := reports.NewService(source, source, source, close.TT133ClosingAccounts())
	if err != nil {
		_ = client.Close()

		return nil, fmt.Errorf("build the reports over %s: %w", database, err)
	}

	return &Books{client: client, reports: service, source: source}, nil
}

// Close gives up the database.
func (b *Books) Close() error { return b.client.Close() }

// --- the ledger -----------------------------------------------------------------

// Chart is the client's chart of accounts.
func (b *Books) Chart(ctx context.Context) (*gl.ChartOfAccounts, error) {
	return b.client.Chart(ctx)
}

// Accounts lists the chart.
func (b *Books) Accounts(ctx context.Context) ([]gl.Account, error) {
	return b.client.Accounts(ctx)
}

// EnsureChart seeds the chart if the client has none.
func (b *Books) EnsureChart(ctx context.Context) error { return b.client.EnsureChart(ctx) }

// TrialBalance is the period's movement by account.
func (b *Books) TrialBalance(ctx context.Context) (practice.TrialBalance, error) {
	return b.client.TrialBalance(ctx)
}

// Voucher reads one posted voucher, which is what makes a figure on a report
// traceable to something an operator can look at.
func (b *Books) Voucher(ctx context.Context, id string) (*gl.Voucher, error) {
	return b.client.Voucher(ctx, id)
}

// VoucherNo is the number the ledger gave a voucher.
//
// A posted line carries a voucher's identity but not its number, so a screen that
// wants to cite one has to ask. Keeping the lookup here rather than in the
// screens means the screens deal in numbers and never in vouchers.
func (b *Books) VoucherNo(ctx context.Context, id string) (string, error) {
	v, err := b.client.Voucher(ctx, id)
	if err != nil {
		return "", err
	}

	return v.VoucherNo(), nil
}

// Lines reads every posted line between two days, both inclusive.
//
// This is the ledger itself rather than a summary of it, which is what the
// drill-through needs: a figure on a report should be openable down to the
// documents that produced it, and a total cannot be opened.
func (b *Books) Lines(ctx context.Context, from, to time.Time) ([]reports.Line, error) {
	return b.source.Lines(ctx, from, to)
}

// Periods lists the accounting periods.
func (b *Books) Periods(ctx context.Context) ([]period.Period, error) {
	return b.client.Periods(ctx)
}

// CreatePeriod opens an accounting period.
//
// Nothing seeds one, and without one every report refuses: a balance sheet is a
// position at the end of a period, and there is no such thing as a position at
// the end of nothing. This is how a first run gets its books open.
func (b *Books) CreatePeriod(ctx context.Context, p period.Period) error {
	return b.client.CreatePeriod(ctx, p)
}

// Ledger is the posting service, for the entry form.
func (b *Books) Ledger() *ledger.Service { return b.client.Ledger() }

// RecordSale records a sale.
func (b *Books) RecordSale(ctx context.Context, cmd sales.Command) (sales.Result, error) {
	return b.client.RecordSale(ctx, cmd)
}

// VATRates is the VAT schedule the money on a document is resolved against.
func (b *Books) VATRates() *tax.Schedule { return b.client.VATRates() }

// --- the reports ----------------------------------------------------------------

// BalanceSheet is the client's position at the end of a period.
func (b *Books) BalanceSheet(ctx context.Context, periodID string) (reports.BalanceSheet, error) {
	return b.reports.BalanceSheet(ctx, periodID)
}

// ProfitAndLoss is a period's result.
func (b *Books) ProfitAndLoss(ctx context.Context, periodID string) (reports.ProfitAndLoss, error) {
	return b.reports.ProfitAndLoss(ctx, periodID)
}
