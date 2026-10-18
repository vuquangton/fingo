// Package trialbalance is the bảng cân đối số phát sinh.
//
// It is the report an accountant opens to ask one question — does the ledger
// balance? — and it is also the index to everything else: every other report is
// a rearrangement of the figures in its rows.
//
// Like the other screens, it is a model and it contains no widgets. The ordering,
// the formatting and the wording are decided here and asserted in tests; drawing
// them belongs to the shell.
package trialbalance

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"fingo/internal/ui/format"
	"fingo/internal/usecase/practice"
)

// Source is the part of the books this screen reads.
type Source interface {
	TrialBalance(ctx context.Context) (practice.TrialBalance, error)
}

// Row is one line of the report.
//
// Debit and Credit arrive already written, as the operator will read them. The
// account code stays separate and unformatted: it is not a number, it is a name,
// and it is the key to the ledger behind the figure.
type Row struct {
	Code   string
	Name   string
	Debit  string
	Credit string
}

var headings = []string{"Mã", "Tên tài khoản", "Phát sinh Nợ", "Phát sinh Có"}

// Model is the screen's state. The zero value is usable.
type Model struct {
	rows       []Row
	debit      string
	credit     string
	difference string
	balanced   bool
	loaded     bool
}

// Load reads the trial balance.
func (m *Model) Load(ctx context.Context, src Source) error {
	balances, err := src.TrialBalance(ctx)
	if err != nil {
		return fmt.Errorf("read the trial balance: %w", err)
	}

	m.rows = make([]Row, 0, len(balances.Rows))
	for _, b := range balances.Rows {
		m.rows = append(m.rows, Row{
			Code: b.AccountCode,
			Name: b.AccountName,

			// MoneyIfAny, not Money: an account with no movement on one side has
			// no figure there. A printed "0" claims somebody measured it and
			// found nothing, which is a different statement.
			Debit:  format.MoneyIfAny(b.Debit),
			Credit: format.MoneyIfAny(b.Credit),
		})
	}

	sort.Slice(m.rows, func(i, j int) bool { return m.rows[i].Code < m.rows[j].Code })

	m.debit = format.Money(balances.Debit)
	m.credit = format.Money(balances.Credit)
	m.balanced = balances.Debit.Equal(balances.Credit)
	m.difference = format.Money(balances.Debit.Sub(balances.Credit).Abs())
	m.loaded = true

	return nil
}

// Rows is what the report shows, in the order it shows it.
func (m *Model) Rows() []Row { return m.rows }

// Totals is the bottom line, already written.
func (m *Model) Totals() (debit, credit string) { return m.debit, m.credit }

// Balanced reports whether total debits equal total credits.
func (m *Model) Balanced() bool { return m.balanced }

// BalanceNote says whether the books balance, and by how much when they do not.
//
// Totals that disagree are not a display problem. They mean a posting lost its
// other half, or something wrote outside a transaction — the ledger itself is
// broken, and every figure on every report derived from it is suspect. Two
// totals side by side, differing by a đồng, is a difference an accountant is
// expected to notice by eye, and that is not a thing to be expected of anybody.
func (m *Model) BalanceNote() string {
	if !m.loaded {
		return ""
	}

	if m.balanced {
		return "Sổ cân: tổng phát sinh Nợ bằng tổng phát sinh Có."
	}

	return fmt.Sprintf(
		"SỔ KHÔNG CÂN: lệch %s đồng. Bút toán mất một vế, hoặc có gì đó ghi ngoài giao dịch. "+
			"Mọi báo cáo lấy từ sổ này đều đáng ngờ cho tới khi tìm ra.",
		m.difference)
}

// Summary states what is on screen.
func (m *Model) Summary() string {
	return fmt.Sprintf("%d tài khoản có phát sinh", len(m.rows))
}

// Table is what the content area draws.
//
// The account code is the first cell of every row on purpose: the view needs a
// key to open the ledger behind the figure, and a table of names and numbers
// with no way back to the vouchers is a printout rather than a book.
func (m *Model) Table() (headings_ []string, rows [][]string, empty string) {
	rows = make([][]string, 0, len(m.rows))

	for _, r := range m.rows {
		rows = append(rows, []string{r.Code, r.Name, r.Debit, r.Credit})
	}

	return headings, rows, m.Empty()
}

// Empty is what to say when there is nothing to show.
func (m *Model) Empty() string {
	if len(m.rows) == 0 {
		return "Kỳ này chưa ghi bút toán nào, nên chưa có số phát sinh."
	}

	return ""
}

// Title is what the status bar calls this screen.
func (m *Model) Title() string { return "Bảng cân đối số phát sinh" }

// A compile-time check that a report heading is never blank: a column with no
// name is a column nobody can read.
var _ = strings.TrimSpace(headings[0])
