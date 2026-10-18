package close

import (
	"errors"
	"fmt"
	"sort"

	"fingo/internal/domain/gl"
	"fingo/internal/money"
)

// AccountBalance is one account's position over a period, in the account's own
// sense.
//
// A revenue account carries its result on the credit side, so a revenue
// account's Amount is its credits less its debits. An expense account carries
// its result on the debit side, so an expense account's Amount is its debits
// less its credits. Either may be negative: a revenue account carrying a debit
// balance is what a sales deduction or a return leaves behind, and it is real.
//
// Amount is signed in the account's own sense rather than as a debit/credit
// pair because the only question a close asks of a balance is which way it
// points — and a pair would let a caller hand over both sides filled in and get
// a plausible, wrong answer.
type AccountBalance struct {
	AccountCode string
	Type        gl.AccountType
	Amount      money.Dong
}

// ClosingAccounts names the two accounts a close posts through.
//
// They are configuration and not constants. TT 46/2025/TT-BTC lets an enterprise
// modify its own chart, so "the result account is 911" is the statutory default
// rather than a fact about the client — the same reason posting.TT133Defaults
// exists as data.
type ClosingAccounts struct {
	// Result is the profit determination account (TK 911 — Xác định kết quả
	// kinh doanh). Revenue and expenses are carried into it first, so that the
	// period's result is computed by the ledger and not by whoever is closing
	// it, and it is emptied into Retained by the same voucher.
	Result string

	// Retained is the undistributed profit account the result lands in (TK 4212
	// — Lợi nhuận sau thuế chưa phân phối năm nay).
	Retained string
}

// TT133ClosingAccounts is the TT133 closing pair — Phụ lục 1 of Circular
// 133/2016/TT-BTC.
//
// 4211 is the prior year's undistributed profit and 4212 the current year's; a
// close within the year lands in 4212. Note that 421 itself is a heading with
// two sub-accounts and cannot be posted to — see gl.ChartOfAccounts.IsPostable.
//
// The rollover of 4212 into 4211 at the start of a new year is a separate
// year-end act and is deliberately not done here: a period close carries one
// period's result, and a close that quietly moved last year's profit as well
// would be two acts wearing one name.
func TT133ClosingAccounts() ClosingAccounts {
	return ClosingAccounts{Result: "911", Retained: "4212"}
}

// Validate reports whether the pair could be posted through.
func (a ClosingAccounts) Validate() error {
	if a.Result == "" {
		return errors.New("a close needs the profit determination account to carry revenue and expenses into")
	}
	if a.Retained == "" {
		return errors.New("a close needs the retained earnings account to carry the result into")
	}
	if a.Result == a.Retained {
		return fmt.Errorf("the profit determination account and the retained earnings account are both %s, "+
			"and a close that posts an account into itself determines nothing", a.Result)
	}

	return nil
}

// closingEntries turns a period's revenue and expense balances into the
// movements that leave every one of them at zero and put the period's result in
// retained earnings.
//
// It is a pure function: it decides what the entries are, and the caller decides
// whether they may be posted. That is what lets the arithmetic be read and
// tested on its own — the ledger's chart and period rules have nothing to say
// about which side a revenue account is cleared on.
//
// The result is the period's profit, positive for a profit and negative for a
// loss, and is exactly what lands in retained earnings.
func closingEntries(balances []AccountBalance, accounts ClosingAccounts) ([]gl.VoucherLine, money.Dong, error) {
	if err := accounts.Validate(); err != nil {
		return nil, money.Zero, err
	}

	// Sorted so that two runs over the same books produce the same voucher, and
	// a report can be compared with the one taken last month.
	ordered := make([]AccountBalance, len(balances))
	copy(ordered, balances)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].AccountCode < ordered[j].AccountCode
	})

	lines := make([]gl.VoucherLine, 0, 2*len(ordered)+1)

	// result is the balance the closing entries leave on the result account,
	// credit-positive: the period's profit.
	result := money.Zero

	clear := func(balance AccountBalance, carries gl.EntrySide) error {
		if balance.Amount.IsZero() {
			return nil
		}
		if balance.AccountCode == "" {
			return errors.New("a closing balance names no account")
		}
		if balance.AccountCode == accounts.Result {
			return fmt.Errorf("%w: %s carries %s, so the period's result is already part-determined "+
				"and would be determined twice",
				ErrResultAccountCarriesBalance, balance.AccountCode, balance.Amount)
		}

		// Clearing an account is posting against the side it carries its
		// balance on: a revenue account is debited out, an expense account
		// credited out. An account carrying a balance the other way — a
		// deduction, a contra — is cleared the other way, or the entry would
		// double what it should remove.
		amount := balance.Amount.Abs()
		side := opposite(carries)
		if balance.Amount.IsNegative() {
			side = carries
		}

		lines = append(lines,
			gl.VoucherLine{AccountCode: balance.AccountCode, Side: side, Amount: amount},
			// The counterpart is what makes the entry an entry: the account is
			// emptied into the result account rather than simply emptied.
			gl.VoucherLine{AccountCode: accounts.Result, Side: opposite(side), Amount: amount},
		)

		if carries == gl.SideCredit {
			result = result.Add(balance.Amount)
		} else {
			result = result.Sub(balance.Amount)
		}

		return nil
	}

	for _, balance := range ordered {
		switch balance.Type {
		case gl.AccountTypeRevenue:
			if err := clear(balance, gl.SideCredit); err != nil {
				return nil, money.Zero, err
			}
		case gl.AccountTypeExpense:
			if err := clear(balance, gl.SideDebit); err != nil {
				return nil, money.Zero, err
			}
		default:
			return nil, money.Zero, fmt.Errorf(
				"account %s is a %s account, and a close clears only revenue and expenses",
				balance.AccountCode, balance.Type)
		}
	}

	// The result account now holds the period's profit on its credit side, or
	// its loss on the debit side. Either way it is carried to retained earnings
	// and left empty, so the next period starts from nothing and this one's
	// result is stated once, where it belongs.
	if !result.IsZero() {
		lines = append(lines,
			gl.VoucherLine{AccountCode: accounts.Retained, Side: carrySide(result), Amount: result.Abs()},
			gl.VoucherLine{AccountCode: accounts.Result, Side: opposite(carrySide(result)), Amount: result.Abs()},
		)
	}

	return lines, result, nil
}

// carrySide is the side a signed, credit-positive balance sits on.
func carrySide(amount money.Dong) gl.EntrySide {
	if amount.IsNegative() {
		return gl.SideDebit
	}

	return gl.SideCredit
}

// opposite returns the other side of the ledger.
func opposite(side gl.EntrySide) gl.EntrySide {
	if side == gl.SideDebit {
		return gl.SideCredit
	}

	return gl.SideDebit
}
