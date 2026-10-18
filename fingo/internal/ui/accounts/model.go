// Package accounts is the chart-of-accounts screen.
//
// It is a presentation model and it contains no widgets. The screen's rules —
// what order accounts are shown in, what a filter matches, what to say when
// there is nothing to show — are decisions about the product, and they are
// asserted in tests. Drawing them is the shell's job.
//
// Source is declared here, in the consumer, rather than next to the database:
// this screen reads one thing, and a port shaped like the whole engine would
// make the screen's dependency look larger than it is. *practice.Client
// satisfies it as it stands, with no adapter.
package accounts

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"fingo/internal/domain/gl"
	"fingo/internal/ui/textfold"
)

// Source is the part of the books this screen reads.
type Source interface {
	Accounts(ctx context.Context) ([]gl.Account, error)
}

// Row is one account as the table shows it.
type Row struct {
	Code string
	Name string

	// Kind and Nature are the accountant's words, not the enum's. "Tài sản" and
	// "Dư Có" are what a column headed "Loại" and "Tính chất" needs; ASSET and
	// CREDIT are what the domain needs, and they are not the same thing.
	Kind   string
	Nature string
}

// The labels. Kept here rather than derived, because they are the operator's
// vocabulary and somebody has to choose them.
var kindLabels = map[gl.AccountType]string{
	gl.AccountTypeAsset:     "Tài sản",
	gl.AccountTypeLiability: "Nợ phải trả",
	gl.AccountTypeEquity:    "Vốn chủ sở hữu",
	gl.AccountTypeRevenue:   "Doanh thu",
	gl.AccountTypeExpense:   "Chi phí",
}

var natureLabels = map[gl.AccountNature]string{
	gl.NatureDebit:  "Dư Nợ",
	gl.NatureCredit: "Dư Có",
	gl.NatureDual:   "Lưỡng tính",
}

// Model is the screen's state.
//
// The zero value is usable: a screen that has not loaded yet shows nothing and
// says so, which is the honest state for a screen whose books have not arrived.
type Model struct {
	all   []Row
	query string
	shown []Row
}

// Load reads the chart and shows all of it.
func (m *Model) Load(ctx context.Context, src Source) error {
	accounts, err := src.Accounts(ctx)
	if err != nil {
		return fmt.Errorf("read the chart of accounts: %w", err)
	}

	m.all = make([]Row, 0, len(accounts))
	for _, a := range accounts {
		m.all = append(m.all, Row{
			Code:   a.Code,
			Name:   a.Name,
			Kind:   kindLabels[a.Type],
			Nature: natureLabels[a.Nature],
		})
	}

	// By code, as strings. "111" before "1121" before "131" is the order a chart
	// of accounts has, and it is the order an accountant reads it in.
	sort.Slice(m.all, func(i, j int) bool { return m.all[i].Code < m.all[j].Code })

	m.applyFilter()

	return nil
}

// Filter narrows what is shown to accounts matching query.
//
// It matches on the account number and on the name, case- and
// accent-insensitively, because an accountant looking for "Phải thu khách hàng"
// types "phai thu" and expects to find it.
func (m *Model) Filter(query string) {
	m.query = query
	m.applyFilter()
}

// Rows is what the table shows, in the order it shows it.
func (m *Model) Rows() []Row { return m.shown }

// Summary states what is on screen.
func (m *Model) Summary() string {
	if m.query == "" {
		return fmt.Sprintf("%d tài khoản", len(m.all))
	}

	return fmt.Sprintf("%d / %d tài khoản", len(m.shown), len(m.all))
}

// Empty is what to say when there are no rows.
//
// An empty chart and a search with no matches are different facts, and an
// operator who cannot tell them apart will conclude the application is broken —
// or, worse, that the account does not exist. Neither state is ever allowed to
// be a blank table.
func (m *Model) Empty() string {
	if len(m.all) == 0 {
		return "Sổ này chưa có tài khoản nào. Hãy khởi tạo hệ thống tài khoản."
	}

	return fmt.Sprintf("Không có tài khoản nào khớp với %q.", strings.TrimSpace(m.query))
}

// Headings are fixed for this screen, so they are a package constant rather
// than something the model computes. The view builds its columns from them.
var headers = []string{"Mã", "Tên tài khoản", "Loại", "Tính chất"}

// Table is what the content area draws: the column headings, the rows as plain
// strings, and the sentence to show when there are none.
//
// This is the seam. Everything the screen decides — the order, what a filter
// matches, which words the operator reads — is settled before this call and
// asserted in tests. What the view does with the result is drawing, and drawing
// is not worth a test.
//
// The headings come back even when there are no rows: a view that loses its
// columns because a search matched nothing has moved the screen under the
// operator's hands.
func (m *Model) Table() (headings []string, rows [][]string, empty string) {
	rows = make([][]string, 0, len(m.shown))

	for _, r := range m.shown {
		rows = append(rows, []string{r.Code, r.Name, r.Kind, r.Nature})
	}

	return headers, rows, m.Empty()
}

// applyFilter recomputes what is shown from the loaded rows and the query.
func (m *Model) applyFilter() {
	needle := textfold.Fold(strings.TrimSpace(m.query))

	if needle == "" {
		m.shown = m.all

		return
	}

	m.shown = make([]Row, 0, len(m.all))

	for _, r := range m.all {
		// The number matches by PREFIX; the name matches anywhere.
		//
		// An accountant typing "1" is asking for the 1xx accounts. Matching the
		// number as a substring instead answers with 214, 511 and 911 as well —
		// every account with a 1 anywhere in it — and the group they wanted is
		// buried in a list of everything. The name is free text and has no such
		// structure, so "khách" belongs in the middle of "Phải thu khách hàng".
		if strings.HasPrefix(textfold.Fold(r.Code), needle) || strings.Contains(textfold.Fold(r.Name), needle) {
			m.shown = append(m.shown, r)
		}
	}
}
