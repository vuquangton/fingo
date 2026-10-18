package shell

import (
	"errors"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"fingo/internal/ui/company"
)

// ErrCompanyCancelled reports that the operator closed the first-run form
// without answering it.
var ErrCompanyCancelled = errors.New("the company form was closed without an answer")

// AskCompany shows the first-run form and returns what the operator entered.
//
// It is the only question a new installation asks, and it is asked once. There
// is no server to name, no port to choose and no password to invent — those are
// the application's business, and a first run that asks an accountant for a
// database connection has misunderstood who is using it.
//
// The rules live in company.Form and are tested there. This file only moves
// strings in and refusals out, which is the whole of what a dialog should do.
func AskCompany(owner walk.Form, form *company.Form) (company.Profile, error) {
	var (
		dlg      *walk.Dialog
		nameEdit *walk.LineEdit
		taxEdit  *walk.LineEdit
		summary  *walk.TextEdit
		startBtn *walk.PushButton
	)

	// Focus FIRST, then write.
	//
	// Moving focus out of the field being edited fires that field's
	// OnEditingFinished, whose success path clears the summary — so a message
	// written before the focus move is erased by the very act of showing it.
	// This cost an hour in the entry-form spike and is the most dangerous thing
	// about a commit-on-focus-loss form.
	refuse := func(msg string) {
		if summary == nil {
			return
		}

		_ = summary.SetFocus()
		// \r\n, not \n: a Win32 edit control does not break on a bare newline.
		_ = summary.SetText("Có vấn đề\r\n\r\n• " + strings.ReplaceAll(msg, "\n", "\r\n"))
	}

	var accepted bool

	accept := func() {
		if err := form.SetName(nameEdit.Text()); err != nil {
			refuse(err.Error())

			return
		}
		if err := form.SetTaxCode(taxEdit.Text()); err != nil {
			refuse(err.Error())

			return
		}
		if _, err := form.Profile(); err != nil {
			refuse(err.Error())

			return
		}

		_ = summary.SetText("")
		accepted = true
		dlg.Accept()
	}

	err := (Dialog{
		AssignTo: &dlg,
		Title:    "FinGo — sổ sách của doanh nghiệp nào?",
		MinSize:  Size{Width: 560, Height: 340},
		Layout:   VBox{},
		Children: []Widget{
			Label{Text: "Đây là lần đầu FinGo mở sổ trên máy này.\nHai thông tin sau in trên mọi chứng từ và tờ khai."},
			TextEdit{AssignTo: &summary, ReadOnly: true, VScroll: false, MinSize: Size{Height: 70}},
			Label{Text: "Tên doanh nghiệp"},
			LineEdit{AssignTo: &nameEdit},
			Label{Text: "Mã số thuế (có thể để trống, điền sau)"},
			LineEdit{AssignTo: &taxEdit},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					PushButton{Text: "Để sau", OnClicked: func() { dlg.Cancel() }},
					PushButton{AssignTo: &startBtn, Text: "Bắt đầu", OnClicked: accept},
				},
			},
		},
	}).Create(owner)
	if err != nil {
		return company.Profile{}, err
	}

	// Enter must accept the form.
	//
	// Found by driving the dialog: pressing Enter did nothing at all, because in
	// a Win32 dialog Enter goes to the *default* button rather than the focused
	// one, and a dialog with no default button simply ignores it. An operator who
	// types their company name and presses Enter has done nothing wrong, and the
	// only thing that happened was nothing — which reads as a hung application.
	//
	// Not fatal if it fails: the button still works by click and by Space.
	_ = dlg.SetDefaultButton(startBtn)

	_ = dlg.Run()

	if !accepted {
		return company.Profile{}, ErrCompanyCancelled
	}

	return form.Profile()
}
