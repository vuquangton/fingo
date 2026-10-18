package statutory

import (
	"github.com/shopspring/decimal"

	"fingo/internal/reports"
)

// row returns an account's trial balance row, or a row of nothing when the books
// have never been posted to it. A box on a form that nothing moved is a zero
// box, not a missing one.
func row(trial reports.TrialBalance, code string) reports.TrialBalanceRow {
	for _, held := range trial.Rows {
		if held.AccountCode == code {
			return held
		}
	}

	return reports.TrialBalanceRow{AccountCode: code}
}

// openingBalance is what an account carried into the period, debit-positive: a
// credit balance comes back negative.
func openingBalance(trial reports.TrialBalance, code string) decimal.Decimal {
	held := row(trial, code)

	return held.OpeningDebit.Sub(held.OpeningCredit)
}

// movement is the period's movement on one side of an account.
func movement(trial reports.TrialBalance, code string, credit bool) decimal.Decimal {
	held := row(trial, code)
	if credit {
		return held.PeriodCredit
	}

	return held.PeriodDebit
}

// accountMovement is the period's movement on an account, debit-positive: what
// the account was debited less what it was credited.
func accountMovement(trial reports.TrialBalance, code string) decimal.Decimal {
	held := row(trial, code)

	return held.PeriodDebit.Sub(held.PeriodCredit)
}
