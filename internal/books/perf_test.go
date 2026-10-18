package books_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/books"
	"fingo/internal/store"
	"fingo/internal/testdb"
)

// seedLines inserts a year of vouchers in two statements.
//
// COPY FROM is refused under row-level security, so the rows are generated in
// the database and inserted with INSERT ... SELECT. That keeps the tenant policy
// in force — which is the point of measuring with it on — and is still fast
// enough that the seeding does not dominate the test.
func seedLines(ctx context.Context, t *testing.T, st *store.Store, scope store.Scope, vouchers int) {
	t.Helper()

	err := st.InClient(ctx, scope, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq,
			                      voucher_date, description, posted_at, posted_by, created_by)
			SELECT gen_random_uuid(), $1, 'PT2026-' || lpad(i::text, 5, '0'), 'GENERAL', 2026, i,
			       DATE '2026-01-01' + ((i % 360) || ' days')::interval,
			       'Bút toán ' || i, NULL, 'bench', 'bench'
			  FROM generate_series(1, $2) AS i`,
			scope.ID(), vouchers); err != nil {
			return fmt.Errorf("insert vouchers: %w", err)
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO voucher_lines (id, client_id, voucher_id, line_order, account_code, side, amount, note)
			SELECT gen_random_uuid(), v.client_id, v.id, s.ord, s.account, s.side, v.seq * 1000 + 100000, ''
			  FROM vouchers v
			 CROSS JOIN (VALUES (1, '1111', 'DEBIT'), (2, '5111', 'CREDIT')) AS s(ord, account, side)`,
		)
		if err != nil {
			return err
		}

		// Post them afterwards: the trigger refuses lines on a posted voucher, so
		// the draft-then-post order is the same one the ledger itself uses.
		_, err = tx.Exec(ctx,
			`UPDATE vouchers SET posted_at = now() WHERE client_id = $1 AND posted_at IS NULL`,
			scope.ID())
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// TestVolumeIsSurvivable seeds a year and measures the reports, because every
// figure is summed from the lines: a slow line table is a slow trial balance.
func TestVolumeIsSurvivable(t *testing.T) {
	if testing.Short() {
		t.Skip("volume test")
	}

	cfg := testdb.Load(t)
	testdb.Open(t, cfg)

	st, err := store.Open(t.Context(), store.Config{DSN: cfg.DSN(), MaxOpenConns: 8})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(st.Close)

	clientID := newUUID(t)
	if _, err := st.Pool().Exec(t.Context(),
		`INSERT INTO clients (id, name, tax_id) VALUES ($1, $2, $3)`,
		clientID, "Volume client", "VOL-"+clientID); err != nil {
		t.Fatalf("insert client: %v", err)
	}

	scope, err := store.NewScope(clientID)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}

	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(), `SELECT provision_client_chart($1)`, clientID)
		return err
	}); err != nil {
		t.Fatalf("chart: %v", err)
	}

	b := books.Open(st, scope)

	period, err := b.EnsureCurrentYear(t.Context(), time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("period: %v", err)
	}

	const vouchers = 25_000
	seedLines(t.Context(), t, st, scope, vouchers)

	start := time.Now()
	tb, err := b.TrialBalance(t.Context(), period.ID)
	if err != nil {
		t.Fatalf("trial balance: %v", err)
	}
	trialBalance := time.Since(start)

	start = time.Now()
	pl, err := b.ProfitAndLoss(t.Context(), period.ID)
	if err != nil {
		t.Fatalf("p&l: %v", err)
	}
	profitAndLoss := time.Since(start)

	start = time.Now()
	lines, err := b.Lines(t.Context(), time.Time{}, period.EndDate)
	if err != nil {
		t.Fatalf("lines: %v", err)
	}
	journal := time.Since(start)

	t.Logf("seed: %d vouchers, %d lines", vouchers, len(lines))
	t.Logf("trial balance: %s (%d rows, %d vouchers)", trialBalance, len(tb.Rows), tb.SourceVoucherCount)
	t.Logf("profit and loss: %s", profitAndLoss)
	t.Logf("journal read: %s", journal)

	if tb.SourceVoucherCount != vouchers {
		t.Fatalf("trial balance counted %d vouchers, want %d", tb.SourceVoucherCount, vouchers)
	}
	if pl.NetProfit <= 0 {
		t.Fatalf("P&L net profit = %d, want a profit", pl.NetProfit)
	}

	// A ceiling, not a target: it exists to catch a pathology (a per-row query,
	// a missing index), not to grade the machine.
	if trialBalance > 15*time.Second {
		t.Fatalf("trial balance took %s over %d lines", trialBalance, len(lines))
	}
	if journal > 15*time.Second {
		t.Fatalf("reading %d lines took %s", len(lines), journal)
	}
}
