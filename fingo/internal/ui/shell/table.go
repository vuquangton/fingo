package shell

import "github.com/lxn/walk"

// tableModel adapts a grid of strings to walk's TableModel.
//
// Like railModel, it decides nothing. The headings, the rows and the wording all
// arrive from a screen that was tested on its own; this only answers walk's
// questions about them, and the answers are the data itself.
type tableModel struct {
	walk.TableModelBase
	rows [][]string
}

// SetRows replaces what the table shows.
//
// RowsReset is published after the swap, not before: walk reads RowCount when it
// handles the event, and announcing the change while the old rows are still in
// place draws the previous screen's data for a frame.
func (m *tableModel) SetRows(rows [][]string) {
	m.rows = rows
	m.PublishRowsReset()
}

func (m *tableModel) RowCount() int { return len(m.rows) }

// Key is what identifies a row to the application: its first cell.
//
// Every table this shell draws leads with the thing the row is *about* — an
// account number, a voucher number — so the first cell is the handle a screen
// needs in order to open it, and the shell never has to be told which column
// that is.
func (m *tableModel) Key(row int) string {
	if row < 0 || row >= len(m.rows) || len(m.rows[row]) == 0 {
		return ""
	}

	return m.rows[row][0]
}

// Value is called by walk for every visible cell, so it is the one place where a
// bad index becomes a panic inside the message loop — which is a hard crash to
// attribute to a screen. It answers with an empty cell instead.
func (m *tableModel) Value(row, col int) any {
	if row < 0 || row >= len(m.rows) {
		return ""
	}

	cells := m.rows[row]
	if col < 0 || col >= len(cells) {
		return ""
	}

	return cells[col]
}
