-- +goose Up
CREATE TABLE IF NOT EXISTS cash_receipts (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL UNIQUE,
    company_profile_id VARCHAR(36) NOT NULL,
    payer_name VARCHAR(150) NOT NULL,
    payer_address VARCHAR(255) NULL,
    reason VARCHAR(255) NOT NULL,
    cash_account_id VARCHAR(36) NOT NULL,
    total_amount_vnd DECIMAL(18, 0) NOT NULL,
    accompanying_documents VARCHAR(100) NULL,
    CONSTRAINT fk_cr_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS cash_payments (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL UNIQUE,
    company_profile_id VARCHAR(36) NOT NULL,
    receiver_name VARCHAR(150) NOT NULL,
    receiver_address VARCHAR(255) NULL,
    reason VARCHAR(255) NOT NULL,
    cash_account_id VARCHAR(36) NOT NULL,
    total_amount_vnd DECIMAL(18, 0) NOT NULL,
    is_non_cash_override BOOLEAN NOT NULL DEFAULT FALSE,
    override_reason VARCHAR(255) NULL,
    accompanying_documents VARCHAR(100) NULL,
    CONSTRAINT fk_cp_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS cash_payments;
DROP TABLE IF EXISTS cash_receipts;
