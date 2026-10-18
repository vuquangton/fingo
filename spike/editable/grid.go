// SPIKE — THROWAWAY. Not production. Delete when the question is answered.
//
// The grid model, with no UI dependency. This is the seam the tests drive:
// SetCell is the commit path a Tab triggers, and Validate is the posting gate.
// The widget wiring lives in main.go and is deliberately untested — it is
// throwaway, and testing it would only pin the toolkit's shape.
package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Columns of the entry grid, in the order the widget renders them.
const (
	colAccount = iota
	colSide
	colAmount
	colDescription
)

// The two sides a movement can be on. Spelled as the ledger spells them.
const (
	sideDebit  = "debit"
	sideCredit = "credit"
)

// Chart answers whether an account code exists in the client's chart.
//
// One method, declared here where it is used, so the grid's rules can be tested
// without the chart of accounts and without a database.
type Chart interface {
	Has(code string) bool
}

// Cell is one row as the widget reads it.
type Cell struct {
	Account     string
	Side        string // "debit", "credit", or "" for a row nobody has filled in
	Amount      int64
	AmountText  string // the amount as grouped for display
	Description string
}

// gridRow is the stored form of a row. AmountText is kept so a rejected edit
// does not destroy what the operator already typed.
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
	return &EntryGrid{chart: chart, rows: make([]gridRow, n)}
}

// RowCount is how many rows the grid holds, filled in or not.
func (g *EntryGrid) RowCount() int { return len(g.rows) }

// AddRow appends an empty row at the end.
func (g *EntryGrid) AddRow() { g.rows = append(g.rows, gridRow{}) }

// RemoveRow takes a row out, moving the ones below it up.
//
// The last row is kept. A grid with no rows has nowhere to type, and an
// operator who deleted their way to an empty screen would have to be given a
// row back anyway — so the row is never removed, it is emptied instead.
func (g *EntryGrid) RemoveRow(row int) {
	if len(g.rows) <= 1 {
		g.rows[0] = gridRow{}
		return
	}

	g.rows = append(g.rows[:row], g.rows[row+1:]...)
}

// Cells returns one row as the widget would read it.
func (g *EntryGrid) Cells(row int) Cell {
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
	switch col {
	case colAccount:
		code := strings.TrimSpace(text)

		// An empty account is a row nobody has filled in, not a mistake. A code
		// that names an account the chart does not hold is a mistake, and it is
		// refused here — at the cell — rather than at the end of the voucher.
		if code != "" && !g.chart.Has(code) {
			return fmt.Errorf("account %s is not in this client's chart of accounts", code)
		}

		g.rows[row].account = code
		return nil

	case colSide:
		side := strings.ToLower(strings.TrimSpace(text))

		switch side {
		case "", sideDebit, sideCredit:
		default:
			return fmt.Errorf("%q is neither a debit nor a credit", strings.TrimSpace(text))
		}

		g.rows[row].side = side
		return nil

	case colAmount:
		amount, err := parseAmount(text)
		if err != nil {
			return err
		}

		g.rows[row].amount = amount
		g.rows[row].amountText = group(amount)
		return nil

	case colDescription:
		g.rows[row].description = strings.TrimSpace(text)
		return nil
	}

	return fmt.Errorf("column %d is not an editable cell", col)
}

// Validate is the posting gate: the rules that span rows.
//
// It is the last place an unbalanced voucher can be caught while somebody is
// still looking at the numbers, so it refuses rather than repairs, and it says
// by how much — an operator reading "the two sides must agree" has to work out
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
			return fmt.Errorf("line %d has an amount but no account", i+1)
		case r.side == "":
			return fmt.Errorf("line %d on account %s has no side", i+1, r.account)
		case r.amount <= 0:
			return fmt.Errorf("line %d on account %s needs an amount greater than zero", i+1, r.account)
		}

		if r.side == sideDebit {
			debit += r.amount
		} else {
			credit += r.amount
		}
	}

	if lines < 2 {
		return fmt.Errorf("a journal entry needs at least two lines, and this one has %d", lines)
	}

	if debit != credit {
		return fmt.Errorf("the entry debits %s and credits %s — the two sides must agree",
			group(debit), group(credit))
	}

	return nil
}

// Totals is the running total the grid shows under its last row. It counts
// whatever is there, balanced or not, because its job is to tell the operator
// which way the entry is out.
func (g *EntryGrid) Totals() (debit, credit int64) {
	for _, r := range g.rows {
		switch r.side {
		case sideDebit:
			debit += r.amount
		case sideCredit:
			credit += r.amount
		}
	}

	return debit, credit
}

// parseAmount reads a whole number of đồng.
//
// The dot is the thousands separator and a comma is refused outright. That
// refusal is the point: the shipped admin UI strips commas as well, so
// "1.234,56" is silently read as 123456 there — a fractional amount turned into
// a different, larger, valid-looking one. VND has no subunit, so the honest
// answer to a fractional entry is no.
func parseAmount(text string) (int64, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0, nil
	}

	if strings.Contains(trimmed, ",") {
		return 0, fmt.Errorf("%q has a fractional part, and đồng has no subunit", trimmed)
	}

	cleaned := strings.NewReplacer(".", "", " ", "", "\u00a0", "", "_", "").Replace(trimmed)

	amount, err := strconv.ParseInt(cleaned, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a whole number of đồng", trimmed)
	}
	if amount <= 0 {
		return 0, fmt.Errorf("%q is not an amount the ledger can post", trimmed)
	}

	return amount, nil
}

// group writes an integer with dots between thousands. Copied from
// web/view.go rather than imported, because a spike must not drag the real
// module in.
func group(units int64) string {
	digits := fmt.Sprintf("%d", units)

	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}

	var b strings.Builder
	b.WriteString(sign)

	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}

	return b.String()
}
