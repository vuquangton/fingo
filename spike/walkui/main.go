// SPIKE — THROWAWAY. Not production. Delete when the question is answered.
//
// Question: is lxn/walk viable for FinGo's operator UI?
//
// Three things decide it:
//  1. a walk TableView over 10,000 rows read from a real MariaDB, with totals,
//     that stays responsive while the rows are still arriving
//  2. keyboard-only operation — F2 new, F3 search, F8 post — plus cell movement
//  3. a modal dialog with validation and a focusable error summary
//
// Deliberately dumb: one file, no abstractions, no tests, no error recovery
// beyond what makes it runnable. It writes one throwaway database and prints
// its own timings, because a prototype has to surface its state.
//
// Run: go run .
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

// ---------------------------------------------------------------------------
// Connection. Hard-coded from fingo/.env, because this is a spike.
// ---------------------------------------------------------------------------

const (
	dbHost   = "127.0.0.1"
	dbPort   = "3306"
	dbUser   = "dev"
	dbPass   = "123456"
	dbParams = "charset=utf8mb4&parseTime=True&loc=UTC"

	// scratchDB is named so nobody can mistake it for a client's books.
	scratchDB = "fingo_spike_walk"

	rowCount = 10_000
)

func dsn(db string) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s", dbUser, dbPass, dbHost, dbPort, db, dbParams)
}

// ---------------------------------------------------------------------------
// The row, and the model the grid reads.
// ---------------------------------------------------------------------------

type ledgerRow struct {
	Date        time.Time
	VoucherNo   string
	AccountCode string
	AccountName string
	Description string
	Debit       int64
	Credit      int64
}

// LedgerModel is the walk table model. Embedded TableModelBase supplies the
// change-notification methods the widget requires.
type LedgerModel struct {
	walk.TableModelBase
	rows []ledgerRow
}

func (m *LedgerModel) RowCount() int { return len(m.rows) }

func (m *LedgerModel) Value(row, col int) interface{} {
	r := m.rows[row]

	switch col {
	case 0:
		return r.Date.Format("2006-01-02")
	case 1:
		return r.VoucherNo
	case 2:
		return r.AccountCode
	case 3:
		return r.AccountName
	case 4:
		return r.Description
	case 5:
		return group(r.Debit)
	case 6:
		return group(r.Credit)
	}

	return ""
}

// group writes an integer with dots between thousands, the Vietnamese
// convention. Copied from web/view.go rather than imported, because a spike
// must not drag the real module in.
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

// ---------------------------------------------------------------------------
// Seeding. A scratch database, dropped and rebuilt every run so the timings
// mean something.
// ---------------------------------------------------------------------------

func seed(ctx context.Context) (*sql.DB, time.Duration, time.Duration, error) {
	admin, err := sql.Open("mysql", dsn(""))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("open admin connection: %w", err)
	}
	defer admin.Close()

	schemaStart := time.Now()

	for _, stmt := range []string{
		"DROP DATABASE IF EXISTS `" + scratchDB + "`",
		"CREATE DATABASE `" + scratchDB + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
	} {
		if _, err := admin.ExecContext(ctx, stmt); err != nil {
			return nil, 0, 0, fmt.Errorf("%s: %w", stmt, err)
		}
	}

	db, err := sql.Open("mysql", dsn(scratchDB))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("open scratch database: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE spike_ledger (
			id           INT AUTO_INCREMENT PRIMARY KEY,
			entry_date   DATE NOT NULL,
			voucher_no   VARCHAR(32) NOT NULL,
			account_code VARCHAR(16) NOT NULL,
			account_name VARCHAR(128) NOT NULL,
			description  VARCHAR(255) NOT NULL,
			debit        BIGINT NOT NULL DEFAULT 0,
			credit       BIGINT NOT NULL DEFAULT 0
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return nil, 0, 0, fmt.Errorf("create scratch table: %w", err)
	}

	schema := time.Since(schemaStart)

	accounts := []struct{ code, name string }{
		{"1111", "Tiền Việt Nam"},
		{"1121", "Tiền gửi ngân hàng VND"},
		{"131", "Phải thu của khách hàng"},
		{"156", "Hàng hóa"},
		{"331", "Phải trả cho người bán"},
		{"5111", "Doanh thu bán hàng hóa"},
		{"632", "Giá vốn hàng bán"},
		{"6422", "Chi phí quản lý kinh doanh"},
	}

	insertStart := time.Now()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("begin: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO spike_ledger
			(entry_date, voucher_no, account_code, account_name, description, debit, credit)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < rowCount; i++ {
		a := accounts[i%len(accounts)]
		day = day.Add(time.Duration(i/len(accounts)) * 0)

		date := time.Date(2026, 1, 1+i/40, 0, 0, 0, 0, time.UTC)
		voucher := fmt.Sprintf("PK2026-%05d", i/2+1)

		var debit, credit int64
		if i%2 == 0 {
			debit = int64(100_000 + (i%97)*13_000)
		} else {
			credit = int64(100_000 + (i%89)*11_000)
		}

		if _, err := stmt.ExecContext(ctx, date, voucher, a.code, a.name,
			"Bút toán kiểm tra "+voucher, debit, credit); err != nil {
			return nil, 0, 0, fmt.Errorf("insert row %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, 0, fmt.Errorf("commit seed: %w", err)
	}

	return db, schema, time.Since(insertStart), nil
}

func fetch(ctx context.Context, db *sql.DB) ([]ledgerRow, time.Duration, error) {
	start := time.Now()

	rows, err := db.QueryContext(ctx, `
		SELECT entry_date, voucher_no, account_code, account_name, description, debit, credit
		FROM spike_ledger
		ORDER BY entry_date, id`)
	if err != nil {
		return nil, 0, fmt.Errorf("query ledger: %w", err)
	}
	defer rows.Close()

	out := make([]ledgerRow, 0, rowCount)

	for rows.Next() {
		var r ledgerRow
		if err := rows.Scan(&r.Date, &r.VoucherNo, &r.AccountCode, &r.AccountName,
			&r.Description, &r.Debit, &r.Credit); err != nil {
			return nil, 0, fmt.Errorf("scan row: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate ledger: %w", err)
	}

	return out, time.Since(start), nil
}

// ---------------------------------------------------------------------------
// The spike itself.
// ---------------------------------------------------------------------------

func main() {
	ctx := context.Background()

	// -dialog drives the validation dialog straight into its error state, so the
	// pattern can be captured without a human clicking anything.
	autoDialog := false
	for _, a := range os.Args[1:] {
		if a == "-dialog" {
			autoDialog = true
		}
	}

	fmt.Println("SPIKE walk-ui — THROWAWAY")
	fmt.Println("scratch database:", scratchDB, "(dropped and rebuilt every run)")

	db, schemaTime, insertTime, err := seed(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "SPIKE seed:", err)
		os.Exit(1)
	}
	defer db.Close()
	fmt.Printf("seed: schema %s, %d rows in %s\n", schemaTime.Round(time.Millisecond), rowCount, insertTime.Round(time.Millisecond))

	model := &LedgerModel{}

	var (
		mw      *walk.MainWindow
		tv      *walk.TableView
		status  *walk.StatusBarItem
		totals  *walk.Label
		search  *walk.LineEdit
		startup = time.Now()
	)

	setStatus := func(s string) {
		if status != nil {
			_ = status.SetText(s)
		}
		fmt.Println("SPIKE", s)
	}

	if err := (MainWindow{
		AssignTo: &mw,
		Title:    "FinGo — walk spike (THROWAWAY)",
		Size:     Size{Width: 1100, Height: 700},
		Layout:   VBox{MarginsZero: true},
		MenuItems: []MenuItem{
			Menu{
				Text: "&Nghiệp vụ",
				Items: []MenuItem{
					Action{Text: "&Mới (F2)", Shortcut: Shortcut{Key: walk.KeyF2}, OnTriggered: func() { setStatus("F2 new voucher") }},
					Action{Text: "&Tìm (F3)", Shortcut: Shortcut{Key: walk.KeyF3}, OnTriggered: func() { if search != nil { _ = search.SetFocus() } }},
					Action{Text: "&Ghi sổ (F8)", Shortcut: Shortcut{Key: walk.KeyF8}, OnTriggered: func() { setStatus("F8 post") }},
				},
			},
		},
		Children: []Widget{
			Composite{
				Layout: HBox{MarginsZero: false},
				Children: []Widget{
					Label{Text: "Tìm:"},
					LineEdit{AssignTo: &search, OnKeyDown: func(key walk.Key) {
						if key == walk.KeyReturn {
							runSearch(model, search.Text(), setStatus)
						}
					}},
					PushButton{Text: "Tìm (F3)", OnClicked: func() { runSearch(model, search.Text(), setStatus) }},
					PushButton{Text: "Hộp thoại kiểm tra…", OnClicked: func() { showValidationDialog(mw, "") }},
				},
			},
			TableView{
				AssignTo:         &tv,
				AlternatingRowBG: true,
				ColumnsOrderable: true,
				MultiSelection:   true,
				Columns: []TableViewColumn{
					{Title: "Ngày", Width: 100},
					{Title: "Số CT", Width: 120},
					{Title: "TK", Width: 70},
					{Title: "Tên tài khoản", Width: 200},
					{Title: "Diễn giải", Width: 260},
					{Title: "Nợ", Width: 120, Alignment: AlignFar},
					{Title: "Có", Width: 120, Alignment: AlignFar},
				},
				Model: model,
			},
			Label{AssignTo: &totals, Text: "đang tải…"},
		},
		StatusBarItems: []StatusBarItem{
			{AssignTo: &status, Text: "khởi động"},
		},
	}).Create(); err != nil {
		fmt.Fprintln(os.Stderr, "SPIKE create window:", err)
		os.Exit(1)
	}

	// Pinned to the corner so a headless screenshot captures the whole window
	// rather than the desktop behind an off-screen edge.
	mw.SetBounds(walk.Rectangle{X: 0, Y: 0, Width: 1090, Height: 660})
	mw.SetVisible(true)
	fmt.Printf("SPIKE window visible after %s — loading %d rows off the UI thread\n",
		time.Since(startup).Round(time.Millisecond), rowCount)

	// The load runs off the UI thread and marshals its result back with
	// Synchronize. This is the pattern the real shell would have to use: walk
	// requires every widget call on the thread that created them, so a slow
	// query must not run there.
	go func() {
		rows, took, err := fetch(ctx, db)
		if err != nil {
			mw.Synchronize(func() { setStatus("load failed: " + err.Error()) })
			return
		}

		mw.Synchronize(func() {
			model.rows = rows
			model.PublishRowsReset()

			var debit, credit int64
			for _, r := range rows {
				debit += r.Debit
				credit += r.Credit
			}

			totals.SetText(fmt.Sprintf("%d dòng · Nợ %s · Có %s · lệch %s",
				len(rows), group(debit), group(credit), group(debit-credit)))
			setStatus(fmt.Sprintf("loaded %d rows from MariaDB in %s", len(rows), took.Round(time.Millisecond)))

			// Question 2, measured without a human at the keyboard: how long
			// does a full scan of the model take under one keystroke?
			runSearch(model, "PK2026-00123", setStatus)

			if autoDialog {
				time.AfterFunc(1200*time.Millisecond, func() {
					mw.Synchronize(func() { showValidationDialog(mw, "một trăm") })
				})
			}
		})
	}()

	mw.Run()
	fmt.Println("SPIKE message pump exited")
}

// runSearch is the naive search — a full scan of the model. The point is to
// feel how 10,000 rows behave under a keystroke, not to be clever.
func runSearch(m *LedgerModel, needle string, setStatus func(string)) {
	needle = strings.TrimSpace(strings.ToLower(needle))
	if needle == "" {
		setStatus("search: nothing to find")
		return
	}

	start := time.Now()
	hits := 0

	for _, r := range m.rows {
		if strings.Contains(strings.ToLower(r.Description), needle) ||
			strings.Contains(strings.ToLower(r.VoucherNo), needle) {
			hits++
		}
	}

	setStatus(fmt.Sprintf("search %q: %d hits of %d rows in %s",
		needle, hits, len(m.rows), time.Since(start).Round(time.Microsecond)))
}

// showValidationDialog is question 3: can a native modal carry the error
// pattern the accounting UI needs — a summary at the top, focused after a
// failed submit, with the fields still named.
func showValidationDialog(owner *walk.MainWindow, prefill string) {
	var (
		dlg        *walk.Dialog
		amountEdit *walk.LineEdit
		summary    *walk.TextEdit
	)

	validate := func() {
		text := strings.TrimSpace(amountEdit.Text())
		if text == "" {
			fail(summary, amountEdit, "Số tiền là bắt buộc.")
			return
		}

		cleaned := strings.NewReplacer(".", "", ",", "", " ", "").Replace(text)
		if cleaned == "" || strings.Trim(cleaned, "0123456789-") != "" {
			fail(summary, amountEdit, fmt.Sprintf("Số tiền phải là số nguyên đồng, không phải %q.", text))
			return
		}

		fmt.Println("SPIKE dialog: accepted amount", text)
		dlg.Accept()
	}

	err := (Dialog{
		AssignTo: &dlg,
		Title:    "Kiểm tra nhập liệu (spike)",
		MinSize:  Size{Width: 520, Height: 300},
		Layout:   VBox{},
		Children: []Widget{
			// The error summary. walk has no role="alert": the only thing that
			// carries the announcement is moving focus here, so it is a real
			// focusable control and not a Label.
			TextEdit{AssignTo: &summary, ReadOnly: true, VScroll: false, MaxLength: 500, MinSize: Size{Height: 70}},
			Label{Text: "Số tiền (đồng)"},
			LineEdit{AssignTo: &amountEdit, Text: prefill},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					PushButton{Text: "Hủy", OnClicked: func() { dlg.Cancel() }},
					PushButton{Text: "Ghi sổ", OnClicked: validate},
				},
			},
		},
	}).Create(owner)

	if err != nil {
		fmt.Fprintln(os.Stderr, "SPIKE dialog:", err)
		return
	}

	fmt.Println("SPIKE dialog: opened; submit empty or a non-integer to see the summary take focus")

	// A prefilled dialog is put straight into its refused state, so the pattern
	// is visible the moment it opens.
	if prefill != "" {
		fail(summary, amountEdit, fmt.Sprintf("Số tiền phải là số nguyên đồng, không phải %q.", prefill))
	}

	dlg.Run()
}

// fail fills the error summary and moves focus to it, which is the whole of the
// pattern in a native control set.
func fail(summary *walk.TextEdit, field *walk.LineEdit, message string) {
	summary.SetText("Có vấn đề\n\n• " + message)
	field.SetFocus()
	summary.SetFocus()
	fmt.Println("SPIKE dialog: refused —", message)
}
