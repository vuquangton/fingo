package trialbalance_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"fingo/internal/ui/trialbalance"
	"fingo/internal/usecase/practice"
)

type books struct {
	balances practice.TrialBalance
	err      error
}

func (b books) TrialBalance(context.Context) (practice.TrialBalance, error) {
	return b.balances, b.err
}

func amount(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// aBalancedBook is a small but complete set of books: it balances, and one
// account (511) has no movement on the debit side at all.
func aBalancedBook() practice.TrialBalance {
	return practice.TrialBalance{
		Rows: []practice.Balance{
			{AccountCode: "331", AccountName: "Phải trả cho người bán", Credit: amount("3000000")},
			{AccountCode: "111", AccountName: "Tiền mặt", Debit: amount("500000")},
			{AccountCode: "511", AccountName: "Doanh thu bán hàng", Credit: amount("2500000")},
			{AccountCode: "131", AccountName: "Phải thu khách hàng", Debit: amount("5000000")},
		},
		Debit:  amount("5500000"),
		Credit: amount("5500000"),
	}
}

func loaded(t *testing.T, src trialbalance.Source) *trialbalance.Model {
	t.Helper()

	m := &trialbalance.Model{}
	if err := m.Load(t.Context(), src); err != nil {
		t.Fatalf("Load: %v", err)
	}

	return m
}

// TestModel_OrdersByAccountCode — a bảng cân đối số phát sinh is read down the
// account numbers, and the numbers are the order.
func TestModel_OrdersByAccountCode(t *testing.T) {
	m := loaded(t, books{balances: aBalancedBook()})

	var codes []string
	for _, r := range m.Rows() {
		codes = append(codes, r.Code)
	}

	if got := strings.Join(codes, ","); got != "111,131,331,511" {
		t.Errorf("codes = %s, want 111,131,331,511", got)
	}
}

// TestModel_WritesMoneyTheVietnameseWay — the report's numbers go through the
// same formatter as everything else, so a figure cannot mean one thing on one
// screen and another on the next.
func TestModel_WritesMoneyTheVietnameseWay(t *testing.T) {
	m := loaded(t, books{balances: aBalancedBook()})

	byCode := map[string]trialbalance.Row{}
	for _, r := range m.Rows() {
		byCode[r.Code] = r
	}

	if got := byCode["131"].Debit; got != "5.000.000" {
		t.Errorf("131 debit = %q, want 5.000.000", got)
	}
	if got := byCode["331"].Credit; got != "3.000.000" {
		t.Errorf("331 credit = %q, want 3.000.000", got)
	}
}

// TestModel_AnUntouchedSideIsBlank — an account with no debit movement has no
// debit figure, and printing "0" in that cell says the opposite of what is true:
// it says somebody measured it and found nothing.
func TestModel_AnUntouchedSideIsBlank(t *testing.T) {
	m := loaded(t, books{balances: aBalancedBook()})

	byCode := map[string]trialbalance.Row{}
	for _, r := range m.Rows() {
		byCode[r.Code] = r
	}

	if got := byCode["511"].Debit; got != "" {
		t.Errorf("511 has no debit movement but its debit cell reads %q", got)
	}
	if got := byCode["111"].Credit; got != "" {
		t.Errorf("111 has no credit movement but its credit cell reads %q", got)
	}
}

// TestModel_SaysWhetherTheBooksBalance — the one question a trial balance is
// opened to answer, and the reason it exists.
//
// Totals that disagree mean the ledger is broken: a posting lost its other half,
// or something wrote outside a transaction. That is not a display problem and it
// must not look like one — two totals side by side, one of them slightly
// different, is a difference an accountant is expected to spot by eye.
func TestModel_SaysWhetherTheBooksBalance(t *testing.T) {
	balanced := loaded(t, books{balances: aBalancedBook()})

	if !balanced.Balanced() {
		t.Error("a book whose totals agree is reported as unbalanced")
	}
	if msg := balanced.BalanceNote(); strings.Contains(strings.ToLower(msg), "lệch") {
		t.Errorf("a balanced book reports %q", msg)
	}

	broken := aBalancedBook()
	broken.Credit = amount("5499999")

	unbalanced := loaded(t, books{balances: broken})

	if unbalanced.Balanced() {
		t.Error("a book whose totals differ by 1 đồng is reported as balanced")
	}

	note := unbalanced.BalanceNote()
	if !strings.Contains(strings.ToLower(note), "lệch") {
		t.Errorf("the note on an unbalanced book reads %q, which does not say it is out", note)
	}
	if !strings.Contains(note, "1") {
		t.Errorf("the note %q does not say by how much", note)
	}
}

// TestModel_ShowsTheTotals — the bottom line of the report, in the same
// convention as the rows.
func TestModel_ShowsTheTotals(t *testing.T) {
	m := loaded(t, books{balances: aBalancedBook()})

	debit, credit := m.Totals()

	if debit != "5.500.000" {
		t.Errorf("total debit = %q, want 5.500.000", debit)
	}
	if credit != "5.500.000" {
		t.Errorf("total credit = %q, want 5.500.000", credit)
	}
}

// TestModel_AnEmptyLedgerSaysSoRatherThanShowingATableOfNothing.
func TestModel_AnEmptyLedgerSaysSo(t *testing.T) {
	m := loaded(t, books{})

	if len(m.Rows()) != 0 {
		t.Fatalf("an empty ledger produced %d rows", len(m.Rows()))
	}

	headings, rows, empty := m.Table()

	if len(headings) == 0 {
		t.Error("an empty ledger produced no headings")
	}
	if len(rows) != 0 {
		t.Errorf("an empty ledger produced %d rows", len(rows))
	}
	if empty == "" {
		t.Error("an empty ledger produced no message")
	}
}

// TestModel_TableCarriesTheAccountCode — the traceability rule.
//
// Every figure on this report has to be openable down to the vouchers that
// produced it. The view gets the account code in the first cell, which is what
// makes that possible: a table of names and numbers with no key to the ledger
// behind it is a printout, not a book.
func TestModel_TableCarriesTheAccountCode(t *testing.T) {
	m := loaded(t, books{balances: aBalancedBook()})

	headings, rows, _ := m.Table()

	want := []string{"Mã", "Tên tài khoản", "Phát sinh Nợ", "Phát sinh Có"}
	if strings.Join(headings, "|") != strings.Join(want, "|") {
		t.Errorf("headings = %v, want %v", headings, want)
	}

	for i, r := range rows {
		if len(r) != len(headings) {
			t.Errorf("row %d has %d cells and there are %d headings", i, len(r), len(headings))
		}
		if r[0] == "" {
			t.Errorf("row %d has no account code, so it cannot be traced", i)
		}
	}
}

// TestModel_LoadReportsAFailure — a report that swallows the error and draws an
// empty page has told the accountant their ledger is empty.
func TestModel_LoadReportsAFailure(t *testing.T) {
	boom := errors.New("the server went away")

	m := &trialbalance.Model{}
	err := m.Load(t.Context(), books{err: boom})

	if err == nil {
		t.Fatal("Load = nil, want the source's error")
	}
	if !errors.Is(err, boom) {
		t.Errorf("Load = %v, want it to wrap %v", err, boom)
	}
}
