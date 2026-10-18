-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- A voucher number becomes a place in a book's sequence for a fiscal year.
--
-- Before this, voucher_no was globally unique, which made per-book numbering
-- impossible and stopped the sequence restarting each year — the numbering that
-- statutory books actually use.
--
-- book / fiscal_year / seq are NULLABLE on purpose. A draft has no number, and
-- if a draft carried ('' , 0, 0) then every draft would collide on the unique
-- key below. NULLs are distinct in a MySQL unique index, so drafts coexist
-- while posted vouchers are strictly unique.

ALTER TABLE vouchers
    ADD COLUMN book VARCHAR(8) NULL AFTER id,
    ADD COLUMN fiscal_year SMALLINT NULL AFTER book,
    ADD COLUMN seq INT NULL AFTER fiscal_year,
    DROP INDEX voucher_no,
    ADD UNIQUE KEY uq_vouchers_number (book, fiscal_year, seq),
    ADD KEY ix_vouchers_no (voucher_no);

-- The next unused place in each book's run for each fiscal year.
CREATE TABLE IF NOT EXISTS voucher_sequences (
    book VARCHAR(8) NOT NULL,
    fiscal_year SMALLINT NOT NULL,
    next_seq INT NOT NULL DEFAULT 1,
    PRIMARY KEY (book, fiscal_year)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS voucher_sequences;

ALTER TABLE vouchers
    DROP INDEX ix_vouchers_no,
    DROP INDEX uq_vouchers_number,
    DROP COLUMN seq,
    DROP COLUMN fiscal_year,
    DROP COLUMN book,
    ADD UNIQUE KEY voucher_no (voucher_no);
