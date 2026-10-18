-- name: CreateIdempotencyKey :exec
-- Recording the key in the same transaction as the voucher is what makes the
-- pair atomic: if the voucher insert rolls back, so does the claim.
INSERT INTO idempotency_keys (key_value, voucher_id, created_at)
VALUES (?, ?, ?);

-- name: FindVoucherByIdempotencyKey :one
SELECT voucher_id FROM idempotency_keys WHERE key_value = ?;
