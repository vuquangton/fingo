package reports

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/gl"
)

// Every row of every report says how many vouchers it came from, and no row
// comes from none: a figure nobody can point at a voucher for is a figure the
// reports do not state.
func TestReports_EveryRowTracesToAVoucher(t *testing.T) {
	f := newFixture(t)

	for _, p := range f.periodsOf(t) {
		t.Run(p.ID, func(t *testing.T) {
			ledger := f.trialBalance(t, p.ID)
			if len(ledger.Rows) == 0 {
				t.Fatalf("the trial balance for %s holds no rows", p.ID)
			}
			for _, row := range ledger.Rows {
				if row.SourceVoucherCount <= 0 {
					t.Errorf("trial balance row %s traces to %d vouchers", row.AccountCode, row.SourceVoucherCount)
				}
			}

			profit := f.profitAndLoss(t, p.ID)
			for _, row := range append(append([]ProfitAndLossRow{}, profit.Revenue...), profit.Expenses...) {
				if row.SourceVoucherCount <= 0 {
					t.Errorf("profit and loss row %s traces to %d vouchers", row.AccountCode, row.SourceVoucherCount)
				}
			}

			sheet := f.balanceSheet(t, p.ID)
			rows := 0
			for _, section := range [][]BalanceSheetRow{sheet.Assets, sheet.Liabilities, sheet.Equity} {
				for _, row := range section {
					rows++
					if row.SourceVoucherCount <= 0 {
						t.Errorf("balance sheet row %s traces to %d vouchers", row.AccountCode, row.SourceVoucherCount)
					}
				}
			}
			if rows == 0 {
				t.Errorf("the balance sheet for %s holds no rows", p.ID)
			}
		})
	}
}

// A report's own total count is every voucher it read, and never fewer than the
// widest row: the report cannot have been read from less than one of its rows.
//
// The three reports read different windows — the trial balance and the balance
// sheet the whole of the books, the profit and loss report the period alone,
// less the closing entries — so their counts differ by what they read and not by
// what they chose to count.
func TestReports_TheReportCountsEveryVoucherItRead(t *testing.T) {
	f := newFixture(t)

	ledger := f.trialBalance(t, january.ID)
	if want := 5; ledger.SourceVoucherCount != want {
		t.Errorf("the trial balance was read from %d vouchers, want %d", ledger.SourceVoucherCount, want)
	}

	for _, row := range ledger.Rows {
		if row.SourceVoucherCount > ledger.SourceVoucherCount {
			t.Errorf("row %s traces to %d vouchers and the report to %d",
				row.AccountCode, row.SourceVoucherCount, ledger.SourceVoucherCount)
		}
	}

	// The balance sheet reads the same lines as the trial balance, so it reads
	// the same vouchers.
	sheet := f.balanceSheet(t, january.ID)
	if sheet.SourceVoucherCount != ledger.SourceVoucherCount {
		t.Errorf("the balance sheet read %d vouchers and the trial balance %d, and both read the whole ledger",
			sheet.SourceVoucherCount, ledger.SourceVoucherCount)
	}

	// January holds four vouchers and the close is one of them: the profit and
	// loss report read the other three, including the one that only moved
	// balance sheet accounts.
	profit := f.profitAndLoss(t, january.ID)
	if want := 3; profit.SourceVoucherCount != want {
		t.Errorf("the profit and loss report was read from %d vouchers, want %d", profit.SourceVoucherCount, want)
	}
	if want := profit.SourceVoucherCount + profit.ExcludedClosingVouchers; want != 4 {
		t.Errorf("the report read %d vouchers and left out %d, which is not the four January holds",
			profit.SourceVoucherCount, profit.ExcludedClosingVouchers)
	}

	for _, row := range append(append([]ProfitAndLossRow{}, profit.Revenue...), profit.Expenses...) {
		if row.SourceVoucherCount > profit.SourceVoucherCount {
			t.Errorf("row %s traces to %d vouchers and the report to %d",
				row.AccountCode, row.SourceVoucherCount, profit.SourceVoucherCount)
		}
	}
}

// The reports read the same ledger, so a figure one of them states has to be the
// figure the lines hold — added up here, in the test, and not by the code under
// test.
func TestReports_AgreeWithTheLinesTheyRead(t *testing.T) {
	f := newFixture(t)

	profit := f.profitAndLoss(t, january.ID)
	ledger := f.trialBalance(t, january.ID)
	sheet := f.balanceSheet(t, january.ID)

	// The period's revenue and expenses, added up here from the vouchers,
	// leaving out the close exactly as the report has to.
	var revenue, expenses decimal.Decimal

	for _, v := range books() {
		if v.id == "v-close" {
			continue
		}
		if v.date.Month() != time.January {
			continue
		}

		for _, line := range v.lines {
			own := line.Amount
			if line.Side == gl.SideDebit {
				own = own.Neg()
			}

			switch line.AccountCode {
			case "5111":
				revenue = revenue.Add(own)
			case "632":
				expenses = expenses.Sub(own)
			}
		}
	}

	if !revenue.Equal(profit.TotalRevenue) {
		t.Errorf("the January vouchers earned %s and the report says %s", revenue, profit.TotalRevenue)
	}
	if !expenses.Equal(profit.TotalExpenses) {
		t.Errorf("the January vouchers spent %s and the report says %s", expenses, profit.TotalExpenses)
	}

	// The assets the balance sheet states, as the closing balances the trial
	// balance states for the same accounts.
	assets := trialRow(t, ledger, "111").ClosingDebit
	assets = assets.Add(trialRow(t, ledger, "131").ClosingDebit)
	assets = assets.Add(trialRow(t, ledger, "156").ClosingDebit)

	if !assets.Equal(sheet.TotalAssets) {
		t.Errorf("the trial balance's asset balances add up to %s and the balance sheet states %s", assets, sheet.TotalAssets)
	}
}
