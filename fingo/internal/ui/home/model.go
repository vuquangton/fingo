// Package home is the work-cycle home: the screen an accountant opens to ask
// what is going on and what to do next.
//
// It is the one screen organised by *cadence* rather than by module. MISA's own
// flow is a calendar — khởi tạo, hàng ngày, hàng tháng/quý, hàng năm — and this
// is where that calendar becomes a page. Everything else in the application is
// where a particular piece of work is done; this is where an operator finds out
// which piece of work is due.
//
// It is a model and contains no widgets.
package home

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/period"
	"fingo/internal/reports"
	"fingo/internal/ui/format"
)

// Source is what the home reads.
//
// Two questions, declared here where they are used: which period is this, and
// what is the position of the books in it. *practice.Client answers the first
// and a reports.Service over it answers the second.
type Source interface {
	Periods(ctx context.Context) ([]period.Period, error)
	BalanceSheet(ctx context.Context, periodID string) (reports.BalanceSheet, error)
}

// Figure is one position, named and written.
type Figure struct {
	Label  string
	Amount decimal.Decimal
}

// The accounts each position is made of.
//
// A decision, not a lookup, and one that is easy to get subtly wrong: cash is
// 111 and 112, receivables are 131, payables are 331, and stock is the whole
// 15x family that is actually goods — 152 materials, 153 tools, 155 finished,
// 156 goods, 157 goods sent out. Leaving 153 in would count tools as stock;
// leaving 157 out would hide goods sold but not yet accepted.
var positionAccounts = []struct {
	label    string
	prefixes []string
}{
	{"Tiền", []string{"111", "112"}},
	{"Phải thu", []string{"131"}},
	{"Phải trả", []string{"331"}},
	{"Tồn kho", []string{"152", "153", "155", "156", "157"}},
}

// Model is the screen's state. The zero value is usable and describes a client
// whose books have not been opened.
type Model struct {
	today  time.Time
	source Source

	current period.Period
	hasOne  bool

	sheet   reports.BalanceSheet
	figures []Figure
}

// New builds the home for a day.
func New(source Source, today time.Time) *Model {
	return &Model{today: today, source: source}
}

// Load reads the periods and, if one is current, the position of the books.
func (m *Model) Load(ctx context.Context) error {
	periods, err := m.source.Periods(ctx)
	if err != nil {
		return fmt.Errorf("read the accounting periods: %w", err)
	}

	current, ok := period.Current(periods, m.today)

	m.current, m.hasOne = current, ok
	m.figures = nil
	m.sheet = reports.BalanceSheet{}

	if !ok {
		return nil
	}

	// The balance sheet is read even when the period turns out to be empty: an
	// open period whose books are still is a business that has started and done
	// nothing, which is a different thing from one whose books were never
	// opened, and it is the difference the operator's next move depends on.
	sheet, err := m.source.BalanceSheet(ctx, current.ID)
	if err != nil {
		return fmt.Errorf("read the position of the books: %w", err)
	}

	m.sheet = sheet
	m.figures = m.positions(sheet)

	return nil
}

// HasPeriod reports whether today falls inside an accounting period.
func (m *Model) HasPeriod() bool { return m.hasOne }

// Period is the current period's id, empty when there is none.
func (m *Model) Period() string {
	if !m.hasOne {
		return ""
	}

	return m.current.ID
}

// Locked reports whether the current period is closed to posting.
func (m *Model) Locked() bool { return m.hasOne && m.current.Locked }

// Figures is the four positions, in the order the screen shows them.
func (m *Model) Figures() []Figure { return m.figures }

// Note is what the screen says about the state of the books.
//
// Three states, and they are not variations on one theme: no period is "open
// your books", a locked period is "this year is closed", and an open period is
// "here is where you are". An operator who cannot tell the first two apart will
// try to post into a locked year, or wait for a report that is never coming.
func (m *Model) Note() string {
	switch {
	case !m.hasOne:
		return "Chưa mở kỳ kế toán nào. Hãy tạo kỳ trước khi nhập chứng từ."

	case m.current.Locked:
		return fmt.Sprintf("Kỳ %s đã khóa sổ. Không nhập được chứng từ vào kỳ này.", m.current.ID)

	case m.isStill():
		return fmt.Sprintf("Kỳ %s đang mở, chưa có bút toán nào.", m.current.ID)

	default:
		return fmt.Sprintf("Kỳ %s đang mở.", m.current.ID)
	}
}

// Summary is the one-line state for the status bar.
func (m *Model) Summary() string {
	if !m.hasOne {
		return "chưa mở kỳ kế toán"
	}

	state := "đang mở"
	if m.current.Locked {
		state = "đã khóa sổ"
	}

	return fmt.Sprintf("kỳ %s · %s", m.current.ID, state)
}

// Table is what the content area draws: the labels, the figures written the
// Vietnamese way, and the sentence to show when there is nothing.
func (m *Model) Table() (headings []string, rows [][]string, empty string) {
	rows = make([][]string, 0, len(m.figures))

	for _, f := range m.figures {
		rows = append(rows, []string{f.Label, format.Money(f.Amount)})
	}

	return []string{"Chỉ tiêu", "Số tiền"}, rows, m.Empty()
}

// Empty is what to say when there are no figures.
func (m *Model) Empty() string {
	if len(m.figures) > 0 {
		return ""
	}

	return m.Note()
}

// Title is what the status bar calls this screen.
func (m *Model) Title() string { return "Hôm nay" }

// isStill reports whether the period holds no movement at all.
func (m *Model) isStill() bool {
	for _, f := range m.figures {
		if !f.Amount.IsZero() {
			return false
		}
	}

	return true
}

// positions totals the accounts each figure is made of.
//
// Assets carry their amount in the account's own sense, so cash and receivables
// add up directly. Payables live on the liability side for the same reason.
func (m *Model) positions(sheet reports.BalanceSheet) []Figure {
	out := make([]Figure, 0, len(positionAccounts))

	for _, p := range positionAccounts {
		total := decimal.Zero

		for _, row := range sheet.Assets {
			if hasPrefix(row.AccountCode, p.prefixes) {
				total = total.Add(row.Amount)
			}
		}
		for _, row := range sheet.Liabilities {
			if hasPrefix(row.AccountCode, p.prefixes) {
				total = total.Add(row.Amount)
			}
		}

		out = append(out, Figure{Label: p.label, Amount: total})
	}

	return out
}

func hasPrefix(code string, prefixes []string) bool {
	for _, p := range prefixes {
		if len(code) >= len(p) && code[:len(p)] == p {
			return true
		}
	}

	return false
}
