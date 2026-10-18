-- name: EnsureVoucherSequence :exec
-- Creates the book's counter on first use. INSERT IGNORE so concurrent callers
-- cannot clobber a counter that already exists.
INSERT IGNORE INTO voucher_sequences (book, fiscal_year, next_seq) VALUES (?, ?, 1);

-- name: ReserveVoucherSequence :one
-- FOR UPDATE holds the row for the rest of the transaction, so two callers
-- cannot take the same place in the run.
SELECT next_seq FROM voucher_sequences
WHERE book = ? AND fiscal_year = ?
FOR UPDATE;

-- name: AdvanceVoucherSequence :exec
UPDATE voucher_sequences SET next_seq = ?
WHERE book = ? AND fiscal_year = ?;

-- name: ListVoucherSequences :many
SELECT book, fiscal_year, next_seq FROM voucher_sequences
ORDER BY book, fiscal_year;

-- name: ListIssuedVoucherNumbers :many
-- Every number actually issued, for gap detection. Drafts are excluded: they
-- hold no place in the run.
SELECT book, fiscal_year, seq FROM vouchers
WHERE seq IS NOT NULL
ORDER BY book, fiscal_year, seq;
