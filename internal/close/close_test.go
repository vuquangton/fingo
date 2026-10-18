package close

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"fingo/internal/document"
	"fingo/internal/domain/gl"
	"fingo/internal/domain/gl/gltest"
	"fingo/internal/money"
	"fingo/internal/period"
	"fingo/internal/usecase/ledger"
)

// The period the fixtures close, and the instant they close it at. Both are
// fixed so the assertions stay deterministic.
const closeVoucherID = "voucher-close-2026-01"

var (
	januaryStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	januaryEnd   = time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	closeAt      = time.Date(2026, 2, 1, 9, 30, 0, 0, time.UTC)
)

// testLedger is a ledger in memory.
//
// It is not a stub: it posts through the domain's own rules over the whole TT133
// chart, so a closing entry that named an account the chart does not hold, or a
// heading that cannot be posted to, would fail here exactly as it would in
// production. What it does not have is storage — the entries stay in a slice.
type testLedger struct {
	t *testing.T

	rules   gl.PostingRules
	entries []testEntry
	seq     int

	posts    int
	requests []string
	keys     map[string]bool

	// vouchers is one entry per Post, so a test can tell one posting carrying
	// several entries from several postings carrying one each.
	vouchers []string
}

// testEntry is one line as the books would hold it, with the date it was posted
// at — which is what makes a period's balances answerable — and the voucher it
// was posted on, which is what makes a single posting tellable from two.
type testEntry struct {
	date    time.Time
	voucher string
	line    gl.VoucherLine
}

func newTestLedger(t *testing.T) *testLedger {
	t.Helper()

	return &testLedger{t: t, rules: gltest.TT133Rules(t)}
}

// seed writes a movement into the books as an ordinary posting would have left
// it: a sale's revenue, a month's cost of goods.
func (b *testLedger) seed(code string, side gl.EntrySide, amount int64, at time.Time) *testLedger {
	b.entries = append(b.entries, testEntry{
		date: at,
		line: gl.VoucherLine{AccountCode: code, Side: side, Amount: money.FromInt(amount)},
	})

	return b
}

// Post numbers the voucher through the domain's rules and keeps its lines.
//
// The idempotency key is enforced as the real ledger enforces it: the rules are
// applied first, so an entry the ledger would refuse is refused without
// consuming a key, and the key is taken by the write — a second request under
// the same key is refused.
func (b *testLedger) Post(_ context.Context, p ledger.Posting) error {
	if p.Voucher == nil {
		return errors.New("a posting needs a voucher")
	}

	number := gltest.NumberFor(b.t, p.Voucher, b.seq+1)
	if err := p.Voucher.Post(b.rules, number, p.At, p.By); err != nil {
		return err
	}
	b.seq++

	if p.RequestKey != "" {
		if b.keys == nil {
			b.keys = map[string]bool{}
		}
		if b.keys[p.RequestKey] {
			return fmt.Errorf("%w (key %s)", gl.ErrDuplicateRequest, p.RequestKey)
		}
		b.keys[p.RequestKey] = true
	}

	b.posts++
	b.requests = append(b.requests, p.RequestKey)
	b.vouchers = append(b.vouchers, p.Voucher.ID())

	for _, l := range p.Voucher.Lines() {
		b.entries = append(b.entries, testEntry{
			date:    p.Voucher.VoucherDate(),
			voucher: p.Voucher.ID(),
			line:    l,
		})
	}

	return nil
}

// voucher returns the lines one voucher was posted with, as the books hold them.
func (b *testLedger) voucher(id string) []gl.VoucherLine {
	out := make([]gl.VoucherLine, 0)

	for _, e := range b.entries {
		if e.voucher == id {
			out = append(out, e.line)
		}
	}

	return out
}

// Balances is the question the close asks of the books: what do this period's
// revenue and expense accounts stand at?
func (b *testLedger) Balances(_ context.Context, p period.Period) ([]AccountBalance, error) {
	debits := map[string]money.Dong{}
	credits := map[string]money.Dong{}

	for _, e := range b.entries {
		if !p.Covers(e.date) {
			continue
		}

		switch e.line.Side {
		case gl.SideDebit:
			debits[e.line.AccountCode] = debits[e.line.AccountCode].Add(e.line.Amount)
		case gl.SideCredit:
			credits[e.line.AccountCode] = credits[e.line.AccountCode].Add(e.line.Amount)
		}
	}

	out := make([]AccountBalance, 0, len(debits)+len(credits))

	for _, code := range touched(debits, credits) {
		account, ok := b.rules.Chart.ByCode(code)
		if !ok {
			b.t.Fatalf("the books hold account %s, which the chart does not", code)
		}

		var amount money.Dong
		switch account.Type {
		case gl.AccountTypeRevenue:
			amount = credits[code].Sub(debits[code])
		case gl.AccountTypeExpense:
			amount = debits[code].Sub(credits[code])
		default:
			continue
		}

		if amount.IsZero() {
			continue
		}

		out = append(out, AccountBalance{AccountCode: code, Type: account.Type, Amount: amount})
	}

	return out, nil
}

// balance is the account's position as the books hold it, debits less credits.
func (b *testLedger) balance(code string) money.Dong {
	var amount money.Dong

	for _, e := range b.entries {
		if e.line.AccountCode != code {
			continue
		}

		if e.line.Side == gl.SideDebit {
			amount = amount.Add(e.line.Amount)
		} else {
			amount = amount.Sub(e.line.Amount)
		}
	}

	return amount
}

// touched returns, once each and in a stable order, every account the two sides
// have postings against.
func touched(sides ...map[string]money.Dong) []string {
	seen := map[string]bool{}
	out := make([]string, 0)

	for _, sums := range sides {
		for code := range sums {
			if seen[code] {
				continue
			}

			seen[code] = true
			out = append(out, code)
		}
	}

	sort.Strings(out)

	return out
}

// testPeriods is a calendar that remembers what has been locked.
type testPeriods struct {
	periods []period.Period

	// failLock is returned by the next Lock and then cleared, so a test can
	// make one close fail part-way and then run it again.
	failLock error
}

func (r *testPeriods) List(context.Context) ([]period.Period, error) {
	out := make([]period.Period, len(r.periods))
	copy(out, r.periods)

	return out, nil
}

func (r *testPeriods) Lock(_ context.Context, id string, at time.Time) error {
	if r.failLock != nil {
		err := r.failLock
		r.failLock = nil

		return err
	}

	for i := range r.periods {
		if r.periods[i].ID != id {
			continue
		}
		if r.periods[i].Locked {
			return fmt.Errorf("%w: %s", period.ErrAlreadyLocked, id)
		}

		lockedAt := at.UTC()
		r.periods[i].Locked = true
		r.periods[i].LockedAt = &lockedAt

		return nil
	}

	return fmt.Errorf("%w: %s", period.ErrUnknownPeriod, id)
}

func (r *testPeriods) find(id string) period.Period {
	for _, p := range r.periods {
		if p.ID == id {
			return p
		}
	}

	return period.Period{}
}

// testDocuments is an AR/AP book whose open documents are stated outright.
type testDocuments struct {
	open []document.Document
	err  error
}

func (d *testDocuments) OpenInPeriod(_ context.Context, periodID string) ([]document.Document, error) {
	if d.err != nil {
		return nil, d.err
	}

	out := make([]document.Document, 0, len(d.open))
	for _, doc := range d.open {
		if doc.PeriodID == periodID {
			out = append(out, doc)
		}
	}

	return out, nil
}

// testReports is the close's report book, with the option of failing a write.
//
// The write that can fail is the interesting one: reporting is the only part of
// a close that happens after the entries have reached the ledger.
type testReports struct {
	inner *MemoryReports

	// failSave is asked about each write in turn, counting from one, and the
	// write it returns an error for does not happen.
	failSave func(save int) error
	saves    int
}

func newTestReports() *testReports {
	return &testReports{inner: NewMemoryReports()}
}

func (r *testReports) Save(ctx context.Context, report Report) error {
	r.saves++

	if r.failSave != nil {
		if err := r.failSave(r.saves); err != nil {
			return err
		}
	}

	return r.inner.Save(ctx, report)
}

func (r *testReports) Get(ctx context.Context, periodID string) (Report, error) {
	return r.inner.Get(ctx, periodID)
}

// fixture is a close wired to a month's books.
type fixture struct {
	ledger  *testLedger
	periods *testPeriods
	docs    *testDocuments
	reports *testReports
	service *Service
	january period.Period
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	january := period.Period{ID: "2026-01", StartDate: januaryStart, EndDate: januaryEnd}

	books := newTestLedger(t)
	periods := &testPeriods{periods: []period.Period{january}}
	book := &testDocuments{}
	reports := newTestReports()

	service, err := NewService(periods, books, books, book, reports, TT133ClosingAccounts())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	return &fixture{
		ledger:  books,
		periods: periods,
		docs:    book,
		reports: reports,
		service: service,
		january: january,
	}
}

func (f *fixture) command() Command {
	return Command{
		PeriodID:  f.january.ID,
		VoucherID: closeVoucherID,
		Operator:  "kt-thuy",
		At:        closeAt,
	}
}

// trade seeds a month of trading: 1,000,000 of revenue and 600,000 of cost, so
// the period's profit is 400,000.
func (f *fixture) trade() *fixture {
	f.ledger.seed("5111", gl.SideCredit, 1_000_000, time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC))
	f.ledger.seed("632", gl.SideDebit, 600_000, time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC))

	return f
}

func (f *fixture) closeIt(ctx context.Context, t *testing.T) Report {
	t.Helper()

	report, err := f.service.ClosePeriod(ctx, f.command())
	if err != nil {
		t.Fatalf("ClosePeriod: %v", err)
	}

	return report
}

// A period's result becomes a fact about that period, and not a running total
// every later report has to re-derive.
func TestClosePeriod_CarriesThePeriodsResultToRetainedEarnings(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	report := f.closeIt(ctx, t)

	if want := money.FromInt(400_000); report.NetProfit.Cmp(want) != 0 {
		t.Errorf("net profit = %s, want %s", report.NetProfit, want)
	}

	// 4212 is an equity account carrying its balance on the credit side, so the
	// books are debits less credits: a profit of 400,000 stands as -400,000.
	if got, want := f.ledger.balance("4212"), money.FromInt(-400_000); got.Cmp(want) != 0 {
		t.Errorf("retained earnings = %s, want %s (a credit of the period's profit)", got, want)
	}
}

// Every revenue and expense account is left at zero, including the profit
// determination account the entries passed through.
func TestClosePeriod_LeavesEveryProfitAndLossAccountAtZero(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	f.closeIt(ctx, t)

	for _, code := range []string{"5111", "632", "911"} {
		if got := f.ledger.balance(code); !got.IsZero() {
			t.Errorf("account %s stands at %s after the close, want 0", code, got)
		}
	}

	balances, err := f.ledger.Balances(ctx, f.january)
	if err != nil {
		t.Fatalf("Balances: %v", err)
	}
	if len(balances) != 0 {
		t.Errorf("the period still carries profit and loss balances: %+v", balances)
	}
}

// A loss is carried the other way, and the accounts are still emptied.
func TestClosePeriod_CarriesALossToRetainedEarnings(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	march := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	f.ledger.seed("5111", gl.SideCredit, 100_000, march)
	f.ledger.seed("632", gl.SideDebit, 250_000, march)

	report := f.closeIt(ctx, t)

	if want := money.FromInt(-150_000); report.NetProfit.Cmp(want) != 0 {
		t.Errorf("net profit = %s, want %s", report.NetProfit, want)
	}
	if got, want := f.ledger.balance("4212"), money.FromInt(150_000); got.Cmp(want) != 0 {
		t.Errorf("retained earnings = %s, want %s (a debit of the period's loss)", got, want)
	}
	if got := f.ledger.balance("911"); !got.IsZero() {
		t.Errorf("the profit determination account stands at %s, want 0", got)
	}
}

// The closing entries are one request, named after the period. Two closes
// racing each other post one set of entries, not two that each carry the same
// profit to retained earnings.
func TestClosePeriod_NamesTheClosingRequestAfterThePeriod(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	f.closeIt(ctx, t)

	want := "period-close:" + f.january.ID
	if len(f.ledger.requests) != 1 || f.ledger.requests[0] != want {
		t.Errorf("requests = %v, want %q alone", f.ledger.requests, want)
	}
}

// A closed period is shut: nothing more is posted into a result that has been
// reported.
func TestClosePeriod_LocksThePeriodItClosed(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	f.closeIt(ctx, t)

	closed := f.periods.find(f.january.ID)
	if !closed.Locked {
		t.Fatal("the period is not locked after being closed")
	}
	if closed.LockedAt == nil || !closed.LockedAt.Equal(closeAt) {
		t.Errorf("locked at %v, want %v", closed.LockedAt, closeAt)
	}
}

// The snapshot says what was closed, and is what a later question about the
// period is answered from — the accounts are empty afterwards, so it cannot be
// recomputed.
func TestClosePeriod_RecordsTheSnapshot(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	report := f.closeIt(ctx, t)

	if report.VoucherNo == "" {
		t.Error("the report names no closing voucher, though entries were posted")
	}

	recorded, err := f.service.Report(ctx, f.january.ID)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}

	if recorded.PeriodID != f.january.ID {
		t.Errorf("report period = %q, want %q", recorded.PeriodID, f.january.ID)
	}
	if recorded.ClosedBy != "kt-thuy" || !recorded.ClosedAt.Equal(closeAt) {
		t.Errorf("report closed by %q at %v, want kt-thuy at %v", recorded.ClosedBy, recorded.ClosedAt, closeAt)
	}
	if recorded.VoucherID != closeVoucherID || recorded.VoucherNo != report.VoucherNo {
		t.Errorf("report voucher = %s/%s, want %s/%s",
			recorded.VoucherID, recorded.VoucherNo, closeVoucherID, report.VoucherNo)
	}
	if len(recorded.Revenues) != 1 || recorded.Revenues[0].AccountCode != "5111" {
		t.Errorf("report revenues = %+v, want 5111 alone", recorded.Revenues)
	}
	if len(recorded.Expenses) != 1 || recorded.Expenses[0].AccountCode != "632" {
		t.Errorf("report expenses = %+v, want 632 alone", recorded.Expenses)
	}
	if want := money.FromInt(400_000); recorded.NetProfit.Cmp(want) != 0 {
		t.Errorf("report net profit = %s, want %s", recorded.NetProfit, want)
	}
}

// A period with money still owed in it is not over, so it does not close.
func TestClosePeriod_RefusesAPeriodWithDocumentsStillOpen(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.docs.open = []document.Document{openInvoice("inv-1", f.january.ID, 5_000_000)}

	_, err := f.service.ClosePeriod(ctx, f.command())
	if !errors.Is(err, ErrOpenDocuments) {
		t.Fatalf("error = %v, want ErrOpenDocuments", err)
	}

	if f.periods.find(f.january.ID).Locked {
		t.Error("the period was locked even though the close was refused")
	}
	if f.ledger.posts != 0 {
		t.Errorf("the ledger holds %d posting(s), want none", f.ledger.posts)
	}
	if _, err := f.reports.Get(ctx, f.january.ID); !errors.Is(err, ErrNoReport) {
		t.Errorf("a report was recorded for a refused close: %v", err)
	}
}

// Forcing is for the cases a bookkeeper knows about and an engine cannot — but
// it has to be explained at the time, and the explanation is kept.
func TestClosePeriod_ForceClosesPastOpenDocumentsAndRecordsWhy(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.docs.open = []document.Document{openInvoice("inv-1", f.january.ID, 5_000_000)}

	cmd := f.command()
	cmd.Force = true
	cmd.Reason = "inv-1 is a bad debt, written off in February"

	report, err := f.service.ClosePeriod(ctx, cmd)
	if err != nil {
		t.Fatalf("forced ClosePeriod: %v", err)
	}

	if !report.Forced || report.ForceReason != cmd.Reason {
		t.Errorf("report says forced=%v reason=%q, want true and %q", report.Forced, report.ForceReason, cmd.Reason)
	}
	if len(report.OpenDocuments) != 1 || report.OpenDocuments[0].ID != "inv-1" {
		t.Errorf("report holds %+v, want the invoice the close was forced past", report.OpenDocuments)
	}
	if !f.periods.find(f.january.ID).Locked {
		t.Error("the forced close did not lock the period")
	}
}

func TestClosePeriod_RefusesToForceWithoutAReason(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.docs.open = []document.Document{openInvoice("inv-1", f.january.ID, 5_000_000)}

	cmd := f.command()
	cmd.Force = true

	if _, err := f.service.ClosePeriod(ctx, cmd); !errors.Is(err, ErrForceReasonRequired) {
		t.Fatalf("error = %v, want ErrForceReasonRequired", err)
	}
}

// A period closes once. Closing it again would carry the same result to
// retained earnings a second time.
func TestClosePeriod_RefusesAPeriodThatIsAlreadyLocked(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.closeIt(ctx, t)

	_, err := f.service.ClosePeriod(ctx, f.command())
	if !errors.Is(err, period.ErrAlreadyLocked) {
		t.Fatalf("error = %v, want period.ErrAlreadyLocked", err)
	}
	if f.ledger.posts != 1 {
		t.Errorf("the ledger holds %d posting(s), want the one closing entry", f.ledger.posts)
	}
}

func TestClosePeriod_RefusesAPeriodTheCalendarDoesNotHold(t *testing.T) {
	f := newFixture(t)

	cmd := f.command()
	cmd.PeriodID = "2026-07"

	if _, err := f.service.ClosePeriod(context.Background(), cmd); !errors.Is(err, period.ErrUnknownPeriod) {
		t.Fatalf("error = %v, want period.ErrUnknownPeriod", err)
	}
}

// A quiet month is still a month that is over. There is simply no voucher for
// it, because an empty voucher is not a voucher.
func TestClosePeriod_ClosesAQuietPeriodWithoutAVoucher(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	report := f.closeIt(ctx, t)

	if f.ledger.posts != 0 {
		t.Errorf("the ledger holds %d posting(s), want none for a period with nothing in it", f.ledger.posts)
	}
	if report.VoucherNo != "" {
		t.Errorf("report voucher no = %q, want empty", report.VoucherNo)
	}
	if !report.NetProfit.IsZero() {
		t.Errorf("net profit = %s, want 0", report.NetProfit)
	}
	if !f.periods.find(f.january.ID).Locked {
		t.Error("the period is not locked after being closed")
	}
}

// The profit determination account is only ever touched by a close, so a balance
// on it means an earlier close did not finish. Carrying it again would determine
// the period's result twice.
func TestClosePeriod_RefusesAResultAccountThatAlreadyCarriesABalance(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.ledger.seed("911", gl.SideDebit, 10_000, time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC))

	_, err := f.service.ClosePeriod(ctx, f.command())
	if !errors.Is(err, ErrResultAccountCarriesBalance) {
		t.Fatalf("error = %v, want ErrResultAccountCarriesBalance", err)
	}
	if f.ledger.posts != 0 {
		t.Error("entries were posted despite the period's result being part-determined")
	}
}

// The entries must be posted before the period is locked, because the ledger
// refuses a posting into a locked period. A failure in between therefore leaves
// a period whose accounts are closed but which is not locked — and running the
// close again finishes the job rather than posting the same result twice.
func TestClosePeriod_CanBeRetriedAfterTheLockFails(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.periods.failLock = errors.New("the database went away")

	if _, err := f.service.ClosePeriod(ctx, f.command()); err == nil {
		t.Fatal("the close reported success though the period was never locked")
	}
	if f.periods.find(f.january.ID).Locked {
		t.Fatal("the period was locked despite the lock failing")
	}

	report := f.closeIt(ctx, t)

	if !f.periods.find(f.january.ID).Locked {
		t.Error("the retry did not lock the period")
	}
	if f.ledger.posts != 1 {
		t.Errorf("the ledger holds %d posting(s), want exactly the one closing entry", f.ledger.posts)
	}
	if got := f.ledger.balance("4212"); got.Cmp(money.FromInt(-400_000)) != 0 {
		t.Errorf("retained earnings = %s, want 400000 of profit carried once", got)
	}
	if got := f.ledger.balance("5111"); !got.IsZero() {
		t.Errorf("revenue stands at %s, want 0", got)
	}
	if want := money.FromInt(400_000); report.NetProfit.Cmp(want) != 0 {
		t.Errorf("retried close net profit = %s, want %s", report.NetProfit, want)
	}

	// The retry finds the accounts already at zero. The snapshot of what was
	// closed is the earlier attempt's, and must not be overwritten with a period
	// that appears to have earned nothing.
	recorded, err := f.service.Report(ctx, f.january.ID)
	if err != nil {
		t.Fatalf("Report after the retry: %v", err)
	}
	if want := money.FromInt(400_000); recorded.NetProfit.Cmp(want) != 0 {
		t.Errorf("the retry recorded a profit of %s, want the %s it closed with", recorded.NetProfit, want)
	}
}

// The snapshot is written before the entries are posted, so a report that cannot
// be recorded stops the close before anything irreversible has happened: the
// books are untouched and nothing has to be undone.
func TestClosePeriod_PostsNothingWhenTheSnapshotCannotBeRecorded(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.reports.failSave = func(int) error { return errors.New("the disk went away") }

	if _, err := f.service.ClosePeriod(ctx, f.command()); err == nil {
		t.Fatal("the close reported success though its snapshot was never recorded")
	}

	if f.ledger.posts != 0 {
		t.Errorf("the ledger holds %d posting(s), want none", f.ledger.posts)
	}
	if got := f.ledger.balance("5111"); got.Cmp(money.FromInt(-1_000_000)) != 0 {
		t.Errorf("revenue stands at %s, want it untouched at -1000000", got)
	}
	if f.periods.find(f.january.ID).Locked {
		t.Error("the period was locked although the close never ran")
	}
}

// Writing the report again with the number of the voucher that closed the period
// can fail on its own. The figures are already on file by then, so the retry
// keeps them and finishes the job instead of recording a period that appears to
// have earned nothing.
func TestClosePeriod_KeepsTheSnapshotWhenRecordingTheVoucherFails(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.reports.failSave = func(save int) error {
		if save == 2 {
			return errors.New("the disk went away")
		}

		return nil
	}

	if _, err := f.service.ClosePeriod(ctx, f.command()); err == nil {
		t.Fatal("the close reported success though it could not record its voucher")
	}
	if f.ledger.posts != 1 {
		t.Fatalf("the ledger holds %d posting(s), want the closing entries", f.ledger.posts)
	}

	f.reports.failSave = nil
	report := f.closeIt(ctx, t)

	if want := money.FromInt(400_000); report.NetProfit.Cmp(want) != 0 {
		t.Errorf("the retry reports a profit of %s, want the %s it closed with", report.NetProfit, want)
	}
	if !f.periods.find(f.january.ID).Locked {
		t.Error("the retry did not lock the period")
	}
	if f.ledger.posts != 1 {
		t.Errorf("the ledger holds %d posting(s), want exactly the one closing entry", f.ledger.posts)
	}
}

// One period closes once. The request key names the period, so a close that
// would post a second set of entries — after a failure part-way and a late
// posting into the period, which is still open — is refused rather than carrying
// the period's result to retained earnings twice.
func TestClosePeriod_RefusesASecondSetOfClosingEntries(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()
	f.reports.failSave = func(save int) error {
		if save == 2 {
			return errors.New("the disk went away")
		}

		return nil
	}
	if _, err := f.service.ClosePeriod(ctx, f.command()); err == nil {
		t.Fatal("the close reported success though it could not record its voucher")
	}

	// A late invoice is posted into the period, which is still open, and the
	// close is run again.
	f.ledger.seed("5111", gl.SideCredit, 250_000, time.Date(2026, 1, 28, 0, 0, 0, 0, time.UTC))
	f.reports.failSave = nil

	_, err := f.service.ClosePeriod(ctx, f.command())
	if !errors.Is(err, gl.ErrDuplicateRequest) {
		t.Fatalf("error = %v, want gl.ErrDuplicateRequest", err)
	}
	if f.ledger.posts != 1 {
		t.Errorf("the ledger holds %d posting(s), want exactly the one closing entry", f.ledger.posts)
	}
}

// Periods close in order. Closing February over an open January would carry
// February's result while January's revenue stayed in the accounts — a "closed"
// period whose profit and loss accounts are not empty.
func TestClosePeriod_RefusesToCloseOutOfOrder(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t).trade()

	february := period.Period{
		ID:        "2026-02",
		StartDate: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC),
	}
	f.periods.periods = append(f.periods.periods, february)

	februaryCommand := f.command()
	februaryCommand.PeriodID = february.ID
	februaryCommand.At = time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

	if _, err := f.service.ClosePeriod(ctx, februaryCommand); !errors.Is(err, ErrEarlierPeriodOpen) {
		t.Fatalf("error = %v, want ErrEarlierPeriodOpen", err)
	}
	if f.periods.find(february.ID).Locked {
		t.Error("February was locked over an open January")
	}
	if f.ledger.posts != 0 {
		t.Errorf("the ledger holds %d posting(s), want none", f.ledger.posts)
	}

	// January first, then February, is the order that works.
	f.closeIt(ctx, t)

	if _, err := f.service.ClosePeriod(ctx, februaryCommand); err != nil {
		t.Fatalf("close February after January: %v", err)
	}
	if !f.periods.find(february.ID).Locked {
		t.Error("February was not locked")
	}
	if f.ledger.posts != 1 {
		t.Errorf("the ledger holds %d posting(s), want January's closing entry alone", f.ledger.posts)
	}
}

func TestClosePeriod_RefusesACommandThatCouldNotBeAttempted(t *testing.T) {
	f := newFixture(t)

	cases := map[string]func(Command) Command{
		"no period":  func(c Command) Command { c.PeriodID = "  "; return c },
		"no voucher": func(c Command) Command { c.VoucherID = ""; return c },
		"no operator": func(c Command) Command {
			c.Operator = ""
			return c
		},
		"no instant": func(c Command) Command { c.At = time.Time{}; return c },
	}

	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := f.service.ClosePeriod(context.Background(), spoil(f.command())); !errors.Is(err, ErrInvalidCommand) {
				t.Fatalf("error = %v, want ErrInvalidCommand", err)
			}
		})
	}
}

func TestNewService_RefusesAnIncompleteConfiguration(t *testing.T) {
	f := newFixture(t)
	books := newTestLedger(t)
	periods := &testPeriods{periods: []period.Period{f.january}}
	docs := &testDocuments{}
	reports := NewMemoryReports()
	accounts := TT133ClosingAccounts()

	cases := map[string]func() (*Service, error){
		"no calendar": func() (*Service, error) {
			return NewService(nil, books, books, docs, reports, accounts)
		},
		"no balances": func() (*Service, error) {
			return NewService(periods, nil, books, docs, reports, accounts)
		},
		"nowhere to post": func() (*Service, error) {
			return NewService(periods, books, nil, docs, reports, accounts)
		},
		"no documents": func() (*Service, error) {
			return NewService(periods, books, books, nil, reports, accounts)
		},
		"no reports": func() (*Service, error) {
			return NewService(periods, books, books, docs, nil, accounts)
		},
		"no result account": func() (*Service, error) {
			return NewService(periods, books, books, docs, reports, ClosingAccounts{Retained: "4212"})
		},
		"one account for both": func() (*Service, error) {
			return NewService(periods, books, books, docs, reports, ClosingAccounts{Result: "911", Retained: "911"})
		},
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := build(); err == nil {
				t.Fatal("an incomplete configuration built a close")
			}
		})
	}
}

// A document still owed in the period, as the AR book would hold it.
func openInvoice(id, periodID string, total int64) document.Document {
	return document.Document{
		ID:        id,
		Type:      document.TypeInvoice,
		PartnerID: "cust-1",
		Date:      januaryStart,
		PeriodID:  periodID,
		Total:     money.New(total, money.Currency("VND")),
		Allocated: money.New(0, money.Currency("VND")),
		Status:    document.StatusOpen,
	}
}
