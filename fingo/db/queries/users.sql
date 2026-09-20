-- name: CreateUser :exec
INSERT INTO users (
    id, company_profile_id, username, email, password_hash, full_name,
    title, employee_code, status, failed_login_attempts, lockout_until,
    password_changed_at, must_change_password_next, mfa_secret, is_mfa_enabled,
    digital_cert_subject, digital_cert_serial
) VALUES (
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?
);

-- name: GetUserByID :one
SELECT id, company_profile_id, username, email, password_hash, full_name,
       title, employee_code, status, failed_login_attempts, lockout_until,
       password_changed_at, must_change_password_next, mfa_secret, is_mfa_enabled,
       digital_cert_subject, digital_cert_serial, last_login_at, last_login_ip,
       created_at, updated_at
FROM users
WHERE id = ? LIMIT 1;

-- name: GetUserByUsername :one
SELECT id, company_profile_id, username, email, password_hash, full_name,
       title, employee_code, status, failed_login_attempts, lockout_until,
       password_changed_at, must_change_password_next, mfa_secret, is_mfa_enabled,
       digital_cert_subject, digital_cert_serial, last_login_at, last_login_ip,
       created_at, updated_at
FROM users
WHERE company_profile_id = ? AND username = ? LIMIT 1;

-- name: GetUserByEmail :one
SELECT id, company_profile_id, username, email, password_hash, full_name,
       title, employee_code, status, failed_login_attempts, lockout_until,
       password_changed_at, must_change_password_next, mfa_secret, is_mfa_enabled,
       digital_cert_subject, digital_cert_serial, last_login_at, last_login_ip,
       created_at, updated_at
FROM users
WHERE company_profile_id = ? AND email = ? LIMIT 1;

-- name: ListUsersByCompany :many
SELECT id, company_profile_id, username, email, password_hash, full_name,
       title, employee_code, status, failed_login_attempts, lockout_until,
       password_changed_at, must_change_password_next, mfa_secret, is_mfa_enabled,
       digital_cert_subject, digital_cert_serial, last_login_at, last_login_ip,
       created_at, updated_at
FROM users
WHERE company_profile_id = ?
ORDER BY created_at DESC;

-- name: UpdateUserStatus :exec
UPDATE users
SET status = ?, lockout_until = ?, failed_login_attempts = ?
WHERE id = ?;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = ?, password_changed_at = ?, must_change_password_next = ?
WHERE id = ?;

-- name: UpdateUserFailedLogin :exec
UPDATE users
SET failed_login_attempts = ?, status = ?, lockout_until = ?
WHERE id = ?;

-- name: UpdateUserSuccessfulLogin :exec
UPDATE users
SET failed_login_attempts = 0, lockout_until = NULL, status = 'ACTIVE',
    last_login_at = ?, last_login_ip = ?
WHERE id = ?;

-- name: AssignUserRole :exec
INSERT INTO user_roles (user_id, role_id, assigned_by)
VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE assigned_at = CURRENT_TIMESTAMP;

-- name: RemoveUserRole :exec
DELETE FROM user_roles
WHERE user_id = ? AND role_id = ?;

-- name: GetUserRoles :many
SELECT r.id, r.code, r.name, r.description, r.is_system, r.created_at, r.updated_at
FROM roles r
JOIN user_roles ur ON r.id = ur.role_id
WHERE ur.user_id = ?
ORDER BY r.code;

-- name: ListRoles :many
SELECT id, code, name, description, is_system, created_at, updated_at
FROM roles
ORDER BY code;

-- name: GetRoleByCode :one
SELECT id, code, name, description, is_system, created_at, updated_at
FROM roles
WHERE code = ? LIMIT 1;

-- name: AssignUserOrgUnitScope :exec
INSERT INTO user_org_unit_scopes (id, user_id, org_unit_id, is_default, include_children)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE is_default = VALUES(is_default), include_children = VALUES(include_children);

-- name: GetUserOrgUnitScopes :many
SELECT s.id, s.user_id, s.org_unit_id, s.is_default, s.include_children, s.created_at,
       b.code as branch_code, b.name as branch_name
FROM user_org_unit_scopes s
JOIN branch_org_units b ON s.org_unit_id = b.id
WHERE s.user_id = ?
ORDER BY s.is_default DESC, b.code ASC;

-- name: RemoveUserOrgUnitScope :exec
DELETE FROM user_org_unit_scopes
WHERE user_id = ? AND org_unit_id = ?;

-- name: ListSoDConflictRules :many
SELECT id, role_code_a, role_code_b, legal_basis, description
FROM sod_conflict_rules
ORDER BY role_code_a, role_code_b;
