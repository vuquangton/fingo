// Package close closes an accounting period.
//
// Closing is one act with four parts, and they are one act because any one of
// them done alone leaves books that disagree with themselves:
//
//  1. Nothing is still owed in the period. A period whose sales are recorded but
//     whose invoices are not settled has a result that is about to move, and
//     reporting it as closed would state a figure that is not yet settled.
//  2. The period's revenue and expense accounts are carried to retained
//     earnings through the profit determination account, so the period's result
//     is a fact about that period. Left open, "profit this year" is a running
//     total that every later report has to re-derive, and cannot.
//  3. The period is locked (period.Repository.Lock — the period package is the
//     only thing that decides what "locked" means), so nothing more can be
//     posted into a result that has been reported. There is no counterpart:
//     reopening is not supported in v1, and a lock that can be undone is a
//     preference rather than a lock. See docs/adr/0002-posting-lifecycle.md.
//  4. The snapshot is written down: what the period earned, what it spent, what
//     it left, the voucher that did it, and who did it. The ledger's entries say
//     what happened; the report says what was closed, and is what a later
//     question about the close is answered from.
//
// The order is forced. The entries must be posted before the period is locked,
// because the ledger refuses a posting into a locked period — including the
// closing entry itself. So the snapshot is written down first and the entries
// after it: a failure part-way leaves the figures on file (they exist nowhere
// else once the accounts are emptied) and a period that is not locked, and
// running the close again finds nothing left to post, keeps the snapshot it
// already has, and finishes the job. The close is safe to retry, and a retry is
// what a failure asks for.
//
// Periods close in order. A close that skipped an open period would carry this
// period's result while the earlier period's revenue and expenses stayed in the
// accounts, which is a "closed" period whose profit and loss accounts are not
// empty — see ErrEarlierPeriodOpen.
package close

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fingo/internal/document"
	"fingo/internal/domain/gl"
	"fingo/internal/money"
	"fingo/internal/period"
	"fingo/internal/usecase/ledger"
)

// Errors a caller may need to tell apart.
var (
	// ErrOpenDocuments is a close of a period that still has documents unsettled
	// in it. The close is refused unless it is forced.
	ErrOpenDocuments = errors.New("the period still has open documents")

	// ErrForceReasonRequired is a forced close with no reason recorded. Forcing
	// past documents that are still owed is exactly the thing an auditor asks
	// about, so the answer has to be written down at the time.
	ErrForceReasonRequired = errors.New("forcing a close past open documents needs a reason")

	// ErrResultAccountCarriesBalance is a close of a period whose profit
	// determination account already carries a balance. That account is only
	// ever touched by a close, so a balance on it means an earlier close did not
	// finish — and carrying it again would determine the period's result twice.
	ErrResultAccountCarriesBalance = errors.New("the profit determination account already carries a balance")

	// ErrEarlierPeriodOpen is a close of a period whose predecessor is still
	// open. Periods close in order: a close that skipped one would carry this
	// period's result while the earlier period's revenue and expenses stayed in
	// the accounts.
	ErrEarlierPeriodOpen = errors.New("an earlier accounting period is still open")

	// ErrNoReport is a lookup of a close report for a period that has none.
	ErrNoReport = errors.New("no close report has been recorded for that period")

	// ErrInvalidCommand is a close that could not be attempted: no period, no
	// voucher, no operator, or no instant to record it at.
	ErrInvalidCommand = errors.New("close: invalid command")
)

// Command describes a period to close.
type Command struct {
	// PeriodID names the period to close.
	PeriodID string

	// VoucherID is the id of the closing voucher. The caller supplies it
	// because it is the handle the entries are looked up by; the ledger
	// allocates the number.
	VoucherID string

	// Operator is who is closing the period.
	Operator string

	// At is the instant the close is recorded at: posted_at on the voucher, the
	// lock's timestamp, and the report's ClosedAt.
	At time.Time

	// Force closes a period that still has documents open in it. It is for the
	// cases a bookkeeper knows about and an engine cannot — a receivable that
	// will never be collected, an invoice written in the wrong period — and it
	// demands the reason below.
	Force bool

	// Reason explains a forced close. Required when Force is set.
	Reason string
}

// Validate reports whether the command describes a close that could be
// attempted.
func (c Command) Validate() error {
	if strings.TrimSpace(c.PeriodID) == "" {
		return fmt.Errorf("%w: a close needs the period it is closing", ErrInvalidCommand)
	}
	if strings.TrimSpace(c.VoucherID) == "" {
		return fmt.Errorf("%w: a close needs the id of its closing voucher", ErrInvalidCommand)
	}
	if strings.TrimSpace(c.Operator) == "" {
		return fmt.Errorf("%w: a close needs the operator who performed it", ErrInvalidCommand)
	}
	if c.At.IsZero() {
		return fmt.Errorf("%w: a close needs the instant it happened", ErrInvalidCommand)
	}
	if c.Force && strings.TrimSpace(c.Reason) == "" {
		return ErrForceReasonRequired
	}

	return nil
}

// Periods supplies the client's calendar and locks a period.
//
// period.Repository satisfies it. The port is narrower than the repository
// because a close creates no periods: it reads the calendar to find the period,
// and locks the one it closed.
type Periods interface {
	// List returns every period, so the one being closed can be found.
	List(ctx context.Context) ([]period.Period, error)

	// Lock marks a period locked, recording when.
	Lock(ctx context.Context, id string, at time.Time) error
}

// Ledger answers what a period's revenue and expense accounts stand at.
//
// Declared here, where it is used: the close needs one question answered and
// should not know whether balances are summed from vouchers, read from a
// materialised table, or held in memory.
//
// The adapter must return an account's movements *within the period asked
// about*. Once every earlier period has been closed those accounts stand at
// zero, so the period's movements are the account's balance at its end — and a
// close that read a running total instead would carry the same profit to
// retained earnings every time it ran.
//
// The profit determination account is reported like any other expense account.
// The close relies on seeing it: a balance on it means an earlier close did not
// finish, and an adapter that quietly left it out would let the same result be
// determined twice.
type Ledger interface {
	// Balances returns the period's revenue and expense accounts and their
	// balances, in each account's own sense. Accounts with nothing in them may
	// be left out; the profit determination account may not.
	Balances(ctx context.Context, p period.Period) ([]AccountBalance, error)
}

// Poster admits a posting to the ledger.
//
// Declared here, where it is used. The close hands its entries to the ledger and
// does not number, validate or store a voucher itself — a close that could write
// one would be a second door into the books.
//
// The poster numbers the voucher and marks it posted, which is what the close
// reads back off it for the report. ledger.Service does.
type Poster interface {
	Post(ctx context.Context, p ledger.Posting) error
}

// Documents reports what is still owed in a period.
type Documents interface {
	// OpenInPeriod returns the period's unsettled documents.
	//
	// A settled document is not owed, and a voided one is owed to nobody, so
	// neither is open. An empty result is the ordinary case: a month with
	// nothing outstanding is a month that may be closed.
	OpenInPeriod(ctx context.Context, periodID string) ([]document.Document, error)
}

// Reports records a close's snapshot, and reads it back.
type Reports interface {
	// Save records the report for its period, replacing any earlier one. A
	// close that was retried after its report was written records the same
	// facts again rather than failing.
	Save(ctx context.Context, r Report) error

	// Get returns the report recorded for a period, or ErrNoReport.
	Get(ctx context.Context, periodID string) (Report, error)
}

// Service closes periods. It is the only production route to a closed period.
type Service struct {
	periods   Periods
	ledger    Ledger
	poster    Poster
	documents Documents
	reports   Reports
	accounts  ClosingAccounts

	// provisions is nil for a close that provides for nothing, which is what a
	// close is until a client has assets or receivables to provide for. See
	// WithProvisions.
	provisions *Provisions
}

// WithProvisions returns the close with the period-end provision calculation
// enabled: the assets it depreciates and the receivables it provides against.
//
// The receiver is a value, so the close it is called on is left as it was and
// the result is the close that provides — a Service is a configuration, and a
// method that changed one in place would make the configuration something a
// caller could be surprised by later.
//
// Nil provisions — the default — means the close carries only what is already in
// the books, which is what every close did before provisions existed.
func (s Service) WithProvisions(p Provisions) (*Service, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	// A provision posts to expenses and to its own accumulation account. It may
	// not post through the profit determination account or into retained
	// earnings: those two are the close's, and an entry that reached either
	// would move the period's result by being posted rather than by being
	// earned.
	for _, named := range []struct{ name, code string }{
		{"depreciation expense", p.Accounts.DepreciationExpense},
		{"accumulated depreciation", p.Accounts.AccumulatedDepreciation},
		{"bad debt expense", p.Accounts.BadDebtExpense},
		{"bad debt provision", p.Accounts.BadDebtProvision},
	} {
		if named.code == s.accounts.Result || named.code == s.accounts.Retained {
			return nil, fmt.Errorf(
				"%w: the %s account is %s, which is where the close carries the period's result",
				ErrInvalidProvision, named.name, named.code)
		}
	}

	// The bands are copied, so that a caller changing their policy afterwards
	// does not change what a configured close provides against.
	p.BadDebt.Bands = append([]AgingBand(nil), p.BadDebt.Bands...)
	s.provisions = &p

	return &s, nil
}

// provide is what the close provides for at the period's end: nothing at all
// when it has no provision calculation, and the depreciation and bad debt
// entries otherwise.
func (s *Service) provide(ctx context.Context, p period.Period) (provision, error) {
	if s.provisions == nil {
		return provision{}, nil
	}

	asOf := p.EndDate.UTC()

	assets, err := s.provisions.Assets.Depreciable(ctx, asOf)
	if err != nil {
		return provision{}, fmt.Errorf("read the assets in service at %s: %w",
			asOf.Format(time.DateOnly), err)
	}

	receivables, err := s.provisions.Receivables.Outstanding(ctx, asOf)
	if err != nil {
		return provision{}, fmt.Errorf("read the receivables outstanding at %s: %w",
			asOf.Format(time.DateOnly), err)
	}

	// The chart is read before any entry is built, not after. Every account a
	// provision posts to is checked against it, so a client whose accumulated
	// depreciation account is a heading finds out here — rather than after the
	// close has written down a snapshot of provisions the ledger then refuses.
	chart, err := s.provisions.Charts.Chart(ctx)
	if err != nil {
		return provision{}, fmt.Errorf("load the chart of accounts: %w", err)
	}

	return provisionsFor(
		assets, receivables,
		monthsSpanned(p.StartDate, p.EndDate), asOf,
		provisionConfig{
			accounts: s.provisions.Accounts,
			badDebt:  s.provisions.BadDebt,
			closing:  s.accounts,
			chart:    chart,
		},
	)
}

// NewService builds the close.
//
// The accounts are injected and checked here rather than defaulted, for the same
// reason posting.Load checks its rules at boot: "the result account is 911" is a
// default and not a fact, and a close that guessed would post to an account the
// client may have moved. What is checked here is that a pair was supplied and
// that it cannot post an account into itself; whether the client's chart will
// accept a posting to either one is the ledger's business, and is refused when
// the entries are posted.
func NewService(
	periods Periods,
	ledger Ledger,
	poster Poster,
	documents Documents,
	reports Reports,
	accounts ClosingAccounts,
) (*Service, error) {
	switch {
	case periods == nil:
		return nil, errors.New("a close needs the client's accounting periods")
	case ledger == nil:
		return nil, errors.New("a close needs the ledger it reads the period's balances from")
	case poster == nil:
		return nil, errors.New("a close needs the ledger it posts the closing entries to")
	case documents == nil:
		return nil, errors.New("a close needs the documents that must be settled before a period may be closed")
	case reports == nil:
		return nil, errors.New("a close needs somewhere to record its report")
	}
	if err := accounts.Validate(); err != nil {
		return nil, err
	}

	return &Service{
		periods:   periods,
		ledger:    ledger,
		poster:    poster,
		documents: documents,
		reports:   reports,
		accounts:  accounts,
	}, nil
}

// ClosePeriod closes one period: it checks that nothing is outstanding in it,
// carries its revenue and expenses to retained earnings, records the snapshot,
// and locks it.
//
// Nothing here is reversible. A period that has been closed cannot be reopened
// in v1 — the entries are in the books and the period is shut — which is why the
// open-documents check comes first and defeats the whole operation when it
// fails.
func (s *Service) ClosePeriod(ctx context.Context, cmd Command) (Report, error) {
	if err := cmd.Validate(); err != nil {
		return Report{}, err
	}

	calendar, err := s.calendar(ctx)
	if err != nil {
		return Report{}, err
	}

	p, err := find(calendar, cmd.PeriodID)
	if err != nil {
		return Report{}, err
	}
	if p.Locked {
		return Report{}, fmt.Errorf("%w: %s", period.ErrAlreadyLocked, p.ID)
	}

	// Periods close in order. An earlier period that is still open holds
	// revenue and expenses this close would not carry, and carrying this
	// period's result without it would leave the profit and loss accounts
	// standing at something other than zero — a "closed" period whose accounts
	// a later report would still be reading.
	if earlier, open := openBefore(calendar, p); open {
		return Report{}, fmt.Errorf(
			"%w: %s covers %s to %s and is not locked, so %s cannot be closed before it",
			ErrEarlierPeriodOpen, earlier.ID,
			earlier.StartDate.UTC().Format(time.DateOnly), earlier.EndDate.UTC().Format(time.DateOnly),
			p.ID)
	}

	open, err := s.documents.OpenInPeriod(ctx, p.ID)
	if err != nil {
		return Report{}, fmt.Errorf("read the documents open in period %s: %w", p.ID, err)
	}
	if len(open) > 0 && !cmd.Force {
		return Report{}, fmt.Errorf(
			"%w: %s has %d document(s) unsettled in it, worth %s in total",
			ErrOpenDocuments, p.ID, len(open), outstandingTotal(open))
	}

	balances, err := s.ledger.Balances(ctx, p)
	if err != nil {
		return Report{}, fmt.Errorf("read the balances of period %s: %w", p.ID, err)
	}

	// held is the period as the ledger holds it, before anything this close
	// provides for is folded in. It is what tells a retry whether anything has
	// been posted into the period since it was closed.
	held := balances

	// What the period provides for is worked out before its result is, because
	// the provisions are part of that result: depreciation and a bad debt
	// provision are expenses of the period, and the closing entries carry every
	// expense to the result account.
	provided, err := s.provide(ctx, p)
	if err != nil {
		return Report{}, fmt.Errorf("close period %s: %w", p.ID, err)
	}

	balances, err = withProvisions(held, provided.expenses)
	if err != nil {
		return Report{}, fmt.Errorf("close period %s: %w", p.ID, err)
	}

	lines, profit, err := closingEntries(balances, s.accounts)
	if err != nil {
		return Report{}, fmt.Errorf("close period %s: %w", p.ID, err)
	}

	// One voucher: the provisions that were charged, and then the entries that
	// clear what the period earned and spent into retained earnings. Written as
	// one, they are one transaction — a close that recorded a provision and then
	// failed before carrying it would leave an expense standing in a period that
	// says it is closed.
	lines = append(append(make([]gl.VoucherLine, 0, len(provided.lines)+len(lines)), provided.lines...), lines...)

	report := Report{
		PeriodID:      p.ID,
		StartDate:     p.StartDate,
		EndDate:       p.EndDate,
		Revenues:      revenuesOf(balances),
		Expenses:      expensesOf(balances),
		NetProfit:     profit,
		Provisions:    provided.summary,
		ClosedAt:      cmd.At.UTC(),
		ClosedBy:      cmd.Operator,
		VoucherID:     cmd.VoucherID,
		Forced:        cmd.Force,
		ForceReason:   strings.TrimSpace(cmd.Reason),
		OpenDocuments: open,
	}

	// recorded is the report of an earlier attempt at this period, when there was
	// one. It is what a retry reads to find out how far the earlier attempt got.
	earlier, err := s.reports.Get(ctx, p.ID)
	if err != nil && !errors.Is(err, ErrNoReport) {
		return Report{}, fmt.Errorf("close period %s: read the report of an earlier attempt: %w", p.ID, err)
	}
	recorded := err == nil

	switch {
	case recorded && len(lines) == 0:
		// Nothing left to post, and a snapshot is already on file: an earlier
		// attempt posted the entries and did not get as far as locking the
		// period. Its snapshot is the truth about what was closed, and this run
		// — which finds the accounts already at zero — must not overwrite it
		// with a period that appears to have earned nothing.
		report = earlier
	default:
		// The snapshot is written down before the entries are posted, because
		// the figures on it exist nowhere else once the accounts are emptied. A
		// failure after this point leaves a report that says what the close set
		// out to record; the period is not locked, so the close is not
		// finished, and running it again completes it.
		//
		// A snapshot an earlier attempt already wrote is left where it is until
		// this one has something better: it was taken from books that attempt
		// went on to close, and a fresh one — read from the emptied accounts —
		// would say the period earned nothing.
		if !recorded {
			if err := s.reports.Save(ctx, report); err != nil {
				return Report{}, fmt.Errorf("close period %s: record the report: %w", p.ID, err)
			}
		}
	}

	// A period with nothing in it still closes and still locks — a quiet month
	// is a month that is over. There is simply no voucher to post, and an empty
	// voucher is not a voucher.
	if len(lines) > 0 {
		voucher := gl.NewVoucher(cmd.VoucherID, description(p, len(provided.lines) > 0), gl.VoucherTypeGeneral, p.EndDate)
		for _, line := range lines {
			if err := voucher.AddLine(line); err != nil {
				return Report{}, fmt.Errorf("close period %s: %w", p.ID, err)
			}
		}

		posting := ledger.Posting{
			Voucher: voucher,
			At:      cmd.At,
			By:      cmd.Operator,

			// One period closes once. The key names the period rather than the
			// attempt, so two closes racing each other cannot both post entries
			// that zero the same accounts and each carry the same profit to
			// retained earnings: the second is refused as a duplicate request.
			RequestKey: "period-close:" + p.ID,
		}

		switch err := s.poster.Post(ctx, posting); {
		case err == nil:
			report.VoucherNo = voucher.VoucherNo()

			// Written again now that the ledger has numbered the voucher, so the
			// snapshot names the entry that closed the period.
			if err := s.reports.Save(ctx, report); err != nil {
				return Report{}, fmt.Errorf("close period %s: record the voucher that closed it: %w", p.ID, err)
			}
		case errors.Is(err, gl.ErrDuplicateRequest) && recorded && nothingLeft(held):
			// The entries for this period are already in the ledger: an earlier
			// attempt posted them and did not get as far as locking the period.
			// The books hold nothing since — every profit and loss account
			// stands at zero — so the entries this close built are that
			// attempt's entries over again, and the snapshot it recorded is the
			// truth about the period. Finishing its work is not a failure.
			report = earlier
		case errors.Is(err, gl.ErrDuplicateRequest):
			return Report{}, fmt.Errorf(
				"close period %s: the closing entries are already in the ledger and the period has been posted to since, so it cannot be closed by posting them a second time: %w",
				p.ID, err)
		default:
			return Report{}, fmt.Errorf("close period %s: post the closing entries: %w", p.ID, err)
		}
	}

	if err := s.periods.Lock(ctx, p.ID, cmd.At); err != nil {
		return Report{}, fmt.Errorf("close period %s: lock the period: %w", p.ID, err)
	}

	return report, nil
}

// Report returns the snapshot recorded when a period was closed.
func (s *Service) Report(ctx context.Context, periodID string) (Report, error) {
	return s.reports.Get(ctx, periodID)
}

// calendar loads the client's accounting periods.
func (s *Service) calendar(ctx context.Context) ([]period.Period, error) {
	periods, err := s.periods.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("load accounting periods: %w", err)
	}

	return periods, nil
}

// find returns the period with an id, or the period package's error saying no
// period has it.
func find(periods []period.Period, id string) (period.Period, error) {
	for _, p := range periods {
		if p.ID == id {
			return p, nil
		}
	}

	return period.Period{}, fmt.Errorf("%w: %s", period.ErrUnknownPeriod, id)
}

// openBefore returns the latest period that ends before this one and is not
// locked, if there is one.
//
// A period is before another when it starts earlier and its end is not inside
// it — which, for a calendar of contiguous periods that cannot overlap, means it
// ends before. The times of day are deliberately not compared: a period is a
// range of dates, exactly as internal/period holds it.
func openBefore(periods []period.Period, p period.Period) (period.Period, bool) {
	var (
		latest period.Period
		open   bool
	)

	for _, other := range periods {
		if !other.StartDate.Before(p.StartDate) || p.Covers(other.EndDate) {
			continue
		}
		if other.Locked {
			continue
		}
		if !open || other.EndDate.After(latest.EndDate) {
			latest, open = other, true
		}
	}

	return latest, open
}

// description is what the closing voucher says it is.
//
// A ledger line with no explanation cannot be defended to an auditor, and this
// one is read months later by somebody asking why the revenue accounts are
// empty. It therefore names the period and says what was done to it.
func description(p period.Period, provisioned bool) string {
	what := "revenue and expenses carried to retained earnings"
	if provisioned {
		what = "provisions posted, then revenue and expenses carried to retained earnings"
	}

	return fmt.Sprintf("Period close %s (%s to %s): %s",
		p.ID, p.StartDate.UTC().Format(time.DateOnly), p.EndDate.UTC().Format(time.DateOnly), what)
}

// outstandingTotal is what the open documents are still owed, for the error
// message that refuses the close.
//
// The amounts are in one currency in practice. Where they are not, this says so
// rather than adding two currencies together into a number that would be wrong
// whichever one it named.
func outstandingTotal(open []document.Document) string {
	total := money.Money{}
	currency := money.Currency("")

	for _, d := range open {
		amount := d.Outstanding()
		if amount.IsZero() {
			continue
		}
		if currency == "" {
			currency, total = amount.Currency(), amount
			continue
		}
		if amount.Currency() != currency {
			return "more than one currency"
		}

		total = total.Add(amount)
	}

	if currency == "" {
		return "nothing"
	}

	return total.String()
}

// revenuesOf and expensesOf split the period's balances the way the report
// states them.
func revenuesOf(balances []AccountBalance) []AccountBalance {
	return ofType(balances, gl.AccountTypeRevenue)
}

func expensesOf(balances []AccountBalance) []AccountBalance {
	return ofType(balances, gl.AccountTypeExpense)
}

func ofType(balances []AccountBalance, want gl.AccountType) []AccountBalance {
	out := make([]AccountBalance, 0, len(balances))
	for _, b := range balances {
		if b.Type == want {
			out = append(out, b)
		}
	}

	return out
}
