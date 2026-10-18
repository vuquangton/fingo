// Package periodrepo stores and retrieves a client's closed accounting periods.
package periodrepo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	mariadb "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/gl"
)

// PeriodStore is the MariaDB adapter for a client's period calendar.
type PeriodStore struct {
	db      *sql.DB
	queries *mariadb.Queries
}

func NewPeriodStore(db *sql.DB) *PeriodStore {
	return &PeriodStore{db: db, queries: mariadb.New(db)}
}

// Load reads the closed periods into a calendar.
func (s *PeriodStore) Load(ctx context.Context) (*gl.Periods, error) {
	rows, err := s.queries.ListClosedPeriods(ctx)
	if err != nil {
		return nil, fmt.Errorf("list closed periods: %w", err)
	}

	periods := gl.NewPeriods()
	for _, r := range rows {
		period := gl.YearMonth{Year: int(r.Year), Month: time.Month(r.Month)}

		// Close refuses a duplicate, so this also detects a database holding
		// the same month twice — which its primary key should prevent.
		if err := periods.Close(period); err != nil {
			return nil, fmt.Errorf("read closed period %s: %w", period, err)
		}
	}

	return periods, nil
}

// Close shuts a period, recording when and by whom.
func (s *PeriodStore) Close(ctx context.Context, period gl.YearMonth, at time.Time, by string) error {
	err := s.queries.ClosePeriod(ctx, mariadb.ClosePeriodParams{
		Year:     int16(period.Year),
		Month:    int8(period.Month),
		ClosedAt: at.UTC(),
		ClosedBy: sql.NullString{String: by, Valid: by != ""},
	})
	if err != nil {
		return fmt.Errorf("close period %s: %w", period, err)
	}

	return nil
}

// Reopen removes the closure, making the period open again.
//
// The closure record is deleted rather than flagged, because a row in
// accounting_periods means "this period is closed". Who reopened a period is
// therefore not currently recorded; that arrives with the audit trail.
func (s *PeriodStore) Reopen(ctx context.Context, period gl.YearMonth) error {
	err := s.queries.ReopenPeriod(ctx, mariadb.ReopenPeriodParams{
		Year:  int16(period.Year),
		Month: int8(period.Month),
	})
	if err != nil {
		return fmt.Errorf("reopen period %s: %w", period, err)
	}

	return nil
}
