-- name: CreateVoucher :exec
INSERT INTO vouchers (
    id, company_profile_id, branch_id, voucher_no, voucher_date, posted_date,
    voucher_type, description, status, total_debit, total_credit,
    currency_code, exchange_rate, source_document_id, source_document_type,
    idempotency_key, created_by, created_at, updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?
);

-- name: InsertVoucherLine :exec
INSERT INTO voucher_lines (
    id, voucher_id, line_order, debit_account_id, credit_account_id,
    debit_account_code, credit_account_code, amount_fc, amount_vnd, note,
    customer_id, vendor_id, employee_id, item_id, warehouse_id,
    cost_center_id, expense_item_id, invoice_no, invoice_date, created_at
) VALUES (
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?
);

-- name: GetVoucherByID :one
SELECT id, company_profile_id, branch_id, voucher_no, voucher_date, posted_date,
       voucher_type, description, status, total_debit, total_credit,
       currency_code, exchange_rate, source_document_id, source_document_type,
       idempotency_key, created_by, created_at, updated_at
FROM vouchers
WHERE id = ?
LIMIT 1;

-- name: GetVoucherByNo :one
SELECT id, company_profile_id, branch_id, voucher_no, voucher_date, posted_date,
       voucher_type, description, status, total_debit, total_credit,
       currency_code, exchange_rate, source_document_id, source_document_type,
       idempotency_key, created_by, created_at, updated_at
FROM vouchers
WHERE company_profile_id = ? AND voucher_type = ? AND voucher_no = ?
LIMIT 1;

-- name: GetVoucherByIdempotencyKey :one
SELECT id, company_profile_id, branch_id, voucher_no, voucher_date, posted_date,
       voucher_type, description, status, total_debit, total_credit,
       currency_code, exchange_rate, source_document_id, source_document_type,
       idempotency_key, created_by, created_at, updated_at
FROM vouchers
WHERE company_profile_id = ? AND idempotency_key = ?
LIMIT 1;

-- name: ListVoucherLinesByVoucherID :many
SELECT id, voucher_id, line_order, debit_account_id, credit_account_id,
       debit_account_code, credit_account_code, amount_fc, amount_vnd, note,
       customer_id, vendor_id, employee_id, item_id, warehouse_id,
       cost_center_id, expense_item_id, invoice_no, invoice_date, created_at
FROM voucher_lines
WHERE voucher_id = ?
ORDER BY line_order ASC;

-- name: UpdateVoucherStatus :exec
UPDATE vouchers
SET status = ?, updated_at = ?
WHERE id = ?;

-- name: UpdateVoucherTotals :exec
UPDATE vouchers
SET total_debit = ?, total_credit = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteVoucherLinesByVoucherID :exec
DELETE FROM voucher_lines
WHERE voucher_id = ?;

-- name: ListVouchersByPeriod :many
SELECT id, company_profile_id, branch_id, voucher_no, voucher_date, posted_date,
       voucher_type, description, status, total_debit, total_credit,
       currency_code, exchange_rate, source_document_id, source_document_type,
       idempotency_key, created_by, created_at, updated_at
FROM vouchers
WHERE company_profile_id = ? AND voucher_date >= ? AND voucher_date <= ?
ORDER BY voucher_date ASC, voucher_no ASC;
