-- System Options Queries

-- name: UpsertSystemOption :exec
INSERT INTO system_options (
    id, company_profile_id, branch_id, category, option_key,
    option_value, data_type, default_value, scope_level,
    description, is_readonly, is_encrypted, updated_by
) VALUES (
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?
) ON DUPLICATE KEY UPDATE
    option_value = VALUES(option_value),
    updated_at = CURRENT_TIMESTAMP,
    updated_by = VALUES(updated_by);

-- name: GetSystemOption :one
SELECT id, company_profile_id, branch_id, category, option_key,
       option_value, data_type, default_value, scope_level,
       description, is_readonly, is_encrypted, created_at, updated_at, updated_by
FROM system_options
WHERE company_profile_id = ?
  AND ((branch_id IS NULL AND sqlc.narg('branch_id') IS NULL) OR branch_id = sqlc.narg('branch_id'))
  AND option_key = ?
LIMIT 1;

-- name: GetEffectiveSystemOption :one
SELECT id, company_profile_id, branch_id, category, option_key,
       option_value, data_type, default_value, scope_level,
       description, is_readonly, is_encrypted, created_at, updated_at, updated_by
FROM system_options
WHERE company_profile_id = ?
  AND option_key = ?
  AND (branch_id = sqlc.narg('branch_id') OR branch_id IS NULL)
ORDER BY CASE WHEN branch_id = sqlc.narg('branch_id') THEN 0 ELSE 1 END
LIMIT 1;

-- name: ListSystemOptionsByCompany :many
SELECT id, company_profile_id, branch_id, category, option_key,
       option_value, data_type, default_value, scope_level,
       description, is_readonly, is_encrypted, created_at, updated_at, updated_by
FROM system_options
WHERE company_profile_id = ?
ORDER BY category ASC, option_key ASC;

-- name: ListSystemOptionsByCategory :many
SELECT id, company_profile_id, branch_id, category, option_key,
       option_value, data_type, default_value, scope_level,
       description, is_readonly, is_encrypted, created_at, updated_at, updated_by
FROM system_options
WHERE company_profile_id = ? AND category = ?
ORDER BY option_key ASC;

-- name: RecordConfigHistory :exec
INSERT INTO system_config_history (
    id, company_profile_id, option_key, old_value, new_value, changed_by, reason, client_ip
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: ListConfigHistory :many
SELECT id, company_profile_id, option_key, old_value, new_value,
       changed_by, changed_at, reason, client_ip
FROM system_config_history
WHERE company_profile_id = ? AND option_key = ?
ORDER BY changed_at DESC
LIMIT ?;

-- Voucher Numbering Queries

-- name: UpsertVoucherNumberingConfig :exec
INSERT INTO voucher_numbering_configs (
    id, company_profile_id, branch_id, voucher_type,
    prefix, pattern, reset_frequency, current_sequence, last_reset_date
) VALUES (
    ?, ?, ?, ?,
    ?, ?, ?, ?, ?
) ON DUPLICATE KEY UPDATE
    prefix = VALUES(prefix),
    pattern = VALUES(pattern),
    reset_frequency = VALUES(reset_frequency),
    current_sequence = VALUES(current_sequence),
    last_reset_date = VALUES(last_reset_date),
    updated_at = CURRENT_TIMESTAMP;

-- name: GetVoucherNumberingConfigByID :one
SELECT id, company_profile_id, branch_id, voucher_type, prefix, pattern,
       reset_frequency, current_sequence, last_reset_date, created_at, updated_at
FROM voucher_numbering_configs
WHERE id = ?
LIMIT 1;

-- name: GetEffectiveVoucherNumberingConfig :one
SELECT id, company_profile_id, branch_id, voucher_type, prefix, pattern,
       reset_frequency, current_sequence, last_reset_date, created_at, updated_at
FROM voucher_numbering_configs
WHERE company_profile_id = ?
  AND voucher_type = ?
  AND (branch_id = sqlc.narg('branch_id') OR branch_id IS NULL)
ORDER BY CASE WHEN branch_id = sqlc.narg('branch_id') THEN 0 ELSE 1 END
LIMIT 1;

-- name: GetEffectiveVoucherNumberingConfigForUpdate :one
SELECT id, company_profile_id, branch_id, voucher_type, prefix, pattern,
       reset_frequency, current_sequence, last_reset_date, created_at, updated_at
FROM voucher_numbering_configs
WHERE company_profile_id = ?
  AND voucher_type = ?
  AND (branch_id = sqlc.narg('branch_id') OR branch_id IS NULL)
ORDER BY CASE WHEN branch_id = sqlc.narg('branch_id') THEN 0 ELSE 1 END
LIMIT 1
FOR UPDATE;

-- name: UpdateVoucherSequence :exec
UPDATE voucher_numbering_configs
SET current_sequence = ?,
    last_reset_date = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: ListVoucherNumberingConfigs :many
SELECT id, company_profile_id, branch_id, voucher_type, prefix, pattern,
       reset_frequency, current_sequence, last_reset_date, created_at, updated_at
FROM voucher_numbering_configs
WHERE company_profile_id = ?
ORDER BY voucher_type ASC;
