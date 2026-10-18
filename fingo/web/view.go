// Package web is the admin UI: server-rendered pages over a Go HTTP API.
//
// # Why it is built the way it is
//
// Every page is rendered by the server from a template. There is no client-side
// state, no store and no client router: a form submits, the server does the work
// and answers with the page as it now stands. That is not only a style choice —
// the deployment is a Wails desktop application that may have no network and no
// build step, and the figures it shows are accounting figures that must be
// right on the first render.
//
// HTMX is loaded as an enhancement, never as a dependency. Without JavaScript
// every form still posts and every page still renders; with it, the same
// responses are swapped into the page instead of replacing it. The one thing
// JavaScript is asked for is the idempotency key, and the server fills in a
// usable one first, so a browser with scripts disabled loses nothing but the
// randomness.
//
// # Errors
//
// A handler writes the error the books returned, as it returned it. There is no
// rewriting, no code-to-message table and no "Something went wrong": an operator
// reading "document d-1 debits 100.000 and credits 90.000" can act on it, and one
// reading "invalid input" cannot. The text is HTML-escaped by html/template,
// which is about the page's safety and not about the wording.
package web

import (
	"strconv"
	"strings"
)

// Amount is one figure on a page.
//
// Units and Text are both carried because there are two audiences. A reader
// wants "1.234.567"; a test asserting that a trial balance balances wants the
// number, not a string it would have to un-format. The page renders Text and
// carries Units in a data attribute, so the number a reader sees and the number
// a test reads are the same number.
type Amount struct {
	// Units is whole đồng. VND has no subunit, so there is no minor unit to
	// divide by and no rounding anywhere on this path.
	Units int64

	// Text is Units as a reader expects to see it: dot-grouped thousands, the
	// Vietnamese convention, with a leading minus for a credit balance.
	Text string
}

// NewAmount renders whole đồng.
func NewAmount(units int64) Amount {
	return Amount{Units: units, Text: group(units)}
}

// IsZero reports whether the figure moves nothing.
func (a Amount) IsZero() bool { return a.Units == 0 }

// String makes an Amount print as its text, so a template that forgets .Text
// still renders something a person can read.
func (a Amount) String() string { return a.Text }

// group writes an integer with dots between thousands, the way Vietnamese
// accounting reads a figure: 1234567 is 1.234.567.
func group(units int64) string {
	digits := strconv.FormatInt(units, 10)

	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}

	var b strings.Builder
	b.Grow(len(digits) + len(digits)/3 + 1)
	b.WriteString(sign)

	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}

		b.WriteRune(r)
	}

	return b.String()
}

// PeriodRow is one accounting period as a page lists it.
type PeriodRow struct {
	ID     string
	From   string
	To     string
	Label  string
	Locked bool
	// Current marks the period a posting today would land in. The dashboard
	// leads with it, because "which period am I in?" is the first question a
	// bookkeeper asks and the last one they want to work out from dates.
	Current bool
}

// AccountRow is one account of the chart.
type AccountRow struct {
	Code     string
	Name     string
	Parent   string
	Type     string
	Nature   string
	Active   bool
	Postable bool
	// Depth indents the row under its heading, so the chart reads as the
	// hierarchy it is rather than as a flat list of numbers.
	Depth int
}

// AccountBalance is one account and what it stands at.
type AccountBalance struct {
	Code   string
	Name   string
	Amount Amount
}

// DocumentRow is one document in a list.
type DocumentRow struct {
	ID          string
	Type        string
	Partner     string
	Date        string
	Total       Amount
	Allocated   Amount
	Outstanding Amount
	Status      string
}

// DocumentLineRow is one line of a document.
type DocumentLineRow struct {
	Account string
	Debit   Amount
	Credit  Amount
}

// AllocationRow is one payment applied to a document.
type AllocationRow struct {
	Payment string
	Date    string
	Amount  Amount
}

// DocumentPage is one document in full.
type DocumentPage struct {
	DocumentRow
	Description string
	VATRate     string
	Lines       []DocumentLineRow
	Allocations []AllocationRow
	// Payable reports whether this document can still take a payment. A paid
	// or void one cannot, and the page shows no form for it.
	Payable bool
}

// Dashboard is the landing page.
//
// It is a summary, not a report. The position figures answer "how are the
// books" at a glance, the work list answers "what next", and anything a reader
// wants to check is one link away rather than a table further down.
type Dashboard struct {
	Period     PeriodRow
	HasPeriod  bool
	Open       []DocumentRow
	Cash       []AccountBalance
	CashTotal  Amount
	Receivable Amount

	// Payable is what the business owes its suppliers (TK 331), the counterpart
	// of Receivable. Both are read off the same trial balance the report shows,
	// so the dashboard and the balance sheet cannot disagree about either.
	Payable Amount

	// Profit is the result to date, read off the profit and loss report rather
	// than summed again here, for the same reason.
	Profit Amount

	// Attention is the work the books themselves can prove is outstanding.
	//
	// A row is only ever a fact already in hand. Overdue invoices, unmatched
	// receipts and unposted drafts are not modelled by this ledger yet, so they
	// are absent rather than shown as a plausible figure nobody computed.
	Attention []AttentionRow

	// Recent is the newest documents, so the page ends where the work left off.
	Recent []DocumentRow

	// Balanced is the trial balance's own check, shown on the dashboard
	// because it is the one number that says whether the books hold together.
	Balanced bool
	Debits   Amount
	Credits  Amount
}

// AttentionRow is one thing the books say needs doing.
//
// Every row names a problem and offers the place to go about it, because a work
// list with no next step is a report, and the operator already has those.
type AttentionRow struct {
	// Count is how many of them there are.
	Count int
	// What names the problem in a few words.
	What string
	// Why carries the figure or the reason, so the row can be checked.
	Why string
	// Action is the verb on the link. Empty renders no link.
	Action string
	// Href is where the action goes.
	Href string
}

// TrialBalanceRow is one account's position.
//
// The closing columns keep both sides rather than one netted figure, because a
// trial balance is checked by adding each column up and comparing the two — a
// report that netted them first could not be checked at all.
type TrialBalanceRow struct {
	Code          string
	Name          string
	Debit         Amount
	Credit        Amount
	ClosingDebit  Amount
	ClosingCredit Amount
	Vouchers      int
}

// TrialBalancePage is the trial balance.
//
// Both pairs of totals are carried and shown. The period's two sides are what
// went into the books this period; the closing two are the position the books
// stand at, and a trial balance is only as trustworthy as the second pair.
type TrialBalancePage struct {
	Period         PeriodRow
	Rows           []TrialBalanceRow
	Debits         Amount
	Credits        Amount
	ClosingDebits  Amount
	ClosingCredits Amount
	Balanced       bool
}

// ProfitAndLossRow is one line of the profit and loss report.
type ProfitAndLossRow struct {
	Code     string
	Name     string
	Amount   Amount
	Vouchers int
	// Heading indents a group's total under the accounts it groups.
	Heading bool
}

// ProfitAndLossPage is the profit and loss report.
type ProfitAndLossPage struct {
	Period        PeriodRow
	Revenue       []ProfitAndLossRow
	Expenses      []ProfitAndLossRow
	TotalRevenue  Amount
	TotalExpenses Amount
	Profit        Amount
}

// BalanceSheetRow is one line of the balance sheet.
type BalanceSheetRow struct {
	Code     string
	Name     string
	Amount   Amount
	Vouchers int
	Heading  bool
}

// BalanceSheetPage is the balance sheet.
type BalanceSheetPage struct {
	Period           PeriodRow
	Assets           []BalanceSheetRow
	Liabilities      []BalanceSheetRow
	Equity           []BalanceSheetRow
	TotalAssets      Amount
	TotalLiabilities Amount
	TotalEquity      Amount
	Balanced         bool
}

// VATRateRow is one VAT rate as a page lists it.
type VATRateRow struct {
	ID   string
	Rate string
	From string
	To   string
	// Current reports whether the rate's window includes today.
	Current bool
}

// PostingRuleRow is one posting rule.
type PostingRuleRow struct {
	Document string
	Side     string
	Account  string
	Posts    string
}

// SettingsPage is the settings screen: periods, VAT rates and posting rules.
type SettingsPage struct {
	Periods      []PeriodRow
	VATRates     []VATRateRow
	PostingRules []PostingRuleRow
	Today        string
}
