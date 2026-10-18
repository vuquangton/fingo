package reports

import (
	"fingo/internal/money"
	"testing"
)

// What a period earned, what it spent, and by which heading.
func TestProfitAndLoss_RevenueLessExpensesByGroup(t *testing.T) {
	f := newFixture(t)
	report := f.profitAndLoss(t, january.ID)

	if len(report.Revenue) != 1 || len(report.Expenses) != 1 {
		t.Fatalf("revenue rows = %+v, expense rows = %+v, want one of each", report.Revenue, report.Expenses)
	}

	earned := profitRow(t, report, "5111")
	if earned.GroupCode != "511" || earned.GroupName == "" {
		t.Errorf("5111 reports under %q (%q), want the 511 heading", earned.GroupCode, earned.GroupName)
	}
	if want := money.FromInt(1_000_000); !earned.Amount.Equal(want) {
		t.Errorf("revenue = %s, want %s", earned.Amount, want)
	}

	spent := profitRow(t, report, "632")
	if spent.GroupCode != "632" {
		t.Errorf("632 reports under %q, want its own code: nothing is above it", spent.GroupCode)
	}
	if want := money.FromInt(600_000); !spent.Amount.Equal(want) {
		t.Errorf("cost of goods = %s, want %s", spent.Amount, want)
	}

	if want := money.FromInt(1_000_000); !report.TotalRevenue.Equal(want) {
		t.Errorf("total revenue = %s, want %s", report.TotalRevenue, want)
	}
	if want := money.FromInt(600_000); !report.TotalExpenses.Equal(want) {
		t.Errorf("total expenses = %s, want %s", report.TotalExpenses, want)
	}
	if want := money.FromInt(400_000); !report.NetProfit.Equal(want) {
		t.Errorf("net profit = %s, want %s", report.NetProfit, want)
	}
}

// A closing entry belongs to the period it closes, and it cancels exactly what
// the report exists to state. Left in, a closed month would report a profit of
// nothing.
func TestProfitAndLoss_LeavesTheClosingEntriesOut(t *testing.T) {
	f := newFixture(t)
	report := f.profitAndLoss(t, january.ID)

	if report.ExcludedClosingVouchers != 1 {
		t.Errorf("excluded %d closing vouchers, want 1", report.ExcludedClosingVouchers)
	}

	// Every January voucher but the close: the report read the stock bought on
	// credit too, and it moves no profit and loss account. The count means the
	// same thing on all three reports — the vouchers the report read.
	if want := 3; report.SourceVoucherCount != want {
		t.Errorf("the report was read from %d vouchers, want %d", report.SourceVoucherCount, want)
	}

	for _, row := range append(append([]ProfitAndLossRow{}, report.Revenue...), report.Expenses...) {
		if row.AccountCode == "911" {
			t.Error("the profit determination account is a row of the profit and loss report")
		}
		if row.SourceVoucherCount == 0 {
			t.Errorf("%s has no voucher behind it", row.AccountCode)
		}
	}

	// The same month, read with the close excluded: the revenue and the cost are
	// both still there, and the close that zeroed them is not.
	if !report.NetProfit.Equal(money.FromInt(400_000)) {
		t.Errorf("net profit = %s, want 400000", report.NetProfit)
	}
}

// An open period reports what it has traded so far, and the balance sheet
// carries exactly that much as an unclosed result. One figure, reached two ways.
func TestProfitAndLoss_ReportsAnOpenPeriod(t *testing.T) {
	f := newFixture(t)
	report := f.profitAndLoss(t, february.ID)

	if want := money.FromInt(500_000); !report.NetProfit.Equal(want) {
		t.Errorf("net profit = %s, want %s", report.NetProfit, want)
	}
	if report.ExcludedClosingVouchers != 0 {
		t.Errorf("excluded %d closing vouchers from an open period, want none", report.ExcludedClosingVouchers)
	}

	sheet := f.balanceSheet(t, february.ID)
	if !sheet.Result.Equal(report.NetProfit) {
		t.Errorf("the open period's result on the balance sheet is %s and the profit and loss report says %s",
			sheet.Result, report.NetProfit)
	}
}

// The profit and loss report's net is what the period did to retained earnings.
//
// Read from the ledger twice and by two different reports — the net the profit
// and loss accounts came to, and the movement the close made on retained
// earnings — the two have to agree, because they are the same figure stated on
// either side of the entry that moved it.
func TestProfitAndLoss_NetEqualsTheChangeInRetainedEarnings(t *testing.T) {
	f := newFixture(t)
	report := f.profitAndLoss(t, january.ID)
	ledger := f.trialBalance(t, january.ID)

	retained := trialRow(t, ledger, "4212")
	change := retained.PeriodCredit.Sub(retained.PeriodDebit)

	if change.IsZero() {
		t.Fatal("the close moved nothing into retained earnings, so this checks nothing")
	}
	if !report.NetProfit.Equal(change) {
		t.Errorf("net profit = %s and retained earnings moved by %s", report.NetProfit, change)
	}
	if want := money.FromInt(400_000); !change.Equal(want) {
		t.Errorf("retained earnings moved by %s, want %s", change, want)
	}
}
