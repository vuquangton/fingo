-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

CREATE TABLE IF NOT EXISTS unit_of_measures (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_uom_code UNIQUE (company_profile_id, code),
    CONSTRAINT fk_uom_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS uom_conversions (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    item_id VARCHAR(36) NULL,
    from_uom_id VARCHAR(36) NOT NULL,
    to_uom_id VARCHAR(36) NOT NULL,
    multiplier DECIMAL(18, 6) NOT NULL,
    conversion_type ENUM('MULTIPLY', 'DIVIDE') NOT NULL DEFAULT 'MULTIPLY',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_uom_conversion UNIQUE (company_profile_id, item_id, from_uom_id, to_uom_id),
    CONSTRAINT fk_uom_conv_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    CONSTRAINT fk_uom_conv_from FOREIGN KEY (from_uom_id) REFERENCES unit_of_measures(id) ON DELETE RESTRICT,
    CONSTRAINT fk_uom_conv_to FOREIGN KEY (to_uom_id) REFERENCES unit_of_measures(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS warehouses (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    branch_id VARCHAR(36) NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(150) NOT NULL,
    address TEXT,
    default_account_id VARCHAR(36) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_warehouse_code UNIQUE (company_profile_id, code),
    CONSTRAINT fk_warehouse_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    CONSTRAINT fk_warehouse_account FOREIGN KEY (default_account_id) REFERENCES accounts(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS bank_accounts (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    branch_id VARCHAR(36) NULL,
    account_number VARCHAR(50) NOT NULL,
    bank_name VARCHAR(150) NOT NULL,
    bank_code VARCHAR(50) NOT NULL,
    branch_name VARCHAR(150),
    currency_code VARCHAR(3) NOT NULL DEFAULT 'VND',
    gl_account_id VARCHAR(36) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_bank_account_num UNIQUE (company_profile_id, account_number),
    CONSTRAINT fk_bank_acc_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    CONSTRAINT fk_bank_acc_currency FOREIGN KEY (company_profile_id, currency_code) REFERENCES currencies(company_profile_id, code) ON DELETE RESTRICT,
    CONSTRAINT fk_bank_acc_gl FOREIGN KEY (gl_account_id) REFERENCES accounts(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS customers (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    tax_code VARCHAR(20),
    address TEXT,
    phone VARCHAR(50),
    email VARCHAR(100),
    contact_person VARCHAR(100),
    payment_term_days INT NOT NULL DEFAULT 30,
    credit_limit DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    enforce_credit_limit BOOLEAN NOT NULL DEFAULT FALSE,
    default_ar_account_id VARCHAR(36) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_customer_code UNIQUE (company_profile_id, code),
    CONSTRAINT fk_customer_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    CONSTRAINT fk_customer_ar_acc FOREIGN KEY (default_ar_account_id) REFERENCES accounts(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS vendors (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    tax_code VARCHAR(20),
    address TEXT,
    phone VARCHAR(50),
    email VARCHAR(100),
    contact_person VARCHAR(100),
    bank_account_number VARCHAR(50),
    bank_name VARCHAR(150),
    bank_branch VARCHAR(150),
    payment_term_days INT NOT NULL DEFAULT 30,
    default_ap_account_id VARCHAR(36) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_vendor_code UNIQUE (company_profile_id, code),
    CONSTRAINT fk_vendor_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    CONSTRAINT fk_vendor_ap_acc FOREIGN KEY (default_ap_account_id) REFERENCES accounts(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS items (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    barcode VARCHAR(100),
    item_type ENUM('MATERIAL', 'TOOL', 'FINISHED_GOOD', 'MERCHANDISE', 'SERVICE') NOT NULL,
    base_uom_id VARCHAR(36) NOT NULL,
    default_warehouse_id VARCHAR(36) NULL,
    inventory_account_id VARCHAR(36) NULL,
    cogs_account_id VARCHAR(36) NOT NULL,
    revenue_account_id VARCHAR(36) NOT NULL,
    default_vat_rate DECIMAL(5, 2) NOT NULL DEFAULT 10.00,
    standard_cost_price DECIMAL(18, 4) NOT NULL DEFAULT 0.0000,
    standard_sale_price DECIMAL(18, 4) NOT NULL DEFAULT 0.0000,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_item_code UNIQUE (company_profile_id, code),
    CONSTRAINT fk_item_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    CONSTRAINT fk_item_base_uom FOREIGN KEY (base_uom_id) REFERENCES unit_of_measures(id) ON DELETE RESTRICT,
    CONSTRAINT fk_item_warehouse FOREIGN KEY (default_warehouse_id) REFERENCES warehouses(id) ON DELETE RESTRICT,
    CONSTRAINT fk_item_inv_acc FOREIGN KEY (inventory_account_id) REFERENCES accounts(id) ON DELETE RESTRICT,
    CONSTRAINT fk_item_cogs_acc FOREIGN KEY (cogs_account_id) REFERENCES accounts(id) ON DELETE RESTRICT,
    CONSTRAINT fk_item_rev_acc FOREIGN KEY (revenue_account_id) REFERENCES accounts(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS employees (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    branch_id VARCHAR(36) NULL,
    code VARCHAR(50) NOT NULL,
    full_name VARCHAR(150) NOT NULL,
    department VARCHAR(100) NOT NULL,
    position VARCHAR(100) NOT NULL,
    citizen_id VARCHAR(20) NOT NULL,
    tax_code VARCHAR(20),
    social_insurance_no VARCHAR(20) NOT NULL,
    base_salary DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    salary_coefficient DECIMAL(5, 2) NOT NULL DEFAULT 1.00,
    bank_account_number VARCHAR(50),
    bank_name VARCHAR(150),
    default_advance_acc VARCHAR(36) NOT NULL,
    default_payroll_acc VARCHAR(36) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_employee_code UNIQUE (company_profile_id, code),
    CONSTRAINT uk_employee_citizen_id UNIQUE (company_profile_id, citizen_id),
    CONSTRAINT uk_employee_tax_code UNIQUE (company_profile_id, tax_code),
    CONSTRAINT uk_employee_social_ins UNIQUE (company_profile_id, social_insurance_no),
    CONSTRAINT fk_employee_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    CONSTRAINT fk_employee_advance_acc FOREIGN KEY (default_advance_acc) REFERENCES accounts(id) ON DELETE RESTRICT,
    CONSTRAINT fk_employee_payroll_acc FOREIGN KEY (default_payroll_acc) REFERENCES accounts(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS employees;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS vendors;
DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS bank_accounts;
DROP TABLE IF EXISTS warehouses;
DROP TABLE IF EXISTS uom_conversions;
DROP TABLE IF EXISTS unit_of_measures;
