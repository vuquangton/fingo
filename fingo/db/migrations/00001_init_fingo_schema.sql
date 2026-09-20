-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

CREATE DATABASE IF NOT EXISTS fingo CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE fingo;

-- General Ledger / Journal Voucher Header (Chứng từ)
CREATE TABLE IF NOT EXISTS vouchers (
    id VARCHAR(36) PRIMARY KEY,
    voucher_no VARCHAR(64) NOT NULL UNIQUE,
    voucher_date DATE NOT NULL,
    posted_date DATE NOT NULL,
    voucher_type ENUM('SALES', 'PURCHASE', 'CASH_RECEIPT', 'CASH_PAYMENT', 'GENERAL') NOT NULL,
    description VARCHAR(500) NULL,
    is_posted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Voucher Details (Bút toán định khoản)
CREATE TABLE IF NOT EXISTS voucher_lines (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL,
    line_order INT NOT NULL,
    debit_account_id VARCHAR(36) NOT NULL,
    credit_account_id VARCHAR(36) NOT NULL,
    amount DECIMAL(18, 4) NOT NULL DEFAULT 0.0000,
    note VARCHAR(255) NULL,
    CONSTRAINT fk_voucher_lines_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS voucher_lines;
DROP TABLE IF EXISTS vouchers;
