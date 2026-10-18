// SPIKE — THROWAWAY. Delete when the question is answered.
//
// Question: can walk carry an accounting ENTRY screen?
//
// Round 1 proved walk can render 10,000 rows read-only. This round asked whether
// it can EDIT them, and the answer came back from the toolkit's own source:
// walk.TableModel has no SetValue, and tableview.go does not contain the word
// "edit" at all. TableView is a read-only virtual ListView.
//
// So this is the workaround the finding forces: a fixed-row form — one LineEdit
// per cell — driven by the same tested EntryGrid. Commit happens on
// OnEditingFinished, which is what Tab and Enter both raise, and the posting gate
// is the same Validate the tests drive.
//
// Run: go run .          (add -demo to drive it into a refusal headlessly)
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

// ledgerChart stands in for the client's chart of accounts, so the spike needs
// no database.
type ledgerChart map[string]bool

func (c ledgerChart) Has(code string) bool { return c[code] }

func demoChart() ledgerChart {
	return ledgerChart{
		"1111": true, "1121": true, "131": true, "331": true,
		"5111": true, "632": true, "156": true, "6422": true,
	}
}

// formRow is what one grid row becomes when the toolkit has no editable grid:
// four controls, committed one at a time.
type formRow struct {
	account *walk.LineEdit
	side    *walk.ComboBox
	amount  *walk.LineEdit
	desc    *walk.LineEdit
}

const formRows = 6

func main() {
	demo := false
	for _, a := range os.Args[1:] {
		if a == "-demo" {
			demo = true
		}
	}

	grid := NewEntryGrid(demoChart(), formRows)

	var (
		mw     *walk.MainWindow
		totals *walk.Label
		errBox *walk.TextEdit
		rows   []*formRow
	)

	showError := func(msg string) {
		if errBox == nil {
			return
		}

		// Focus FIRST, then write.
		//
		// Moving focus out of the cell being edited fires that cell's
		// OnEditingFinished, whose success path clears the summary — so a
		// message written before the focus move is erased by the very act of
		// showing it. This cost an hour to find and is the single most
		// dangerous thing about a commit-on-focus-loss form.
		if msg != "" {
			_ = errBox.SetFocus()
		}

		// \r\n, not \n: a Win32 edit control does not break on a bare newline.
		if err := errBox.SetText(strings.ReplaceAll(msg, "\n", "\r\n")); err != nil {
			fmt.Println("SPIKE SetText error:", err)
		}

		if msg != "" {
			fmt.Println("SPIKE refused:", strings.ReplaceAll(msg, "\r\n", " "))
			fmt.Printf("SPIKE summary now holds %d chars\n", len(errBox.Text()))
		}
	}

	refresh := func() {
		if totals == nil {
			return
		}
		debit, credit := grid.Totals()
		totals.SetText(fmt.Sprintf("Nợ %s · Có %s · lệch %s",
			group(debit), group(credit), group(debit-credit)))
	}

	commit := func(row, col int, text string) {
		if err := grid.SetCell(row, col, text); err != nil {
			showError("Có vấn đề\n\n• " + err.Error())
			return
		}

		showError("")
		refresh()
	}

	header := Composite{
		Layout: Grid{Columns: 4, Spacing: 8},
		Children: []Widget{
			Label{Text: "Tài khoản"}, Label{Text: "Nợ / Có"},
			Label{Text: "Số tiền (đồng)"}, Label{Text: "Diễn giải"},
		},
	}

	children := []Widget{header}

	for i := 0; i < formRows; i++ {
		r := &formRow{}
		rows = append(rows, r)
		row := i

		children = append(children, Composite{
			Layout: Grid{Columns: 4, Spacing: 8},
			Children: []Widget{
				LineEdit{AssignTo: &r.account, OnEditingFinished: func() {
					commit(row, colAccount, r.account.Text())
				}},
				ComboBox{AssignTo: &r.side, Model: []string{"", sideDebit, sideCredit},
					OnCurrentIndexChanged: func() {
						commit(row, colSide, r.side.Text())
					}},
				LineEdit{AssignTo: &r.amount, OnEditingFinished: func() {
					commit(row, colAmount, r.amount.Text())
				}},
				LineEdit{AssignTo: &r.desc, OnEditingFinished: func() {
					commit(row, colDescription, r.desc.Text())
				}},
			},
		})
	}

	post := func() {
		if err := grid.Validate(); err != nil {
			showError("Không ghi sổ được\n\n• " + err.Error())
			return
		}

		showError("")
		if errBox != nil {
			errBox.SetText("Đã ghi sổ.")
		}
		fmt.Println("SPIKE posted: the entry balances")
	}

	children = append(children,
		Label{AssignTo: &totals, Text: "Nợ 0 · Có 0 · lệch 0"},
		// The error summary. walk has no role="alert"; moving focus here is the
		// only thing that carries the announcement.
		TextEdit{AssignTo: &errBox, VScroll: false, MinSize: Size{Height: 90}},
		Composite{
			Layout: HBox{},
			Children: []Widget{
				HSpacer{},
				PushButton{Text: "Ghi sổ (F8)", OnClicked: post},
			},
		},
	)

	if err := (MainWindow{
		AssignTo: &mw,
		Title:    "FinGo — entry form spike (THROWAWAY)",
		Layout:   VBox{},
		Size:     Size{Width: 1000, Height: 640},
		MenuItems: []MenuItem{
			Menu{Text: "&Nghiệp vụ", Items: []MenuItem{
				Action{Text: "&Ghi sổ (F8)", Shortcut: Shortcut{Key: walk.KeyF8}, OnTriggered: post},
			}},
		},
		Children: children,
	}).Create(); err != nil {
		fmt.Fprintln(os.Stderr, "SPIKE create window:", err)
		os.Exit(1)
	}

	mw.SetBounds(walk.Rectangle{X: 0, Y: 0, Width: 1000, Height: 660})
	mw.SetVisible(true)
	fmt.Println("SPIKE entry form up. Type an account, a side, an amount — Tab commits each cell.")

	if demo {
		// Drive it into the state worth photographing: a complete-looking entry
		// whose two sides do not agree.
		time.AfterFunc(1500*time.Millisecond, func() {
			mw.Synchronize(func() {
				rows[0].account.SetText("1111")
				rows[0].side.SetCurrentIndex(1)
				rows[0].amount.SetText("1.000.000")
				rows[0].desc.SetText("Thu tiền khách hàng")

				rows[1].account.SetText("5111")
				rows[1].side.SetCurrentIndex(2)
				rows[1].amount.SetText("900.000")
				rows[1].desc.SetText("Doanh thu bán hàng")

				commit(0, colAccount, "1111")
				commit(0, colSide, sideDebit)
				commit(0, colAmount, "1.000.000")
				commit(1, colAccount, "5111")
				commit(1, colSide, sideCredit)
				commit(1, colAmount, "900.000")

				post()
			})
		})
	}

	mw.Run()
}
