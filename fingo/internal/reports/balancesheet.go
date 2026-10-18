package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/gl"
	"fingo/internal/period"
)

// BalanceSheet is what a client owns, what it owes, and what is left for its
// owners, at the end of a period.
//
// It is a position at an instant, so it is read from every line up to the
// period's end and not only from the period's own: an asset bought three years
// ago is still an asset, and a report that showed only this month's movements
// would be a trial balance wearing a balance sheet's name.
type BalanceSheet struct {
	PeriodID string
	AsOf     time.Time

	Assets      []BalanceSheetRow
	Liabilities []BalanceSheetRow
	Equity      []BalanceSheetRow

	TotalAssets      decimal.Decimal
	TotalLiabilities decimal.Decimal

	// TotalEquity includes Result: what the owners have is what has been carried
	// to them plus what the period has earned and nobody has carried yet.
	TotalEquity decimal.Decimal

	// Result is the period's result that is still standing in the profit and
	// loss accounts — what no close has carried to equity yet.
	//
	// It belongs in equity. Until a close moves it, the profit and loss accounts
	// hold it, and a balance sheet that left it out would show assets exceeding
	// liabilities and equity by exactly the profit, which is the one thing a
	// balance sheet may not do. Once the period is closed this is zero and the
	// same figure sits in the equity accounts instead.
	//
	// It is part of TotalEquity and it is not a row: the accounts that hold it
	// are the profit and loss accounts, whose own balances are stated once, as
	// this figure, rather than account by account on a balance sheet.
	Result decimal.Decimal

	// SourceVoucherCount is how many distinct vouchers the whole report was
	// summed from.
	SourceVoucherCount int
}

// BalanceSheetRow is one account on the balance sheet.
type BalanceSheetRow struct {
	AccountCode string
	AccountName string

	// Amount is in the account's own sense: an asset positive when it is held,
	// a liability and equity positive when they are owed or owned.
	Amount decimal.Decimal

	// SourceVoucherCount is how many distinct vouchers put a figure on this row.
	SourceVoucherCount int
}

// Balanced reports whether what the client owns equals what it owes together
// with what is left for its owners.
//
// It is a check on the report and not a property of it: the identity holds for
// any set of posted vouchers, so a balance sheet that does not balance was not
// summed from posted vouchers.
func (b BalanceSheet) Balanced() bool {
	return b.TotalAssets.Equal(b.TotalLiabilities.Add(b.TotalEquity))
}

// BalanceSheet reads the client's balance sheet at the end of a period.
func (s *Service) BalanceSheet(ctx context.Context, periodID string) (BalanceSheet, error) {
	wanted, chart, err := s.realm(ctx, periodID)
	if err != nil {
		return BalanceSheet{}, err
	}

	lines, err := s.journal.Lines(ctx, time.Time{}, wanted.EndDate)
	if err != nil {
		return BalanceSheet{}, fmt.Errorf("read the ledger to %s: %w", dayOf(wanted.EndDate).Format(time.DateOnly), err)
	}

	return buildBalanceSheet(wanted, lines, chart)
}

// buildBalanceSheet sums everything the books hold into assets, liabilities,
// equity and the result nothing has carried yet.
func buildBalanceSheet(wanted period.Period, lines []Line, chart *gl.ChartOfAccounts) (BalanceSheet, error) {
	found, err := movements(lines, time.Time{}, wanted.EndDate, nil)
	if err != nil {
		return BalanceSheet{}, err
	}

	report := BalanceSheet{
		PeriodID:    wanted.ID,
		AsOf:        wanted.EndDate,
		Assets:      make([]BalanceSheetRow, 0, len(found)),
		Liabilities: make([]BalanceSheetRow, 0, len(found)),
		Equity:      make([]BalanceSheetRow, 0, len(found)),
	}

	all := make(map[string]bool)

	for _, code := range sortedCodes(found) {
		account, err := accountOf(chart, code)
		if err != nil {
			return BalanceSheet{}, err
		}

		held := found[code]
		debit := held.openingDebit.Add(held.periodDebit)
		credit := held.openingCredit.Add(held.periodCredit)

		row := BalanceSheetRow{
			AccountCode:        code,
			AccountName:        account.Name,
			SourceVoucherCount: len(held.carried),
		}

		for voucher := range held.carried {
			all[voucher] = true
		}

		switch account.Type {
		case gl.AccountTypeAsset:
			row.Amount = debit.Sub(credit)
			report.Assets = append(report.Assets, row)
			report.TotalAssets = report.TotalAssets.Add(row.Amount)
		case gl.AccountTypeLiability:
			row.Amount = credit.Sub(debit)
			report.Liabilities = append(report.Liabilities, row)
			report.TotalLiabilities = report.TotalLiabilities.Add(row.Amount)
		case gl.AccountTypeEquity:
			row.Amount = credit.Sub(debit)
			report.Equity = append(report.Equity, row)
		case gl.AccountTypeRevenue:
			report.Result = report.Result.Add(credit.Sub(debit))
		case gl.AccountTypeExpense:
			report.Result = report.Result.Sub(debit.Sub(credit))
		default:
			// The five account types are a closed set and the chart refuses any
			// other, so this is unreachable today. It is here because the day a
			// sixth type arrives, an account that fell through this switch would
			// leave the sheet a little out with nothing to say why.
			return BalanceSheet{}, fmt.Errorf(
				"%w: account %s is a %s account, and a balance sheet has no section for it",
				ErrUnknownAccount, code, account.Type)
		}
	}

	// The equity rows are totalled here rather than inside the switch, so that
	// the result standing in the profit and loss accounts is added once, after
	// every account has been read.
	for _, row := range report.Equity {
		report.TotalEquity = report.TotalEquity.Add(row.Amount)
	}
	report.TotalEquity = report.TotalEquity.Add(report.Result)

	report.SourceVoucherCount = len(all)

	return report, nil
}
