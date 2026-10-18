// Package journal is the sổ nhật ký chung: every document in a period, one row
// each, in the order they were written.
//
// It is the book of first entry — the list an accountant reads to answer "what
// happened this month" — and, since every other book is a rearrangement of the
// same lines, it is also the plainest view of the ledger there is.
//
// It is a model and contains no widgets.
package journal

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

// Source is what the journal reads.
//
// The lines carry a document's identity but not its number, so the number is
// asked for separately — once per document, not once per line.
type Source interface {
	Lines(ctx context.Context, from, to time.Time) ([]reports.Line, error)
	VoucherNo(ctx context.Context, id string) (string, error)
}

// Entry is one document.
type Entry struct {
	Date      time.Time
	VoucherID string
	VoucherNo string

	// Lines is how many movements the document carries. A short journal entry
	// and a long one look the same in a total, and an accountant scanning a
	// month wants to see which is which.
	Lines int

	// Total is the document's amount: the sum of one side of it.
	Total decimal.Decimal

	// Balanced is whether its two sides agree. It should always be true — the
	// ledger refuses anything else — and it is checked because a journal that
	// reported an impossible document as ordinary would be reporting a book that
	// cannot exist.
	Balanced bool
}

// Model is the screen's state. The zero value is usable.
type Model struct {
	source Source

	entries []Entry
	from    time.Time
	to      time.Time
}

// New opens a journal.
func New(source Source) *Model { return &Model{source: source} }

// Load reads the documents between two days, both inclusive.
func (m *Model) Load(ctx context.Context, from, to time.Time) error {
	lines, err := m.source.Lines(ctx, from, to)
	if err != nil {
		return fmt.Errorf("read the journal: %w", err)
	}

	m.from, m.to = from, to
	m.entries = nil

	// Grouped by document. The lines arrive in whatever order the query chose,
	// so the groups are built first and ordered afterwards.
	type totals struct {
		date   time.Time
		lines  int
		debit  decimal.Decimal
		credit decimal.Decimal
	}

	grouped := map[string]*totals{}
	order := make([]string, 0, len(lines))

	for _, l := range lines {
		t, ok := grouped[l.VoucherID]
		if !ok {
			t = &totals{date: l.VoucherDate}
			grouped[l.VoucherID] = t
			order = append(order, l.VoucherID)
		}

		t.lines++

		if l.Side == gl.SideCredit {
			t.credit = t.credit.Add(l.Amount)
		} else {
			t.debit = t.debit.Add(l.Amount)
		}
	}

	numbers := map[string]string{}
	entries := make([]Entry, 0, len(order))

	for _, id := range order {
		t := grouped[id]

		number, known := numbers[id]
		if !known {
			// Once per document. The number is not on the line, so it has to be
			// asked for — and a voucher with four lines would otherwise be four
			// round trips to read one number.
			read, err := m.source.VoucherNo(ctx, id)
			if err != nil {
				read = ""
			}

			numbers[id] = read
			number = read
		}

		entries = append(entries, Entry{
			Date:      t.date,
			VoucherID: id,
			VoucherNo: number,
			Lines:     t.lines,

			// One side, not both. Summing every line would double every figure
			// in the journal.
			Total:    t.debit,
			Balanced: t.debit.Equal(t.credit),
		})
	}

	// By date, then by number, then by id. Two documents on one day need an
	// order that does not change between openings, and the number is the order
	// an accountant reads them in.
	sort.SliceStable(entries, func(i, j int) bool {
		if !sameDay(entries[i].Date, entries[j].Date) {
			return entries[i].Date.Before(entries[j].Date)
		}
		if entries[i].VoucherNo != entries[j].VoucherNo {
			return entries[i].VoucherNo < entries[j].VoucherNo
		}

		return entries[i].VoucherID < entries[j].VoucherID
	})

	m.entries = entries

	return nil
}

// Entries is every document, in the order the journal reads them.
func (m *Model) Entries() []Entry { return m.entries }

// Unbalanced is the documents whose two sides do not agree.
//
// It should always be empty. It is not a warning to be dismissed when it is not:
// a voucher with one side missing is a ledger that cannot be reconciled, and the
// only honest thing to do with one is to say so where somebody will see it.
func (m *Model) Unbalanced() []Entry {
	var out []Entry

	for _, e := range m.entries {
		if !e.Balanced {
			out = append(out, e)
		}
	}

	return out
}

// Note is what the screen says about the journal as a whole.
func (m *Model) Note() string {
	if len(m.entries) == 0 {
		return fmt.Sprintf("Kỳ này chưa ghi chứng từ nào, từ %s đến %s.", day(m.from), day(m.to))
	}

	if out := m.Unbalanced(); len(out) > 0 {
		numbers := make([]string, 0, len(out))
		for _, e := range out {
			numbers = append(numbers, reference(e))
		}

		return fmt.Sprintf(
			"CẢNH BÁO: %d chứng từ không cân — %s. Sổ sách đã bị ghi ngoài cổng kiểm tra.",
			len(out), strings.Join(numbers, ", "))
	}

	return fmt.Sprintf("%d chứng từ, từ %s đến %s.", len(m.entries), day(m.from), day(m.to))
}

// Title is what the status bar calls this screen.
func (m *Model) Title() string { return "Sổ nhật ký chung" }

// Summary is the one-line state for the status bar.
func (m *Model) Summary() string {
	return fmt.Sprintf("%d chứng từ", len(m.entries))
}

// Table is what the content area draws.
func (m *Model) Table() (headings []string, rows [][]string, empty string) {
	headings = []string{"Ngày", "Số chứng từ", "Số dòng", "Số tiền"}

	rows = make([][]string, 0, len(m.entries))

	for _, e := range m.entries {
		amount := format.Money(e.Total)
		if !e.Balanced {
			// A document that cannot exist is marked where it is read, not in a
			// footnote.
			amount += "  ⚠ không cân"
		}

		rows = append(rows, []string{
			day(e.Date),
			reference(e),
			fmt.Sprintf("%d", e.Lines),
			amount,
		})
	}

	return headings, rows, m.Empty()
}

// Empty is what to say when there is nothing to show.
func (m *Model) Empty() string {
	if len(m.entries) > 0 {
		return ""
	}

	return m.Note()
}

// reference is how a document is cited: its number, or its id when the number
// could not be read. A blank cell would say the document came from nowhere.
func reference(e Entry) string {
	if e.VoucherNo != "" {
		return e.VoucherNo
	}

	if len(e.VoucherID) > 8 {
		return strings.ToUpper(e.VoucherID[:8])
	}

	return strings.ToUpper(e.VoucherID)
}

func day(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.Format("02/01/2006")
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()

	return ay == by && am == bm && ad == bd
}
