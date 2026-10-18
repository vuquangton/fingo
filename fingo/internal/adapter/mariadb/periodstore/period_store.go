// Package periodstore stores a client's accounting periods in MariaDB.
//
// It is the adapter for period.Repository. The name is deliberately distinct
// from periodrepo, which serves the older (year, month) model — see the note at
// the top of migration 00011.
package periodstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mariadb "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/period"
)

var _ period.Repository = (*Store)(nil)

// Store is the MariaDB adapter for period.Repository.
type Store struct {
	db      *sql.DB
	queries *mariadb.Queries
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db, queries: mariadb.New(db)}
}

// List returns every period, oldest first.
func (s *Store) List(ctx context.Context) ([]period.Period, error) {
	rows, err := s.queries.ListAccountingPeriods(ctx)
	if err != nil {
		return nil, fmt.Errorf("list accounting periods: %w", err)
	}

	out := make([]period.Period, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPeriod(r.ID, r.StartDate, r.EndDate, r.IsLocked, r.LockedAt))
	}

	return out, nil
}

// Create stores a period, refusing one that would overlap or leave a gap.
//
// The whole set is read and held for the transaction. A new period overlaps or
// leaves a gap only in relation to the others, so checking it against a set that
// is changing underfoot would let two creates each look valid alone and be
// invalid together.
func (s *Store) Create(ctx context.Context, p period.Period) error {
	if err := p.Validate(); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("create accounting period %s: begin: %w", p.ID, err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.queries.WithTx(tx)

	existing, err := q.ListAccountingPeriodsForUpdate(ctx)
	if err != nil {
		return fmt.Errorf("create accounting period %s: read the calendar: %w", p.ID, err)
	}

	calendar, err := period.NewCalendar(periodsOf(existing))
	if err != nil {
		return err
	}
	if err := calendar.Add(p); err != nil {
		return err
	}

	if err := q.CreateAccountingPeriod(ctx, mariadb.CreateAccountingPeriodParams{
		ID:        p.ID,
		StartDate: p.StartDate,
		EndDate:   p.EndDate,
	}); err != nil {
		return fmt.Errorf("create accounting period %s: %w", p.ID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("create accounting period %s: commit: %w", p.ID, err)
	}

	return nil
}

// Lock marks a period locked.
//
// The row is read and held first, so that "already locked" and "no such period"
// are told apart from a successful lock rather than collapsed into one silent
// update that matched nothing.
func (s *Store) Lock(ctx context.Context, id string, at time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("lock accounting period %s: begin: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.queries.WithTx(tx)

	row, err := q.GetAccountingPeriodForUpdate(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", period.ErrUnknownPeriod, id)
	}
	if err != nil {
		return fmt.Errorf("lock accounting period %s: %w", id, err)
	}
	if row.IsLocked {
		return fmt.Errorf("%w: %s", period.ErrAlreadyLocked, id)
	}

	if err := q.LockAccountingPeriod(ctx, mariadb.LockAccountingPeriodParams{
		LockedAt: sql.NullTime{Time: at.UTC(), Valid: true},
		ID:       id,
	}); err != nil {
		return fmt.Errorf("lock accounting period %s: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("lock accounting period %s: commit: %w", id, err)
	}

	return nil
}

// toPeriod assembles a domain period from a stored row.
func toPeriod(id string, start, end time.Time, locked bool, lockedAt sql.NullTime) period.Period {
	p := period.Period{ID: id, StartDate: start, EndDate: end, Locked: locked}

	if lockedAt.Valid {
		at := lockedAt.Time.UTC()
		p.LockedAt = &at
	}

	return p
}

func periodsOf(rows []mariadb.ListAccountingPeriodsForUpdateRow) []period.Period {
	out := make([]period.Period, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPeriod(r.ID, r.StartDate, r.EndDate, r.IsLocked, r.LockedAt))
	}

	return out
}
