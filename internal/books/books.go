// Package books assembles one client's books.
//
// The reports service needs three ports and every one of them is a question about
// the same rows. Putting them together is assembly, and assembly belongs where the
// application is assembled — not inside the reports, which must not know how a
// voucher is stored, and not inside the store, which must not know what a trial
// balance is.
package books

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/close"
	"fingo/internal/domain/gl"
	"fingo/internal/money"
	"fingo/internal/period"
	"fingo/internal/reports"
	"fingo/internal/store"
)

// Books is one client's books, as a set of ports the reports read through.
type Books struct {
	store *store.Store
	scope store.Scope
}

// Open scopes a store to one client.
func Open(st *store.Store, scope store.Scope) *Books {
	return &Books{store: st, scope: scope}
}

// Service builds the reports over these books.
func (b *Books) Service() (*reports.Service, error) {
	return reports.NewService(b, b, b, close.TT133ClosingAccounts())
}

// TrialBalance reads the trial balance for a period.
func (b *Books) TrialBalance(ctx context.Context, periodID string) (reports.TrialBalance, error) {
	service, err := b.Service()
	if err != nil {
		return reports.TrialBalance{}, err
	}
	return service.TrialBalance(ctx, periodID)
}

// ProfitAndLoss reads the period result.
func (b *Books) ProfitAndLoss(ctx context.Context, periodID string) (reports.ProfitAndLoss, error) {
	service, err := b.Service()
	if err != nil {
		return reports.ProfitAndLoss{}, err
	}
	return service.ProfitAndLoss(ctx, periodID)
}

// BalanceSheet reads the position at the period end.
func (b *Books) BalanceSheet(ctx context.Context, periodID string) (reports.BalanceSheet, error) {
	service, err := b.Service()
	if err != nil {
		return reports.BalanceSheet{}, err
	}
	return service.BalanceSheet(ctx, periodID)
}

// AccountLines returns one account's movements between two dates: the drill-down
// a reader follows from a report row to the vouchers behind it.
func (b *Books) AccountLines(ctx context.Context, account string, from, to time.Time) ([]reports.Line, error) {
	lines, err := b.Lines(ctx, from, to)
	if err != nil {
		return nil, err
	}

	out := make([]reports.Line, 0, len(lines))
	for _, l := range lines {
		if l.AccountCode == account {
			out = append(out, l)
		}
	}
	return out, nil
}

// List implements reports.Periods.
func (b *Books) List(ctx context.Context) ([]period.Period, error) {
	var out []period.Period

	err := b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT p.id, p.start_date, p.end_date, l.locked_at
			  FROM accounting_periods p
			  LEFT JOIN accounting_period_locks l ON l.period_id = p.id
			 ORDER BY p.start_date`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var p period.Period
			var lockedAt *time.Time
			if err := rows.Scan(&p.ID, &p.StartDate, &p.EndDate, &lockedAt); err != nil {
				return err
			}
			p.Locked = lockedAt != nil
			p.LockedAt = lockedAt
			out = append(out, p)
		}
		return rows.Err()
	})

	return out, err
}

// Lines implements reports.Journal: only posted lines, by calendar day.
func (b *Books) Lines(ctx context.Context, from, to time.Time) ([]reports.Line, error) {
	var out []reports.Line

	err := b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT v.id, v.voucher_date, l.account_code, l.side, l.amount
			  FROM voucher_lines l
			  JOIN vouchers v ON v.id = l.voucher_id
			 WHERE v.posted_at IS NOT NULL
			   AND ($1 = '0001-01-01'::date OR v.voucher_date >= $1)
			   AND v.voucher_date <= $2
			 ORDER BY v.voucher_date, v.voucher_no, l.line_order`,
			from, to)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var l reports.Line
			var side string
			var amount int64
			if err := rows.Scan(&l.VoucherID, &l.VoucherDate, &l.AccountCode, &side, &amount); err != nil {
				return err
			}
			l.Side = gl.EntrySide(side)
			l.Amount = money.FromInt(amount)
			out = append(out, l)
		}
		return rows.Err()
	})

	return out, err
}

// Chart implements ledger.ChartSource.
func (b *Books) Chart(ctx context.Context) (*gl.ChartOfAccounts, error) {
	chart := gl.NewChartOfAccounts()

	err := b.store.Read(ctx, b.scope, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT code, name, COALESCE(parent_code, ''), account_type, nature, active
			  FROM accounts
			 ORDER BY code`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var a gl.Account
			var accountType, nature string
			if err := rows.Scan(&a.Code, &a.Name, &a.ParentCode, &accountType, &nature, &a.IsActive); err != nil {
				return err
			}
			a.Type = gl.AccountType(accountType)
			a.Nature = gl.AccountNature(nature)
			if err := chart.Load(a); err != nil {
				return fmt.Errorf("load account %s: %w", a.Code, err)
			}
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}

	return chart, nil
}

// EnsureCurrentYear opens a calendar-year period when the client has none.
//
// A report is a position at the end of a period, and there is no such thing as a
// position at the end of nothing. A client whose books were created but whose
// year was never opened would otherwise get "no period" from every screen.
func (b *Books) EnsureCurrentYear(ctx context.Context, now time.Time) (period.Period, error) {
	existing, err := b.List(ctx)
	if err != nil {
		return period.Period{}, err
	}
	if len(existing) > 0 {
		return existing[len(existing)-1], nil
	}

	p := period.Period{
		StartDate: time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(now.Year(), 12, 31, 0, 0, 0, 0, time.UTC),
	}

	err = b.store.InClient(ctx, b.scope, func(tx pgx.Tx) error {
		// The id is the database's to generate: the column is a uuid and will not
		// accept a readable "FY2026". A period is identified by its range anyway.
		return tx.QueryRow(ctx, `
			INSERT INTO accounting_periods (client_id, name, start_date, end_date)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (client_id, start_date, end_date) DO NOTHING
			RETURNING id`,
			b.scope.ID(), fmt.Sprintf("Năm %d", now.Year()), p.StartDate, p.EndDate).Scan(&p.ID)
	})
	if err != nil {
		return period.Period{}, err
	}

	return p, nil
}
