-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

USE fingo;

CREATE TABLE IF NOT EXISTS branch_org_units (
    id VARCHAR(36) PRIMARY KEY,
    parent_id VARCHAR(36) NULL,
    company_profile_id VARCHAR(36) NOT NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    unit_type ENUM('HEAD_OFFICE', 'BRANCH', 'REP_OFFICE', 'BUSINESS_LOCATION', 'DEPARTMENT') NOT NULL,
    accounting_governance ENUM('INDEPENDENT', 'DEPENDENT', 'COST_CENTER') NOT NULL DEFAULT 'DEPENDENT',
    tax_filing_mechanism ENUM('CENTRALIZED', 'DECENTRALIZED', 'ALLOCATED') NOT NULL DEFAULT 'CENTRALIZED',
    tax_code VARCHAR(14) NULL,
    tax_authority_code VARCHAR(20) NULL,
    tax_authority_name VARCHAR(255) NULL,
    province_city_code VARCHAR(10) NOT NULL,
    address VARCHAR(500) NOT NULL,
    manager_name VARCHAR(150) NULL,
    chief_accountant VARCHAR(150) NULL,
    internal_receivable_account VARCHAR(32) NOT NULL DEFAULT '1361',
    internal_payable_account VARCHAR(32) NOT NULL DEFAULT '3361',
    has_own_einvoice BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_branch_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE CASCADE,
    CONSTRAINT fk_branch_parent FOREIGN KEY (parent_id) REFERENCES branch_org_units(id) ON DELETE SET NULL,
    UNIQUE KEY uk_company_branch_code (company_profile_id, code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS branch_org_units;
