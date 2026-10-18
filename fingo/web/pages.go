package web

import (
	"time"

	"fingo/internal/period"
	"fingo/internal/reports"
)

// This file turns a report into the page that renders it.
//
// It is deliberately source-independent: it knows what a trial balance, a profit
// and loss report and a balance sheet are, and how this application shows one,
// and it knows nothing about where the lines came from. A client's books read
// from MariaDB and the in-memory books produce the same report types, so both
// produce the same page through here — and a column added to a page cannot be
// added to one of them and quietly forgotten in the other.
//
// The period row is passed in rather than derived. A page has to say which
// period it is showing, and the calendar belongs to whatever is holding the
// books; everything else on the page comes from the report.

// NewPeriodRow renders a period for a page.
//
// Whether it is current is passed in rather than worked out here: which period
// a posting today would land in is the calendar's answer, and this only has to
// say what that period looks like on a screen.
func NewPeriodRow(p period.Period, current bool) PeriodRow {
	from := p.StartDate.Format(time.DateOnly)
	to := p.EndDate.Format(time.DateOnly)

	return PeriodRow{
		ID:      p.ID,
		From:    from,
		To:      to,
		Label:   p.ID + " (" + from + " – " + to + ")",
		Locked:  p.Locked,
		Current: current,
	}
}

// NewTrialBalancePage renders a trial balance.
func NewTrialBalancePage(period PeriodRow, report reports.TrialBalance) TrialBalancePage {
	page := TrialBalancePage{
		Period:         period,
		Debits:         NewAmount(report.PeriodDebit.IntPart()),
		Credits:        NewAmount(report.PeriodCredit.IntPart()),
		ClosingDebits:  NewAmount(report.ClosingDebit.IntPart()),
		ClosingCredits: NewAmount(report.ClosingCredit.IntPart()),
	}

	for _, row := range report.Rows {
		page.Rows = append(page.Rows, TrialBalanceRow{
			Code:          row.AccountCode,
			Name:          row.AccountName,
			Debit:         NewAmount(row.PeriodDebit.IntPart()),
			Credit:        NewAmount(row.PeriodCredit.IntPart()),
			ClosingDebit:  NewAmount(row.ClosingDebit.IntPart()),
			ClosingCredit: NewAmount(row.ClosingCredit.IntPart()),
			Vouchers:      row.SourceVoucherCount,
		})
	}

	// Both pairs have to agree. The period's two sides are what the operator
	// watched go in; the closing two are the position they are about to rely
	// on, and a ledger that balances in movement but not in position has lost
	// something between the two.
	page.Balanced = report.PeriodDebit.Equal(report.PeriodCredit) &&
		report.ClosingDebit.Equal(report.ClosingCredit)

	return page
}

// NewProfitAndLossPage renders a profit and loss report.
func NewProfitAndLossPage(period PeriodRow, report reports.ProfitAndLoss) ProfitAndLossPage {
	page := ProfitAndLossPage{
		Period:        period,
		TotalRevenue:  NewAmount(report.TotalRevenue.IntPart()),
		TotalExpenses: NewAmount(report.TotalExpenses.IntPart()),
		Profit:        NewAmount(report.NetProfit.IntPart()),
	}

	for _, row := range report.Revenue {
		page.Revenue = append(page.Revenue, profitRow(row))
	}
	for _, row := range report.Expenses {
		page.Expenses = append(page.Expenses, profitRow(row))
	}

	return page
}

// NewBalanceSheetPage renders a balance sheet.
func NewBalanceSheetPage(period PeriodRow, report reports.BalanceSheet) BalanceSheetPage {
	page := BalanceSheetPage{
		Period:           period,
		TotalAssets:      NewAmount(report.TotalAssets.IntPart()),
		TotalLiabilities: NewAmount(report.TotalLiabilities.IntPart()),
		TotalEquity:      NewAmount(report.TotalEquity.IntPart()),
	}

	for _, row := range report.Assets {
		page.Assets = append(page.Assets, balanceRow(row))
	}
	for _, row := range report.Liabilities {
		page.Liabilities = append(page.Liabilities, balanceRow(row))
	}
	for _, row := range report.Equity {
		page.Equity = append(page.Equity, balanceRow(row))
	}

	page.Balanced = report.Balanced()

	return page
}

// profitRow renders one line of the profit and loss report.
func profitRow(row reports.ProfitAndLossRow) ProfitAndLossRow {
	return ProfitAndLossRow{
		Code:     row.AccountCode,
		Name:     row.AccountName,
		Amount:   NewAmount(row.Amount.IntPart()),
		Vouchers: row.SourceVoucherCount,
		// A row whose code is its own group is a heading; one that reports
		// under a group above it is a detail line.
		Heading: row.GroupCode == row.AccountCode,
	}
}

// balanceRow renders one line of the balance sheet.
func balanceRow(row reports.BalanceSheetRow) BalanceSheetRow {
	return BalanceSheetRow{
		Code:     row.AccountCode,
		Name:     row.AccountName,
		Amount:   NewAmount(row.Amount.IntPart()),
		Vouchers: row.SourceVoucherCount,
	}
}
