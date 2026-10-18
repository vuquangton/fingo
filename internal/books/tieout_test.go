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

// TestReportsAgreeWithEachOther and with the ledger they came from.
//
// Each report can look right on its own screen and still disagree with its
// neighbours: a trial balance that balances, a balance sheet that balances, and a
// profit that is neither. This test reads the vouchers, then asserts the four
// figures that must be the same figure told four ways.
func TestReportsAgreeWithEachOther(t *testing.T) {
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
		clientID, "Tie-out client", "TIE-"+clientID); err != nil {
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

	day := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	period, err := b.EnsureCurrentYear(t.Context(), day)
	if err != nil {
		t.Fatalf("period: %v", err)
	}

	post := func(description string, lines ...ledger.Line) {
		t.Helper()
		if _, err := ledger.Post(t.Context(), st, scope, ledger.Request{
			ID:          newUUID(t),
			Description: description,
			Actor:       "tieout",
			Date:        day,
			Type:        gl.VoucherTypeGeneral,
			Lines:       lines,
		}); err != nil {
			t.Fatalf("post %s: %v", description, err)
		}
	}

	// Capital, a sale with output VAT, the collection, a cost with input VAT, the payment.
	post("Góp vốn",
		ledger.Line{Account: "1111", Side: gl.SideDebit, Amount: money.FromInt(5_000_000)},
		ledger.Line{Account: "4111", Side: gl.SideCredit, Amount: money.FromInt(5_000_000)})

	post("Bán hàng",
		ledger.Line{Account: "131", Side: gl.SideDebit, Amount: money.FromInt(1_100_000)},
		ledger.Line{Account: "5111", Side: gl.SideCredit, Amount: money.FromInt(1_000_000)},
		ledger.Line{Account: "33311", Side: gl.SideCredit, Amount: money.FromInt(100_000)})

	post("Thu tiền",
		ledger.Line{Account: "1111", Side: gl.SideDebit, Amount: money.FromInt(1_100_000)},
		ledger.Line{Account: "131", Side: gl.SideCredit, Amount: money.FromInt(1_100_000)})

	post("Chi phí",
		ledger.Line{Account: "6422", Side: gl.SideDebit, Amount: money.FromInt(300_000)},
		ledger.Line{Account: "1331", Side: gl.SideDebit, Amount: money.FromInt(30_000)},
		ledger.Line{Account: "331", Side: gl.SideCredit, Amount: money.FromInt(330_000)})

	post("Trả tiền",
		ledger.Line{Account: "331", Side: gl.SideDebit, Amount: money.FromInt(330_000)},
		ledger.Line{Account: "1111", Side: gl.SideCredit, Amount: money.FromInt(330_000)})

	// What the ledger itself says, straight from the lines.
	lines, err := b.Lines(t.Context(), time.Time{}, period.EndDate)
	if err != nil {
		t.Fatalf("lines: %v", err)
	}

	var debit, credit int64
	touched := map[string]bool{}
	for _, l := range lines {
		touched[l.AccountCode] = true
		if l.Side == gl.SideDebit {
			debit += l.Amount.Int64()
		} else {
			credit += l.Amount.Int64()
		}
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

	// 1. The trial balance is the ledger, summed: same debits, same credits.
	if tb.PeriodDebit.Int64() != debit || tb.PeriodCredit.Int64() != credit {
		t.Fatalf("trial balance period debit/credit = %d/%d, ledger says %d/%d",
			tb.PeriodDebit.Int64(), tb.PeriodCredit.Int64(), debit, credit)
	}

	// 2. It balances, both in the period and in closing.
	if !tb.PeriodDebit.Equal(tb.PeriodCredit) {
		t.Fatalf("period does not balance: %d vs %d", tb.PeriodDebit.Int64(), tb.PeriodCredit.Int64())
	}
	if !tb.ClosingDebit.Equal(tb.ClosingCredit) {
		t.Fatalf("closing does not balance: %d vs %d", tb.ClosingDebit.Int64(), tb.ClosingCredit.Int64())
	}

	// 3. Every account the ledger touched has a row, and no row invents an account.
	rows := make(map[string]bool, len(tb.Rows))
	for _, row := range tb.Rows {
		rows[row.AccountCode] = true
	}
	for account := range touched {
		if !rows[account] {
			t.Fatalf("account %s has movements but no trial balance row", account)
		}
	}
	for account := range rows {
		if !touched[account] {
			t.Fatalf("account %s has a trial balance row but no movements", account)
		}
	}

	// 4. The profit is revenue less expenses, and it is what the balance sheet is
	// still carrying as the period's result.
	if !pl.NetProfit.Equal(pl.TotalRevenue.Sub(pl.TotalExpenses)) {
		t.Fatalf("net profit %d is not revenue %d less expenses %d",
			pl.NetProfit.Int64(), pl.TotalRevenue.Int64(), pl.TotalExpenses.Int64())
	}
	if pl.NetProfit.Int64() != 700_000 {
		t.Fatalf("net profit = %d, want 700,000 (1,000,000 revenue less 300,000 cost)",
			pl.NetProfit.Int64())
	}
	if !bs.Result.Equal(pl.NetProfit) {
		t.Fatalf("balance sheet result %d is not the profit and loss net profit %d",
			bs.Result.Int64(), pl.NetProfit.Int64())
	}

	// 5. The balance sheet balances, with the unclosed result inside equity.
	right := bs.TotalLiabilities.Add(bs.TotalEquity)
	if !bs.TotalAssets.Equal(right) {
		t.Fatalf("assets %d do not equal liabilities %d plus equity %d = %d",
			bs.TotalAssets.Int64(), bs.TotalLiabilities.Int64(), bs.TotalEquity.Int64(), right.Int64())
	}

	// The figures a reader would check by hand.
	// Cash 5,000,000 + 1,100,000 - 330,000 = 5,770,000, plus the 30,000 input VAT
	// still recoverable on 1331. The first version of this test forgot the VAT and
	// expected 5,770,000; the balance sheet was right.
	if bs.TotalAssets.Int64() != 5_800_000 {
		t.Fatalf("assets = %d, want 5,800,000 (cash 5,770,000 + input VAT 30,000)",
			bs.TotalAssets.Int64())
	}
}

