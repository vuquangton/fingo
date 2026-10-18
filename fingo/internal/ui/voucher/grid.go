// Package voucher is the journal-entry grid: chứng từ nghiệp vụ khác.
//
// It is the screen for the entries no module owns — the accountant's own
// adjustments — and it is the one place where the two sides of a voucher are
// typed by hand. Everything the ledger must never accept passes through here
// first.
//
// The model contains no widgets. SetCell is the commit path a Tab triggers,
// Validate is the posting gate, and both are asserted in tests. The widget
// wiring lives in the shell and is deliberately untested.
package voucher

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/gl"
	"fingo/internal/ui/format"
	"github.com/google/uuid"
)

// Columns of the entry grid, in the order the widget renders them.
const (
	ColAccount = iota
	ColSide
	ColAmount
	ColDescription
)

// The two sides a movement can be on, spelled as the ledger spells them.
const (
	SideDebit  = "debit"
	SideCredit = "credit"
)

// Chart answers whether an account code can be posted to.
//
// One method, declared here where it is used, so the grid's rules can be tested
// without the chart of accounts and without a database. *gl.ChartOfAccounts
// satisfies it as it stands.
//
// It asks about *posting*, not about existence. A header account such as 111 is
// a real account and is still the wrong thing to write in a voucher: the
// movement belongs to the account underneath it, and a ledger that accepted both
// would report the same money twice.
type Chart interface {
	IsPostable(code string) bool
}

// Cell is one row as the widget reads it.
type Cell struct {
	Account     string
	Side        string // "debit", "credit", or "" for a row nobody has filled in
	Amount      int64
	AmountText  string // the amount as grouped for display
	Description string
}

// gridRow is the stored form of a row. amountText is kept so that a rejected
// edit does not destroy what the operator already typed.
type gridRow struct {
	account     string
	side        string
	amount      int64
	amountText  string
	description string
}

// EntryGrid is an editable voucher grid.
type EntryGrid struct {
	chart Chart
	rows  []gridRow
}

// NewEntryGrid builds an empty grid with room for n rows.
func NewEntryGrid(chart Chart, n int) *EntryGrid {
	if n < 1 {
		n = 1
	}

	return &EntryGrid{chart: chart, rows: make([]gridRow, n)}
}

// RowCount is how many rows the grid holds, filled in or not.
func (g *EntryGrid) RowCount() int { return len(g.rows) }

// AddRow appends an empty row at the end.
func (g *EntryGrid) AddRow() { g.rows = append(g.rows, gridRow{}) }

// RemoveRow takes a row out, moving the ones below it up.
//
// The last row is kept. A grid with no rows has nowhere to type, and an operator
// who deleted their way to an empty screen would have to be given a row back
// anyway — so the row is never removed, it is emptied instead.
func (g *EntryGrid) RemoveRow(row int) {
	if row < 0 || row >= len(g.rows) {
		return
	}

	if len(g.rows) <= 1 {
		g.rows[0] = gridRow{}

		return
	}

	g.rows = append(g.rows[:row], g.rows[row+1:]...)
}

// Cells returns one row as the widget would read it.
func (g *EntryGrid) Cells(row int) Cell {
	if row < 0 || row >= len(g.rows) {
		return Cell{}
	}

	r := g.rows[row]

	return Cell{
		Account:     r.account,
		Side:        r.side,
		Amount:      r.amount,
		AmountText:  r.amountText,
		Description: r.description,
	}
}

// SetCell commits one cell — what a Tab or an Enter out of the cell does.
//
// It applies the rules that belong to a single cell and refuses the edit
// otherwise, so the operator finds out at the cell they are standing in rather
// than at the end of the voucher. Rules that span rows are Validate's job.
func (g *EntryGrid) SetCell(row, col int, text string) error {
	if row < 0 || row >= len(g.rows) {
		return fmt.Errorf("dòng %d không có trong lưới", row+1)
	}

	switch col {
	case ColAccount:
		code := strings.TrimSpace(text)

		// An empty account is a row nobody has filled in, not a mistake. A code
		// that cannot be posted to is a mistake, and it is refused here — at the
		// cell — rather than at the end of the voucher.
		if code != "" && !g.chart.IsPostable(code) {
			return fmt.Errorf("tài khoản %s không hạch toán được — hãy chọn tài khoản chi tiết", code)
		}

		g.rows[row].account = code

		return nil

	case ColSide:
		side := strings.ToLower(strings.TrimSpace(text))

		switch side {
		case "", SideDebit, SideCredit:
		default:
			return fmt.Errorf("%q không phải là Nợ hay Có", strings.TrimSpace(text))
		}

		g.rows[row].side = side

		return nil

	case ColAmount:
		amount, err := parseAmount(text)
		if err != nil {
			return err
		}

		g.rows[row].amount = amount
		g.rows[row].amountText = format.Money(decimal.NewFromInt(amount))

		return nil

	case ColDescription:
		g.rows[row].description = strings.TrimSpace(text)

		return nil
	}

	return fmt.Errorf("cột %d không phải là ô nhập được", col)
}

// Validate is the posting gate: the rules that span rows.
//
// It is the last place an unbalanced voucher can be caught while somebody is
// still looking at the numbers, so it refuses rather than repairs, and it says
// by how much — an operator reading "hai bên phải bằng nhau" has to work out
// which way they disagree, and one reading both totals does not.
func (g *EntryGrid) Validate() error {
	var (
		debit, credit int64
		lines         int
	)

	for i, r := range g.rows {
		// A row nobody has touched is not part of the entry: the grid renders
		// more rows than most vouchers need.
		if r.account == "" && r.side == "" && r.amount == 0 {
			continue
		}

		lines++

		switch {
		case r.account == "":
			return fmt.Errorf("dòng %d có số tiền nhưng chưa có tài khoản", i+1)
		case r.side == "":
			return fmt.Errorf("dòng %d tài khoản %s chưa ghi Nợ hay Có", i+1, r.account)
		case r.amount <= 0:
			return fmt.Errorf("dòng %d tài khoản %s phải có số tiền lớn hơn 0", i+1, r.account)
		}

		if r.side == SideDebit {
			debit += r.amount
		} else {
			credit += r.amount
		}
	}

	if lines < 2 {
		return fmt.Errorf("một bút toán phải có ít nhất hai dòng, bút toán này có %d", lines)
	}

	if debit != credit {
		return fmt.Errorf("bút toán ghi Nợ %s và ghi Có %s — hai bên phải bằng nhau",
			format.Money(decimal.NewFromInt(debit)), format.Money(decimal.NewFromInt(credit)))
	}

	return nil
}

// Totals is the running total the grid shows under its last row. It counts
// whatever is there, balanced or not, because its job is to tell the operator
// which way the entry is out.
func (g *EntryGrid) Totals() (debit, credit int64) {
	for _, r := range g.rows {
		switch r.side {
		case SideDebit:
			debit += r.amount
		case SideCredit:
			credit += r.amount
		}
	}

	return debit, credit
}

// Voucher turns the grid into the voucher the ledger will store.
//
// It is a rule rather than plumbing, which is why it lives here and is tested
// here. The order of the lines, which side each one lands on, and whether the
// row's note travels with it are all decisions — a screen that got them wrong
// would post a voucher that balances and says the wrong thing, and a voucher
// that balances is one nobody looks at twice.
//
// It refuses rather than repairs, for the same reason Validate does: the ledger
// would refuse it too, but by then the operator has lost the screen and is
// reading a database error about a voucher they can no longer see.
func (g *EntryGrid) Voucher(id string, date time.Time, description string) (*gl.Voucher, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}

	v := gl.NewVoucher(id, description, gl.VoucherTypeGeneral, date)

	// One-based, and counted over the lines actually written rather than over
	// the grid's rows: the grid renders more rows than most vouchers need, and a
	// voucher numbered 1, 2, 5 is a voucher that looks like it lost something.
	order := 0

	for _, r := range g.rows {
		if r.account == "" && r.side == "" && r.amount == 0 {
			continue
		}

		order++

		line := gl.VoucherLine{
			// A line is an entity of its own, and gets an identity of its own.
			//
			// The tempting thing is the voucher's id with the line number
			// appended, and it overflows: journal_lines.id is VARCHAR(36) and a
			// UUID is exactly 36 characters, so "-1" makes 38 and MariaDB refuses
			// the insert — on the first voucher anybody enters. The integration
			// test missed it because it chose a short id for itself.
			ID:          uuid.NewString(),
			VoucherID:   id,
			LineOrder:   order,
			AccountCode: r.account,
			Side:        sideOf(r.side),
			Amount:      decimal.NewFromInt(r.amount),
			Note:        r.description,
		}

		if err := v.AddLine(line); err != nil {
			return nil, fmt.Errorf("dòng %d: %w", order, err)
		}
	}

	return v, nil
}

// sideOf maps the grid's two strings onto the ledger's enum.
//
// SetCell has already refused anything else, so the default cannot be reached by
// typing. It is not a debit by accident: an unknown side is a bug in this
// package, and calling it a credit would hide one.
func sideOf(side string) gl.EntrySide {
	if side == SideCredit {
		return gl.SideCredit
	}

	return gl.SideDebit
}

// parseAmount reads a whole number of đồng.
//
// The parsing itself is format.Amount's, so the grid reads numbers exactly the
// way every other screen does. What is added here is the rule that đồng has no
// subunit: a fractional amount cannot be rounded into agreement with the other
// side of the entry, so it has to be refused.
//
// That refusal is the point. The shipped admin UI strips commas as well as dots,
// so "1.234,56" is silently read there as 123456 — a fractional amount turned
// into a different, larger, valid-looking one.
func parseAmount(text string) (int64, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0, nil
	}

	amount, err := format.Amount(trimmed)
	if err != nil {
		return 0, err
	}

	if !amount.IsInteger() {
		return 0, fmt.Errorf("%q có phần lẻ, mà đồng không có đơn vị nhỏ hơn", trimmed)
	}

	whole := amount.IntPart()
	if whole <= 0 {
		return 0, fmt.Errorf("%q không phải là số tiền hạch toán được", trimmed)
	}

	return whole, nil
}
