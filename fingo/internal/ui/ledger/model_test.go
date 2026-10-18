package ledger_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/gl"
	"fingo/internal/reports"
	"fingo/internal/ui/ledger"
)

var (
	from = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to   = time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
)

func at(day int) time.Time {
	return time.Date(2026, 3, day, 0, 0, 0, 0, time.UTC)
}

func amount(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// book is a Source over fixed lines, and it counts the lookups so the test can
// hold the model to reading each voucher once.
type book struct {
	lines []reports.Line

	numbers map[string]string
	lookups []string

	linesErr   error
	voucherErr error
}

func (b *book) Lines(context.Context, time.Time, time.Time) ([]reports.Line, error) {
	return b.lines, b.linesErr
}

func (b *book) VoucherNo(_ context.Context, id string) (string, error) {
	b.lookups = append(b.lookups, id)

	if b.voucherErr != nil {
		return "", b.voucherErr
	}
	if n, ok := b.numbers[id]; ok {
		return n, nil
	}

	return "", errors.New("no such voucher")
}

// march is one account's movements in a month: two debits and one credit, with a
// second account's line mixed in to prove the filter works.
func march() *book {
	return &book{
		lines: []reports.Line{
			{VoucherID: "v-2", VoucherDate: at(5), AccountCode: "1111", Side: gl.SideCredit, Amount: amount("300000")},
			{VoucherID: "v-1", VoucherDate: at(3), AccountCode: "1111", Side: gl.SideDebit, Amount: amount("1000000")},
			{VoucherID: "v-9", VoucherDate: at(4), AccountCode: "5111", Side: gl.SideCredit, Amount: amount("5000000")},
			{VoucherID: "v-3", VoucherDate: at(9), AccountCode: "1111", Side: gl.SideDebit, Amount: amount("250000")},
		},
		numbers: map[string]string{
			"v-1": "PK2026-0001",
			"v-2": "PK2026-0002",
			"v-3": "PK2026-0003",
		},
	}
}

func loaded(t *testing.T, src ledger.Source, account string) *ledger.Model {
	t.Helper()

	m := ledger.New(src, account)
	if err := m.Load(t.Context(), from, to); err != nil {
		t.Fatalf("Load: %v", err)
	}

	return m
}

// TestModel_ShowsOnlyThisAccountsLines — a sổ chi tiết is one account. A ledger
// that quietly included another would still balance and would still be wrong.
func TestModel_ShowsOnlyThisAccountsLines(t *testing.T) {
	m := loaded(t, march(), "1111")

	if got := len(m.Entries()); got != 3 {
		t.Fatalf("the ledger has %d entries, want 3", got)
	}

	for _, e := range m.Entries() {
		if e.VoucherID == "v-9" {
			t.Error("the ledger for 1111 includes a line on 5111")
		}
	}
}

// TestModel_OrdersByDate — the running balance only means anything in order, and
// the order is the ledger's, not the query's.
func TestModel_OrdersByDate(t *testing.T) {
	m := loaded(t, march(), "1111")

	dates := make([]int, 0, 3)
	for _, e := range m.Entries() {
		dates = append(dates, e.Date.Day())
	}

	if dates[0] != 3 || dates[1] != 5 || dates[2] != 9 {
		t.Errorf("the entries are dated %v, want 3, 5, 9", dates)
	}
}

// TestModel_PutsEachMovementOnItsOwnSide — a sổ chi tiết has a Nợ column and a
// Có column, and a movement belongs in one of them. A row carrying both, or a
// negative amount in a positive column, is not a ledger anybody reconciles.
func TestModel_PutsEachMovementOnItsOwnSide(t *testing.T) {
	m := loaded(t, march(), "1111")

	entries := m.Entries()

	if !entries[0].Debit.Equal(amount("1000000")) || !entries[0].Credit.IsZero() {
		t.Errorf("the first movement is debit %s credit %s, want 1.000.000 and nothing",
			entries[0].Debit, entries[0].Credit)
	}
	if !entries[1].Credit.Equal(amount("300000")) || !entries[1].Debit.IsZero() {
		t.Errorf("the second movement is debit %s credit %s, want nothing and 300.000",
			entries[1].Debit, entries[1].Credit)
	}
}

// TestModel_RunsABalance — the column that makes it a ledger rather than a list,
// and the reason the order matters.
func TestModel_RunsABalance(t *testing.T) {
	m := loaded(t, march(), "1111")

	want := []string{"1000000", "700000", "950000"}

	for i, e := range m.Entries() {
		if e.Running.String() != want[i] {
			t.Errorf("entry %d runs to %s, want %s", i+1, e.Running, want[i])
		}
	}

	if got := m.Closing().String(); got != "950000" {
		t.Errorf("the closing balance is %s, want 950000", got)
	}
}

// TestModel_NamesTheVoucherBehindEachLine — the whole point of the screen.
//
// A ledger that shows amounts and dates but not which voucher produced them is a
// list of numbers; the reference is what lets an accountant open the document,
// and it is the last link in the chain from a reported figure to the paper it
// came from.
func TestModel_NamesTheVoucherBehindEachLine(t *testing.T) {
	m := loaded(t, march(), "1111")

	want := []string{"PK2026-0001", "PK2026-0002", "PK2026-0003"}

	for i, e := range m.Entries() {
		if e.VoucherNo != want[i] {
			t.Errorf("entry %d cites %q, want %q", i+1, e.VoucherNo, want[i])
		}
		if e.VoucherID == "" {
			t.Errorf("entry %d keeps no voucher id, so it cannot be opened", i+1)
		}
	}
}

// TestModel_ReadsEachVoucherOnce — the number is not on the line, so it has to be
// looked up, and looking it up per line rather than per voucher turns one screen
// into N queries.
//
// It is bounded and it is worth bounding: an account with two hundred movements
// in a month, each leg of a voucher repeating the same id, is a screen that would
// otherwise make two hundred round trips to read three documents.
func TestModel_ReadsEachVoucherOnce(t *testing.T) {
	src := &book{
		lines: []reports.Line{
			{VoucherID: "v-1", VoucherDate: at(3), AccountCode: "1111", Side: gl.SideDebit, Amount: amount("100")},
			{VoucherID: "v-1", VoucherDate: at(3), AccountCode: "1111", Side: gl.SideDebit, Amount: amount("200")},
			{VoucherID: "v-2", VoucherDate: at(4), AccountCode: "1111", Side: gl.SideDebit, Amount: amount("300")},
			{VoucherID: "v-1", VoucherDate: at(5), AccountCode: "1111", Side: gl.SideDebit, Amount: amount("400")},
		},
		numbers: map[string]string{"v-1": "PK2026-0001", "v-2": "PK2026-0002"},
	}

	loaded(t, src, "1111")

	if len(src.lookups) != 2 {
		t.Errorf("the model looked up %d vouchers for 2 distinct ids: %v", len(src.lookups), src.lookups)
	}
}

// TestModel_KeepsTheLineWhenItsVoucherCannotBeRead — a line whose document is
// missing is a fact about the books, and hiding it would make the ledger total
// disagree with the account it belongs to. The line stays and says what it knows.
func TestModel_KeepsTheLineWhenItsVoucherCannotBeRead(t *testing.T) {
	src := march()
	src.voucherErr = errors.New("the voucher is gone")

	m := loaded(t, src, "1111")

	if got := len(m.Entries()); got != 3 {
		t.Fatalf("the ledger has %d entries, want 3 — the lines are still in the books", got)
	}

	e := m.Entries()[0]
	if e.VoucherNo != "" {
		t.Errorf("entry 1 cites a voucher number it could not read: %q", e.VoucherNo)
	}
	if e.VoucherID == "" {
		t.Error("entry 1 has neither a number nor an id, so nothing identifies it")
	}
	if !e.Running.Equal(amount("1000000")) {
		t.Errorf("the running balance is %s, want 1000000 — a missing reference is not a missing amount", e.Running)
	}
}

// TestModel_AnAccountWithNoMovementSaysSo — never an empty table.
func TestModel_AnAccountWithNoMovementSaysSo(t *testing.T) {
	m := loaded(t, march(), "9999")

	if len(m.Entries()) != 0 {
		t.Fatalf("an account with no lines produced %d entries", len(m.Entries()))
	}

	headings, rows, empty := m.Table()

	if len(headings) == 0 {
		t.Error("an empty ledger produced no headings")
	}
	if len(rows) != 0 {
		t.Errorf("an empty ledger produced %d rows", len(rows))
	}
	if !strings.Contains(empty, "9999") {
		t.Errorf("the message %q does not name the account", empty)
	}
}

// TestModel_TableIsWhatTheViewDraws — headings, cells, and every row carrying the
// voucher id the drill-through needs.
func TestModel_TableIsWhatTheViewDraws(t *testing.T) {
	m := loaded(t, march(), "1111")

	headings, rows, _ := m.Table()

	want := []string{"Ngày", "Số chứng từ", "Diễn giải", "Phát sinh Nợ", "Phát sinh Có", "Lũy kế"}
	if strings.Join(headings, "|") != strings.Join(want, "|") {
		t.Errorf("headings = %v, want %v", headings, want)
	}

	if len(rows) != 3 {
		t.Fatalf("the table has %d rows, want 3", len(rows))
	}

	for i, r := range rows {
		if len(r) != len(headings) {
			t.Errorf("row %d has %d cells and there are %d headings", i, len(r), len(headings))
		}
		if r[1] == "" {
			t.Errorf("row %d cites no voucher", i)
		}
	}

	// The money is grouped the Vietnamese way, like every other screen.
	if rows[0][3] != "1.000.000" {
		t.Errorf("the first debit reads %q, want 1.000.000", rows[0][3])
	}
	if rows[0][4] != "" {
		t.Errorf("the first credit reads %q, want an empty cell", rows[0][4])
	}
	if rows[0][5] != "1.000.000" {
		t.Errorf("the first running total reads %q, want 1.000.000", rows[0][5])
	}
}

// TestModel_LoadReportsAFailureFromTheLines.
func TestModel_LoadReportsAFailureFromTheLines(t *testing.T) {
	boom := errors.New("the server went away")

	m := ledger.New(&book{linesErr: boom}, "1111")

	err := m.Load(t.Context(), from, to)
	if err == nil {
		t.Fatal("Load = nil, want the source's error")
	}
	if !errors.Is(err, boom) {
		t.Errorf("Load = %v, want it to wrap %v", err, boom)
	}
}
