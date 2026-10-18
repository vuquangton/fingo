-- name: AllocateVoucherSeries :exec
-- Allocates the next number in a series, creating the series on first use.
--
-- One statement, deliberately. It inserts a new series at 1, or advances an
-- existing one, and either way it takes the row lock for the rest of the
-- transaction — so the read that follows sees this transaction's own value and
-- no other allocator can interleave.
--
-- The obvious alternative — INSERT IGNORE and then SELECT ... FOR UPDATE — also
-- works, but under load the ignored insert leaves the transaction holding a
-- shared lock on a row it then wants exclusively, which deadlocks against
-- another allocator doing the same thing.
INSERT INTO voucher_series (id, prefix, fiscal_year, last_number)
VALUES (?, ?, ?, 1)
ON DUPLICATE KEY UPDATE last_number = last_number + 1;

-- name: CurrentVoucherSeries :one
-- The value this transaction has just written. No FOR UPDATE: the allocation
-- above already holds the row, and a transaction always sees its own changes.
SELECT last_number FROM voucher_series
WHERE prefix = ? AND fiscal_year = ?;

-- name: ListVoucherSeries :many
SELECT id, prefix, fiscal_year, last_number FROM voucher_series
ORDER BY prefix ASC, fiscal_year ASC;
