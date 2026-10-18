package reports

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"fingo/internal/close"
	"fingo/internal/domain/gl"
	"fingo/internal/money"
	"fingo/internal/period"
)

// The identities are properties of posted vouchers, not of the books these
// tests happen to hold, so they have to hold for any set of them: months of
// trading entries, every one balanced, every period closed the way
// internal/close closes one.
func TestReports_BalanceForAnySetOfPostedVouchers(t *testing.T) {
	calendar := sixMonths()
	lines := generatedBooks(calendar)
	service := generatedService(t, calendar, lines)

	profitable, losing := 0, 0

	for _, p := range calendar {
		t.Run(p.ID, func(t *testing.T) {
			ledger := readTrialBalance(t, service, p.ID)
			if !ledger.Balanced() {
				t.Errorf("opening %s/%s, period %s/%s, closing %s/%s",
					ledger.OpeningDebit, ledger.OpeningCredit,
					ledger.PeriodDebit, ledger.PeriodCredit,
					ledger.ClosingDebit, ledger.ClosingCredit)
			}

			for _, row := range ledger.Rows {
				if row.SourceVoucherCount <= 0 {
					t.Errorf("row %s traces to %d vouchers", row.AccountCode, row.SourceVoucherCount)
				}
			}

			sheet := readBalanceSheet(t, service, p.ID)
			if !sheet.Balanced() {
				t.Errorf("assets %s, liabilities %s, equity %s",
					sheet.TotalAssets, sheet.TotalLiabilities, sheet.TotalEquity)
			}

			// The close carried the month's result to retained earnings, so the
			// profit and loss report and the movement on 4212 are one figure.
			profit := readProfitAndLoss(t, service, p.ID)
			retained := rowFor(t, ledger.Rows, "4212")
			if movement := retained.PeriodCredit.Sub(retained.PeriodDebit); !movement.Equal(profit.NetProfit) {
				t.Errorf("the month's net profit is %s and retained earnings moved by %s", profit.NetProfit, movement)
			}

			// The result is carried the way its sign says, in both directions.
			if profit.NetProfit.IsNegative() {
				if !retained.PeriodDebit.IsPositive() {
					t.Errorf("a loss of %s was carried as a credit", profit.NetProfit)
				}
			} else if !retained.PeriodCredit.IsPositive() && !profit.NetProfit.IsZero() {
				t.Errorf("a profit of %s was carried as a debit", profit.NetProfit)
			}
		})
	}

	// Both directions were exercised, so neither branch of the carry is dead
	// code in this test.
	for _, p := range calendar {
		if readProfitAndLoss(t, service, p.ID).NetProfit.IsNegative() {
			losing++
		} else {
			profitable++
		}
	}
	if profitable == 0 || losing == 0 {
		t.Errorf("the generated books made %d profitable months and %d losing ones, want at least one of each",
			profitable, losing)
	}
}

// sixMonths is half a year of one-month periods.
func sixMonths() []period.Period {
	out := make([]period.Period, 0, 6)

	for month := 1; month <= 6; month++ {
		start := time.Date(2026, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		out = append(out, period.Period{
			ID:        start.Format("2006-01"),
			StartDate: start,
			EndDate:   start.AddDate(0, 1, -1),
		})
	}

	return out
}

// generatedBooks posts four balanced vouchers a month — money in and out,
// revenue earned, costs incurred — and then closes the month: revenue and
// expenses into 911, and 911 into retained earnings.
//
// The generator is fixed, so the books are the same on every run and a failure
// is a failure and not a flake.
func generatedBooks(calendar []period.Period) []Line {
	random := rand.New(rand.NewPCG(7, 11))

	debitNature := []string{"111", "131", "156", "632", "6422"}
	creditNature := []string{"331", "4111", "5111", "5118"}

	var lines []Line

	for month, p := range calendar {
		revenue := map[string]int64{}
		expenses := map[string]int64{}

		for n := 0; n < 4; n++ {
			id := fmt.Sprintf("v-%d-%d", month, n)
			date := p.StartDate.AddDate(0, 0, 1+n)
			amount := random.Int64N(1_000_000) + 1

			debit := debitNature[random.IntN(len(debitNature))]
			credit := creditNature[random.IntN(len(creditNature))]

			lines = append(lines,
				Line{VoucherID: id, VoucherDate: date, AccountCode: debit, Side: gl.SideDebit, Amount: money.FromInt(amount)},
				Line{VoucherID: id, VoucherDate: date, AccountCode: credit, Side: gl.SideCredit, Amount: money.FromInt(amount)},
			)

			if credit == "5111" || credit == "5118" {
				revenue[credit] += amount
			}
			if debit == "632" || debit == "6422" {
				expenses[debit] += amount
			}
		}

		// A month's depreciation: an expense against a contra-asset, which
		// carries its balance on the side an asset normally does not. The
		// reports classify by the chart's type and not by the side, and this is
		// the shape that would show it if they did otherwise.
		{
			id := fmt.Sprintf("dep-%d", month)
			date := p.StartDate.AddDate(0, 0, 20)
			amount := random.Int64N(100_000) + 1

			lines = append(lines,
				Line{VoucherID: id, VoucherDate: date, AccountCode: "6422", Side: gl.SideDebit, Amount: money.FromInt(amount)},
				Line{VoucherID: id, VoucherDate: date, AccountCode: "2141", Side: gl.SideCredit, Amount: money.FromInt(amount)},
			)
			expenses["6422"] += amount
		}

		id := fmt.Sprintf("close-%d", month)
		date := p.EndDate
		net := int64(0)
		for _, code := range []string{"5111", "5118"} {
			if amount := revenue[code]; amount > 0 {
				lines = append(lines,
					Line{VoucherID: id, VoucherDate: date, AccountCode: code, Side: gl.SideDebit, Amount: money.FromInt(amount)},
					Line{VoucherID: id, VoucherDate: date, AccountCode: "911", Side: gl.SideCredit, Amount: money.FromInt(amount)},
				)
				net += amount
			}
		}

		for _, code := range []string{"632", "6422"} {
			if amount := expenses[code]; amount > 0 {
				lines = append(lines,
					Line{VoucherID: id, VoucherDate: date, AccountCode: "911", Side: gl.SideDebit, Amount: money.FromInt(amount)},
					Line{VoucherID: id, VoucherDate: date, AccountCode: code, Side: gl.SideCredit, Amount: money.FromInt(amount)},
				)
				net -= amount
			}
		}

		switch {
		case net > 0:
			lines = append(lines,
				Line{VoucherID: id, VoucherDate: date, AccountCode: "911", Side: gl.SideDebit, Amount: money.FromInt(net)},
				Line{VoucherID: id, VoucherDate: date, AccountCode: "4212", Side: gl.SideCredit, Amount: money.FromInt(net)},
			)
		case net < 0:
			lines = append(lines,
				Line{VoucherID: id, VoucherDate: date, AccountCode: "4212", Side: gl.SideDebit, Amount: money.FromInt(-net)},
				Line{VoucherID: id, VoucherDate: date, AccountCode: "911", Side: gl.SideCredit, Amount: money.FromInt(-net)},
			)
		}
	}

	return lines
}

func generatedService(t *testing.T, calendar []period.Period, lines []Line) *Service {
	t.Helper()

	chart := gl.NewChartOfAccounts()
	for _, account := range gl.TT133Chart() {
		if err := chart.Add(account); err != nil {
			t.Fatalf("seed chart with %s: %v", account.Code, err)
		}
	}

	service, err := NewService(
		&testPeriods{periods: calendar},
		&testJournal{lines: lines},
		testCharts{chart: chart},
		close.TT133ClosingAccounts(),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	return service
}

func readTrialBalance(t *testing.T, service *Service, periodID string) TrialBalance {
	t.Helper()

	report, err := service.TrialBalance(context.Background(), periodID)
	if err != nil {
		t.Fatalf("TrialBalance(%s): %v", periodID, err)
	}

	return report
}

func readProfitAndLoss(t *testing.T, service *Service, periodID string) ProfitAndLoss {
	t.Helper()

	report, err := service.ProfitAndLoss(context.Background(), periodID)
	if err != nil {
		t.Fatalf("ProfitAndLoss(%s): %v", periodID, err)
	}

	return report
}

func readBalanceSheet(t *testing.T, service *Service, periodID string) BalanceSheet {
	t.Helper()

	report, err := service.BalanceSheet(context.Background(), periodID)
	if err != nil {
		t.Fatalf("BalanceSheet(%s): %v", periodID, err)
	}

	return report
}

func rowFor(t *testing.T, rows []TrialBalanceRow, code string) TrialBalanceRow {
	t.Helper()

	for _, row := range rows {
		if row.AccountCode == code {
			return row
		}
	}

	t.Fatalf("the trial balance holds no row for %s", code)

	return TrialBalanceRow{}
}
