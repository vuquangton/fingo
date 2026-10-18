// Package voucherstore allocates voucher numbers from a client's series, in
// MariaDB.
//
// The name is deliberately distinct from glrepo, which allocates numbers for the
// older book-keyed model — see the note at the top of migration 00012.
package voucherstore

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	mariadb "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/voucher"
)

var _ voucher.SeriesRepository = (*Store)(nil)

// Store is the MariaDB adapter for voucher.SeriesRepository.
type Store struct {
	db      *sql.DB
	queries *mariadb.Queries
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db, queries: mariadb.New(db)}
}

// Next allocates the next number in a series, creating the series on first use.
//
// The whole allocation is one transaction, and the row lock the insert takes is
// held until it commits. Two callers therefore cannot read the same counter, and
// neither can skip a number: the advance is part of the same lock, not a
// separate step that could be ordered differently.
//
// A crash between allocating and using a number still leaves a gap. That is not
// a concurrency fault — it is a number taken and never printed, which a book can
// account for and a numbering system should not try to hide.
func (s *Store) Next(ctx context.Context, prefix voucher.Prefix, year int) (int, error) {
	if !prefix.Valid() {
		return 0, fmt.Errorf("voucher prefix %q is not a usable code", prefix)
	}
	if year < 1 {
		return 0, fmt.Errorf("a voucher series needs a fiscal year, got %d", year)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("allocate %s-%d: begin: %w", prefix, year, err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.queries.WithTx(tx)

	params := mariadb.CurrentVoucherSeriesParams{
		Prefix:     string(prefix),
		FiscalYear: int16(year),
	}

	if err := q.AllocateVoucherSeries(ctx, mariadb.AllocateVoucherSeriesParams{
		ID:         uuid.NewString(),
		Prefix:     params.Prefix,
		FiscalYear: params.FiscalYear,
	}); err != nil {
		return 0, fmt.Errorf("allocate %s-%d: %w", prefix, year, err)
	}

	allocated, err := q.CurrentVoucherSeries(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("allocate %s-%d: read back: %w", prefix, year, err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("allocate %s-%d: commit: %w", prefix, year, err)
	}

	return int(allocated), nil
}
