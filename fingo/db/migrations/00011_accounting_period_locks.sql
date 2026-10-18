-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Accounting periods as date ranges, with an irreversible lock.
--
-- NOTE: this coexists with `accounting_periods`, which was reshaped in 00004
-- into a set of closed (year, month) rows. They are two models of the same idea
-- and one of them has to go — a month set cannot express contiguity or an
-- arbitrary range, and a range carries an identity a month does not. That
-- consolidation is a separate decision and is not made here.
--
-- The name is deliberately distinct so nothing in this migration can clobber the
-- existing table.

CREATE TABLE IF NOT EXISTS accounting_period_locks (
    id VARCHAR(36) PRIMARY KEY,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    is_locked BOOLEAN NOT NULL DEFAULT FALSE,
    locked_at DATETIME(6) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Listing periods is a range scan in date order, which is the order the
    -- index stores them in.
    KEY ix_period_locks_span (start_date, end_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS accounting_period_locks;
