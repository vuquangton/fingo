-- +goose Up
-- Migration: 00006_create_accounting_spine.sql
-- Modules: Currency, ExchangeRate, Account, FiscalYear, AccountingPeriod, CostCenter, ExpenseItem

USE fingo;

-- 1. Currencies Table
CREATE TABLE IF NOT EXISTS currencies (
    code VARCHAR(3) NOT NULL,
    company_profile_id VARCHAR(36) NOT NULL,
    name VARCHAR(100) NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    decimal_places INT NOT NULL DEFAULT 2,
    is_base BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (company_profile_id, code),
    CONSTRAINT fk_curr_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 2. Daily Exchange Rates Table
CREATE TABLE IF NOT EXISTS exchange_rates (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    currency_code VARCHAR(3) NOT NULL,
    rate_date DATE NOT NULL,
    rate_type ENUM('BUY_TRANSFER', 'SELL_TRANSFER', 'CENTRAL_SBV') NOT NULL DEFAULT 'BUY_TRANSFER',
    rate DECIMAL(18, 6) NOT NULL,
    source_bank VARCHAR(100) NOT NULL DEFAULT 'VIETCOMBANK',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR(36) NOT NULL,
    CONSTRAINT fk_fx_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    CONSTRAINT fk_fx_currency FOREIGN KEY (company_profile_id, currency_code) REFERENCES currencies(company_profile_id, code) ON DELETE RESTRICT,
    UNIQUE KEY uk_currency_date_type (company_profile_id, currency_code, rate_date, rate_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 3. Chart of Accounts (COA) Table
CREATE TABLE IF NOT EXISTS accounts (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    code VARCHAR(32) NOT NULL,
    name VARCHAR(255) NOT NULL,
    english_name VARCHAR(255) NULL,
    parent_id VARCHAR(36) NULL,
    account_level INT NOT NULL DEFAULT 1,
    nature ENUM('DEBIT', 'CREDIT', 'HERMAPHRODITE', 'NO_BALANCE') NOT NULL,
    category ENUM('ASSET', 'LIABILITY', 'EQUITY', 'REVENUE', 'EXPENSE', 'OTHER_INCOME', 'OTHER_EXPENSE', 'SUMMARY') NOT NULL,
    is_leaf BOOLEAN NOT NULL DEFAULT TRUE,
    is_foreign_currency BOOLEAN NOT NULL DEFAULT FALSE,
    requires_partner BOOLEAN NOT NULL DEFAULT FALSE,
    requires_bank_account BOOLEAN NOT NULL DEFAULT FALSE,
    requires_cost_center BOOLEAN NOT NULL DEFAULT FALSE,
    requires_expense_item BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_acc_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    CONSTRAINT fk_acc_parent FOREIGN KEY (parent_id) REFERENCES accounts(id) ON DELETE RESTRICT,
    UNIQUE KEY uk_company_account_code (company_profile_id, code),
    INDEX idx_accounts_parent (parent_id),
    INDEX idx_accounts_category (company_profile_id, category)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Re-attach voucher_lines foreign keys to upgraded accounts table
ALTER TABLE voucher_lines ADD CONSTRAINT fk_voucher_lines_debit FOREIGN KEY (debit_account_id) REFERENCES accounts(id);
ALTER TABLE voucher_lines ADD CONSTRAINT fk_voucher_lines_credit FOREIGN KEY (credit_account_id) REFERENCES accounts(id);

-- 4. Fiscal Years Table
CREATE TABLE IF NOT EXISTS fiscal_years (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    year INT NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    status ENUM('OPEN', 'SOFT_LOCKED', 'HARD_LOCKED', 'CLOSED') NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_fy_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    UNIQUE KEY uk_company_fiscal_year (company_profile_id, year)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 5. Accounting Periods Table
CREATE TABLE IF NOT EXISTS accounting_periods (
    id VARCHAR(36) PRIMARY KEY,
    fiscal_year_id VARCHAR(36) NOT NULL,
    company_profile_id VARCHAR(36) NOT NULL,
    period_number INT NOT NULL,
    name VARCHAR(50) NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    lock_date DATE NOT NULL,
    status ENUM('OPEN', 'SOFT_LOCKED', 'HARD_LOCKED', 'AUDITED') NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_ap_fy FOREIGN KEY (fiscal_year_id) REFERENCES fiscal_years(id) ON DELETE RESTRICT,
    CONSTRAINT fk_ap_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    UNIQUE KEY uk_company_fy_period (company_profile_id, fiscal_year_id, period_number),
    INDEX idx_ap_dates (company_profile_id, start_date, end_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 6. Cost Centers Table
CREATE TABLE IF NOT EXISTS cost_centers (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    branch_id VARCHAR(36) NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    parent_id VARCHAR(36) NULL,
    is_leaf BOOLEAN NOT NULL DEFAULT TRUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_cc_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    CONSTRAINT fk_cc_branch FOREIGN KEY (branch_id) REFERENCES branch_org_units(id) ON DELETE SET NULL,
    CONSTRAINT fk_cc_parent FOREIGN KEY (parent_id) REFERENCES cost_centers(id) ON DELETE RESTRICT,
    UNIQUE KEY uk_company_cost_center_code (company_profile_id, code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 7. Expense Items Table
CREATE TABLE IF NOT EXISTS expense_items (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(64) NOT NULL,
    parent_id VARCHAR(36) NULL,
    is_leaf BOOLEAN NOT NULL DEFAULT TRUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_ei_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    CONSTRAINT fk_ei_parent FOREIGN KEY (parent_id) REFERENCES expense_items(id) ON DELETE RESTRICT,
    UNIQUE KEY uk_company_expense_item_code (company_profile_id, code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

ALTER TABLE voucher_lines DROP FOREIGN KEY fk_voucher_lines_debit;
ALTER TABLE voucher_lines DROP FOREIGN KEY fk_voucher_lines_credit;

DROP TABLE IF EXISTS expense_items;
DROP TABLE IF EXISTS cost_centers;
DROP TABLE IF EXISTS accounting_periods;
DROP TABLE IF EXISTS fiscal_years;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS exchange_rates;
DROP TABLE IF EXISTS currencies;
