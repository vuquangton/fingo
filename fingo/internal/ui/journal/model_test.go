package journal_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/gl"
	"fingo/internal/reports"
	"fingo/internal/ui/journal"
)

var (
	from = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to   = time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
)

func at(day int) time.Time { return time.Date(2026, 3, day, 0, 0, 0, 0, time.UTC) }

func amount(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// book is a Source over fixed lines, counting voucher lookups.
type book struct {
	lines   []reports.Line
	numbers map[string]string
	lookups []string
	err     error
}

func (b *book) Lines(context.Context, time.Time, time.Time) ([]reports.Line, error) {
	return b.lines, b.err
}

func (b *book) VoucherNo(_ context.Context, id string) (string, error) {
	b.lookups = append(b.lookups, id)

	if n, ok := b.numbers[id]; ok {
		return n, nil
	}

	return "", errors.New("no such voucher")
}

// march is three vouchers, out of order, one of them deliberately unbalanced.
func march() *book {
	return &book{
		lines: []reports.Line{
			// v-2: one debit, two credits, totalling 300.000 — balanced.
			{VoucherID: "v-2", VoucherDate: at(5), AccountCode: "1111", Side: gl.SideDebit, Amount: amount("300000")},
			{VoucherID: "v-2", VoucherDate: at(5), AccountCode: "5111", Side: gl.SideCredit, Amount: amount("200000")},
			{VoucherID: "v-2", VoucherDate: at(5), AccountCode: "3331", Side: gl.SideCredit, Amount: amount("100000")},

			// v-1: two lines, 1.000.000 both sides — balanced.
			{VoucherID: "v-1", VoucherDate: at(3), AccountCode: "1111", Side: gl.SideDebit, Amount: amount("1000000")},
			{VoucherID: "v-1", VoucherDate: at(3), AccountCode: "131", Side: gl.SideCredit, Amount: amount("1000000")},

			// v-3: debits 250.000 and credits 200.000. The ledger should never
			// have accepted it, and if it is here the books are broken.
			{VoucherID: "v-3", VoucherDate: at(9), AccountCode: "632", Side: gl.SideDebit, Amount: amount("250000")},
			{VoucherID: "v-3", VoucherDate: at(9), AccountCode: "156", Side: gl.SideCredit, Amount: amount("200000")},
		},
		numbers: map[string]string{
			"v-1": "PK2026-0001",
			"v-2": "PK2026-0002",
			"v-3": "PK2026-0003",
		},
	}
}

func loaded(t *testing.T, src journal.Source) *journal.Model {
	t.Helper()

	m := journal.New(src)
	if err := m.Load(t.Context(), from, to); err != nil {
		t.Fatalf("Load: %v", err)
	}

	return m
}

// TestModel_OneRowPerVoucherNotPerLine — the journal lists documents. A voucher
// with four lines is one document, and a list that showed four rows for it would
// tell an accountant they had written four times as much as they had.
func TestModel_OneRowPerVoucherNotPerLine(t *testing.T) {
	m := loaded(t, march())

	if got := len(m.Entries()); got != 3 {
		t.Fatalf("the journal has %d entries, want 3 vouchers", got)
	}

	for _, e := range m.Entries() {
		if e.VoucherID == "v-2" && e.Lines != 3 {
			t.Errorf("v-2 is recorded as %d lines, want 3", e.Lines)
		}
	}
}

// TestModel_OrdersByDate — a journal is read in the order the documents were
// written, and the query's order is not that.
func TestModel_OrdersByDate(t *testing.T) {
	m := loaded(t, march())

	got := make([]int, 0, 3)
	for _, e := range m.Entries() {
		got = append(got, e.Date.Day())
	}

	if got[0] != 3 || got[1] != 5 || got[2] != 9 {
		t.Errorf("the journal is dated %v, want 3, 5, 9", got)
	}
}

// TestModel_TotalsAVoucherByItsDebits — a voucher's amount is one side of it, not
// both. Summing every line would double every figure in the journal.
func TestModel_TotalsAVoucherByItsDebits(t *testing.T) {
	m := loaded(t, march())

	want := map[string]string{"v-1": "1000000", "v-2": "300000", "v-3": "250000"}

	for _, e := range m.Entries() {
		if got := e.Total.String(); got != want[e.VoucherID] {
			t.Errorf("%s totals %s, want %s", e.VoucherID, got, want[e.VoucherID])
		}
	}
}

// TestModel_MarksAVoucherThatDoesNotBalance — and this is the check the journal
// exists to make possible.
//
// The ledger refuses an unbalanced voucher, so one should never be here. If one
// is, something wrote outside the gate: a migration, a repair, a direct insert.
// A list that totalled such a voucher by its debits and said nothing would report
// a book that cannot exist as though it were ordinary.
func TestModel_MarksAVoucherThatDoesNotBalance(t *testing.T) {
	m := loaded(t, march())

	byID := map[string]journal.Entry{}
	for _, e := range m.Entries() {
		byID[e.VoucherID] = e
	}

	if !byID["v-1"].Balanced {
		t.Error("v-1 debits 1.000.000 and credits 1.000.000, and is reported as out")
	}
	if !byID["v-2"].Balanced {
		t.Error("v-2 debits 300.000 and credits 300.000, and is reported as out")
	}
	if byID["v-3"].Balanced {
		t.Error("v-3 debits 250.000 and credits 200.000, and is reported as balanced")
	}

	if len(m.Unbalanced()) != 1 {
		t.Errorf("the journal names %d unbalanced vouchers, want 1", len(m.Unbalanced()))
	}
	if !strings.Contains(m.Note(), "không cân") {
		t.Errorf("the note %q does not warn about the unbalanced voucher", m.Note())
	}
}

// TestModel_NamesEachVoucherByItsNumber — a journal citing UUIDs is a journal
// nobody can use, and the number is what an accountant writes on a printout.
func TestModel_NamesEachVoucherByItsNumber(t *testing.T) {
	m := loaded(t, march())

	want := map[string]string{"v-1": "PK2026-0001", "v-2": "PK2026-0002", "v-3": "PK2026-0003"}

	for _, e := range m.Entries() {
		if e.VoucherNo != want[e.VoucherID] {
			t.Errorf("%s is cited as %q, want %q", e.VoucherID, e.VoucherNo, want[e.VoucherID])
		}
	}
}

// TestModel_ReadsEachVoucherOnce — four lines on one document is one lookup, not
// four. The number is not on the line, so it has to be asked for.
func TestModel_ReadsEachVoucherOnce(t *testing.T) {
	src := march()
	loaded(t, src)

	if len(src.lookups) != 3 {
		t.Errorf("the model looked up %d vouchers for 3 distinct ids: %v", len(src.lookups), src.lookups)
	}
}

// TestModel_KeepsAVoucherWhoseNumberCannotBeRead — the document is in the books
// whether or not its number could be fetched, and dropping the row would make
// the journal disagree with the accounts it summarises.
func TestModel_KeepsAVoucherWhoseNumberCannotBeRead(t *testing.T) {
	src := &book{
		lines: []reports.Line{
			{VoucherID: "v-1", VoucherDate: at(3), AccountCode: "1111", Side: gl.SideDebit, Amount: amount("500")},
			{VoucherID: "v-1", VoucherDate: at(3), AccountCode: "5111", Side: gl.SideCredit, Amount: amount("500")},
		},
		numbers: map[string]string{}, // nothing readable
	}

	m := loaded(t, src)

	if len(m.Entries()) != 1 {
		t.Fatalf("the journal has %d entries, want 1", len(m.Entries()))
	}

	e := m.Entries()[0]
	if e.VoucherNo != "" {
		t.Errorf("the entry cites a number it could not read: %q", e.VoucherNo)
	}
	if e.VoucherID == "" {
		t.Error("the entry has neither a number nor an id")
	}
	if !e.Total.Equal(amount("500")) {
		t.Errorf("the total is %s, want 500 — a missing number is not a missing amount", e.Total)
	}
}

// TestModel_AnEmptyPeriodSaysSo — never an empty table.
func TestModel_AnEmptyPeriodSaysSo(t *testing.T) {
	m := loaded(t, &book{})

	if len(m.Entries()) != 0 {
		t.Fatalf("an empty period produced %d entries", len(m.Entries()))
	}

	headings, rows, empty := m.Table()

	if len(headings) == 0 {
		t.Error("an empty journal produced no headings")
	}
	if len(rows) != 0 {
		t.Errorf("an empty journal produced %d rows", len(rows))
	}
	if empty == "" {
		t.Error("an empty journal says nothing")
	}
	if m.Note() == "" {
		t.Error("an empty journal has no note")
	}
}

// TestModel_TableIsWhatTheViewDraws.
func TestModel_TableIsWhatTheViewDraws(t *testing.T) {
	m := loaded(t, march())

	headings, rows, _ := m.Table()

	want := []string{"Ngày", "Số chứng từ", "Số dòng", "Số tiền"}
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

	if rows[0][3] != "1.000.000" {
		t.Errorf("the first total reads %q, want 1.000.000", rows[0][3])
	}
}

// TestModel_LoadReportsAFailure.
func TestModel_LoadReportsAFailure(t *testing.T) {
	boom := errors.New("the server went away")

	m := journal.New(&book{err: boom})

	err := m.Load(t.Context(), from, to)
	if err == nil {
		t.Fatal("Load = nil, want the source's error")
	}
	if !errors.Is(err, boom) {
		t.Errorf("Load = %v, want it to wrap %v", err, boom)
	}
}
