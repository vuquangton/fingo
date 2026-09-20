-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

USE fingo;

CREATE TABLE IF NOT EXISTS company_profile (
    id VARCHAR(36) PRIMARY KEY,
    tax_code VARCHAR(14) NOT NULL UNIQUE,
    legal_name VARCHAR(255) NOT NULL,
    trade_name VARCHAR(255) NULL,
    english_name VARCHAR(255) NULL,
    address VARCHAR(500) NOT NULL,
    province_city VARCHAR(100) NULL,
    district_ward VARCHAR(100) NULL,
    phone VARCHAR(50) NULL,
    email VARCHAR(100) NULL,
    website VARCHAR(100) NULL,
    legal_representative VARCHAR(150) NOT NULL,
    representative_position VARCHAR(100) NULL,
    chief_accountant VARCHAR(150) NOT NULL,
    tax_authority_code VARCHAR(20) NOT NULL,
    tax_authority_name VARCHAR(255) NOT NULL,
    state_budget_chapter VARCHAR(10) NULL,
    regime ENUM('TT133_2016', 'TT99_2025') NOT NULL DEFAULT 'TT133_2016',
    base_currency VARCHAR(3) NOT NULL DEFAULT 'VND',
    fiscal_year_start_month TINYINT NOT NULL DEFAULT 1,
    vat_method ENUM('DEDUCTION', 'DIRECT') NOT NULL DEFAULT 'DEDUCTION',
    costing_method ENUM('FIFO', 'MOVING_WEIGHTED_AVG', 'PERIODIC_AVG') NOT NULL DEFAULT 'MOVING_WEIGHTED_AVG',
    business_type ENUM('TRADING', 'SERVICE', 'PRODUCTION', 'MIXED') NOT NULL DEFAULT 'TRADING',
    registered_banks JSON NULL,
    einvoice_config JSON NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    lock_date DATETIME NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS company_profile;
