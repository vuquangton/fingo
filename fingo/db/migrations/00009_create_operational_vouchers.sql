-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

USE fingo;

-- Drop obsolete stub tables from initial schema
DROP TABLE IF EXISTS voucher_lines;
DROP TABLE IF EXISTS vouchers;

-- 1. General Vouchers
CREATE TABLE vouchers (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    branch_id VARCHAR(36) NULL,
    voucher_no VARCHAR(50) NOT NULL,
    voucher_date DATE NOT NULL,
    posted_date DATE NOT NULL,
    voucher_type ENUM(
        'GENERAL', 'CASH_RECEIPT', 'CASH_PAYMENT',
        'BANK_RECEIPT', 'BANK_PAYMENT',
        'SALES', 'SALES_RETURN',
        'PURCHASE', 'PURCHASE_RETURN',
        'STOCK_INWARD', 'STOCK_OUTWARD',
        'ASSET', 'PAYROLL'
    ) NOT NULL DEFAULT 'GENERAL',
    description VARCHAR(500) NOT NULL,
    status ENUM('DRAFT', 'POSTED', 'CANCELLED') NOT NULL DEFAULT 'DRAFT',
    total_debit DECIMAL(18, 0) NOT NULL DEFAULT 0,
    total_credit DECIMAL(18, 0) NOT NULL DEFAULT 0,
    currency_code VARCHAR(3) NOT NULL DEFAULT 'VND',
    exchange_rate DECIMAL(18, 6) NOT NULL DEFAULT 1.000000,
    source_document_id VARCHAR(36) NULL,
    source_document_type VARCHAR(50) NULL,
    idempotency_key VARCHAR(64) NULL UNIQUE,
    created_by VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_voucher_company FOREIGN KEY (company_profile_id) REFERENCES company_profile (id) ON DELETE RESTRICT,
    UNIQUE KEY uq_voucher_company_no (company_profile_id, voucher_type, voucher_no),
    INDEX idx_voucher_date (company_profile_id, voucher_date),
    INDEX idx_voucher_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 2. Voucher Lines (Double-Entry Balanced Detail)
CREATE TABLE voucher_lines (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL,
    line_order INT NOT NULL,
    debit_account_id VARCHAR(36) NOT NULL,
    credit_account_id VARCHAR(36) NOT NULL,
    debit_account_code VARCHAR(20) NOT NULL,
    credit_account_code VARCHAR(20) NOT NULL,
    amount_fc DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    note VARCHAR(255) NULL,
    customer_id VARCHAR(36) NULL,
    vendor_id VARCHAR(36) NULL,
    employee_id VARCHAR(36) NULL,
    item_id VARCHAR(36) NULL,
    warehouse_id VARCHAR(36) NULL,
    cost_center_id VARCHAR(36) NULL,
    expense_item_id VARCHAR(36) NULL,
    invoice_no VARCHAR(50) NULL,
    invoice_date DATE NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_vl_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers (id) ON DELETE CASCADE,
    CONSTRAINT fk_vl_debit_acc FOREIGN KEY (debit_account_id) REFERENCES accounts (id) ON DELETE RESTRICT,
    CONSTRAINT fk_vl_credit_acc FOREIGN KEY (credit_account_id) REFERENCES accounts (id) ON DELETE RESTRICT,
    INDEX idx_vl_accounts (debit_account_code, credit_account_code),
    INDEX idx_vl_counterparty (customer_id, vendor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS voucher_lines;
DROP TABLE IF EXISTS vouchers;
