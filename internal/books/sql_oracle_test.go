package books_test

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/books"
	"fingo/internal/domain/gl"
	"fingo/internal/ledger"
	"fingo/internal/money"
	"fingo/internal/store"
	"fingo/internal/testdb"
)

// TestReportsEqualSQLRecomputation compares the application's three statements with
// a recomputation done entirely in SQL from voucher_lines.
//
// The point is independence: every other report test compares one Go report with
// another Go report, which agrees even when both are wrong. This one asks the
// database what the books say, using only the account type and the lines, and diffs
// the answer against what the practice would be shown.
func TestReportsEqualSQLRecomputation(t *testing.T) {
	cfg := testdb.Load(t)
	testdb.Open(t, cfg)

	st, err := store.Open(t.Context(), store.Config{DSN: cfg.DSN(), MaxOpenConns: 4})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(st.Close)

	clientID := newUUID(t)
	if _, err := st.Pool().Exec(t.Context(),
		`INSERT INTO clients (id, name, tax_id) VALUES ($1, $2, $3)`,
		clientID, "SQL oracle client", "SQLOR-"+clientID); err != nil {
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

	day := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	period, err := b.EnsureCurrentYear(t.Context(), day)
	if err != nil {
		t.Fatalf("period: %v", err)
	}

	post := func(description string, lines ...ledger.Line) {
		t.Helper()
		if _, err := ledger.Post(t.Context(), st, scope, ledger.Request{
			ID:          newUUID(t),
			Description: description,
			Actor:       "oracle",
			Date:        day,
			Type:        gl.VoucherTypeGeneral,
			Lines:       lines,
		}); err != nil {
			t.Fatalf("post %s: %v", description, err)
		}
	}

	// A book with every account class represented, balanced by construction.
	post("Góp vốn",
		ledger.Line{Account: "1111", Side: gl.SideDebit, Amount: money.FromInt(10_000_000)},
		ledger.Line{Account: "4111", Side: gl.SideCredit, Amount: money.FromInt(10_000_000)})
	post("Bán hàng có VAT",
		ledger.Line{Account: "131", Side: gl.SideDebit, Amount: money.FromInt(2_200_000)},
		ledger.Line{Account: "5111", Side: gl.SideCredit, Amount: money.FromInt(2_000_000)},
		ledger.Line{Account: "33311", Side: gl.SideCredit, Amount: money.FromInt(200_000)})
	post("Giá vốn",
		ledger.Line{Account: "632", Side: gl.SideDebit, Amount: money.FromInt(1_200_000)},
		ledger.Line{Account: "156", Side: gl.SideCredit, Amount: money.FromInt(1_200_000)})
	post("Chi phí quản lý",
		ledger.Line{Account: "6422", Side: gl.SideDebit, Amount: money.FromInt(300_000)},
		ledger.Line{Account: "331", Side: gl.SideCredit, Amount: money.FromInt(300_000)})
	post("Thu tiền khách",
		ledger.Line{Account: "1111", Side: gl.SideDebit, Amount: money.FromInt(2_200_000)},
		ledger.Line{Account: "131", Side: gl.SideCredit, Amount: money.FromInt(2_200_000)})

	// ---- independent SQL recomputation ----
	type totals struct {
		assets, liabilities, equity, revenue, expense int64
	}
	var sqlTotals totals

	err = st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			WITH balances AS (
			  SELECT a.code, a.account_type,
			         SUM(CASE WHEN l.side = 'DEBIT' THEN l.amount ELSE -l.amount END) AS net
			    FROM voucher_lines l
			    JOIN vouchers v ON v.id = l.voucher_id
			    JOIN accounts a ON a.client_id = l.client_id AND a.code = l.account_code
			   WHERE v.posted_at IS NOT NULL
			     AND v.voucher_date BETWEEN $1 AND $2
			   GROUP BY a.code, a.account_type
			)
			-- Signed, not positive-only: a credit balance in an asset account
			-- (accumulated depreciation, or inventory that has been issued) reduces
			-- assets, and filtering on sign would double-count it as nothing.
			SELECT
			  COALESCE(SUM(CASE WHEN account_type = 'ASSET'     THEN net END), 0),
			  COALESCE(SUM(CASE WHEN account_type = 'LIABILITY' THEN -net END), 0),
			  COALESCE(SUM(CASE WHEN account_type = 'EQUITY'    THEN -net END), 0),
			  COALESCE(SUM(CASE WHEN account_type = 'REVENUE'   THEN -net END), 0),
			  COALESCE(SUM(CASE WHEN account_type = 'EXPENSE'   THEN net END), 0)
			  FROM balances`,
			period.StartDate, period.EndDate).
			Scan(&sqlTotals.assets, &sqlTotals.liabilities, &sqlTotals.equity,
				&sqlTotals.revenue, &sqlTotals.expense)
	})
	if err != nil {
		t.Fatalf("sql recomputation: %v", err)
	}

	tb, err := b.TrialBalance(t.Context(), period.ID)
	if err != nil {
		t.Fatalf("trial balance: %v", err)
	}
	pl, err := b.ProfitAndLoss(t.Context(), period.ID)
	if err != nil {
		t.Fatalf("p&l: %v", err)
	}
	bs, err := b.BalanceSheet(t.Context(), period.ID)
	if err != nil {
		t.Fatalf("balance sheet: %v", err)
	}

	sqlProfit := sqlTotals.revenue - sqlTotals.expense
	sqlAssets := sqlTotals.assets
	sqlRight := sqlTotals.liabilities + sqlTotals.equity + sqlProfit

	if pl.TotalRevenue.Int64() != sqlTotals.revenue {
		t.Fatalf("revenue: app %d, SQL %d", pl.TotalRevenue.Int64(), sqlTotals.revenue)
	}
	if pl.TotalExpenses.Int64() != sqlTotals.expense {
		t.Fatalf("expenses: app %d, SQL %d", pl.TotalExpenses.Int64(), sqlTotals.expense)
	}
	if pl.NetProfit.Int64() != sqlProfit {
		t.Fatalf("net profit: app %d, SQL %d", pl.NetProfit.Int64(), sqlProfit)
	}
	if bs.TotalAssets.Int64() != sqlAssets {
		t.Fatalf("assets: app %d, SQL %d", bs.TotalAssets.Int64(), sqlAssets)
	}
	if bs.TotalLiabilities.Int64() != sqlTotals.liabilities {
		t.Fatalf("liabilities: app %d, SQL %d", bs.TotalLiabilities.Int64(), sqlTotals.liabilities)
	}
	if !bs.TotalAssets.Equal(bs.TotalLiabilities.Add(bs.TotalEquity)) {
		t.Fatalf("balance sheet does not balance: %d vs %d + %d",
			bs.TotalAssets.Int64(), bs.TotalLiabilities.Int64(), bs.TotalEquity.Int64())
	}
	if sqlAssets != sqlRight {
		t.Fatalf("SQL balance sheet does not balance: %d vs %d", sqlAssets, sqlRight)
	}

	t.Logf("SQL: assets=%d liabilities=%d equity=%d revenue=%d expense=%d profit=%d",
		sqlTotals.assets, sqlTotals.liabilities, sqlTotals.equity,
		sqlTotals.revenue, sqlTotals.expense, sqlProfit)
	t.Logf("app: assets=%d liabilities=%d equity=%d revenue=%d expense=%d profit=%d",
		bs.TotalAssets.Int64(), bs.TotalLiabilities.Int64(), bs.TotalEquity.Int64(),
		pl.TotalRevenue.Int64(), pl.TotalExpenses.Int64(), pl.NetProfit.Int64())

	// The trial balance's own totals must equal the SQL movement totals.
	var sqlDebit, sqlCredit int64
	if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), `
			SELECT COALESCE(SUM(CASE WHEN side='DEBIT' THEN amount END),0),
			       COALESCE(SUM(CASE WHEN side='CREDIT' THEN amount END),0)
			  FROM voucher_lines`).Scan(&sqlDebit, &sqlCredit)
	}); err != nil {
		t.Fatalf("sql totals: %v", err)
	}
	if tb.PeriodDebit.Int64() != sqlDebit || tb.PeriodCredit.Int64() != sqlCredit {
		t.Fatalf("trial balance totals: app %d/%d, SQL %d/%d",
			tb.PeriodDebit.Int64(), tb.PeriodCredit.Int64(), sqlDebit, sqlCredit)
	}
}
