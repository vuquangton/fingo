-- name: CreateCashReceipt :exec
INSERT INTO cash_receipts (
    id,
    voucher_id,
    company_profile_id,
    payer_name,
    payer_address,
    reason,
    cash_account_id,
    total_amount_vnd,
    accompanying_documents
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: GetCashReceiptByID :one
SELECT 
    id,
    voucher_id,
    company_profile_id,
    payer_name,
    payer_address,
    reason,
    cash_account_id,
    total_amount_vnd,
    accompanying_documents
FROM cash_receipts
WHERE id = ?;

-- name: GetCashReceiptByVoucherID :one
SELECT 
    id,
    voucher_id,
    company_profile_id,
    payer_name,
    payer_address,
    reason,
    cash_account_id,
    total_amount_vnd,
    accompanying_documents
FROM cash_receipts
WHERE voucher_id = ?;

-- name: CreateCashPayment :exec
INSERT INTO cash_payments (
    id,
    voucher_id,
    company_profile_id,
    receiver_name,
    receiver_address,
    reason,
    cash_account_id,
    total_amount_vnd,
    is_non_cash_override,
    override_reason,
    accompanying_documents
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: GetCashPaymentByID :one
SELECT 
    id,
    voucher_id,
    company_profile_id,
    receiver_name,
    receiver_address,
    reason,
    cash_account_id,
    total_amount_vnd,
    is_non_cash_override,
    override_reason,
    accompanying_documents
FROM cash_payments
WHERE id = ?;

-- name: GetCashPaymentByVoucherID :one
SELECT 
    id,
    voucher_id,
    company_profile_id,
    receiver_name,
    receiver_address,
    reason,
    cash_account_id,
    total_amount_vnd,
    is_non_cash_override,
    override_reason,
    accompanying_documents
FROM cash_payments
WHERE voucher_id = ?;
