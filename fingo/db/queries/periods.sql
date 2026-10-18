-- name: ListClosedPeriods :many
SELECT year, month, closed_at, closed_by
FROM accounting_periods
ORDER BY year, month;

-- name: ClosePeriod :exec
-- Idempotent at the storage level; the domain refuses a double close, so this
-- is reached only when the caller believes the period is open.
INSERT INTO accounting_periods (year, month, closed_at, closed_by)
VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE closed_at = VALUES(closed_at), closed_by = VALUES(closed_by);

-- name: ReopenPeriod :exec
DELETE FROM accounting_periods WHERE year = ? AND month = ?;
