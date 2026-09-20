-- +goose Up
-- Migration: 00004_create_user_role_permission.sql
-- Module: User, Role, Permission & Access Control (BRD-FIN-SYS-003)

-- 1. Users Table
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    username VARCHAR(64) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255) NOT NULL,
    title VARCHAR(100) NULL,
    employee_code VARCHAR(64) NULL,
    status ENUM('PENDING_ACTIVATION', 'ACTIVE', 'SUSPENDED', 'LOCKED', 'TERMINATED') NOT NULL DEFAULT 'PENDING_ACTIVATION',
    failed_login_attempts INT NOT NULL DEFAULT 0,
    lockout_until TIMESTAMP NULL,
    password_changed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    must_change_password_next BOOLEAN NOT NULL DEFAULT TRUE,
    mfa_secret VARCHAR(128) NULL,
    is_mfa_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    digital_cert_subject VARCHAR(255) NULL,
    digital_cert_serial VARCHAR(128) NULL,
    last_login_at TIMESTAMP NULL,
    last_login_ip VARCHAR(64) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_users_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE CASCADE,
    CONSTRAINT uq_users_company_username UNIQUE (company_profile_id, username),
    CONSTRAINT uq_users_company_email UNIQUE (company_profile_id, email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 2. Roles Table
CREATE TABLE IF NOT EXISTS roles (
    id VARCHAR(36) PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(255) NULL,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 3. Permissions Table
CREATE TABLE IF NOT EXISTS permissions (
    id VARCHAR(36) PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    module VARCHAR(32) NOT NULL,
    resource VARCHAR(64) NOT NULL,
    action VARCHAR(32) NOT NULL,
    description VARCHAR(255) NULL,
    CONSTRAINT uq_perm_resource_action UNIQUE (module, resource, action)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 4. User-Role Association (N-N)
CREATE TABLE IF NOT EXISTS user_roles (
    user_id VARCHAR(36) NOT NULL,
    role_id VARCHAR(36) NOT NULL,
    assigned_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    assigned_by VARCHAR(36) NULL,
    PRIMARY KEY (user_id, role_id),
    CONSTRAINT fk_user_roles_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_roles_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 5. Role-Permission Association (N-N)
CREATE TABLE IF NOT EXISTS role_permissions (
    role_id VARCHAR(36) NOT NULL,
    permission_id VARCHAR(36) NOT NULL,
    PRIMARY KEY (role_id, permission_id),
    CONSTRAINT fk_role_perms_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    CONSTRAINT fk_role_perms_perm FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 6. User OrgUnit / Branch Data Scope (Row-Level Security)
CREATE TABLE IF NOT EXISTS user_org_unit_scopes (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    org_unit_id VARCHAR(36) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    include_children BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_user_scopes_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_scopes_org_unit FOREIGN KEY (org_unit_id) REFERENCES branch_org_units(id) ON DELETE CASCADE,
    CONSTRAINT uq_user_scope UNIQUE (user_id, org_unit_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 7. Statutory Segregation of Duties Conflict Rules Table
CREATE TABLE IF NOT EXISTS sod_conflict_rules (
    id VARCHAR(36) PRIMARY KEY,
    role_code_a VARCHAR(64) NOT NULL,
    role_code_b VARCHAR(64) NOT NULL,
    legal_basis VARCHAR(255) NOT NULL,
    description VARCHAR(500) NOT NULL,
    CONSTRAINT uq_sod_pair UNIQUE (role_code_a, role_code_b)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Seed Authoritative Vietnamese Accounting Roles
INSERT INTO roles (id, code, name, description, is_system) VALUES
('role-01', 'ROLE_SYSTEM_ADMIN', 'Quản trị hệ thống', 'Quản trị kỹ thuật, người dùng, phân quyền', TRUE),
('role-02', 'ROLE_DIRECTOR', 'Giám đốc', 'Người đại diện pháp luật, phê duyệt chi, xem báo cáo', TRUE),
('role-03', 'ROLE_CHIEF_ACCOUNTANT', 'Kế toán trưởng', 'Phê duyệt chứng từ, ghi sổ, khóa kỳ, ký BCTC', TRUE),
('role-04', 'ROLE_GENERAL_ACCOUNTANT', 'Kế toán tổng hợp', 'Lập bút toán tổng hợp, kết chuyển, đối chiếu', TRUE),
('role-05', 'ROLE_SALES_ACCOUNTANT', 'Kế toán bán hàng', 'Lập hóa đơn bán ra, theo dõi công nợ phải thu', TRUE),
('role-06', 'ROLE_PURCHASE_ACCOUNTANT', 'Kế toán mua hàng', 'Lập hóa đơn mua vào, theo dõi công nợ phải trả', TRUE),
('role-07', 'ROLE_CASHIER', 'Thủ quỹ', 'Quản lý thu chi tiền mặt, quỹ tiền mặt', TRUE),
('role-08', 'ROLE_WAREHOUSE_KEEPER', 'Thủ kho', 'Quản lý xuất nhập tồn kho vật chất', TRUE),
('role-09', 'ROLE_AUDITOR', 'Kiểm toán viên', 'Xem dữ liệu và sổ sách kế toán (Read-only)', TRUE);

-- +goose Down
DROP TABLE IF EXISTS sod_conflict_rules;
DROP TABLE IF EXISTS user_org_unit_scopes;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;
