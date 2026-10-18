package reports

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/domain/gl"
	"fingo/internal/period"
)

// A report says why it could not be read, rather than returning a book that
// looks empty: a ledger nobody could reach is not a ledger with nothing in it.
func TestReports_ReportWhyTheyCouldNotBeRead(t *testing.T) {
	ctx := context.Background()

	readers := map[string]func(*Service) error{
		"trial balance": func(s *Service) error {
			_, err := s.TrialBalance(ctx, january.ID)
			return err
		},
		"profit and loss": func(s *Service) error {
			_, err := s.ProfitAndLoss(ctx, january.ID)
			return err
		},
		"balance sheet": func(s *Service) error {
			_, err := s.BalanceSheet(ctx, january.ID)
			return err
		},
	}

	broken := map[string]func(*fixture) string{
		"the calendar is unreadable": func(f *fixture) string {
			f.service.periods = &testPeriods{err: errors.New("the calendar is unreadable")}
			return "accounting periods"
		},
		"the ledger is unreadable": func(f *fixture) string {
			f.journal.err = errors.New("the ledger is unreadable")
			return "the ledger is unreadable"
		},
		"the chart is unreadable": func(f *fixture) string {
			f.service.chart = testCharts{err: errors.New("the chart is unreadable")}
			return "the chart is unreadable"
		},
	}

	for name, breakIt := range broken {
		t.Run(name, func(t *testing.T) {
			for reader, read := range readers {
				f := newFixture(t)
				want := breakIt(f)

				err := read(f.service)
				if err == nil {
					t.Fatalf("%s reported success", reader)
				}
				if !strings.Contains(err.Error(), want) {
					t.Errorf("%s: error = %v, want it to mention %q", reader, err, want)
				}
			}
		})
	}
}

// An account whose heading the chart no longer holds is still reported, under
// its own name: a client who reorganised their chart must not lose the figure.
func TestProfitAndLoss_GroupsAnAccountWhoseHeadingIsGone(t *testing.T) {
	f := newFixture(t)

	// The same chart, without 511 above 5111.
	chart := gl.NewChartOfAccounts()
	for _, account := range gl.TT133Chart() {
		if account.Code == "511" {
			continue
		}
		if err := chart.Add(account); err != nil {
			t.Fatalf("seed chart with %s: %v", account.Code, err)
		}
	}
	f.service.chart = testCharts{chart: chart}

	row := profitRow(t, f.profitAndLoss(t, january.ID), "5111")

	if row.GroupCode != "5111" {
		t.Errorf("5111 reports under %q, want itself when nothing is above it", row.GroupCode)
	}
	own, known := chart.ByCode("5111")
	if !known {
		t.Fatal("the seeded chart does not hold the account under test")
	}

	if row.GroupName != own.Name {
		t.Errorf("group name = %q, want the account's own name %q", row.GroupName, own.Name)
	}
	if row.AccountName != own.Name {
		t.Errorf("account name = %q, want %q", row.AccountName, own.Name)
	}
}

// A line that is neither a debit nor a credit stops the report. Guessing a side
// would put a figure in a column nothing supports; dropping the line would leave
// a report a little out that still says it balances.
func TestMovements_RefusesALineThatIsNeitherSide(t *testing.T) {
	posted := []Line{
		{VoucherID: "v1", VoucherDate: januaryStart, AccountCode: "111", Side: gl.SideDebit, Amount: decimal.NewFromInt(100)},
		{VoucherID: "v2", VoucherDate: januaryStart, AccountCode: "111", Amount: decimal.NewFromInt(50)},
	}

	if _, err := movements(posted, time.Time{}, january.EndDate, nil); !errors.Is(err, ErrUnusableLine) {
		t.Fatalf("error = %v, want ErrUnusableLine", err)
	}

	for _, read := range []func(*Service) error{
		func(s *Service) error { _, err := s.TrialBalance(context.Background(), january.ID); return err },
		func(s *Service) error { _, err := s.ProfitAndLoss(context.Background(), january.ID); return err },
		func(s *Service) error { _, err := s.BalanceSheet(context.Background(), january.ID); return err },
	} {
		f := newFixture(t)
		f.journal.lines = posted

		if err := read(f.service); !errors.Is(err, ErrUnusableLine) {
			t.Fatalf("error = %v, want ErrUnusableLine", err)
		}
	}
}

// A negative amount is the other side written in the wrong place, so a report
// refuses it rather than subtracting it from the column it was given.
func TestMovements_RefusesANegativeAmount(t *testing.T) {
	posted := []Line{
		{VoucherID: "v1", VoucherDate: januaryStart, AccountCode: "111", Side: gl.SideDebit, Amount: decimal.NewFromInt(-100)},
	}

	if _, err := movements(posted, time.Time{}, january.EndDate, nil); !errors.Is(err, ErrUnusableLine) {
		t.Fatalf("error = %v, want ErrUnusableLine", err)
	}

	f := newFixture(t)
	f.journal.lines = posted

	if _, err := f.service.TrialBalance(context.Background(), january.ID); !errors.Is(err, ErrUnusableLine) {
		t.Fatalf("error = %v, want ErrUnusableLine", err)
	}
}

// A line that moves nothing is not a voucher behind a figure, so it is not
// counted as one, and it moves no column.
func TestMovements_LeavesOutALineThatMovesNothing(t *testing.T) {
	posted := []Line{
		{VoucherID: "v1", VoucherDate: januaryStart, AccountCode: "111", Side: gl.SideDebit, Amount: decimal.NewFromInt(100)},
		{VoucherID: "v2", VoucherDate: januaryStart, AccountCode: "111", Side: gl.SideCredit, Amount: decimal.Zero},
	}

	found, err := movements(posted, time.Time{}, january.EndDate, nil)
	if err != nil {
		t.Fatalf("movements: %v", err)
	}

	held := found["111"]
	if want := decimal.NewFromInt(100); !held.periodDebit.Equal(want) {
		t.Errorf("debits = %s, want %s", held.periodDebit, want)
	}
	if !held.periodCredit.IsZero() {
		t.Errorf("credits = %s, want nothing: the line moved nothing", held.periodCredit)
	}
	if got := len(held.carried); got != 1 {
		t.Errorf("the account counts %d vouchers, want the one that moved it", got)
	}
}

// What a voucher before the calendar holds arrives as an opening balance, and is
// the period's own movement when the period is read.
func TestTrialBalance_CarriesInWhatCameBeforeThePeriod(t *testing.T) {
	f := newFixture(t)

	opened := trialRow(t, f.trialBalance(t, january.ID), "111")
	if want := decimal.NewFromInt(5_000_000); !opened.OpeningDebit.Equal(want) {
		t.Errorf("January opens with %s, want %s carried in from December", opened.OpeningDebit, want)
	}
	if !opened.PeriodDebit.IsZero() {
		t.Errorf("January moved cash by %s, and nothing was posted to it in January", opened.PeriodDebit)
	}

	moved := trialRow(t, f.trialBalance(t, december.ID), "111")
	if !moved.OpeningDebit.IsZero() {
		t.Errorf("December opens with %s, and nothing was posted before it", moved.OpeningDebit)
	}
	if want := decimal.NewFromInt(5_000_000); !moved.PeriodDebit.Equal(want) {
		t.Errorf("December moved cash by %s, want %s", moved.PeriodDebit, want)
	}
}

// A loss is a result too, and it lands in equity the same way: negative, and
// with the balance sheet still balanced.
func TestBalanceSheet_CarriesALossIntoEquity(t *testing.T) {
	f := newFixture(t)

	f.journal.lines = append(f.journal.lines,
		Line{
			VoucherID:   "v-february-cost",
			VoucherDate: time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC),
			AccountCode: "632",
			Side:        gl.SideDebit,
			Amount:      decimal.NewFromInt(900_000),
		},
		Line{
			VoucherID:   "v-february-cost",
			VoucherDate: time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC),
			AccountCode: "331",
			Side:        gl.SideCredit,
			Amount:      decimal.NewFromInt(900_000),
		},
	)

	profit := f.profitAndLoss(t, february.ID)
	if want := decimal.NewFromInt(-400_000); !profit.NetProfit.Equal(want) {
		t.Errorf("net profit = %s, want a loss of %s", profit.NetProfit, want)
	}

	sheet := f.balanceSheet(t, february.ID)
	if want := decimal.NewFromInt(-400_000); !sheet.Result.Equal(want) {
		t.Errorf("the unclosed result is %s, want %s", sheet.Result, want)
	}
	if !sheet.Balanced() {
		t.Errorf("a period that lost money does not balance: assets %s, liabilities %s, equity %s",
			sheet.TotalAssets, sheet.TotalLiabilities, sheet.TotalEquity)
	}
}

// A period nobody posted to reports nothing rather than refusing: an empty month
// is a month with no movements, not a month that cannot be read.
func TestReports_ReadAPeriodWithNothingInIt(t *testing.T) {
	f := newFixture(t)
	f.journal.lines = nil

	report := f.trialBalance(t, january.ID)
	if len(report.Rows) != 0 || !report.Balanced() {
		t.Errorf("trial balance = %+v, want no rows and nothing to balance", report)
	}
	if report.SourceVoucherCount != 0 {
		t.Errorf("the report was summed from %d vouchers, and the books are empty", report.SourceVoucherCount)
	}

	sheet := f.balanceSheet(t, january.ID)
	if !sheet.Balanced() || !sheet.TotalAssets.IsZero() {
		t.Errorf("balance sheet = %+v, want nothing on every line", sheet)
	}
}

// The port hands a report lines rather than totals, and each report asks for the
// window it needs: the whole of the books for a position, the period alone for a
// movement.
func TestReports_AskForTheLinesTheyNeed(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	if _, err := f.service.TrialBalance(ctx, january.ID); err != nil {
		t.Fatalf("TrialBalance: %v", err)
	}
	if !f.journal.askedFrom.IsZero() {
		t.Errorf("the trial balance asked from %v, want the beginning of the books", f.journal.askedFrom)
	}
	if got := dayOf(f.journal.askedTo); !got.Equal(dayOf(january.EndDate)) {
		t.Errorf("the trial balance asked to %v, want %v", got, january.EndDate)
	}

	if _, err := f.service.ProfitAndLoss(ctx, january.ID); err != nil {
		t.Fatalf("ProfitAndLoss: %v", err)
	}
	if got := dayOf(f.journal.askedFrom); !got.Equal(dayOf(january.StartDate)) {
		t.Errorf("the profit and loss report asked from %v, want the period's start %v", got, january.StartDate)
	}
	if got := dayOf(f.journal.askedTo); !got.Equal(dayOf(january.EndDate)) {
		t.Errorf("the profit and loss report asked to %v, want the period's end %v", got, january.EndDate)
	}

	if _, err := f.service.BalanceSheet(ctx, january.ID); err != nil {
		t.Fatalf("BalanceSheet: %v", err)
	}
	if !f.journal.askedFrom.IsZero() {
		t.Errorf("the balance sheet asked from %v, want the beginning of the books", f.journal.askedFrom)
	}
}

// A result account the chart does not hold is a fault in the configuration, not
// a report to read: the close's own entries would stay in, cancel the period's
// revenue and expenses, and the report would state a profit of nothing without
// saying why.
func TestReports_RefuseAResultAccountTheBooksDoNotUse(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	// The same books, but the report is told the close went through an account
	// the chart does not hold.
	service, err := NewService(f.periods, f.journal, testCharts{chart: f.chart}, ClosingAccounts{
		Result:   "9110",
		Retained: "4212",
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	readers := map[string]func() error{
		"trial balance":   func() error { _, err := service.TrialBalance(ctx, january.ID); return err },
		"profit and loss": func() error { _, err := service.ProfitAndLoss(ctx, january.ID); return err },
		"balance sheet":   func() error { _, err := service.BalanceSheet(ctx, january.ID); return err },
	}

	for report, read := range readers {
		err := read()
		if !errors.Is(err, ErrUnknownAccount) {
			t.Fatalf("%s: error = %v, want ErrUnknownAccount", report, err)
		}
		if !strings.Contains(err.Error(), "9110") {
			t.Errorf("%s: error = %v, want it to name the account", report, err)
		}
	}

	// The fault is worth refusing: read with the right account, the same month
	// states a real profit.
	if f.profitAndLoss(t, january.ID).NetProfit.IsZero() {
		t.Fatal("the fixture's own report states a profit of nothing, so this test proves nothing")
	}
}

// A chart source that hands over no chart is a fault, not a panic.
func TestReports_RefuseAChartThatIsNotThere(t *testing.T) {
	f := newFixture(t)
	f.service.chart = testCharts{}

	for _, read := range []func() error{
		func() error { _, err := f.service.TrialBalance(context.Background(), january.ID); return err },
		func() error { _, err := f.service.ProfitAndLoss(context.Background(), january.ID); return err },
		func() error { _, err := f.service.BalanceSheet(context.Background(), january.ID); return err },
	} {
		if err := read(); !errors.Is(err, ErrInvalidService) {
			t.Fatalf("error = %v, want ErrInvalidService", err)
		}
	}
}

// A period that is not a range is a fault in the calendar. Reporting the whole
// book as an opening balance — which is what an inverted period does — would be
// a report nobody could tell was wrong.
func TestReports_RefuseAPeriodThatIsNotARange(t *testing.T) {
	f := newFixture(t)
	f.periods.periods = append(f.periods.periods, period.Period{
		ID:        "2026-07",
		StartDate: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	})

	if _, err := f.service.TrialBalance(context.Background(), "2026-07"); err == nil {
		t.Fatal("a period that runs backwards was reported without complaint")
	}
}

// A line nobody can follow back to a voucher is not traceable, and five vouchers
// sharing an empty id would be reported as one.
func TestMovements_RefusesALineWithNoVoucher(t *testing.T) {
	posted := []Line{
		{VoucherDate: januaryStart, AccountCode: "111", Side: gl.SideDebit, Amount: decimal.NewFromInt(100)},
	}

	if _, err := movements(posted, time.Time{}, january.EndDate, nil); !errors.Is(err, ErrUnusableLine) {
		t.Fatalf("error = %v, want ErrUnusableLine", err)
	}

	f := newFixture(t)
	f.journal.lines = posted

	if _, err := f.service.TrialBalance(context.Background(), january.ID); !errors.Is(err, ErrUnusableLine) {
		t.Fatalf("error = %v, want ErrUnusableLine", err)
	}
}

// Both ends of a period belong to it: a voucher dated on the first day is the
// period's own movement, and one dated on the last day is too.
func TestTrialBalance_ThePeriodIncludesBothItsDates(t *testing.T) {
	f := newFixture(t)

	f.journal.lines = []Line{
		// The day before the period opens.
		{VoucherID: "v-before", VoucherDate: january.StartDate.AddDate(0, 0, -1), AccountCode: "111", Side: gl.SideDebit, Amount: decimal.NewFromInt(100)},
		// The first day of it.
		{VoucherID: "v-first", VoucherDate: january.StartDate, AccountCode: "111", Side: gl.SideDebit, Amount: decimal.NewFromInt(200)},
		// The last day of it, at the last minute of it.
		{VoucherID: "v-last", VoucherDate: time.Date(2026, 1, 31, 23, 59, 0, 0, time.UTC), AccountCode: "111", Side: gl.SideDebit, Amount: decimal.NewFromInt(400)},
		// The day the next period opens.
		{VoucherID: "v-after", VoucherDate: january.EndDate.AddDate(0, 0, 1), AccountCode: "111", Side: gl.SideDebit, Amount: decimal.NewFromInt(800)},
	}

	row := trialRow(t, f.trialBalance(t, january.ID), "111")

	if want := decimal.NewFromInt(100); !row.OpeningDebit.Equal(want) {
		t.Errorf("opening = %s, want %s from the day before the period", row.OpeningDebit, want)
	}
	if want := decimal.NewFromInt(600); !row.PeriodDebit.Equal(want) {
		t.Errorf("the period moved %s, want %s: its own two dates and nothing else", row.PeriodDebit, want)
	}
	if got, want := row.SourceVoucherCount, 3; got != want {
		t.Errorf("the row traces to %d vouchers, want %d", got, want)
	}
}

// The profit and loss report counts the vouchers behind its figures, not the
// vouchers the account saw in some earlier period an adapter handed over anyway.
func TestProfitAndLoss_CountsOnlyTheVouchersBehindItsFigures(t *testing.T) {
	f := newFixture(t)

	lines := []Line{
		// December: revenue earned before the period the report is asked for.
		{VoucherID: "v-december", VoucherDate: december.EndDate, AccountCode: "5111", Side: gl.SideCredit, Amount: decimal.NewFromInt(700)},
		// January: the period's own.
		{VoucherID: "v-january", VoucherDate: january.EndDate, AccountCode: "5111", Side: gl.SideCredit, Amount: decimal.NewFromInt(300)},
	}

	report, err := buildProfitAndLoss(january, lines, f.chart, ClosingAccounts{Result: "911", Retained: "4212"})
	if err != nil {
		t.Fatalf("buildProfitAndLoss: %v", err)
	}

	rows := 0
	for _, row := range report.Revenue {
		rows++
		if row.AccountCode != "5111" {
			continue
		}
		if want := decimal.NewFromInt(300); !row.Amount.Equal(want) {
			t.Errorf("revenue = %s, want %s earned in the period", row.Amount, want)
		}
		if got, want := row.SourceVoucherCount, 1; got != want {
			t.Errorf("the row traces to %d vouchers, want %d: the December one is not behind a January figure", got, want)
		}
	}
	if rows != 1 {
		t.Errorf("the report holds %d revenue rows, want one", rows)
	}
	if want := 1; report.SourceVoucherCount != want {
		t.Errorf("the report was read from %d vouchers, want %d", report.SourceVoucherCount, want)
	}

	// And an account the period did nothing to at all is no row, however much it
	// carries in.
	carried := []Line{
		{VoucherID: "v-december", VoucherDate: december.EndDate, AccountCode: "5111", Side: gl.SideCredit, Amount: decimal.NewFromInt(700)},
	}

	empty, err := buildProfitAndLoss(january, carried, f.chart, ClosingAccounts{Result: "911", Retained: "4212"})
	if err != nil {
		t.Fatalf("buildProfitAndLoss: %v", err)
	}
	if len(empty.Revenue) != 0 || len(empty.Expenses) != 0 {
		t.Errorf("the report holds rows for a period that did nothing: %+v / %+v", empty.Revenue, empty.Expenses)
	}
	if empty.SourceVoucherCount != 0 {
		t.Errorf("the report was read from %d vouchers, and the period holds none", empty.SourceVoucherCount)
	}
	if !empty.NetProfit.IsZero() {
		t.Errorf("net profit = %s, want nothing", empty.NetProfit)
	}
}
