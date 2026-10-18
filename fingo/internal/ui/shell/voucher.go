package shell

import (
	"fmt"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/shopspring/decimal"

	"fingo/internal/ui/format"
	"fingo/internal/ui/voucher"
)

// formRows is how many lines the entry form offers.
//
// Fixed, because walk cannot give it a variable number: see the note on
// entryForm. Six is more than most adjustment vouchers need and few enough that
// the screen does not scroll at the size the window opens at.
const formRows = 6

// entryForm is the journal-entry screen.
//
// walk's TableView is read-only — TableModel has no SetValue, and tableview.go
// does not contain the word "edit" anywhere — so there is no editable grid to
// reach for. This is the workaround that finding forces: a fixed number of rows,
// four controls each, committed one cell at a time when the cell loses focus.
//
// It contains no rules. The grid it drives decides what an account code may be,
// what an amount may be, and whether the entry balances; this moves strings in
// and refusals out, which is the whole of what a form should do.
type entryForm struct {
	grid *voucher.EntryGrid

	// onPost records the entry and its description. It runs on the widget's
	// thread, so a screen that writes to a database belongs on a worker — this
	// returns once it has decided whether to clear the form.
	onPost func(voucher.EntryGrid, string) error

	// guard keeps a panic in a cell or a post from taking the application down.
	guard func(what string, f func())

	rows        []*formRow
	description *walk.LineEdit
	totals      *walk.Label
	summary     *walk.TextEdit

	verdict *walk.Label
}

// formRow is one line of the form: four controls, committed separately.
type formRow struct {
	account *walk.LineEdit
	side    *walk.ComboBox
	amount  *walk.LineEdit
	desc    *walk.LineEdit
}

// newEntryForm builds the form's controls, bound to grid.
func newEntryForm(grid *voucher.EntryGrid, onPost func(voucher.EntryGrid, string) error) *entryForm {
	return &entryForm{grid: grid, onPost: onPost}
}

// bind points the form at a grid — a fresh one each time the screen is opened,
// so an abandoned entry does not reappear under the next voucher.
func (f *entryForm) bind(grid *voucher.EntryGrid) {
	f.grid = grid
	f.clear()
	f.refresh()
}

// widgets builds the form.
func (f *entryForm) widgets() []Widget {
	header := Composite{
		Layout: Grid{Columns: 4, Spacing: 8},
		Children: []Widget{
			Label{Text: "Tài khoản"}, Label{Text: "Nợ / Có"},
			Label{Text: "Số tiền (đồng)"}, Label{Text: "Diễn giải"},
		},
	}

	children := []Widget{
		Composite{
			Layout: Grid{Columns: 2, Spacing: 8},
			Children: []Widget{
				Label{Text: "Diễn giải"},
				LineEdit{AssignTo: &f.description},
			},
		},
		header,
	}

	for i := 0; i < formRows; i++ {
		r := &formRow{}
		f.rows = append(f.rows, r)

		row := i

		children = append(children, Composite{
			Layout: Grid{Columns: 4, Spacing: 8},
			Children: []Widget{
				LineEdit{AssignTo: &r.account, OnEditingFinished: func() {
					f.commit(row, voucher.ColAccount, r.account.Text())
				}},
				ComboBox{
					AssignTo: &r.side,
					Model:    []string{"", voucher.SideDebit, voucher.SideCredit},
					OnCurrentIndexChanged: func() {
						f.commit(row, voucher.ColSide, r.side.Text())
					},
				},
				LineEdit{AssignTo: &r.amount, OnEditingFinished: func() {
					f.commit(row, voucher.ColAmount, r.amount.Text())
				}},
				LineEdit{AssignTo: &r.desc, OnEditingFinished: func() {
					f.commit(row, voucher.ColDescription, r.desc.Text())
				}},
			},
		})
	}

	children = append(children,
		Label{AssignTo: &f.totals, Text: "Nợ 0 · Có 0 · lệch 0"},
		// The refusal summary. walk has no role="alert"; moving focus here is the
		// only thing that carries the announcement, which is why refuse() focuses
		// before it writes.
		TextEdit{AssignTo: &f.summary, ReadOnly: true, VScroll: false, MinSize: Size{Height: 80}},
		Composite{
			Layout: HBox{},
			Children: []Widget{
				Label{AssignTo: &f.verdict, Text: ""},
				HSpacer{},
				PushButton{Text: "Xóa dòng trống", OnClicked: func() { f.clearEmptyTail() }},
				PushButton{Text: "Ghi sổ (F8)", OnClicked: func() { f.post() }},
			},
		},
	)

	return children
}

// commit applies one cell and reports a refusal at the cell the operator is
// standing in rather than at the end of the voucher.
func (f *entryForm) commit(row, col int, text string) {
	f.guarded("nhập ô", func() { f.setCell(row, col, text) })
}

// post hands the entry to the recorder.
func (f *entryForm) post() {
	f.guarded("ghi sổ", f.record)
}

// guarded runs one of the form's actions with the shell's panic guard around it.
//
// A panic in a cell handler is the same class of danger as one in the rail or
// the table: it aborts the process where it stands, and a windowsgui application
// has nowhere to print why.
func (f *entryForm) guarded(what string, run func()) {
	if f.guard == nil {
		run()

		return
	}

	f.guard(what, run)
}

// setCell is the body of commit, so that the guard's recover sits above it.
func (f *entryForm) setCell(row, col int, text string) {
	if f.grid == nil {
		return
	}

	if err := f.grid.SetCell(row, col, text); err != nil {
		f.refuse("Có vấn đề\r\n\r\n• " + err.Error())

		return
	}

	f.refuse("")
	f.refresh()
}

// refuse shows a message, or clears the summary.
//
// Focus FIRST, then write. Moving focus out of the cell being edited fires that
// cell's OnEditingFinished, whose success path clears the summary — so a message
// written before the focus move is erased by the very act of showing it. This
// cost an hour to find in the spike and is the single most dangerous thing about
// a commit-on-focus-loss form.
func (f *entryForm) refuse(msg string) {
	if f.summary == nil {
		return
	}

	if msg != "" {
		_ = f.summary.SetFocus()
	}

	_ = f.summary.SetText(msg)
}

// refresh rewrites the running totals.
func (f *entryForm) refresh() {
	if f.totals == nil || f.grid == nil {
		return
	}

	debit, credit := f.grid.Totals()

	f.totals.SetText(fmt.Sprintf("Nợ %s · Có %s · lệch %s",
		money(debit), money(credit), money(debit-credit)))
}

// record is the body of post, so that the guard's recover sits above it.
func (f *entryForm) record() {
	if f.grid == nil {
		return
	}

	if err := f.grid.Validate(); err != nil {
		f.refuse("Không ghi sổ được\r\n\r\n• " + err.Error())

		return
	}

	if f.onPost == nil {
		f.refuse("Chưa nối được sổ sách.")

		return
	}

	if err := f.onPost(*f.grid, f.description.Text()); err != nil {
		f.refuse("Không ghi sổ được\r\n\r\n• " + err.Error())

		return
	}

	// The form is not cleared here. Whether the entry was recorded is the
	// recorder's answer, and a grid that clears itself on a failure loses the
	// operator's typing — which is the one thing they cannot get back.
}

// accepted is called once the ledger has taken the entry.
func (f *entryForm) accepted(voucherNo string) {
	f.clear()
	f.refresh()

	f.verdict.SetText("Đã ghi sổ " + voucherNo)
	f.refuse("")
}

// clear empties every control and the grid behind them.
func (f *entryForm) clear() {
	for _, r := range f.rows {
		if r.account != nil {
			r.account.SetText("")
		}
		if r.side != nil {
			r.side.SetCurrentIndex(0)
		}
		if r.amount != nil {
			r.amount.SetText("")
		}
		if r.desc != nil {
			r.desc.SetText("")
		}
	}

	if f.verdict != nil {
		f.verdict.SetText("")
	}

	if f.description != nil {
		f.description.SetText("")
	}

	if f.grid != nil {
		for i := 0; i < f.grid.RowCount(); i++ {
			_ = f.grid.SetCell(i, voucher.ColAccount, "")
			_ = f.grid.SetCell(i, voucher.ColSide, "")
			_ = f.grid.SetCell(i, voucher.ColAmount, "")
			_ = f.grid.SetCell(i, voucher.ColDescription, "")
		}
	}
}

// clearEmptyTail drops the blank rows between what is filled and the end, so the
// next line to type on is the one after the last entry rather than six rows
// below it.
func (f *entryForm) clearEmptyTail() {
	if f.grid == nil {
		return
	}

	// Remove from the bottom, because removing a row moves the ones below it up.
	for i := f.grid.RowCount() - 1; i >= 0; i-- {
		if rowIsEmpty(f.grid, i) && i > f.lastFilled() {
			f.grid.RemoveRow(i)
		}
	}

	f.refresh()
}

// lastFilled is the index of the last row the operator has put something in, or
// -1 when the grid is empty.
func (f *entryForm) lastFilled() int {
	for i := f.grid.RowCount() - 1; i >= 0; i-- {
		if !rowIsEmpty(f.grid, i) {
			return i
		}
	}

	return -1
}

func rowIsEmpty(g *voucher.EntryGrid, i int) bool {
	c := g.Cells(i)

	return c.Account == "" && c.Side == "" && c.Amount == 0 && c.Description == ""
}

// money writes a whole number of đồng for the totals line, the same way every
// other screen writes one.
func money(amount int64) string {
	return format.Money(decimal.NewFromInt(amount))
}
