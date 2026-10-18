// Package reports reads a client's books.
//
// Every figure here is summed from posted voucher lines — the ledger as it
// stands when the report is asked for. Nothing is read from a stored total: a
// balance table is a second answer that can drift from the lines it was
// computed from, and when the two disagree nothing can say which is right. The
// only thing these reports reach storage for is lines, through Journal.
//
// # Traceability
//
// Every row says how many distinct vouchers it was summed from, and a row is
// emitted only for an account that at least one voucher line touched. A figure
// nobody can point at a voucher for is a figure that cannot be defended to an
// auditor, so the reports do not state one: a chart account nobody has posted
// to has no row, rather than a row of zeroes with nothing behind it.
//
// # What each report reads
//
//   - Trial balance: everything up to the period's end, so an account shows what
//     it carried in, what the period did to it, and what it stands at.
//   - Profit and loss: the period's own lines, with the closing entries left
//     out. See ProfitAndLoss.
//   - Balance sheet: everything up to the period's end, because a balance sheet
//     is a position at an instant and not a period's movement.
package reports

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/close"
	"fingo/internal/domain/gl"
	"fingo/internal/period"
	"fingo/internal/usecase/ledger"
)

// ErrUnknownAccount is a figure the reports cannot place: the ledger holds a
// line on an account the chart does not, or the account is one a report cannot
// state — the account a close carries the result through, when the chart no
// longer holds it, or an account of a kind the balance sheet has no section for.
//
// Refused rather than skipped. An account the reports cannot find a type for is
// an account that would vanish from the balance sheet — a balance silently
// missing, with the identity it should have satisfied quietly failing to.
var ErrUnknownAccount = errors.New("reports: the ledger holds an account the chart does not")

// ErrUnusableLine is a line the reports cannot place: one that is neither a
// debit nor a credit, or one that moves a negative amount.
//
// Refused rather than placed, and refused rather than dropped: a report that
// guessed a side would state a figure under a column nothing supports, one that
// treated a negative debit as a credit would put it under the wrong column, and
// one that dropped the line would be a little out and would still say it
// balanced.
var ErrUnusableLine = errors.New("reports: a line is not a movement the ledger could have posted")

// ErrInvalidService is a report service built without what it needs to read the
// books.
var ErrInvalidService = errors.New("reports: incomplete service")

// ClosingAccounts names the accounts a close carries a period's result through.
//
// It is the close's own type, aliased rather than restated: a second definition
// of "the result account is 911" is a second answer, and the two would disagree
// the first time a client moved the account.
type ClosingAccounts = close.ClosingAccounts

// Periods supplies the client's accounting periods.
//
// period.Repository satisfies it; the port is narrower because a report reads
// the calendar and never writes to it.
type Periods interface {
	// List returns every period, so the one a report is asked for can be found.
	List(ctx context.Context) ([]period.Period, error)
}

// Line is one posted voucher line.
//
// It carries the voucher it came from, because traceability is a claim about
// which voucher a figure came out of and not merely that some voucher exists:
// the id is what a reader follows back.
type Line struct {
	VoucherID   string
	VoucherDate time.Time
	AccountCode string
	Side        gl.EntrySide
	Amount      decimal.Decimal
}

// Journal is the ledger, read as lines.
//
// Declared here, where it is used: the reports need the movements themselves and
// must not be handed a total somebody else summed — the summing is the report.
type Journal interface {
	// Lines returns the posted voucher lines between two dates.
	//
	// Both ends are inclusive and compared by calendar day. A zero from means
	// the beginning of the books, which is what a balance sheet asks for.
	// Drafts are not lines: only what was posted is in the books.
	Lines(ctx context.Context, from, to time.Time) ([]Line, error)
}

// Service reads a client's books.
type Service struct {
	periods Periods
	journal Journal
	chart   ledger.ChartSource
	closing ClosingAccounts
}

// NewService builds the reports.
//
// The chart is injected because a report has to know what kind of account each
// account is before it can put a figure under assets or under revenue, and the
// closing accounts because a profit and loss report has to tell a closing entry
// from a trading one.
func NewService(periods Periods, journal Journal, chart ledger.ChartSource, closing ClosingAccounts) (*Service, error) {
	switch {
	case periods == nil:
		return nil, fmt.Errorf("%w: reports need the client's accounting periods", ErrInvalidService)
	case journal == nil:
		return nil, fmt.Errorf("%w: reports need the ledger to read lines from", ErrInvalidService)
	case chart == nil:
		return nil, fmt.Errorf("%w: reports need the chart of accounts", ErrInvalidService)
	}
	if err := closing.Validate(); err != nil {
		return nil, err
	}

	return &Service{periods: periods, journal: journal, chart: chart, closing: closing}, nil
}

// realm returns the period a report is asked for, and the chart its accounts are
// classified by.
//
// The closing accounts are checked against the chart here, once, for every
// report. The profit and loss report tells a closing entry from a trading one by
// the result account, and a result account the chart does not hold is a close
// the report would not recognise: the closing entries would stay in, cancel the
// period's revenue and expenses, and the report would state a profit of nothing
// — with no error, which is the worst way to be wrong.
func (s *Service) realm(ctx context.Context, periodID string) (period.Period, *gl.ChartOfAccounts, error) {
	periods, err := s.periods.List(ctx)
	if err != nil {
		return period.Period{}, nil, fmt.Errorf("load accounting periods: %w", err)
	}

	wanted, err := find(periods, periodID)
	if err != nil {
		return period.Period{}, nil, err
	}
	if err := wanted.Validate(); err != nil {
		return period.Period{}, nil, err
	}

	chart, err := s.chart.Chart(ctx)
	if err != nil {
		return period.Period{}, nil, fmt.Errorf("load the chart of accounts: %w", err)
	}
	if chart == nil {
		return period.Period{}, nil, fmt.Errorf("%w: the chart of accounts is empty", ErrInvalidService)
	}

	if _, known := chart.ByCode(s.closing.Result); !known {
		return period.Period{}, nil, fmt.Errorf(
			"%w: %s, the account a close carries the period's result through, so the report could not tell a closing entry from a trading one",
			ErrUnknownAccount, s.closing.Result)
	}

	return wanted, chart, nil
}

// find returns the period with an id, or the period package's error saying no
// period has it.
func find(periods []period.Period, id string) (period.Period, error) {
	for _, p := range periods {
		if p.ID == id {
			return p, nil
		}
	}

	return period.Period{}, fmt.Errorf("%w: %s", period.ErrUnknownPeriod, id)
}

// movement is what one account carried into a period, what the period did to it,
// and which vouchers did either.
type movement struct {
	openingDebit, openingCredit decimal.Decimal
	periodDebit, periodCredit   decimal.Decimal

	// carried is every voucher that put anything on this account, and
	// periodCarried the ones that did it inside the period. A row whose figure
	// is a position counts the first; a row whose figure is the period's own
	// movement counts the second, because a voucher from an earlier period is
	// not behind a figure this period made.
	carried       map[string]bool
	periodCarried map[string]bool
}

// movements sums lines by account, splitting each account into what it carried
// into the period and what the period did to it.
//
// Vouchers in excluded are left out entirely — every line of them, not only the
// lines on the account that caused the exclusion. A closing entry is one entry:
// half of it is not a movement of the period.
//
// A zero from means the whole of the books, which lands every line in the period
// bucket and leaves the opening one empty. That is what a balance sheet wants:
// the position at a date, not the period's part of it.
//
// A line that is neither a debit nor a credit stops the report. The alternative
// is to invent a side for it, and a figure in a column nothing put it in is
// exactly what a ledger exists to rule out.
func movements(lines []Line, from, to time.Time, excluded map[string]bool) (map[string]*movement, error) {
	out := make(map[string]*movement)

	start, end := dayOf(from), dayOf(to)
	before := !from.IsZero()

	for _, line := range lines {
		if excluded[line.VoucherID] {
			continue
		}

		// Out of the window is out of the report, and a line the report does not
		// read is not a line it can refuse.
		date := dayOf(line.VoucherDate)
		if date.After(end) {
			continue
		}

		// A line nobody can follow back is not traceable, and one voucher id
		// shared by five vouchers would report five as one.
		if line.VoucherID == "" {
			return nil, fmt.Errorf("%w: a line on account %s came from no voucher",
				ErrUnusableLine, line.AccountCode)
		}
		if line.Side != gl.SideDebit && line.Side != gl.SideCredit {
			return nil, fmt.Errorf("%w: voucher %s carries %s on account %s",
				ErrUnusableLine, line.VoucherID, line.Side, line.AccountCode)
		}
		if line.Amount.IsNegative() {
			return nil, fmt.Errorf("%w: voucher %s moves %s on account %s, and a negative amount is the other side written in the wrong place",
				ErrUnusableLine, line.VoucherID, line.Amount, line.AccountCode)
		}

		// A line that moves nothing moves nothing: it is not a voucher behind a
		// figure, and counting it as one would overstate the trace.
		if line.Amount.IsZero() {
			continue
		}

		m := out[line.AccountCode]
		if m == nil {
			m = &movement{carried: make(map[string]bool), periodCarried: make(map[string]bool)}
			out[line.AccountCode] = m
		}

		m.add(line, before && date.Before(start))
	}

	return out, nil
}

// add puts one line into the bucket it belongs to. The side and the amount have
// already been checked, so a debit is a debit and everything else is a credit.
func (m *movement) add(line Line, opening bool) {
	debit := line.Side == gl.SideDebit

	switch {
	case opening && debit:
		m.openingDebit = m.openingDebit.Add(line.Amount)
	case opening:
		m.openingCredit = m.openingCredit.Add(line.Amount)
	case debit:
		m.periodDebit = m.periodDebit.Add(line.Amount)
		m.periodCarried[line.VoucherID] = true
	default:
		m.periodCredit = m.periodCredit.Add(line.Amount)
		m.periodCarried[line.VoucherID] = true
	}

	m.carried[line.VoucherID] = true
}

// closingVouchers returns the vouchers that carry a period's result into equity:
// the ones with a line on the profit determination account.
//
// That account is only ever touched by a close, which is what makes it the mark
// of one. A voucher that mentions it is a closing entry through and through, so
// the whole voucher is the entry and the whole voucher is left out of a profit
// and loss report.
func closingVouchers(lines []Line, result string) map[string]bool {
	out := make(map[string]bool)

	for _, line := range lines {
		// A line that moved nothing did not make a voucher a closing one, for
		// the same reason it is not a voucher behind a figure.
		if line.AccountCode == result && !line.Amount.IsZero() {
			out[line.VoucherID] = true
		}
	}

	return out
}

// accountOf returns the chart account a code names, or ErrUnknownAccount.
func accountOf(chart *gl.ChartOfAccounts, code string) (gl.Account, error) {
	account, known := chart.ByCode(code)
	if !known {
		return gl.Account{}, fmt.Errorf("%w: %s", ErrUnknownAccount, code)
	}

	return account, nil
}

// groupOf returns the heading an account reports under: the account above it, or
// the account itself when nothing is above it.
//
// This is what "by account group" means here — TT133 keeps 5111 under 511, and a
// code with no parent is its own heading. A group whose own account is missing
// from the chart falls back to the account being grouped, because a report that
// refused to place a figure it can otherwise explain would be worse than one
// that places it under its own name.
func groupOf(chart *gl.ChartOfAccounts, account gl.Account) gl.Account {
	if account.ParentCode == "" {
		return account
	}
	if parent, known := chart.ByCode(account.ParentCode); known {
		return parent
	}

	return account
}

// sortedCodes returns the accounts a set of movements holds, in a stable order,
// so that two runs over the same books produce the same report.
func sortedCodes(found map[string]*movement) []string {
	out := make([]string, 0, len(found))
	for code := range found {
		out = append(out, code)
	}

	sort.Strings(out)

	return out
}

// netted puts an account's two sides onto the one it stands on.
//
// A trial balance shows a balance, and a balance is on one side: an account that
// took 100 in and paid 40 out stands at 60 debit, not at 100 debit and 40
// credit. The columns that keep both sides are the period's own movements, which
// are a turnover and not a position.
func netted(debit, credit decimal.Decimal) (decimal.Decimal, decimal.Decimal) {
	net := debit.Sub(credit)
	if net.IsNegative() {
		return decimal.Zero, net.Neg()
	}

	return net, decimal.Zero
}

// dayOf drops the time of day, in UTC.
//
// A period is a range of dates and the ledger records instants in UTC, exactly
// as internal/period holds it. Comparing instants would put a voucher posted at
// 23:00 on the closing date into the next period.
func dayOf(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}

	u := t.UTC()

	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}
