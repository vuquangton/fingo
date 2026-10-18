// Package books is the in-memory implementation of the pages' Books port.
//
// It is not a mock. The chart of accounts is gl.ChartOfAccounts, the AR/AP
// documents and their settlement rules are internal/document, the tax is
// internal/vat, the posting decisions are internal/posting's TT133 rule set, and
// every report is internal/reports reading posted lines. What is missing is the
// storage: the vouchers live in a slice, so the application runs and its tests
// pass with no database.
//
// That is a deliberate seam rather than a shortcut. The pages depend on
// web.Books, which says what they ask for; this file says how one answer is
// assembled. The MariaDB adapter is a second implementation of the same port and
// nothing above it changes when it arrives.
//
// Known gap: there is no partner directory here. The invoice form takes the
// customer's name as free text and it is stored as the document's partner
// identifier. internal/partner is where that becomes a real lookup.
package books

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"fingo/internal/close"
	"fingo/internal/document"
	domaindoc "fingo/internal/domain/document"
	"fingo/internal/domain/gl"
	"fingo/internal/money"
	"fingo/internal/period"
	"fingo/internal/posting"
	"fingo/internal/vat"
	"fingo/web"
)

// currency is the only currency these books are kept in.
//
// VND has no subunit, so a minor unit is a đồng and nothing on this path
// divides. A second currency is a real feature and not a matter of adding a
// column: internal/money exists so that the day it arrives, the amounts already
// name their currency.
const currency = money.Currency("VND")

// Option configures a set of books.
type Option func(*Books)

// WithClock replaces the clock the books read "today" from.
//
// Injected so that a test can put the books in a known period without waiting
// for the calendar to agree with it.
func WithClock(now func() time.Time) Option {
	return func(b *Books) {
		if now != nil {
			b.now = now
		}
	}
}

// Books is a client's books held in memory.
type Books struct {
	mu sync.Mutex

	chart   *gl.ChartOfAccounts
	periods []period.Period
	vat     *vat.Registry
	docs    *document.MemoryRepository

	// codes is the chart's account codes, in order.
	//
	// gl.ChartOfAccounts looks an account up by code but does not enumerate
	// them, and the chart screen has to list them. Keeping the order here
	// rather than sorting a copy on every read also means the chart is browsed
	// in the order it was written.
	codes []string

	// vouchers is the ledger: every posted line, in the order it was posted.
	vouchers []entry

	// journals is the manual journal entries. A journal is a voucher and not an
	// AR/AP document — nobody owes it and it settles nothing — so it is kept
	// beside the document book rather than forced into it.
	journals []journal

	// notes is a document's description. internal/document carries what a
	// document is worth and not what it is for; the narration lives here.
	notes map[string]string
	// partners is a document's counterparty, by the name the form collected.
	partners map[string]string

	seq int
	now func() time.Time
}

// entry is one posted line.
type entry struct {
	VoucherID   string
	VoucherDate time.Time
	AccountCode string
	Side        gl.EntrySide
	Amount      decimal.Decimal
}

// journal is one manual journal entry.
type journal struct {
	ID          string
	Date        time.Time
	Description string
	Lines       []entry
}

// New builds a client's books, with the TT133 chart, a period for the year and
// a standard VAT rate.
func New(opts ...Option) (*Books, error) {
	b := &Books{
		chart:    gl.NewChartOfAccounts(),
		vat:      vat.New(),
		docs:     document.NewMemoryRepository(),
		notes:    make(map[string]string),
		partners: make(map[string]string),
		now:      func() time.Time { return time.Now().UTC() },
	}

	for _, opt := range opts {
		opt(b)
	}

	if err := b.seed(); err != nil {
		return nil, err
	}

	return b, nil
}

// seed installs the chart, a period and a VAT rate.
//
// A client with no chart cannot keep books and a client with no period cannot
// post: both are required before the first page renders, because a UI that opens
// on an empty screen and refuses every entry is worse than one that says what is
// missing.
func (b *Books) seed() error {
	for _, account := range gl.TT133Chart() {
		if err := b.chart.Add(account); err != nil {
			return fmt.Errorf("seed the chart of accounts: %w", err)
		}

		b.codes = append(b.codes, account.Code)
	}
	if err := b.chart.Validate(); err != nil {
		return fmt.Errorf("seed the chart of accounts: %w", err)
	}

	sort.Strings(b.codes)

	year := b.now().Year()
	from := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(year, time.December, 31, 0, 0, 0, 0, time.UTC)

	b.periods = []period.Period{{ID: fmt.Sprintf("FY%d", year), StartDate: from, EndDate: to}}

	// The standard rate. Others — the 8% window, a zero rate — are added from
	// the settings screen, and the registry refuses a window that overlaps one
	// it already holds.
	if err := b.vat.Create(vat.VATRate{
		ID:            "VAT10",
		Rate:          vat.BasisPoints(1000),
		EffectiveFrom: from,
	}); err != nil {
		return fmt.Errorf("seed the VAT rates: %w", err)
	}

	return nil
}

// current returns the period a posting today would land in, and whether there is
// one.
//
// The rule is the calendar's, not this implementation's: a second copy of it
// here is a second answer, and the two would disagree the first time one of them
// was changed. See period.Current.
func (b *Books) current() (period.Period, bool) {
	return period.Current(b.periods, b.now())
}

// periodFor returns the period a date falls in.
//
// A document outside every period is refused rather than filed somewhere: which
// period an entry belongs to decides which year's accounts it changes, and
// guessing is the one thing a ledger must not do.
func (b *Books) periodFor(date time.Time) (period.Period, error) {
	for _, p := range b.periods {
		if p.Covers(date) {
			return p, nil
		}
	}

	return period.Period{}, fmt.Errorf(
		"no accounting period covers %s; add one in Settings before posting to it",
		date.Format(time.DateOnly))
}

// periodRow renders a period for a page.
func (b *Books) periodRow(p period.Period) web.PeriodRow {
	current, ok := b.current()

	return web.NewPeriodRow(p, ok && current.ID == p.ID)
}

// nextID mints the next identifier for a kind of record.
//
// Readable and sequential rather than a UUID: this is an identifier an operator
// reads off a screen and quotes in a message, and "inv-0007" survives that where
// a hex string does not.
func (b *Books) nextID(prefix string) string {
	b.seq++

	return fmt.Sprintf("%s-%04d", prefix, b.seq)
}

// resolve returns the account code the TT133 posting rules name for a decision.
//
// The rules are asked rather than read off a table written here: 131 belongs on
// the debit side of a sales invoice because the rule set says so, and a second
// copy of that answer is a second answer.
func (b *Books) resolve(rules *posting.RuleSet, kind domaindoc.Kind, side gl.EntrySide, accountType gl.AccountType) (string, error) {
	code := rules.Resolve(kind, side, accountType)
	if strings.TrimSpace(code) == "" {
		return "", fmt.Errorf("the posting rules answer nothing for %s/%s/%s", kind, side, accountType)
	}
	if _, known := b.chart.ByCode(code); !known {
		return "", fmt.Errorf("the posting rules name account %s, which the chart of accounts does not hold", code)
	}

	return code, nil
}

// post records one voucher, refusing one whose two sides do not agree.
//
// This is the ledger's own rule and the last place it can be broken: a voucher
// that does not balance would put a figure under a column that nothing supports,
// and every report afterwards would sum it.
func (b *Books) post(id string, date time.Time, description string, lines []entry) error {
	debits, credits := decimal.Zero, decimal.Zero

	for i := range lines {
		switch lines[i].Side {
		case gl.SideDebit:
			debits = debits.Add(lines[i].Amount)
		case gl.SideCredit:
			credits = credits.Add(lines[i].Amount)
		default:
			return fmt.Errorf("voucher %s carries a line that is neither a debit nor a credit", id)
		}

		lines[i].VoucherID = id
		lines[i].VoucherDate = date
	}

	if !debits.Equal(credits) {
		return fmt.Errorf("voucher %s debits %s and credits %s", id, debits, credits)
	}

	b.vouchers = append(b.vouchers, lines...)

	return nil
}

// tt133 loads the TT133 posting rule set once.
var (
	rulesOnce sync.Once
	rulesSet  *posting.RuleSet
	rulesErr  error
)

func tt133() (*posting.RuleSet, error) {
	rulesOnce.Do(func() { rulesSet, rulesErr = posting.Load(posting.TT133Defaults()) })

	return rulesSet, rulesErr
}

// closingAccounts is the TT133 closing pair, which the reports need to tell a
// closing entry from a trading one.
func closingAccounts() close.ClosingAccounts { return close.TT133ClosingAccounts() }
