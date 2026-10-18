// Package ledgerrepo reads a client's posted ledger out of MariaDB.
//
// It is the storage behind internal/reports. The reports sum posted voucher
// lines and nothing else, so the only question this package answers is "which
// lines". Everything the reports decide — what a figure means, which account it
// belongs under, whether the two sides agree — stays in the reports, because
// that is the part that has to be true of any ledger and not of this table.
package ledgerrepo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	mariadb "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/gl"
	"fingo/internal/period"
	"fingo/internal/reports"
	"fingo/internal/usecase/ledger"
)

// beginningOfBooks is what a zero `from` becomes.
//
// The Journal port says a zero from means "from the beginning of the books", and
// the reports rely on it: the trial balance and the balance sheet both ask for
// everything up to a period's end, because an account has to show what it
// carried in as well as what the period did to it. A comparison against NULL is
// never true rather than always true, so encoding the open end as NULL would
// make every balance sheet read as though the client had never traded. A date
// before any voucher is the honest encoding, and it keeps the range a range.
var beginningOfBooks = time.Date(1900, time.January, 1, 0, 0, 0, 0, time.UTC)

// Periods is the calendar, as this package needs it.
//
// Declared here rather than imported from the use case that happens to have the
// method, so this adapter depends on a shape and not on practice.Client. That
// client satisfies it as it stands.
type Periods interface {
	Periods(ctx context.Context) ([]period.Period, error)
}

// Chart is the chart of accounts, as this package needs it.
//
// The same shape as ledger.ChartSource: a report has to know what kind of
// account each account is before a figure can go under assets or under revenue.
type Chart interface {
	Chart(ctx context.Context) (*gl.ChartOfAccounts, error)
}

// Source reads a client's posted lines, periods and chart.
//
// One type for the three ports rather than three objects, because all three mean
// the same client's books and a caller would otherwise assemble them from the
// same client anyway. The whole of the wiring is
// reports.NewService(source, source, source, closing).
type Source struct {
	queries *mariadb.Queries
	periods Periods
	chart   Chart
}

var (
	_ reports.Journal    = (*Source)(nil)
	_ reports.Periods    = (*Source)(nil)
	_ ledger.ChartSource = (*Source)(nil)
)

// New builds the source over a client's books.
//
// The database is taken rather than opened: a second pool to the same database
// buys nothing, and the client already owns one. See practice.Client.DB.
func New(db *sql.DB, periods Periods, chart Chart) *Source {
	return &Source{queries: mariadb.New(db), periods: periods, chart: chart}
}

// Lines returns the posted voucher lines between two dates, both ends included.
//
// A draft is not a line. is_posted is the filter that keeps one out, and it is
// the same promise the reports are built on: an unposted entry is not part of
// the books, and a figure summed from one is a figure nobody has signed off.
func (s *Source) Lines(ctx context.Context, from, to time.Time) ([]reports.Line, error) {
	if from.IsZero() {
		from = beginningOfBooks
	}

	rows, err := s.queries.ListPostedLinesInRange(ctx, mariadb.ListPostedLinesInRangeParams{
		FromDate: dayOf(from),
		ToDate:   dayOf(to),
	})
	if err != nil {
		return nil, fmt.Errorf("read the posted lines to %s: %w", to.Format(time.DateOnly), err)
	}

	out := make([]reports.Line, 0, len(rows))

	for _, r := range rows {
		amount, err := decimal.NewFromString(r.Amount)
		if err != nil {
			return nil, fmt.Errorf("voucher %s, line %s: stored amount %q is not a number: %w",
				r.VoucherID, r.ID, r.Amount, err)
		}

		out = append(out, reports.Line{
			VoucherID:   r.VoucherID,
			VoucherDate: r.VoucherDate,
			AccountCode: r.AccountCode,
			Side:        gl.EntrySide(r.Side),
			Amount:      amount,
		})
	}

	return out, nil
}

// List returns the client's accounting periods.
func (s *Source) List(ctx context.Context) ([]period.Period, error) { return s.periods.Periods(ctx) }

// Chart returns the client's chart of accounts.
func (s *Source) Chart(ctx context.Context) (*gl.ChartOfAccounts, error) { return s.chart.Chart(ctx) }

// dayOf truncates an instant to its calendar day.
//
// Both ends of the range are compared by calendar day and voucher_date is a DATE
// with no time in it. Handing the driver a time of day would put the upper bound
// inside the last day rather than at the end of it, so a report for a period
// ending on a day that has postings would leave that day's later entries out —
// quietly, and only for entries timed after the hour the report was run.
func dayOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
