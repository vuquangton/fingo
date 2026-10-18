// Package ledger is the sổ chi tiết tài khoản: one account's movements in a
// period, in order, with a running balance and the voucher behind each line.
//
// It is the far end of the traceability spine. A figure on a balance sheet is a
// total; this is the list of documents that produced it, and an accountant who
// cannot get here from there is reading a printout rather than a set of books.
//
// It is a model and contains no widgets.
package ledger

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/gl"
	"fingo/internal/reports"
	"fingo/internal/ui/format"
)

// Source is what the ledger reads.
//
// Two questions, declared here where they are used. The lines carry a voucher's
// *identity* but not its number, so the number is asked for separately — and
// asked once per document rather than once per line.
type Source interface {
	Lines(ctx context.Context, from, to time.Time) ([]reports.Line, error)
	VoucherNo(ctx context.Context, id string) (string, error)
}

// Entry is one movement, as the table shows it.
type Entry struct {
	Date      time.Time
	VoucherID string
	VoucherNo string

	Debit  decimal.Decimal
	Credit decimal.Decimal

	// Running is the balance carried down to and including this movement. It
	// only means anything because the entries are in date order.
	Running decimal.Decimal
}

// Model is the screen's state. The zero value is usable and shows nothing.
type Model struct {
	source  Source
	account string

	entries []Entry
	from    time.Time
	to      time.Time

	// lookupFailed records that at least one voucher could not be read. The
	// lines are still the books; only their citation is missing, and the screen
	// says so rather than pretending the reference was never there.
	lookupFailed bool
}

// New opens a ledger for one account.
func New(source Source, account string) *Model {
	return &Model{source: source, account: account}
}

// Load reads the account's movements between two days, both inclusive.
func (m *Model) Load(ctx context.Context, from, to time.Time) error {
	lines, err := m.source.Lines(ctx, from, to)
	if err != nil {
		return fmt.Errorf("read the lines for %s: %w", m.account, err)
	}

	m.from, m.to = from, to
	m.entries = nil
	m.lookupFailed = false

	mine := make([]reports.Line, 0, len(lines))
	for _, l := range lines {
		if l.AccountCode == m.account {
			mine = append(mine, l)
		}
	}

	// By date, then by voucher, so that two movements on one day have an order
	// the running balance can be read off. Sorting only by date would leave the
	// order of a day's movements to the query, and the balance column would be
	// a different sequence every time the screen was opened.
	sort.SliceStable(mine, func(i, j int) bool {
		if !sameDay(mine[i].VoucherDate, mine[j].VoucherDate) {
			return mine[i].VoucherDate.Before(mine[j].VoucherDate)
		}

		return mine[i].VoucherID < mine[j].VoucherID
	})

	numbers := map[string]string{}

	running := decimal.Zero

	for _, l := range mine {
		number, known := numbers[l.VoucherID]
		if !known {
			// Once per document. A month's ledger repeats the same voucher on
			// many lines — both legs of it, at least — and a lookup per line
			// would be a round trip per line.
			read, err := m.source.VoucherNo(ctx, l.VoucherID)
			if err != nil {
				m.lookupFailed = true
			}

			numbers[l.VoucherID] = read
			number = read
		}

		e := Entry{
			Date:      l.VoucherDate,
			VoucherID: l.VoucherID,
			VoucherNo: number,
		}

		switch l.Side {
		case gl.SideCredit:
			e.Credit = l.Amount
			running = running.Sub(l.Amount)
		default:
			e.Debit = l.Amount
			running = running.Add(l.Amount)
		}

		e.Running = running

		m.entries = append(m.entries, e)
	}

	return nil
}

// Entries is the account's movements, in the order the ledger reads them.
func (m *Model) Entries() []Entry { return m.entries }

// Account is the account this ledger is for.
func (m *Model) Account() string { return m.account }

// Closing is the balance the period's movements leave.
//
// It is the sum of the movements read, so when the ledger was loaded from the
// start of the books it is the account's balance and when it was loaded for one
// period it is that period's movement. The screen says which.
func (m *Model) Closing() decimal.Decimal {
	if len(m.entries) == 0 {
		return decimal.Zero
	}

	return m.entries[len(m.entries)-1].Running
}

// Title is what the status bar calls this screen.
func (m *Model) Title() string { return "Sổ chi tiết tài khoản " + m.account }

// Summary states what is on screen.
func (m *Model) Summary() string {
	return fmt.Sprintf("%d dòng · %s đến %s", len(m.entries), day(m.from), day(m.to))
}

// Table is what the content area draws.
func (m *Model) Table() (headings []string, rows [][]string, empty string) {
	headings = []string{"Ngày", "Số chứng từ", "Diễn giải", "Phát sinh Nợ", "Phát sinh Có", "Lũy kế"}

	rows = make([][]string, 0, len(m.entries))

	for _, e := range m.entries {
		reference := e.VoucherNo
		if reference == "" {
			// The id is not a number an accountant would recognise, but it is
			// true, and a blank cell would say the line came from nowhere.
			reference = shortID(e.VoucherID)
		}

		rows = append(rows, []string{
			day(e.Date),
			reference,
			"",
			format.MoneyIfAny(e.Debit),
			format.MoneyIfAny(e.Credit),
			format.Money(e.Running),
		})
	}

	return headings, rows, m.Empty()
}

// Empty is what to say when there is nothing to show.
func (m *Model) Empty() string {
	if len(m.entries) > 0 {
		return ""
	}

	return fmt.Sprintf(
		"Tài khoản %s không có phát sinh nào từ %s đến %s.",
		m.account, day(m.from), day(m.to))
}

// day writes a date the way a Vietnamese accountant writes one.
func day(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.Format("02/01/2006")
}

// shortID is enough of an identifier to recognise and short enough for a column.
func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}

	return strings.ToUpper(id[:8])
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()

	return ay == by && am == bm && ad == bd
}
