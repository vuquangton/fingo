package reports

import (
	"context"
	"fmt"
	"time"

	"fingo/internal/domain/gl"
	"fingo/internal/money"
	"fingo/internal/period"
)

// TrialBalance is every account the period's books hold, with what it carried
// in, what the period did to it, and what it stands at.
//
// It is the report a bookkeeper reconciles against, so it carries all three
// columns: a movement that leaves an account where it started is a movement that
// still has to be visible, and a balance with no opening beside it cannot be
// checked.
type TrialBalance struct {
	PeriodID  string
	StartDate time.Time
	EndDate   time.Time

	Rows []TrialBalanceRow

	OpeningDebit, OpeningCredit money.Dong
	PeriodDebit, PeriodCredit   money.Dong
	ClosingDebit, ClosingCredit money.Dong

	// SourceVoucherCount is how many distinct vouchers the whole report was
	// summed from.
	SourceVoucherCount int
}

// TrialBalanceRow is one account in a trial balance.
//
// Every figure on the row is in whole đồng and comes from the voucher lines the
// row's SourceVoucherCount counts.
type TrialBalanceRow struct {
	AccountCode string
	AccountName string
	Type        gl.AccountType

	// Opening and Closing are netted onto the side the account stands on: a
	// balance is a position, and a position is on one side.
	OpeningDebit, OpeningCredit money.Dong
	ClosingDebit, ClosingCredit money.Dong

	// Period keeps both sides, because what an account did in a period is a
	// debit total and a credit total, and not the difference between them.
	PeriodDebit, PeriodCredit money.Dong

	// SourceVoucherCount is how many distinct vouchers put a figure on this
	// row. It is never zero: a row exists because a voucher line made it.
	SourceVoucherCount int
}

// Balanced reports whether the trial balance's debits equal its credits — in
// every column, not just the closing one.
//
// The period's movements balance because every voucher is posted balanced; the
// opening and closing columns balance because the books up to a date are a set
// of balanced vouchers too. A column that does not balance means the lines the
// report was read from are not a set of posted vouchers, which is the one thing
// the ledger promises.
func (t TrialBalance) Balanced() bool {
	return t.OpeningDebit.Equal(t.OpeningCredit) &&
		t.PeriodDebit.Equal(t.PeriodCredit) &&
		t.ClosingDebit.Equal(t.ClosingCredit)
}

// TrialBalance reads the client's trial balance for a period.
//
// It reads every line up to the period's end and does the summing itself: there
// is no stored balance behind it, so the report cannot disagree with the ledger
// it was read from.
func (s *Service) TrialBalance(ctx context.Context, periodID string) (TrialBalance, error) {
	wanted, chart, err := s.realm(ctx, periodID)
	if err != nil {
		return TrialBalance{}, err
	}

	lines, err := s.journal.Lines(ctx, time.Time{}, wanted.EndDate)
	if err != nil {
		return TrialBalance{}, fmt.Errorf("read the ledger to %s: %w", dayOf(wanted.EndDate).Format(time.DateOnly), err)
	}

	return buildTrialBalance(wanted, lines, chart)
}

// buildTrialBalance sums lines into a trial balance.
//
// A pure function of what it is given: the summing is the report, so it can be
// read, tested and compared without a ledger, a period or a database.
func buildTrialBalance(wanted period.Period, lines []Line, chart *gl.ChartOfAccounts) (TrialBalance, error) {
	found, err := movements(lines, wanted.StartDate, wanted.EndDate, nil)
	if err != nil {
		return TrialBalance{}, err
	}

	report := TrialBalance{
		PeriodID:  wanted.ID,
		StartDate: wanted.StartDate,
		EndDate:   wanted.EndDate,
		Rows:      make([]TrialBalanceRow, 0, len(found)),
	}

	all := make(map[string]bool)

	for _, code := range sortedCodes(found) {
		account, err := accountOf(chart, code)
		if err != nil {
			return TrialBalance{}, err
		}

		held := found[code]

		openingDebit, openingCredit := netted(held.openingDebit, held.openingCredit)
		closingDebit, closingCredit := netted(
			held.openingDebit.Add(held.periodDebit),
			held.openingCredit.Add(held.periodCredit),
		)

		report.Rows = append(report.Rows, TrialBalanceRow{
			AccountCode:        code,
			AccountName:        account.Name,
			Type:               account.Type,
			OpeningDebit:       openingDebit,
			OpeningCredit:      openingCredit,
			PeriodDebit:        held.periodDebit,
			PeriodCredit:       held.periodCredit,
			ClosingDebit:       closingDebit,
			ClosingCredit:      closingCredit,
			SourceVoucherCount: len(held.carried),
		})

		report.OpeningDebit = report.OpeningDebit.Add(openingDebit)
		report.OpeningCredit = report.OpeningCredit.Add(openingCredit)
		report.PeriodDebit = report.PeriodDebit.Add(held.periodDebit)
		report.PeriodCredit = report.PeriodCredit.Add(held.periodCredit)
		report.ClosingDebit = report.ClosingDebit.Add(closingDebit)
		report.ClosingCredit = report.ClosingCredit.Add(closingCredit)

		for voucher := range held.carried {
			all[voucher] = true
		}
	}

	report.SourceVoucherCount = len(all)

	return report, nil
}
