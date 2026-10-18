package close

import (
	"errors"
	"reflect"
	"testing"

	"fingo/internal/domain/gl"
	"fingo/internal/money"
)

func revenue(code string, amount int64) AccountBalance {
	return AccountBalance{AccountCode: code, Type: gl.AccountTypeRevenue, Amount: money.FromInt(amount)}
}

func expense(code string, amount int64) AccountBalance {
	return AccountBalance{AccountCode: code, Type: gl.AccountTypeExpense, Amount: money.FromInt(amount)}
}

// voucherOf builds the real thing out of the entries, so a test asserts that
// what a close would post is a voucher the domain accepts, and not merely a set
// of lines that look plausible. AddLine refuses an unknown side, a zero or
// fractional amount and a missing account as it goes.
func voucherOf(t *testing.T, lines []gl.VoucherLine) *gl.Voucher {
	t.Helper()

	v := gl.NewVoucher("voucher-close-test", "closing entries", gl.VoucherTypeGeneral, januaryEnd)
	for i, l := range lines {
		if err := v.AddLine(l); err != nil {
			t.Fatalf("closing line %d (%+v): %v", i+1, l, err)
		}
	}

	return v
}

// totalOn is everything one account carries on one side: the result account is
// touched by the revenue leg, the expense leg and the final leg, so what it comes
// to is the sum and not any single line.
func totalOn(lines []gl.VoucherLine, code string, side gl.EntrySide) money.Dong {
	total := money.Zero
	for _, l := range lines {
		if l.AccountCode == code && l.Side == side {
			total = total.Add(l.Amount)
		}
	}

	return total
}

func lineOn(lines []gl.VoucherLine, code string, side gl.EntrySide) (money.Dong, bool) {
	for _, l := range lines {
		if l.AccountCode == code && l.Side == side {
			return l.Amount, true
		}
	}

	return money.Zero, false
}

// The entries a close posts: revenue into the result account on the debit side
// (clearing its credit balance), expenses into it on the credit side, and the
// net result out to retained earnings.
func TestClosingEntries_CarriesRevenueAndExpensesToRetainedEarnings(t *testing.T) {
	lines, profit, err := closingEntries(
		[]AccountBalance{revenue("5111", 1_000_000), expense("632", 600_000)},
		TT133ClosingAccounts(),
	)
	if err != nil {
		t.Fatalf("closingEntries: %v", err)
	}

	if want := money.FromInt(400_000); profit.Cmp(want) != 0 {
		t.Errorf("profit = %s, want %s", profit, want)
	}
	if err := voucherOf(t, lines).ValidateBalance(); err != nil {
		t.Errorf("the entries are not a legal voucher: %v", err)
	}

	assertLine(t, lines, "5111", gl.SideDebit, 1_000_000) // revenue is debited out
	assertLine(t, lines, "632", gl.SideCredit, 600_000)   // expense is credited out
	assertLine(t, lines, "4212", gl.SideCredit, 400_000)

	// The result account takes the revenue in, the expenses out, and is emptied
	// of the difference — leaving it, as it must be left, at nothing.
	if got, want := totalOn(lines, "911", gl.SideCredit), money.FromInt(1_000_000); got.Cmp(want) != 0 {
		t.Errorf("the result account was credited %s, want %s", got, want)
	}
	if got, want := totalOn(lines, "911", gl.SideDebit), money.FromInt(1_000_000); got.Cmp(want) != 0 {
		t.Errorf("the result account was debited %s, want %s", got, want)
	}
	assertResultAccountEmpty(t, lines)
}

// A loss leaves the result account on the other side, and retained earnings is
// debited.
func TestClosingEntries_CarriesALossToRetainedEarnings(t *testing.T) {
	lines, profit, err := closingEntries(
		[]AccountBalance{revenue("5111", 100_000), expense("632", 250_000)},
		TT133ClosingAccounts(),
	)
	if err != nil {
		t.Fatalf("closingEntries: %v", err)
	}

	if want := money.FromInt(-150_000); profit.Cmp(want) != 0 {
		t.Errorf("profit = %s, want %s", profit, want)
	}
	if err := voucherOf(t, lines).ValidateBalance(); err != nil {
		t.Errorf("the entries are not a legal voucher: %v", err)
	}

	assertLine(t, lines, "4212", gl.SideDebit, 150_000)
	assertResultAccountEmpty(t, lines)
}

// A revenue account can carry a debit balance — under TT133 a sales deduction is
// posted to 511 itself rather than to an account of its own — and clearing it
// means crediting it, not debiting it. Clearing every revenue account the same
// way would double the deduction instead of removing it.
func TestClosingEntries_ClearsAnAccountCarryingItsBalanceTheOtherWay(t *testing.T) {
	lines, profit, err := closingEntries(
		[]AccountBalance{revenue("5111", 1_000_000), revenue("5118", -50_000), expense("632", 400_000)},
		TT133ClosingAccounts(),
	)
	if err != nil {
		t.Fatalf("closingEntries: %v", err)
	}

	if want := money.FromInt(550_000); profit.Cmp(want) != 0 {
		t.Errorf("profit = %s, want %s", profit, want)
	}
	if err := voucherOf(t, lines).ValidateBalance(); err != nil {
		t.Errorf("the entries are not a legal voucher: %v", err)
	}

	assertLine(t, lines, "5118", gl.SideCredit, 50_000)
	assertResultAccountEmpty(t, lines)
}

// assertResultAccountEmpty checks the profit determination account is left at
// nothing, which is what makes the close a close rather than a second account
// holding the period's result beside retained earnings.
func assertResultAccountEmpty(t *testing.T, lines []gl.VoucherLine) {
	t.Helper()

	net := totalOn(lines, "911", gl.SideCredit).Sub(totalOn(lines, "911", gl.SideDebit))
	if !net.IsZero() {
		t.Errorf("the result account is left at %s, want 0", net)
	}
}

// A period with nothing in it has nothing to post. An empty voucher is not a
// voucher.
func TestClosingEntries_ProducesNothingForAQuietPeriod(t *testing.T) {
	lines, profit, err := closingEntries(nil, TT133ClosingAccounts())
	if err != nil {
		t.Fatalf("closingEntries: %v", err)
	}

	if len(lines) != 0 {
		t.Errorf("lines = %+v, want none", lines)
	}
	if !profit.IsZero() {
		t.Errorf("profit = %s, want 0", profit)
	}
}

// The same books close the same way twice: a report can be compared with the one
// taken last month.
func TestClosingEntries_ProducesTheSameEntriesWhateverOrderTheBalancesArriveIn(t *testing.T) {
	accounts := TT133ClosingAccounts()

	forward, _, err := closingEntries(
		[]AccountBalance{revenue("5111", 1_000_000), expense("632", 600_000), expense("642", 100_000)},
		accounts,
	)
	if err != nil {
		t.Fatalf("closingEntries: %v", err)
	}

	backward, _, err := closingEntries(
		[]AccountBalance{expense("642", 100_000), expense("632", 600_000), revenue("5111", 1_000_000)},
		accounts,
	)
	if err != nil {
		t.Fatalf("closingEntries: %v", err)
	}

	if len(forward) != len(backward) {
		t.Fatalf("the same balances produced %d lines one way and %d the other", len(forward), len(backward))
	}
	for i := range forward {
		if !reflect.DeepEqual(forward[i], backward[i]) {
			t.Errorf("line %d = %+v, want %+v", i+1, backward[i], forward[i])
		}
	}
}

// A close clears revenue and expenses. An account of any other type in the set
// is a fault in whatever handed the balances over, and posting it would move
// capital or an asset through the profit and loss account.
func TestClosingEntries_RefusesAnAccountThatIsNotProfitAndLoss(t *testing.T) {
	_, _, err := closingEntries(
		[]AccountBalance{{AccountCode: "4212", Type: gl.AccountTypeEquity, Amount: money.FromInt(1)}},
		TT133ClosingAccounts(),
	)
	if err == nil {
		t.Fatal("a capital account was accepted into a close")
	}
}

// A balance on the profit determination account means an earlier close did not
// finish; carrying it again would determine the period's result twice.
func TestClosingEntries_RefusesTheResultAccountCarryingABalance(t *testing.T) {
	_, _, err := closingEntries(
		[]AccountBalance{revenue("5111", 1_000_000), expense("911", 10_000)},
		TT133ClosingAccounts(),
	)
	if !errors.Is(err, ErrResultAccountCarriesBalance) {
		t.Fatalf("error = %v, want ErrResultAccountCarriesBalance", err)
	}
}

func TestClosingAccounts_RefusesAnIncompleteOrSelfCancellingPair(t *testing.T) {
	cases := map[string]ClosingAccounts{
		"no result account": {Retained: "4212"},
		"no retained account": {
			Result: "911",
		},
		"both the same": {Result: "911", Retained: "911"},
	}

	for name, accounts := range cases {
		t.Run(name, func(t *testing.T) {
			if err := accounts.Validate(); err == nil {
				t.Fatal("an incomplete pair was accepted")
			}
		})
	}

	if err := TT133ClosingAccounts().Validate(); err != nil {
		t.Fatalf("the TT133 pair is refused: %v", err)
	}
}

// The closing pair must be postable in the chart the client keeps books in:
// 421 itself is a heading with two sub-accounts and cannot carry a posting,
// which is why the retained account is 4212.
func TestTT133ClosingAccounts_AreAccountsTheChartWillAcceptAPostingTo(t *testing.T) {
	chart := gl.NewChartOfAccounts()
	for _, account := range gl.TT133Chart() {
		if err := chart.Add(account); err != nil {
			t.Fatalf("seed chart with %s: %v", account.Code, err)
		}
	}

	accounts := TT133ClosingAccounts()
	for _, code := range []string{accounts.Result, accounts.Retained} {
		if !chart.IsPostable(code) {
			t.Errorf("account %s cannot be posted to", code)
		}
	}
}

func assertLine(t *testing.T, lines []gl.VoucherLine, code string, side gl.EntrySide, want int64) {
	t.Helper()

	got, found := lineOn(lines, code, side)
	if !found {
		t.Errorf("no %s line on account %s, want one of %d", side, code, want)
		return
	}
	if got.Cmp(money.FromInt(want)) != 0 {
		t.Errorf("account %s %s line = %s, want %d", code, side, got, want)
	}
}
