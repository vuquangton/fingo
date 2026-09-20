-- name: GetAccountByCode :one
SELECT id, code, name, parent_id, account_type, nature, is_active, created_at, updated_at
FROM accounts
WHERE code = ? LIMIT 1;

-- name: ListActiveAccounts :many
SELECT id, code, name, parent_id, account_type, nature, is_active, created_at, updated_at
FROM accounts
WHERE is_active = TRUE
ORDER BY code ASC;

-- name: CreateAccount :exec
INSERT INTO accounts (id, code, name, parent_id, account_type, nature, is_active)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: CreateVoucher :exec
INSERT INTO vouchers (id, voucher_no, voucher_date, posted_date, voucher_type, description, is_posted)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: CreateVoucherLine :exec
INSERT INTO voucher_lines (id, voucher_id, line_order, debit_account_id, credit_account_id, amount, note)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListVoucherLinesByVoucherID :many
SELECT id, voucher_id, line_order, debit_account_id, credit_account_id, amount, note
FROM voucher_lines
WHERE voucher_id = ?
ORDER BY line_order ASC;
