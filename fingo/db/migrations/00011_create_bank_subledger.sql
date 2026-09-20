-- +goose Up
CREATE TABLE IF NOT EXISTS bank_transactions (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL UNIQUE,
    company_profile_id VARCHAR(36) NOT NULL,
    bank_account_id VARCHAR(36) NOT NULL,
    transaction_type ENUM('CREDIT_ADVICE', 'DEBIT_ADVICE', 'TRANSFER_ORDER') NOT NULL,
    counterparty_account_no VARCHAR(50) NULL,
    counterparty_bank_name VARCHAR(150) NULL,
    counterparty_name VARCHAR(150) NULL,
    bank_reference_no VARCHAR(100) NULL,
    fee_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    vat_fee_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    total_amount_vnd DECIMAL(18, 0) NOT NULL,
    CONSTRAINT fk_bt_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers (id) ON DELETE RESTRICT,
    CONSTRAINT fk_bt_bank FOREIGN KEY (bank_account_id) REFERENCES bank_accounts (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS bank_transactions;
