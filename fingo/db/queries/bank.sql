-- name: CreateBankTransaction :exec
INSERT INTO bank_transactions (
    id,
    voucher_id,
    company_profile_id,
    bank_account_id,
    transaction_type,
    counterparty_account_no,
    counterparty_bank_name,
    counterparty_name,
    bank_reference_no,
    fee_amount_vnd,
    vat_fee_amount_vnd,
    total_amount_vnd
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: GetBankTransactionByID :one
SELECT 
    id,
    voucher_id,
    company_profile_id,
    bank_account_id,
    transaction_type,
    counterparty_account_no,
    counterparty_bank_name,
    counterparty_name,
    bank_reference_no,
    fee_amount_vnd,
    vat_fee_amount_vnd,
    total_amount_vnd
FROM bank_transactions
WHERE id = ?;

-- name: GetBankTransactionByVoucherID :one
SELECT 
    id,
    voucher_id,
    company_profile_id,
    bank_account_id,
    transaction_type,
    counterparty_account_no,
    counterparty_bank_name,
    counterparty_name,
    bank_reference_no,
    fee_amount_vnd,
    vat_fee_amount_vnd,
    total_amount_vnd
FROM bank_transactions
WHERE voucher_id = ?;
