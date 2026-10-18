package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/gl"
	"fingo/internal/period"
)

// ProfitAndLoss is what a period earned and what it spent, account by account
// and heading by heading.
//
// # Closing entries are left out
//
// A close carries the period's revenue and expenses into retained earnings, in
// entries dated inside the period it closes. Summed with the rest they would
// cancel the very figures the report exists to state, and a closed month would
// report a profit of nothing.
//
// So the closing entries are recognised — they are the vouchers that touch the
// profit determination account, 911 — and left out whole. What is left is what
// the period traded, which is also what the close carried: the report's net
// profit is the change the same lines made to retained earnings.
//
// Whole is the word to watch. A voucher that touches the result account is a
// closing entry through and through, so every line of it is left out. A client
// who bundles an ordinary sale into the same voucher as a closing line loses
// that sale from this report, and nothing here will say so: keep closing entries
// to what they close.
type ProfitAndLoss struct {
	PeriodID  string
	StartDate time.Time
	EndDate   time.Time

	Revenue  []ProfitAndLossRow
	Expenses []ProfitAndLossRow

	TotalRevenue  decimal.Decimal
	TotalExpenses decimal.Decimal

	// NetProfit is revenue less expenses: positive for a profit, negative for a
	// loss, and exactly what a close would carry to retained earnings.
	NetProfit decimal.Decimal

	// ExcludedClosingVouchers is how many vouchers the report left out because
	// they carry the period's result into equity. It is stated rather than
	// silently absent: a reader can see the exclusion instead of having to trust
	// it.
	ExcludedClosingVouchers int

	// SourceVoucherCount is how many distinct vouchers the report read: the
	// period's own, less the closing entries it left out. It is every voucher
	// the period holds, and not only those that touched a revenue or expense
	// account, so that it means the same thing here as on the other reports.
	SourceVoucherCount int
}

// ProfitAndLossRow is one profit and loss account, and what it did in the
// period.
type ProfitAndLossRow struct {
	// Group is the heading the account reports under — "511" for 5111, and the
	// account's own code where nothing is above it.
	GroupCode string
	GroupName string

	AccountCode string
	AccountName string

	// Amount is in the account's own sense: revenue positive when earned,
	// expenses positive when spent. A deduction posted against a revenue account
	// comes out negative, which is what a deduction is.
	Amount decimal.Decimal

	// SourceVoucherCount is how many distinct vouchers put a figure on this row.
	SourceVoucherCount int
}

// ProfitAndLoss reads the client's profit and loss for a period.
func (s *Service) ProfitAndLoss(ctx context.Context, periodID string) (ProfitAndLoss, error) {
	wanted, chart, err := s.realm(ctx, periodID)
	if err != nil {
		return ProfitAndLoss{}, err
	}

	lines, err := s.journal.Lines(ctx, wanted.StartDate, wanted.EndDate)
	if err != nil {
		return ProfitAndLoss{}, fmt.Errorf("read the ledger for %s: %w", wanted.ID, err)
	}

	return buildProfitAndLoss(wanted, lines, chart, s.closing)
}

// buildProfitAndLoss sums a period's revenue and expense lines by account.
func buildProfitAndLoss(
	wanted period.Period,
	lines []Line,
	chart *gl.ChartOfAccounts,
	closing ClosingAccounts,
) (ProfitAndLoss, error) {
	excluded := closingVouchers(lines, closing.Result)
	found, err := movements(lines, wanted.StartDate, wanted.EndDate, excluded)
	if err != nil {
		return ProfitAndLoss{}, err
	}

	report := ProfitAndLoss{
		PeriodID:                wanted.ID,
		StartDate:               wanted.StartDate,
		EndDate:                 wanted.EndDate,
		Revenue:                 make([]ProfitAndLossRow, 0, len(found)),
		Expenses:                make([]ProfitAndLossRow, 0, len(found)),
		ExcludedClosingVouchers: len(excluded),
	}

	all := make(map[string]bool)

	for _, code := range sortedCodes(found) {
		held := found[code]

		for voucher := range held.periodCarried {
			all[voucher] = true
		}

		// The period did nothing to this account, whatever it carried in and
		// whatever an adapter handed over: there is no figure here and no
		// voucher behind one.
		if held.periodDebit.IsZero() && held.periodCredit.IsZero() {
			continue
		}

		account, err := accountOf(chart, code)
		if err != nil {
			return ProfitAndLoss{}, err
		}

		// An account of any other type moved in the period without being an
		// earning or a cost — a receivable collected, a payable paid. It is in
		// the trial balance and on the balance sheet, and it is not profit.
		if account.Type != gl.AccountTypeRevenue && account.Type != gl.AccountTypeExpense {
			continue
		}

		group := groupOf(chart, account)

		amount := held.periodCredit.Sub(held.periodDebit)
		if account.Type == gl.AccountTypeExpense {
			amount = held.periodDebit.Sub(held.periodCredit)
		}

		row := ProfitAndLossRow{
			GroupCode:          group.Code,
			GroupName:          group.Name,
			AccountCode:        code,
			AccountName:        account.Name,
			Amount:             amount,
			SourceVoucherCount: len(held.periodCarried),
		}

		if account.Type == gl.AccountTypeRevenue {
			report.Revenue = append(report.Revenue, row)
			report.TotalRevenue = report.TotalRevenue.Add(amount)
		} else {
			report.Expenses = append(report.Expenses, row)
			report.TotalExpenses = report.TotalExpenses.Add(amount)
		}
	}

	report.NetProfit = report.TotalRevenue.Sub(report.TotalExpenses)
	report.SourceVoucherCount = len(all)

	return report, nil
}
