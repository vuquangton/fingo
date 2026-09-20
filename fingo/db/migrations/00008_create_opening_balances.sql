-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

CREATE TABLE IF NOT EXISTS opening_batches (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    as_of_date DATE NOT NULL,
    status ENUM('DRAFT', 'VALIDATED', 'COMMITTED', 'LOCKED') NOT NULL DEFAULT 'DRAFT',
    total_debit DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    total_credit DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    committed_at TIMESTAMP NULL,
    committed_by VARCHAR(36) NULL,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_opening_batch_company_date UNIQUE (company_profile_id, as_of_date),
    CONSTRAINT fk_opening_batch_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS account_opening_balances (
    id VARCHAR(36) PRIMARY KEY,
    batch_id VARCHAR(36) NOT NULL,
    account_id VARCHAR(36) NOT NULL,
    currency_code VARCHAR(3) NOT NULL DEFAULT 'VND',
    debit_amount_fc DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    credit_amount_fc DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    exchange_rate DECIMAL(18, 6) NOT NULL DEFAULT 1.000000,
    debit_amount_vnd DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    credit_amount_vnd DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_account_opening UNIQUE (batch_id, account_id, currency_code),
    CONSTRAINT fk_acc_open_batch FOREIGN KEY (batch_id) REFERENCES opening_batches(id) ON DELETE CASCADE,
    CONSTRAINT fk_acc_open_acc FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS customer_opening_balances (
    id VARCHAR(36) PRIMARY KEY,
    batch_id VARCHAR(36) NOT NULL,
    customer_id VARCHAR(36) NOT NULL,
    invoice_no VARCHAR(50) NULL,
    invoice_date DATE NULL,
    due_date DATE NULL,
    currency_code VARCHAR(3) NOT NULL DEFAULT 'VND',
    debit_amount_fc DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    credit_amount_fc DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    exchange_rate DECIMAL(18, 6) NOT NULL DEFAULT 1.000000,
    debit_amount_vnd DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    credit_amount_vnd DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_cust_open_batch FOREIGN KEY (batch_id) REFERENCES opening_batches(id) ON DELETE CASCADE,
    CONSTRAINT fk_cust_open_cust FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS vendor_opening_balances (
    id VARCHAR(36) PRIMARY KEY,
    batch_id VARCHAR(36) NOT NULL,
    vendor_id VARCHAR(36) NOT NULL,
    bill_no VARCHAR(50) NULL,
    bill_date DATE NULL,
    due_date DATE NULL,
    currency_code VARCHAR(3) NOT NULL DEFAULT 'VND',
    debit_amount_fc DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    credit_amount_fc DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    exchange_rate DECIMAL(18, 6) NOT NULL DEFAULT 1.000000,
    debit_amount_vnd DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    credit_amount_vnd DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_vend_open_batch FOREIGN KEY (batch_id) REFERENCES opening_batches(id) ON DELETE CASCADE,
    CONSTRAINT fk_vend_open_vend FOREIGN KEY (vendor_id) REFERENCES vendors(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS inventory_opening_balances (
    id VARCHAR(36) PRIMARY KEY,
    batch_id VARCHAR(36) NOT NULL,
    warehouse_id VARCHAR(36) NOT NULL,
    item_id VARCHAR(36) NOT NULL,
    uom_id VARCHAR(36) NOT NULL,
    quantity DECIMAL(18, 4) NOT NULL DEFAULT 0.0000,
    unit_cost DECIMAL(18, 4) NOT NULL DEFAULT 0.0000,
    total_amount_vnd DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    batch_number VARCHAR(50) NULL,
    expiry_date DATE NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_inventory_open UNIQUE (batch_id, warehouse_id, item_id, batch_number),
    CONSTRAINT fk_inv_open_batch FOREIGN KEY (batch_id) REFERENCES opening_batches(id) ON DELETE CASCADE,
    CONSTRAINT fk_inv_open_wh FOREIGN KEY (warehouse_id) REFERENCES warehouses(id) ON DELETE RESTRICT,
    CONSTRAINT fk_inv_open_item FOREIGN KEY (item_id) REFERENCES items(id) ON DELETE RESTRICT,
    CONSTRAINT fk_inv_open_uom FOREIGN KEY (uom_id) REFERENCES unit_of_measures(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS asset_opening_balances (
    id VARCHAR(36) PRIMARY KEY,
    batch_id VARCHAR(36) NOT NULL,
    asset_code VARCHAR(50) NOT NULL,
    asset_name VARCHAR(255) NOT NULL,
    asset_account_id VARCHAR(36) NOT NULL,
    depreciation_account_id VARCHAR(36) NOT NULL,
    cost_account_id VARCHAR(36) NOT NULL,
    department_id VARCHAR(36) NULL,
    acquisition_date DATE NOT NULL,
    start_depreciation_date DATE NOT NULL,
    original_cost DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    accumulated_depreciation DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    net_book_value DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    useful_life_months INT NOT NULL,
    remaining_life_months INT NOT NULL,
    monthly_depreciation DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_asset_open_code UNIQUE (batch_id, asset_code),
    CONSTRAINT fk_asset_open_batch FOREIGN KEY (batch_id) REFERENCES opening_batches(id) ON DELETE CASCADE,
    CONSTRAINT fk_asset_open_asset_acc FOREIGN KEY (asset_account_id) REFERENCES accounts(id) ON DELETE RESTRICT,
    CONSTRAINT fk_asset_open_depr_acc FOREIGN KEY (depreciation_account_id) REFERENCES accounts(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS asset_opening_balances;
DROP TABLE IF EXISTS inventory_opening_balances;
DROP TABLE IF EXISTS vendor_opening_balances;
DROP TABLE IF EXISTS customer_opening_balances;
DROP TABLE IF EXISTS account_opening_balances;
DROP TABLE IF EXISTS opening_batches;
