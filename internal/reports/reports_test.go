package reports

import (
	"context"
	"errors"
	"testing"
	"time"

	"fingo/internal/close"
	"fingo/internal/domain/gl"
	"fingo/internal/money"
	"fingo/internal/period"
)

// testPeriods is a fixed calendar.
type testPeriods struct {
	periods []period.Period
	err     error
}

func (p *testPeriods) List(context.Context) ([]period.Period, error) {
	if p.err != nil {
		return nil, p.err
	}

	return p.periods, nil
}

// testJournal is a ledger held as lines.
//
// It filters the way the port says it filters — by calendar day, both ends
// inclusive, a zero from meaning the beginning of the books — so a report that
// misreads the contract reads the wrong lines here too.
type testJournal struct {
	lines []Line
	err   error

	// askedFrom and askedTo are the window the last read asked for, so a test
	// can check that each report asks for the lines it needs and no more.
	askedFrom time.Time
	askedTo   time.Time
}

func (j *testJournal) Lines(_ context.Context, from, to time.Time) ([]Line, error) {
	j.askedFrom, j.askedTo = from, to

	if j.err != nil {
		return nil, j.err
	}

	start, end := dayOf(from), dayOf(to)

	out := make([]Line, 0, len(j.lines))

	for _, line := range j.lines {
		date := dayOf(line.VoucherDate)

		if !from.IsZero() && date.Before(start) {
			continue
		}
		if date.After(end) {
			continue
		}

		out = append(out, line)
	}

	return out, nil
}

// testCharts is the client's chart of accounts.
type testCharts struct {
	chart *gl.ChartOfAccounts
	err   error
}

func (c testCharts) Chart(context.Context) (*gl.ChartOfAccounts, error) {
	if c.err != nil {
		return nil, c.err
	}

	return c.chart, nil
}

// Fixture periods. January is closed by the books below; February is left open,
// which is what a report has to carry into equity as an unclosed result.
var (
	januaryStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	december = period.Period{
		ID:        "2025-12",
		StartDate: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
	}
	january = period.Period{
		ID:        "2026-01",
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
	}
	february = period.Period{
		ID:        "2026-02",
		StartDate: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC),
	}
)

// voucher is one posted entry: an id, the date it was posted at, and its lines.
type voucher struct {
	id    string
	date  time.Time
	lines []gl.VoucherLine
}

func (v voucher) reportLines() []Line {
	out := make([]Line, 0, len(v.lines))

	for _, line := range v.lines {
		out = append(out, Line{
			VoucherID:   v.id,
			VoucherDate: v.date,
			AccountCode: line.AccountCode,
			Side:        line.Side,
			Amount:      line.Amount,
		})
	}

	return out
}

func dr(code string, amount int64) gl.VoucherLine {
	return gl.VoucherLine{AccountCode: code, Side: gl.SideDebit, Amount: money.FromInt(amount)}
}

func cr(code string, amount int64) gl.VoucherLine {
	return gl.VoucherLine{AccountCode: code, Side: gl.SideCredit, Amount: money.FromInt(amount)}
}

// books is one client's ledger: capital paid in, goods bought on credit, a sale,
// the cost of what was sold, the close that carried January's result to equity,
// and a February sale nobody has closed yet.
func books() []voucher {
	return []voucher{
		{
			id: "v-capital", date: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
			lines: []gl.VoucherLine{dr("111", 5_000_000), cr("4111", 5_000_000)},
		},
		{
			id: "v-stock", date: time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
			lines: []gl.VoucherLine{dr("156", 2_000_000), cr("331", 2_000_000)},
		},
		{
			id: "v-sale", date: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			lines: []gl.VoucherLine{dr("131", 1_000_000), cr("5111", 1_000_000)},
		},
		{
			id: "v-cost", date: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC),
			lines: []gl.VoucherLine{dr("632", 600_000), cr("156", 600_000)},
		},
		{
			// The close, exactly as internal/close writes one: revenue and
			// expenses into 911, and 911 into retained earnings.
			id: "v-close", date: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
			lines: []gl.VoucherLine{
				dr("5111", 1_000_000), cr("911", 1_000_000),
				dr("911", 600_000), cr("632", 600_000),
				dr("911", 400_000), cr("4212", 400_000),
			},
		},
		{
			id: "v-february", date: time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC),
			lines: []gl.VoucherLine{dr("131", 500_000), cr("5111", 500_000)},
		},
	}
}

// fixture is a report service over those books.
type fixture struct {
	service *Service
	journal *testJournal
	periods *testPeriods
	chart   *gl.ChartOfAccounts
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	chart := gl.NewChartOfAccounts()
	for _, account := range gl.TT133Chart() {
		if err := chart.Add(account); err != nil {
			t.Fatalf("seed chart with %s: %v", account.Code, err)
		}
	}

	lines := make([]Line, 0)

	for _, v := range books() {
		lines = append(lines, v.reportLines()...)
	}

	journal := &testJournal{lines: lines}
	periods := &testPeriods{periods: []period.Period{december, january, february}}

	service, err := NewService(periods, journal, testCharts{chart: chart}, close.TT133ClosingAccounts())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	return &fixture{service: service, journal: journal, periods: periods, chart: chart}
}

func (f *fixture) trialBalance(t *testing.T, periodID string) TrialBalance {
	t.Helper()

	report, err := f.service.TrialBalance(context.Background(), periodID)
	if err != nil {
		t.Fatalf("TrialBalance(%s): %v", periodID, err)
	}

	return report
}

func (f *fixture) profitAndLoss(t *testing.T, periodID string) ProfitAndLoss {
	t.Helper()

	report, err := f.service.ProfitAndLoss(context.Background(), periodID)
	if err != nil {
		t.Fatalf("ProfitAndLoss(%s): %v", periodID, err)
	}

	return report
}

func (f *fixture) balanceSheet(t *testing.T, periodID string) BalanceSheet {
	t.Helper()

	report, err := f.service.BalanceSheet(context.Background(), periodID)
	if err != nil {
		t.Fatalf("BalanceSheet(%s): %v", periodID, err)
	}

	return report
}

func (f *fixture) periodsOf(t *testing.T) []period.Period {
	t.Helper()

	return f.periods.periods
}

// trialRow returns one account's row.
func trialRow(t *testing.T, report TrialBalance, code string) TrialBalanceRow {
	t.Helper()

	for _, row := range report.Rows {
		if row.AccountCode == code {
			return row
		}
	}

	t.Fatalf("the trial balance holds no row for %s: %+v", code, report.Rows)

	return TrialBalanceRow{}
}

// profitRow returns one account's row, from either side of the report.
func profitRow(t *testing.T, report ProfitAndLoss, code string) ProfitAndLossRow {
	t.Helper()

	for _, row := range append(append([]ProfitAndLossRow{}, report.Revenue...), report.Expenses...) {
		if row.AccountCode == code {
			return row
		}
	}

	t.Fatalf("the profit and loss report holds no row for %s: %+v / %+v", code, report.Revenue, report.Expenses)

	return ProfitAndLossRow{}
}

// sheetRow returns one account's row from the section that holds it.
func sheetRow(t *testing.T, rows []BalanceSheetRow, code string) BalanceSheetRow {
	t.Helper()

	for _, row := range rows {
		if row.AccountCode == code {
			return row
		}
	}

	t.Fatalf("the balance sheet holds no row for %s in this section: %+v", code, rows)

	return BalanceSheetRow{}
}

// The trial balance adds up: debits equal credits in every column, for every
// period the calendar holds. This is the one thing a set of posted vouchers
// cannot fail to do, so a report that fails it was not read from vouchers.
func TestTrialBalance_DebitsEqualCredits(t *testing.T) {
	f := newFixture(t)

	for _, p := range f.periodsOf(t) {
		t.Run(p.ID, func(t *testing.T) {
			report := f.trialBalance(t, p.ID)

			if !report.Balanced() {
				t.Errorf("the trial balance does not balance: opening %s/%s, period %s/%s, closing %s/%s",
					report.OpeningDebit, report.OpeningCredit,
					report.PeriodDebit, report.PeriodCredit,
					report.ClosingDebit, report.ClosingCredit)
			}
		})
	}
}

// And it adds up because the rows do, not because a total was computed
// separately from them.
func TestTrialBalance_TheRowsMakeTheTotals(t *testing.T) {
	f := newFixture(t)
	report := f.trialBalance(t, january.ID)

	var openingDebit, openingCredit, periodDebit, periodCredit, closingDebit, closingCredit money.Dong

	for _, row := range report.Rows {
		openingDebit = openingDebit.Add(row.OpeningDebit)
		openingCredit = openingCredit.Add(row.OpeningCredit)
		periodDebit = periodDebit.Add(row.PeriodDebit)
		periodCredit = periodCredit.Add(row.PeriodCredit)
		closingDebit = closingDebit.Add(row.ClosingDebit)
		closingCredit = closingCredit.Add(row.ClosingCredit)
	}

	checks := map[string][2]money.Dong{
		"opening debit":  {openingDebit, report.OpeningDebit},
		"opening credit": {openingCredit, report.OpeningCredit},
		"period debit":   {periodDebit, report.PeriodDebit},
		"period credit":  {periodCredit, report.PeriodCredit},
		"closing debit":  {closingDebit, report.ClosingDebit},
		"closing credit": {closingCredit, report.ClosingCredit},
	}

	for name, pair := range checks {
		if !pair[0].Equal(pair[1]) {
			t.Errorf("%s total = %s, and its rows add up to %s", name, pair[1], pair[0])
		}
	}
}

// An account shows what it carried in, what the period did to it, and what it
// stands at — the three columns a bookkeeper reconciles against.
func TestTrialBalance_ShowsOpeningPeriodAndClosing(t *testing.T) {
	f := newFixture(t)
	report := f.trialBalance(t, january.ID)

	cases := map[string]struct {
		openingDebit, openingCredit money.Dong
		periodDebit, periodCredit   money.Dong
		closingDebit, closingCredit money.Dong
	}{
		// Capital paid in before the period: carried in, untouched by January.
		"4111": {money.Zero, money.FromInt(5_000_000), money.Zero, money.Zero, money.Zero, money.FromInt(5_000_000)},
		// A receivable raised in the period: 1,000,000 debit, and it stands there.
		"131": {money.Zero, money.Zero, money.FromInt(1_000_000), money.Zero, money.FromInt(1_000_000), money.Zero},
		// Revenue earned and then closed out: the movement stays, the balance goes.
		"5111": {money.Zero, money.Zero, money.FromInt(1_000_000), money.FromInt(1_000_000), money.Zero, money.Zero},
		// Stock bought and partly sold: netted, and shown on one side only.
		"156": {money.Zero, money.Zero, money.FromInt(2_000_000), money.FromInt(600_000), money.FromInt(1_400_000), money.Zero},
		// The profit determination account passes through and ends empty.
		"911": {money.Zero, money.Zero, money.FromInt(1_000_000), money.FromInt(1_000_000), money.Zero, money.Zero},
	}

	for code, want := range cases {
		t.Run(code, func(t *testing.T) {
			row := trialRow(t, report, code)

			if !row.OpeningDebit.Equal(want.openingDebit) || !row.OpeningCredit.Equal(want.openingCredit) {
				t.Errorf("opening = %s/%s, want %s/%s", row.OpeningDebit, row.OpeningCredit, want.openingDebit, want.openingCredit)
			}
			if !row.PeriodDebit.Equal(want.periodDebit) || !row.PeriodCredit.Equal(want.periodCredit) {
				t.Errorf("period = %s/%s, want %s/%s", row.PeriodDebit, row.PeriodCredit, want.periodDebit, want.periodCredit)
			}
			if !row.ClosingDebit.Equal(want.closingDebit) || !row.ClosingCredit.Equal(want.closingCredit) {
				t.Errorf("closing = %s/%s, want %s/%s", row.ClosingDebit, row.ClosingCredit, want.closingDebit, want.closingCredit)
			}
		})
	}
}

// Every row says how many vouchers it came from, and the count is the vouchers
// that actually touched the account — not the vouchers in the period.
func TestTrialBalance_RowsCountTheirVouchers(t *testing.T) {
	f := newFixture(t)
	report := f.trialBalance(t, january.ID)

	cases := map[string]int{
		"111":  1, // the capital
		"131":  1, // the sale
		"156":  2, // bought, then part of it sold
		"5111": 2, // earned, then closed
		"911":  1, // the close alone
		"4212": 1, // the close alone
	}

	for code, want := range cases {
		if got := trialRow(t, report, code).SourceVoucherCount; got != want {
			t.Errorf("%s was summed from %d vouchers, want %d", code, got, want)
		}
	}

	if want := 5; report.SourceVoucherCount != want {
		t.Errorf("the report was summed from %d vouchers, want %d", report.SourceVoucherCount, want)
	}
}

// The report does not list the chart: an account nobody has posted to has no
// figure, and a figure with no voucher behind it is what the reports refuse to
// state.
func TestTrialBalance_ListsOnlyAccountsWithPostings(t *testing.T) {
	f := newFixture(t)
	report := f.trialBalance(t, january.ID)

	posted := map[string]bool{"111": true, "131": true, "156": true, "331": true, "4111": true, "5111": true, "632": true, "911": true, "4212": true}

	for _, row := range report.Rows {
		if !posted[row.AccountCode] {
			t.Errorf("the report holds a row for %s, which nothing was posted to", row.AccountCode)
		}
	}
	if len(report.Rows) != len(posted) {
		t.Errorf("the report holds %d rows, want %d", len(report.Rows), len(posted))
	}
}

// A posting to an account the chart does not hold is refused: the figure cannot
// be classified, and dropping it would take a balance out of the balance sheet
// without saying so.
func TestTrialBalance_RefusesAnAccountTheChartDoesNotHold(t *testing.T) {
	f := newFixture(t)
	f.journal.lines = append(f.journal.lines, Line{
		VoucherID:   "v-strange",
		VoucherDate: time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC),
		AccountCode: "9999",
		Side:        gl.SideDebit,
		Amount:      money.FromInt(1),
	})

	_, err := f.service.TrialBalance(context.Background(), january.ID)
	if !errors.Is(err, ErrUnknownAccount) {
		t.Fatalf("error = %v, want ErrUnknownAccount", err)
	}
	if _, err := f.service.BalanceSheet(context.Background(), january.ID); !errors.Is(err, ErrUnknownAccount) {
		t.Fatalf("balance sheet error = %v, want ErrUnknownAccount", err)
	}
}

func TestReports_RefusesAPeriodTheCalendarDoesNotHold(t *testing.T) {
	f := newFixture(t)

	_, err := f.service.TrialBalance(context.Background(), "2026-07")
	if !errors.Is(err, period.ErrUnknownPeriod) {
		t.Fatalf("error = %v, want period.ErrUnknownPeriod", err)
	}
}

func TestNewService_RefusesAnIncompleteConfiguration(t *testing.T) {
	f := newFixture(t)

	cases := map[string]func() (*Service, error){
		"no calendar": func() (*Service, error) {
			return NewService(nil, f.journal, testCharts{chart: f.chart}, close.TT133ClosingAccounts())
		},
		"no ledger": func() (*Service, error) {
			return NewService(f.periods, nil, testCharts{chart: f.chart}, close.TT133ClosingAccounts())
		},
		"no chart": func() (*Service, error) {
			return NewService(f.periods, f.journal, nil, close.TT133ClosingAccounts())
		},
		"no closing accounts": func() (*Service, error) {
			return NewService(f.periods, f.journal, testCharts{chart: f.chart}, ClosingAccounts{})
		},
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := build(); err == nil {
				t.Fatal("an incomplete service was built")
			}
		})
	}
}
