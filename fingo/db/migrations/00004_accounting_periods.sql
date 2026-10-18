-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Periods become the single authority on which months are closed.
--
-- Before this, three overlapping notions of a period existed: this table, the
-- closing package's SystemLockConfig, and a bare lock date. Only the table
-- survives, and it now holds one row per CLOSED period — a row's existence is
-- the fact, so there is no status column to drift out of step with the rows.
--
-- The table was unused by any code, so it is reshaped in place rather than
-- migrated. Any row that existed described a period, not a closure, and would
-- have been meaningless here.

ALTER TABLE accounting_periods
    DROP PRIMARY KEY,
    DROP COLUMN id,
    DROP COLUMN name,
    DROP COLUMN start_date,
    DROP COLUMN end_date,
    DROP COLUMN is_closed,
    DROP COLUMN created_at,
    ADD COLUMN year SMALLINT NOT NULL,
    ADD COLUMN month TINYINT NOT NULL,
    ADD COLUMN closed_at DATETIME(6) NOT NULL,
    ADD COLUMN closed_by VARCHAR(36) NULL,
    ADD PRIMARY KEY (year, month);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

ALTER TABLE accounting_periods
    DROP PRIMARY KEY,
    DROP COLUMN year,
    DROP COLUMN month,
    DROP COLUMN closed_at,
    DROP COLUMN closed_by,
    ADD COLUMN id VARCHAR(36) NOT NULL,
    ADD COLUMN name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN start_date DATE NOT NULL DEFAULT '1970-01-01',
    ADD COLUMN end_date DATE NOT NULL DEFAULT '1970-01-01',
    ADD COLUMN is_closed BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;
