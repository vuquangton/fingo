-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

USE fingo;

-- 1. System Options Table
CREATE TABLE IF NOT EXISTS system_options (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    branch_id VARCHAR(36) NULL,
    category VARCHAR(32) NOT NULL,
    option_key VARCHAR(64) NOT NULL,
    option_value TEXT NOT NULL,
    data_type VARCHAR(16) NOT NULL,
    default_value TEXT NOT NULL,
    scope_level VARCHAR(16) NOT NULL DEFAULT 'COMPANY',
    description VARCHAR(255) NULL,
    is_readonly BOOLEAN NOT NULL DEFAULT FALSE,
    is_encrypted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    updated_by VARCHAR(36) NULL,
    branch_scope_id VARCHAR(36) GENERATED ALWAYS AS (COALESCE(branch_id, '00000000-0000-0000-0000-000000000000')) STORED,
    CONSTRAINT fk_system_options_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE CASCADE,
    CONSTRAINT fk_system_options_branch FOREIGN KEY (branch_id) REFERENCES branch_org_units(id) ON DELETE CASCADE,
    UNIQUE KEY uk_system_option_scope (company_profile_id, branch_scope_id, option_key),
    INDEX idx_system_options_lookup (company_profile_id, category, option_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 2. Voucher Numbering Configurations Table
CREATE TABLE IF NOT EXISTS voucher_numbering_configs (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    branch_id VARCHAR(36) NULL,
    voucher_type VARCHAR(64) NOT NULL,
    prefix VARCHAR(32) NOT NULL,
    pattern VARCHAR(128) NOT NULL,
    reset_frequency ENUM('MONTHLY', 'YEARLY', 'CONTINUOUS') NOT NULL DEFAULT 'MONTHLY',
    current_sequence BIGINT NOT NULL DEFAULT 0,
    last_reset_date DATE NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    branch_scope_id VARCHAR(36) GENERATED ALWAYS AS (COALESCE(branch_id, '00000000-0000-0000-0000-000000000000')) STORED,
    CONSTRAINT fk_voucher_numbering_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE CASCADE,
    CONSTRAINT fk_voucher_numbering_branch FOREIGN KEY (branch_id) REFERENCES branch_org_units(id) ON DELETE CASCADE,
    UNIQUE KEY uk_voucher_numbering_scope (company_profile_id, branch_scope_id, voucher_type),
    INDEX idx_voucher_numbering_lookup (company_profile_id, voucher_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 3. System Configuration Audit History Table
CREATE TABLE IF NOT EXISTS system_config_history (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    option_key VARCHAR(64) NOT NULL,
    old_value TEXT NULL,
    new_value TEXT NOT NULL,
    changed_by VARCHAR(36) NOT NULL,
    changed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reason VARCHAR(255) NULL,
    client_ip VARCHAR(64) NULL,
    INDEX idx_config_history_key (company_profile_id, option_key, changed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS system_config_history;
DROP TABLE IF EXISTS voucher_numbering_configs;
DROP TABLE IF EXISTS system_options;
