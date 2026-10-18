-- name: ListAccountingPeriods :many
SELECT id, start_date, end_date, is_locked, locked_at
FROM accounting_period_locks
ORDER BY start_date ASC;

-- name: ListAccountingPeriodsForUpdate :many
-- The same list, held for the rest of the transaction.
--
-- Creating a period has to be checked against every existing one — a new period
-- overlaps or leaves a gap only in relation to its neighbours — so the check and
-- the insert must see a set nobody else is changing.
SELECT id, start_date, end_date, is_locked, locked_at
FROM accounting_period_locks
ORDER BY start_date ASC
FOR UPDATE;

-- name: GetAccountingPeriodForUpdate :one
SELECT id, start_date, end_date, is_locked, locked_at
FROM accounting_period_locks
WHERE id = ? LIMIT 1
FOR UPDATE;

-- name: CreateAccountingPeriod :exec
INSERT INTO accounting_period_locks (id, start_date, end_date, is_locked, locked_at)
VALUES (?, ?, ?, FALSE, NULL);

-- name: LockAccountingPeriod :exec
UPDATE accounting_period_locks SET is_locked = TRUE, locked_at = ?
WHERE id = ?;
