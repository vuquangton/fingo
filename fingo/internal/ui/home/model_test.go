package home_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/period"
	"fingo/internal/reports"
	"fingo/internal/ui/home"
)

var today = time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// books is a Source over fixed data.
type books struct {
	periods      []period.Period
	balancesheet reports.BalanceSheet
	periodErr    error
	sheetErr     error
}

func (b books) Periods(context.Context) ([]period.Period, error) {
	return b.periods, b.periodErr
}

func (b books) BalanceSheet(context.Context, string) (reports.BalanceSheet, error) {
	return b.balancesheet, b.sheetErr
}

func amount(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// aYear is one open period covering 2026.
func aYear() []period.Period {
	return []period.Period{{
		ID:        "FY2026",
		StartDate: day(2026, 1, 1),
		EndDate:   day(2026, 12, 31),
	}}
}

// aPosition is a balance sheet with the accounts a position figure is made of.
func aPosition() reports.BalanceSheet {
	return reports.BalanceSheet{
		PeriodID: "FY2026",
		AsOf:     today,
		Assets: []reports.BalanceSheetRow{
			{AccountCode: "111", AccountName: "Tiền mặt", Amount: amount("5000000"), SourceVoucherCount: 3},
			{AccountCode: "112", AccountName: "Tiền gửi ngân hàng", Amount: amount("12000000"), SourceVoucherCount: 2},
			{AccountCode: "131", AccountName: "Phải thu khách hàng", Amount: amount("8000000"), SourceVoucherCount: 4},
			{AccountCode: "152", AccountName: "Nguyên liệu, vật liệu", Amount: amount("2000000"), SourceVoucherCount: 1},
			{AccountCode: "156", AccountName: "Hàng hóa", Amount: amount("6000000"), SourceVoucherCount: 2},
		},
		Liabilities: []reports.BalanceSheetRow{
			{AccountCode: "331", AccountName: "Phải trả cho người bán", Amount: amount("7000000"), SourceVoucherCount: 5},
		},
		TotalAssets:      amount("33000000"),
		TotalLiabilities: amount("7000000"),
	}
}

func loaded(t *testing.T, src home.Source) *home.Model {
	t.Helper()

	m := home.New(src, today)
	if err := m.Load(t.Context()); err != nil {
		t.Fatalf("Load: %v", err)
	}

	return m
}

// TestModel_WithNoPeriodSaysSo — a fresh installation has no accounting period,
// and a home screen that showed zeroes would say the business has nothing rather
// than that the books have not been opened.
//
// This is the state every first run is in, so it is the state most users see
// first.
func TestModel_WithNoPeriodSaysSo(t *testing.T) {
	m := loaded(t, books{})

	if m.HasPeriod() {
		t.Error("a client with no periods reports having one")
	}
	if m.Period() != "" {
		t.Errorf("Period() = %q, want empty", m.Period())
	}
	if len(m.Figures()) != 0 {
		t.Errorf("a client with no period produced %d figures", len(m.Figures()))
	}
	if m.Note() == "" {
		t.Error("a client with no period says nothing about it")
	}
	if !strings.Contains(strings.ToLower(m.Note()), "kỳ") {
		t.Errorf("the note %q does not mention the period", m.Note())
	}
}

// TestModel_ReportsTheCurrentPeriod — the one thing the header cannot work out
// for itself, and the thing every figure on every screen is relative to.
func TestModel_ReportsTheCurrentPeriod(t *testing.T) {
	m := loaded(t, books{periods: aYear(), balancesheet: aPosition()})

	if !m.HasPeriod() {
		t.Fatal("a client in an open period reports having none")
	}
	if m.Period() != "FY2026" {
		t.Errorf("Period() = %q, want FY2026", m.Period())
	}
	if m.Locked() {
		t.Error("an open period reports itself locked")
	}
}

// TestModel_SaysWhenThePeriodIsLocked — a locked period is the difference
// between "you may post this" and "you may not", and an operator who cannot tell
// will type the whole voucher before finding out.
func TestModel_SaysWhenThePeriodIsLocked(t *testing.T) {
	closed := aYear()
	at := day(2026, 12, 31)
	closed[0].Locked = true
	closed[0].LockedAt = &at

	m := loaded(t, books{periods: closed, balancesheet: aPosition()})

	if !m.Locked() {
		t.Error("a locked period reports itself open")
	}
	if note := strings.ToLower(m.Note()); !strings.Contains(note, "khóa") {
		t.Errorf("the note %q does not say the period is closed", m.Note())
	}
}

// TestModel_ShowsAPeriodThatIsNotToday — the current period is the one today
// falls in, and a client whose books are a year behind must be told which year
// they are looking at rather than being shown today's.
func TestModel_AClientWhoseBooksLagBehindIsShownTheirOwnYear(t *testing.T) {
	old := []period.Period{{
		ID:        "FY2024",
		StartDate: day(2024, 1, 1),
		EndDate:   day(2024, 12, 31),
	}}

	m := loaded(t, books{periods: old, balancesheet: aPosition()})

	// The engine's rule, and the right one: a client whose books are a year
	// behind gets last year's figures, not an empty screen and a shrug.
	if !m.HasPeriod() {
		t.Fatal("a client with books in FY2024 reports having no period")
	}
	if m.Period() != "FY2024" {
		t.Errorf("Period() = %q, want FY2024 — the latest period, which is where the books are", m.Period())
	}
}

// TestModel_TakesTheLatestWhenTodayFallsInNone — with several periods behind us
// and none covering today, the latest is the one the books are being kept in.
func TestModel_TakesTheLatestWhenTodayFallsInNone(t *testing.T) {
	several := []period.Period{
		{ID: "FY2023", StartDate: day(2023, 1, 1), EndDate: day(2023, 12, 31)},
		{ID: "FY2024", StartDate: day(2024, 1, 1), EndDate: day(2024, 12, 31)},
		{ID: "FY2025", StartDate: day(2025, 1, 1), EndDate: day(2025, 12, 31)},
	}

	m := loaded(t, books{periods: several, balancesheet: aPosition()})

	if m.Period() != "FY2025" {
		t.Errorf("Period() = %q, want FY2025", m.Period())
	}
}

// TestModel_ThePositionFigures — the four numbers the home screen exists to show,
// and the rule that makes them: which accounts each one is made of.
//
// The rule is a decision, not a lookup. Cash is 111 and 112; receivables are
// 131; payables are 331; stock is 152, 153, 155, 156 and 157. Getting one of
// them wrong produces a plausible number under a heading nobody re-checks.
func TestModel_ThePositionFigures(t *testing.T) {
	m := loaded(t, books{periods: aYear(), balancesheet: aPosition()})

	want := []struct{ label, total string }{
		{"Tiền", "17000000"},    // 5.000.000 + 12.000.000
		{"Phải thu", "8000000"}, // 131
		{"Phải trả", "7000000"}, // 331
		{"Tồn kho", "8000000"},  // 2.000.000 + 6.000.000
	}

	got := m.Figures()
	if len(got) != len(want) {
		t.Fatalf("the home has %d figures, want %d", len(got), len(want))
	}

	for i, w := range want {
		if got[i].Label != w.label {
			t.Errorf("figure %d is %q, want %q", i, got[i].Label, w.label)
		}
		if !got[i].Amount.Equal(amount(w.total)) {
			t.Errorf("%s = %s, want %s", w.label, got[i].Amount, w.total)
		}
	}
}

// TestModel_FiguresAreWrittenTheVietnameseWay — the same formatter as every
// other screen, so a figure cannot mean one thing here and another on a report.
//
// "17.000.000" is seventeen million. "17,000,000" is the same number written in
// a convention the person reading it does not use, and the two differ by three
// orders of magnitude if they apply their own.
func TestModel_FiguresAreWrittenTheVietnameseWay(t *testing.T) {
	m := loaded(t, books{periods: aYear(), balancesheet: aPosition()})

	_, rows, _ := m.Table()

	if len(rows) != 4 {
		t.Fatalf("the table has %d rows, want 4 figures", len(rows))
	}

	for _, r := range rows {
		if len(r) != 2 {
			t.Fatalf("row %v does not have a label and an amount", r)
		}
	}

	joined := ""
	for _, r := range rows {
		joined += r[1] + " "
	}

	if !strings.Contains(joined, "17.000.000") {
		t.Errorf("the figures read %q, want Vietnamese grouping", joined)
	}
	if strings.Contains(joined, "17,000,000") {
		t.Errorf("the figures read %q, which is the English convention", joined)
	}
}

// TestModel_APeriodWithNothingInItIsNotTheSameAsNoPeriod — an open period whose
// books are empty is a business that has started and done nothing; no period at
// all is a business whose books have not been opened. The screen has to tell
// them apart, because the operator's next move is different in each.
func TestModel_APeriodWithNothingInItIsNotTheSameAsNoPeriod(t *testing.T) {
	empty := loaded(t, books{periods: aYear()})
	none := loaded(t, books{})

	if !empty.HasPeriod() {
		t.Error("a client in an open period with no movement reports having none")
	}
	if empty.Note() == none.Note() {
		t.Errorf("both states read %q", empty.Note())
	}
	if len(empty.Figures()) == 0 {
		t.Error("an open period with no movement produced no figures at all")
	}
}

// TestModel_LoadReportsAFailureFromThePeriods.
func TestModel_LoadReportsAFailureFromThePeriods(t *testing.T) {
	boom := errors.New("the server went away")

	m := home.New(books{periodErr: boom}, today)

	err := m.Load(t.Context())
	if err == nil {
		t.Fatal("Load = nil, want the source's error")
	}
	if !errors.Is(err, boom) {
		t.Errorf("Load = %v, want it to wrap %v", err, boom)
	}
}

// TestModel_LoadReportsAFailureFromTheBalanceSheet — a home screen that silently
// shows zeroes because the report failed has told the operator the business has
// no money.
func TestModel_LoadReportsAFailureFromTheBalanceSheet(t *testing.T) {
	boom := errors.New("the report failed")

	m := home.New(books{periods: aYear(), sheetErr: boom}, today)

	if err := m.Load(t.Context()); err == nil {
		t.Fatal("Load = nil, want the source's error")
	}
}
