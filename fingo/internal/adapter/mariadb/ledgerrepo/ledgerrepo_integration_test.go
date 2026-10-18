package ledgerrepo_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/adapter/mariadb/glrepo"
	"fingo/internal/adapter/mariadb/ledgerrepo"
	"fingo/internal/adapter/mariadb/testsupport"
	"fingo/internal/close"
	"fingo/internal/domain/gl"
	"fingo/internal/period"
	"fingo/internal/reports"
	"fingo/internal/usecase/ledger"
	"fingo/internal/usecase/practice"
	"fingo/web"
	"fingo/web/books"
)

// theDay is the day the in-memory books are told it is, so the period they seed
// for themselves is the one the tests ask the reports for.
var theDay = time.Date(2026, time.March, 15, 9, 0, 0, 0, time.UTC)

// periodID is the accounting period both ledgers are read for. The in-memory
// books name their seeded period the same way, so one string addresses both.
const periodID = "FY2026"

var fy2026 = period.Period{
	ID:        periodID,
	StartDate: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	EndDate:   time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC),
}

var (
	firstDay  = time.Date(2026, time.March, 5, 9, 0, 0, 0, time.UTC)
	secondDay = time.Date(2026, time.March, 6, 9, 0, 0, 0, time.UTC)
)

// movement is one side of a journal entry.
type movement struct {
	account string
	side    gl.EntrySide
	amount  int64
}

// journal is one entry, written the way both ledgers can write it: a set of
// account movements and nothing else.
type journal struct {
	day         time.Time
	description string
	lines       []movement
}

// theJournals is the same small set of entries posted to both ledgers.
//
// A cash sale and the cost of the goods it sold. Between them they touch an
// asset, a revenue account, an expense account and another asset, so all three
// reports have something on every part of themselves to disagree about.
var theJournals = []journal{
	{day: firstDay, description: "Bán hàng thu tiền mặt", lines: []movement{
		{account: "1111", side: gl.SideDebit, amount: 1_000_000},
		{account: "5111", side: gl.SideCredit, amount: 1_000_000},
	}},
	{day: secondDay, description: "Giá vốn hàng bán", lines: []movement{
		{account: "632", side: gl.SideDebit, amount: 400_000},
		{account: "156", side: gl.SideCredit, amount: 400_000},
	}},
}

// The reports read a real MariaDB ledger to the same figures as the books they
// were developed against.
//
// Two implementations of one port, compared against each other rather than
// against numbers written down here. The in-memory books behind the admin UI are
// the ones the reports' own tests were written on; this adapter is the one a
// client's actual database is read through. A hard-coded expectation would go on
// passing after the two had drifted apart, and two ways of reading the same
// ledger disagreeing is the failure the port exists to prevent.
func TestSource_TrialBalanceMatchesTheInMemoryBooks(t *testing.T) {
	memory, service := bothLedgers(t)

	ctx := context.Background()

	page, err := memory.TrialBalance(ctx)
	if err != nil {
		t.Fatalf("in-memory TrialBalance: %v", err)
	}

	report, err := service.TrialBalance(ctx, periodID)
	if err != nil {
		t.Fatalf("TrialBalance from MariaDB: %v", err)
	}

	if !same(page.Debits, report.PeriodDebit) || !same(page.Credits, report.PeriodCredit) {
		t.Errorf("the period's movement differs: in memory %s/%s, MariaDB %s/%s",
			page.Debits.Text, page.Credits.Text, report.PeriodDebit, report.PeriodCredit)
	}
	if !same(page.ClosingDebits, report.ClosingDebit) || !same(page.ClosingCredits, report.ClosingCredit) {
		t.Errorf("the closing position differs: in memory %s/%s, MariaDB %s/%s",
			page.ClosingDebits.Text, page.ClosingCredits.Text, report.ClosingDebit, report.ClosingCredit)
	}
	if page.Balanced != report.Balanced() {
		t.Errorf("in memory says balanced=%v, MariaDB says %v", page.Balanced, report.Balanced())
	}

	onPage := make(map[string]figure, len(page.Rows))
	for _, r := range page.Rows {
		onPage[r.Code] = figure{
			periodDebit:   decimal.NewFromInt(r.Debit.Units),
			periodCredit:  decimal.NewFromInt(r.Credit.Units),
			closingDebit:  decimal.NewFromInt(r.ClosingDebit.Units),
			closingCredit: decimal.NewFromInt(r.ClosingCredit.Units),
			vouchers:      r.Vouchers,
		}
	}

	inReport := make(map[string]figure, len(report.Rows))
	for _, r := range report.Rows {
		inReport[r.AccountCode] = figure{
			periodDebit:   r.PeriodDebit,
			periodCredit:  r.PeriodCredit,
			closingDebit:  r.ClosingDebit,
			closingCredit: r.ClosingCredit,
			vouchers:      r.SourceVoucherCount,
		}
	}

	// Two empty reports agree perfectly and prove nothing, so the rows the
	// entries must have produced are asserted before the comparison is trusted.
	// Without this the test would pass on a query that returned no lines at all.
	want := []struct {
		code                        string
		debit, credit               int64
		closingDebit, closingCredit int64
	}{
		{"1111", 1_000_000, 0, 1_000_000, 0},
		{"5111", 0, 1_000_000, 0, 1_000_000},
		{"632", 400_000, 0, 400_000, 0},
		{"156", 0, 400_000, 0, 400_000},
	}

	if len(report.Rows) != len(want) {
		t.Fatalf("the trial balance has %d rows, want %d: the entries posted are not all in it",
			len(report.Rows), len(want))
	}

	for _, w := range want {
		row, ok := inReport[w.code]
		if !ok {
			t.Errorf("the trial balance has no row for %s", w.code)
			continue
		}

		if !row.periodDebit.Equal(decimal.NewFromInt(w.debit)) ||
			!row.periodCredit.Equal(decimal.NewFromInt(w.credit)) {
			t.Errorf("%s moved %s/%s, want %d/%d",
				w.code, row.periodDebit, row.periodCredit, w.debit, w.credit)
		}
		if !row.closingDebit.Equal(decimal.NewFromInt(w.closingDebit)) ||
			!row.closingCredit.Equal(decimal.NewFromInt(w.closingCredit)) {
			t.Errorf("%s stands at %s/%s, want %d/%d",
				w.code, row.closingDebit, row.closingCredit, w.closingDebit, w.closingCredit)
		}
	}

	compare(t, "trial balance", onPage, inReport)
}

// The profit and loss report agrees between the two ledgers, line for line.
func TestSource_ProfitAndLossMatchesTheInMemoryBooks(t *testing.T) {
	memory, service := bothLedgers(t)

	ctx := context.Background()

	page, err := memory.ProfitAndLoss(ctx)
	if err != nil {
		t.Fatalf("in-memory ProfitAndLoss: %v", err)
	}

	report, err := service.ProfitAndLoss(ctx, periodID)
	if err != nil {
		t.Fatalf("ProfitAndLoss from MariaDB: %v", err)
	}

	if !same(page.TotalRevenue, report.TotalRevenue) || !same(page.TotalExpenses, report.TotalExpenses) {
		t.Errorf("the totals differ: in memory revenue %s expenses %s, MariaDB revenue %s expenses %s",
			page.TotalRevenue.Text, page.TotalExpenses.Text, report.TotalRevenue, report.TotalExpenses)
	}
	if !same(page.Profit, report.NetProfit) {
		t.Errorf("the result differs: in memory %s, MariaDB %s", page.Profit.Text, report.NetProfit)
	}

	// A report with nothing in it agrees with another report with nothing in it,
	// so the figures are asserted before the comparison is trusted.
	if got, want := report.TotalRevenue.String(), "1000000"; got != want {
		t.Errorf("revenue = %s, want %s", got, want)
	}
	if got, want := report.TotalExpenses.String(), "400000"; got != want {
		t.Errorf("expenses = %s, want %s", got, want)
	}
	if got, want := report.NetProfit.String(), "600000"; got != want {
		t.Errorf("net profit = %s, want %s: 1.000.000 earned less 400.000 spent", got, want)
	}

	compare(t, "revenue", profitFigures(page.Revenue), profitReportFigures(report.Revenue))
	compare(t, "expenses", profitFigures(page.Expenses), profitReportFigures(report.Expenses))
}

// The balance sheet agrees between the two ledgers, on every side of it.
func TestSource_BalanceSheetMatchesTheInMemoryBooks(t *testing.T) {
	memory, service := bothLedgers(t)

	ctx := context.Background()

	page, err := memory.BalanceSheet(ctx)
	if err != nil {
		t.Fatalf("in-memory BalanceSheet: %v", err)
	}

	report, err := service.BalanceSheet(ctx, periodID)
	if err != nil {
		t.Fatalf("BalanceSheet from MariaDB: %v", err)
	}

	if !same(page.TotalAssets, report.TotalAssets) ||
		!same(page.TotalLiabilities, report.TotalLiabilities) ||
		!same(page.TotalEquity, report.TotalEquity) {
		t.Errorf("the totals differ: in memory assets %s liabilities %s equity %s, "+
			"MariaDB assets %s liabilities %s equity %s",
			page.TotalAssets.Text, page.TotalLiabilities.Text, page.TotalEquity.Text,
			report.TotalAssets, report.TotalLiabilities, report.TotalEquity)
	}
	if page.Balanced != report.Balanced() {
		t.Errorf("in memory says balanced=%v, MariaDB says %v", page.Balanced, report.Balanced())
	}

	// An empty balance sheet balances, exactly like a real one, so the position
	// is asserted before the comparison is trusted.
	if got, want := report.TotalAssets.String(), "600000"; got != want {
		t.Errorf("total assets = %s, want %s: 1.000.000 received less 400.000 of goods", got, want)
	}
	if !report.Balanced() {
		t.Errorf("the balance sheet does not balance: assets %s, liabilities %s, equity %s",
			report.TotalAssets, report.TotalLiabilities, report.TotalEquity)
	}

	compare(t, "assets", balanceFigures(page.Assets), balanceReportFigures(report.Assets))
	compare(t, "liabilities", balanceFigures(page.Liabilities), balanceReportFigures(report.Liabilities))
	compare(t, "equity", balanceFigures(page.Equity), balanceReportFigures(report.Equity))
}

// A draft is not part of the books, so it must not reach a report read from the
// database either.
//
// The domain already refuses to include one. This is the guarantee that survives
// a future query: the filter is in the SQL, and SQL is where a line can be let in
// by accident.
func TestSource_LeavesUnpostedDraftsOut(t *testing.T) {
	ctx, client, db := newBooks(t)
	service, source := sourceOf(t, client)

	// Balanced, but never posted — written straight through the repository, the
	// way a half-finished entry reaches storage.
	draft := gl.NewVoucher("v-draft", "Bút toán nháp", gl.VoucherTypeGeneral, firstDay)
	for _, line := range []gl.VoucherLine{
		{AccountCode: "1111", Side: gl.SideDebit, Amount: decimal.NewFromInt(500)},
		{AccountCode: "5111", Side: gl.SideCredit, Amount: decimal.NewFromInt(500)},
	} {
		if err := draft.AddLine(line); err != nil {
			t.Fatalf("build the draft: %v", err)
		}
	}

	if err := glrepo.NewVoucherStore(db).SaveVoucher(ctx, draft, ""); err != nil {
		t.Fatalf("save the draft: %v", err)
	}

	lines, err := source.Lines(ctx, time.Time{}, fy2026.EndDate)
	if err != nil {
		t.Fatalf("Lines: %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("a draft reached the ledger: %d lines, want none", len(lines))
	}

	if err := client.CreatePeriod(ctx, fy2026); err != nil {
		t.Fatalf("CreatePeriod: %v", err)
	}

	report, err := service.TrialBalance(ctx, periodID)
	if err != nil {
		t.Fatalf("TrialBalance: %v", err)
	}
	if len(report.Rows) != 0 || !report.PeriodDebit.IsZero() || !report.PeriodCredit.IsZero() {
		t.Fatalf("a draft reached the trial balance: %d rows, period debit %s credit %s",
			len(report.Rows), report.PeriodDebit, report.PeriodCredit)
	}
}

// Both ends of the day range are inside it, and a zero `from` means the
// beginning of the books rather than nothing at all.
//
// The zero case is not hypothetical: the trial balance and the balance sheet
// both ask for everything up to a period's end, because an account has to show
// what it carried in. An open end encoded as NULL would be never true rather
// than always true, and every balance sheet would read as though the client had
// never traded — while still rendering, and still looking like a report.
func TestSource_TheDayRangeIsInclusiveAndOpenAtTheStart(t *testing.T) {
	ctx, client, _ := newBooks(t)
	_, source := sourceOf(t, client)

	from := time.Date(2026, time.March, 5, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.March, 7, 0, 0, 0, 0, time.UTC)

	// An entry the day before the window opens, then one on each end of it.
	days := []time.Time{from.AddDate(0, 0, -1), from, to}

	for i, day := range days {
		postJournal(t, ctx, client, fmt.Sprintf("v-%d", i+1), journal{
			day:         day,
			description: "Kiểm tra khoảng ngày",
			lines: []movement{
				{account: "1111", side: gl.SideDebit, amount: 100},
				{account: "5111", side: gl.SideCredit, amount: 100},
			},
		})
	}

	lines, err := source.Lines(ctx, from, to)
	if err != nil {
		t.Fatalf("Lines: %v", err)
	}

	// Two entries inside the window, two lines each. The third is a day early.
	if len(lines) != 4 {
		t.Fatalf("lines inside the window = %d, want 4: both ends are inclusive, and only both ends", len(lines))
	}

	for _, l := range lines {
		if !l.VoucherDate.Equal(from) && !l.VoucherDate.Equal(to) {
			t.Errorf("a line dated %s is outside the window", l.VoucherDate.Format(time.DateOnly))
		}
	}

	// With no lower bound, all three entries are in the books.
	all, err := source.Lines(ctx, time.Time{}, to)
	if err != nil {
		t.Fatalf("Lines with no lower bound: %v", err)
	}
	if len(all) != 6 {
		t.Fatalf("lines to the end of the window = %d, want 6: a zero from means the beginning of the books", len(all))
	}
}

// figure is one report row, whichever report it came from.
//
// A trial balance row carries four columns and the others carry one, so the ones
// a report does not have stay zero and are not compared.
type figure struct {
	periodDebit   decimal.Decimal
	periodCredit  decimal.Decimal
	closingDebit  decimal.Decimal
	closingCredit decimal.Decimal
	amount        decimal.Decimal
	vouchers      int
}

// compare checks one report's rows against the same report read the other way.
func compare(t *testing.T, what string, onPage, inReport map[string]figure) {
	t.Helper()

	if len(onPage) != len(inReport) {
		t.Errorf("%s: the page has %d rows and the report has %d", what, len(onPage), len(inReport))
	}

	for code, want := range onPage {
		got, ok := inReport[code]
		if !ok {
			t.Errorf("%s: %s is on the page and not in the report", what, code)
			continue
		}

		if !want.periodDebit.Equal(got.periodDebit) || !want.periodCredit.Equal(got.periodCredit) {
			t.Errorf("%s: %s moved %s/%s on the page and %s/%s in the report",
				what, code, want.periodDebit, want.periodCredit, got.periodDebit, got.periodCredit)
		}
		if !want.closingDebit.Equal(got.closingDebit) || !want.closingCredit.Equal(got.closingCredit) {
			t.Errorf("%s: %s stands at %s/%s on the page and %s/%s in the report",
				what, code, want.closingDebit, want.closingCredit, got.closingDebit, got.closingCredit)
		}
		if !want.amount.Equal(got.amount) {
			t.Errorf("%s: %s is %s on the page and %s in the report", what, code, want.amount, got.amount)
		}
		if want.vouchers != got.vouchers {
			t.Errorf("%s: %s was summed from %d vouchers on the page and %d in the report",
				what, code, want.vouchers, got.vouchers)
		}
	}

	for code := range inReport {
		if _, ok := onPage[code]; !ok {
			t.Errorf("%s: %s is in the report and not on the page", what, code)
		}
	}
}

func profitFigures(rows []web.ProfitAndLossRow) map[string]figure {
	out := make(map[string]figure, len(rows))
	for _, r := range rows {
		out[r.Code] = figure{amount: decimal.NewFromInt(r.Amount.Units), vouchers: r.Vouchers}
	}

	return out
}

func profitReportFigures(rows []reports.ProfitAndLossRow) map[string]figure {
	out := make(map[string]figure, len(rows))
	for _, r := range rows {
		out[r.AccountCode] = figure{amount: r.Amount, vouchers: r.SourceVoucherCount}
	}

	return out
}

func balanceFigures(rows []web.BalanceSheetRow) map[string]figure {
	out := make(map[string]figure, len(rows))
	for _, r := range rows {
		out[r.Code] = figure{amount: decimal.NewFromInt(r.Amount.Units), vouchers: r.Vouchers}
	}

	return out
}

func balanceReportFigures(rows []reports.BalanceSheetRow) map[string]figure {
	out := make(map[string]figure, len(rows))
	for _, r := range rows {
		out[r.AccountCode] = figure{amount: r.Amount, vouchers: r.SourceVoucherCount}
	}

	return out
}

// same reports whether a figure a page carries equals one a report states.
//
// The pages hold whole đồng as an integer and the reports hold a decimal, because
// that is what the domain computes with. This is the one place the two
// representations meet, and comparing them is the test's job rather than the
// page's.
func same(page web.Amount, report decimal.Decimal) bool {
	return decimal.NewFromInt(page.Units).Equal(report)
}

// bothLedgers posts the same journals through both ledgers and returns the two
// ways of reading them.
//
// The same entries, so a difference in any report is a difference in how the
// ledger was read and nothing else.
func bothLedgers(t *testing.T) (*books.Books, *reports.Service) {
	t.Helper()

	ctx := context.Background()

	memory, err := books.New(books.WithClock(func() time.Time { return theDay }))
	if err != nil {
		t.Fatalf("books.New: %v", err)
	}

	for i, j := range theJournals {
		in := web.JournalInput{Date: j.day, Description: j.description}
		for _, l := range j.lines {
			in.Lines = append(in.Lines, web.JournalLineInput{
				AccountCode: l.account,
				Side:        string(l.side),
				Amount:      l.amount,
			})
		}

		if _, err := memory.CreateJournal(ctx, in); err != nil {
			t.Fatalf("in-memory journal %d: %v", i+1, err)
		}
	}

	_, client, _ := newBooks(t)

	if err := client.CreatePeriod(ctx, fy2026); err != nil {
		t.Fatalf("CreatePeriod: %v", err)
	}

	for i, j := range theJournals {
		postJournal(t, ctx, client, fmt.Sprintf("v-%d", i+1), j)
	}

	service, _ := sourceOf(t, client)

	return memory, service
}

// postJournal writes one entry to MariaDB and posts it.
func postJournal(t *testing.T, ctx context.Context, client *practice.Client, id string, j journal) {
	t.Helper()

	v := gl.NewVoucher(id, j.description, gl.VoucherTypeGeneral, j.day)

	for _, l := range j.lines {
		if err := v.AddLine(gl.VoucherLine{
			AccountCode: l.account,
			Side:        l.side,
			Amount:      decimal.NewFromInt(l.amount),
		}); err != nil {
			t.Fatalf("build voucher %s: %v", id, err)
		}
	}

	if err := client.Ledger().Post(ctx, ledger.Posting{
		Voucher:    v,
		At:         j.day,
		By:         "operator-1",
		RequestKey: "req-" + id,
	}); err != nil {
		t.Fatalf("post voucher %s: %v", id, err)
	}
}

// newBooks opens a fresh client database with the real TT133 chart in it.
//
// The database name is unique per test and dropped afterwards, so tests can run
// against one server without seeing each other's books.
func newBooks(t *testing.T) (context.Context, *practice.Client, *sql.DB) {
	t.Helper()

	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, name := testsupport.MigratedDatabase(t, cfg, admin)
	testsupport.SeedChart(t, db)

	ctx := context.Background()

	client, err := practice.Open(ctx, cfg, name)
	if err != nil {
		t.Fatalf("practice.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close the client: %v", err)
		}
	})

	return ctx, client, db
}

// sourceOf builds the reports over this adapter, reading a client's own pool.
func sourceOf(t *testing.T, client *practice.Client) (*reports.Service, *ledgerrepo.Source) {
	t.Helper()

	source := ledgerrepo.New(client.DB(), client, client)

	service, err := reports.NewService(source, source, source, close.TT133ClosingAccounts())
	if err != nil {
		t.Fatalf("reports.NewService: %v", err)
	}

	return service, source
}
