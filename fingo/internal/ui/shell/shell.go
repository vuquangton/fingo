// Package shell is the application window.
//
// It is a widget layer and nothing else. What the rail contains lives in
// internal/ui/nav, what a first run collects lives in internal/ui/company, and
// neither of those knows that walk exists. Everything this package decides is
// layout, and layout is not worth a test — so there are none here, on purpose.
package shell

import (
	"fmt"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"fingo/internal/ui/nav"
	"fingo/internal/ui/voucher"
)

// Options is what the window needs to open.
type Options struct {
	// Title is the window caption.
	Title string

	// Client, Period and Locked are the three facts the header always answers:
	// whose books, which period, and whether it can still be posted to. They sit
	// above everything because a figure means nothing without them.
	Client string
	Period string
	Locked bool

	// Rail is the navigation, already arranged by nav.BuildTree.
	Rail []*nav.Node

	// Open is what the shell can actually open. It is a runtime fact, not a
	// field on an entry, so nothing silently becomes available.
	Open nav.Available

	// Status is what the status bar says before anything happens.
	Status string

	// OnOpen is called when the operator selects a rail entry the shell can
	// open. The shell does not load screens itself — loading is a database read
	// and a screen's rules, and both live outside this package — so it says what
	// it is doing and hands the id over.
	//
	// It is called on the widget's thread, from inside the rail's selection
	// event, so anything slow belongs on a worker that comes back through
	// Synchronize.
	OnOpen func(id string)

	// OnDrill is called when the operator opens a row of the current table — a
	// double click, or Enter.
	//
	// It is given the row's first cell, which is the thing the row is about:
	// an account number, a voucher number. The shell does not know which screen
	// is on, so a caller that has several tables decides for itself what the key
	// means.
	OnDrill func(key string)
}

// Shell is the application window.
type Shell struct {
	mw      *walk.MainWindow
	rail    *walk.TreeView
	table   *walk.TableView
	content *walk.TextEdit
	status  *walk.StatusBarItem
	header  *walk.Label

	open nav.Available

	onOpen func(id string)

	onDrill func(key string)

	// The entry form, and the panel that holds it. Built once with the window
	// and shown when a screen needs it, because walk's declarative widgets are
	// created with their parent and cannot be conjured later.
	form      *entryForm
	formPanel *walk.Composite
}

// New builds a shell. Nothing is on screen until Create or Run.
func New() *Shell { return &Shell{} }

// Window is the underlying window, for callers that need an owner for a dialog.
func (s *Shell) Window() *walk.MainWindow { return s.mw }

// Create builds the window and shows it, without starting the message pump.
//
// Split from Run so a caller can decide what to do *after* there is something on
// screen: a second copy of the application has to say so somewhere, and saying
// it before a window exists means a message box with no owner.
func (s *Shell) Create(o Options) error {
	s.open = o.Open
	s.onOpen = o.OnOpen
	s.onDrill = o.OnDrill

	// Built before the window, because its widgets are children of the window
	// and are created with it. It is bound to a grid later, when a screen opens.
	s.form = newEntryForm(nil, nil)
	s.form.guard = s.guard

	header := HeaderText(o.Client, o.Period, o.Locked)

	welcome := "FinGo\n\n" +
		"Chọn một mục ở cột bên trái.\n\n" +
		"Mục nào chưa có phần xử lý sẽ nói rõ lý do, thay vì mở một màn hình trống."

	if err := (MainWindow{
		AssignTo: &s.mw,
		Title:    o.Title,
		Size:     Size{Width: 1100, Height: 720},
		MenuItems: []MenuItem{
			Menu{
				Text: "&Nghiệp vụ",
				Items: []MenuItem{
					Action{Text: "&Mới (F2)", Shortcut: Shortcut{Key: walk.KeyF2},
						OnTriggered: func() { s.SetStatus("F2 — chưa có chứng từ nào để tạo") }},
					Action{Text: "&Tìm (F3)", Shortcut: Shortcut{Key: walk.KeyF3},
						OnTriggered: func() { s.SetStatus("F3 — chưa có gì để tìm") }},
					Action{Text: "&Ghi sổ (F8)", Shortcut: Shortcut{Key: walk.KeyF8},
						OnTriggered: func() {
							// The same action the form's button performs. A menu
							// item that says it does one thing and does another
							// is worse than no menu item.
							if s.formPanel != nil && s.formPanel.Visible() {
								s.form.post()

								return
							}

							s.SetStatus("F8 — không có chứng từ nào đang nhập")
						}},
				},
			},
			Menu{
				Text: "&Trợ giúp",
				Items: []MenuItem{
					Action{Text: "&Giới thiệu", OnTriggered: func() {
						_ = walk.MsgBox(s.mw, "FinGo",
							"FinGo — phần mềm kế toán doanh nghiệp nhỏ và vừa.\nChế độ kế toán: TT133.",
							walk.MsgBoxIconInformation)
					}},
				},
			},
		},
		Layout: VBox{MarginsZero: true},
		Children: []Widget{
			Composite{
				Layout: HBox{MarginsZero: false},
				Children: []Widget{
					Label{AssignTo: &s.header, Text: header},
					HSpacer{},
				},
			},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					TreeView{
						AssignTo:             &s.rail,
						Model:                newRailModel(o.Rail),
						MinSize:              Size{Width: 250},
						MaxSize:              Size{Width: 250},
						OnCurrentItemChanged: func() { s.guardRail() },
					},
					TableView{
						AssignTo:            &s.table,
						Model:               &tableModel{},
						Visible:             false,
						AlternatingRowBG:    true,
						ColumnsSizable:      true,
						LastColumnStretched: true,
						OnItemActivated:     func() { s.guardDrill() },
					},
					TextEdit{
						AssignTo: &s.content,
						ReadOnly: true,
						Text:     welcome,
					},
					Composite{
						AssignTo: &s.formPanel,
						Layout:   VBox{},
						Visible:  false,
						Children: s.form.widgets(),
					},
				},
			},
		},
		StatusBarItems: []StatusBarItem{
			{AssignTo: &s.status, Text: o.Status, Width: 600},
		},
	}).Create(); err != nil {
		return fmt.Errorf("create the application window: %w", err)
	}

	s.mw.SetBounds(walk.Rectangle{X: 0, Y: 0, Width: 1100, Height: 700})
	s.mw.SetVisible(true)

	return nil
}

// Run starts the message pump. It returns when the window closes.
func (s *Shell) Run() { s.mw.Run() }

// Synchronize marshals f onto the goroutine that owns the widgets.
//
// walk requires every widget call to happen on the goroutine that created them.
// Anything arriving from a worker — a boot step, a query — has to come through
// here, and routing it through one method is what stops a screen getting it
// wrong by omission.
func (s *Shell) Synchronize(f func()) {
	if s.mw == nil {
		return
	}

	s.mw.Synchronize(f)
}

// SetStatus writes the status bar.
func (s *Shell) SetStatus(text string) {
	if s.status == nil {
		return
	}

	_ = s.status.SetText(text)
}

// SetHeader rewrites the header once the books are open and the client is known.
func (s *Shell) SetHeader(client, period string, locked bool) {
	if s.header == nil {
		return
	}

	_ = s.header.SetText(HeaderText(client, period, locked))
}

// ShowTable puts a screen's table in the content area.
//
// When there are no rows the table is replaced by the screen's own sentence
// rather than shown empty. That is the product rule — never an empty table —
// and enforcing it here means no screen can forget it: a screen that returns no
// rows has not failed, it has asked for its message to be read aloud.
func (s *Shell) ShowTable(headings []string, rows [][]string, empty string) {
	if s.table == nil || s.content == nil {
		return
	}

	if len(rows) == 0 {
		s.showMessage(empty)

		return
	}

	if err := s.setColumns(headings); err != nil {
		s.showMessage("Không dựng được bảng: " + err.Error())

		return
	}

	if model, ok := s.table.Model().(*tableModel); ok {
		model.SetRows(rows)
	}

	s.content.SetVisible(false)
	s.table.SetVisible(true)
	s.hideForm()
	s.SetFocusToTable()
}

// ShowEntryForm puts the journal-entry form in the content area, bound to grid.
//
// onPost records the entry and reports what the ledger said. It runs on the
// widget's thread, so a screen that writes to a database belongs on a worker
// that comes back through Synchronize.
func (s *Shell) ShowEntryForm(grid *voucher.EntryGrid, onPost func(voucher.EntryGrid, string) error) {
	if s.formPanel == nil || s.form == nil {
		return
	}

	s.form.onPost = onPost
	s.form.bind(grid)

	s.table.SetVisible(false)
	s.content.SetVisible(false)
	s.formPanel.SetVisible(true)

	s.form.focusFirst()
}

// EntryRecorded tells the form the ledger took the entry, so it can clear itself
// and stand ready for the next document.
func (s *Shell) EntryRecorded(voucherNo string) {
	if s.form != nil {
		s.form.accepted(voucherNo)
	}
}

// ShowMessage replaces the content area with a sentence.
func (s *Shell) ShowMessage(text string) { s.showMessage(text) }

func (s *Shell) showMessage(text string) {
	if s.table == nil || s.content == nil {
		return
	}

	// Focus before writing, for the same reason the first-run form does it:
	// the widget being left fires its own handler, and a handler that clears the
	// message would wipe the words as they are shown.
	_ = s.content.SetFocus()
	_ = s.content.SetText(text)

	s.table.SetVisible(false)
	s.content.SetVisible(true)
	s.hideForm()
}

// drill hands the selected row to the caller.
//
// A table row is the first half of a sentence: an account number on a trial
// balance means "there is more to see here", and this is what says so.
func (s *Shell) drill() {
	if s.onDrill == nil || s.table == nil {
		return
	}

	model, ok := s.table.Model().(*tableModel)
	if !ok {
		return
	}

	key := model.Key(s.table.CurrentIndex())
	if key == "" {
		return
	}

	s.onDrill(key)
}

// hideForm takes the entry form off the screen.
//
// Every content mode has to hide the other two. Leaving one up does not fail
// loudly: the screen simply arrives with the previous one still beside it, and
// the table it was meant to fill is squeezed into half the window — which is how
// this was found, in a screenshot, after it had already been committed.
func (s *Shell) hideForm() {
	if s.formPanel != nil {
		s.formPanel.SetVisible(false)
	}
}

// focusFirst puts the cursor in the first cell of the form, so the operator can
// start typing the moment the screen opens.
func (f *entryForm) focusFirst() {
	if len(f.rows) == 0 || f.rows[0].account == nil {
		return
	}

	_ = f.rows[0].account.SetFocus()
}

// SetFocusToTable puts the keyboard on the table, so the arrow keys work the
// moment a screen opens. A table the operator has to click before they can use
// it is a table that looks broken for one keystroke.
func (s *Shell) SetFocusToTable() {
	if s.table != nil && s.table.Visible() {
		_ = s.table.SetFocus()
	}
}

// setColumns rebuilds the table's columns from a screen's headings.
//
// Rebuilt rather than reused: the next screen has different columns, and a
// clearance of the list is the only thing that makes the count match. Walk has
// no "replace the columns" call, so the list is emptied and refilled.
func (s *Shell) setColumns(headings []string) error {
	columns := s.table.Columns()

	if err := columns.Clear(); err != nil {
		return fmt.Errorf("clear the columns: %w", err)
	}

	for _, heading := range headings {
		column := walk.NewTableViewColumn()

		column.SetName(heading)
		_ = column.SetWidth(columnWidth(heading))
		_ = column.SetAlignment(walk.AlignNear)

		if err := columns.Add(column); err != nil {
			return fmt.Errorf("add the column %q: %w", heading, err)
		}
	}

	return nil
}

// columnWidth gives a heading a starting width. Walk lets the operator drag
// them, so this only has to be close enough not to look broken.
func columnWidth(heading string) int {
	switch heading {
	case "Mã":
		return 90
	case "Tên tài khoản":
		return 340
	default:
		return 130
	}
}

// showSelected renders the entry the operator picked.
//
// An entry whose engine does not exist is refused in words — "chưa hỗ trợ" and
// the reason — rather than opening an empty screen. An empty screen teaches the
// operator that the application is broken; this teaches them the feature is not
// built yet, which is a different and true thing.
func (s *Shell) showSelected() {
	if s.rail == nil || s.content == nil {
		return
	}

	item := s.rail.CurrentItem()
	if item == nil {
		return
	}

	node, ok := item.(railItem)
	if !ok || !node.node.HasEntry() {
		return
	}

	entry := node.node.Entry()

	if err := entry.Open(s.open); err != nil {
		s.showMessage(err.Error())
		s.SetStatus(entry.Label + " — chưa hỗ trợ")

		return
	}

	// The entry exists, so something has to load it. The shell does not know
	// how: loading is a database read and a screen's rules, and both live
	// outside this package. The shell says what it is doing and hands over.
	s.SetStatus(entry.Label + " — đang tải…")

	if s.onOpen != nil {
		s.onOpen(entry.ID)
	}
}
