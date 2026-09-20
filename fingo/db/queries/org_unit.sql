-- name: CreateBranchOrgUnit :exec
INSERT INTO branch_org_units (
    id, parent_id, company_profile_id, code, name, unit_type,
    accounting_governance, tax_filing_mechanism, tax_code, tax_authority_code,
    tax_authority_name, province_city_code, address, manager_name,
    chief_accountant, internal_receivable_account, internal_payable_account,
    has_own_einvoice, is_active
) VALUES (
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?,
    ?, ?
);

-- name: GetBranchOrgUnitByID :one
SELECT id, parent_id, company_profile_id, code, name, unit_type,
       accounting_governance, tax_filing_mechanism, tax_code, tax_authority_code,
       tax_authority_name, province_city_code, address, manager_name,
       chief_accountant, internal_receivable_account, internal_payable_account,
       has_own_einvoice, is_active, created_at, updated_at
FROM branch_org_units
WHERE id = ?
LIMIT 1;

-- name: GetBranchOrgUnitByCode :one
SELECT id, parent_id, company_profile_id, code, name, unit_type,
       accounting_governance, tax_filing_mechanism, tax_code, tax_authority_code,
       tax_authority_name, province_city_code, address, manager_name,
       chief_accountant, internal_receivable_account, internal_payable_account,
       has_own_einvoice, is_active, created_at, updated_at
FROM branch_org_units
WHERE company_profile_id = ? AND code = ?
LIMIT 1;

-- name: ListBranchOrgUnitsByCompany :many
SELECT id, parent_id, company_profile_id, code, name, unit_type,
       accounting_governance, tax_filing_mechanism, tax_code, tax_authority_code,
       tax_authority_name, province_city_code, address, manager_name,
       chief_accountant, internal_receivable_account, internal_payable_account,
       has_own_einvoice, is_active, created_at, updated_at
FROM branch_org_units
WHERE company_profile_id = ? AND is_active = TRUE
ORDER BY code ASC;

-- name: UpdateBranchOrgUnit :exec
UPDATE branch_org_units
SET name = ?,
    accounting_governance = ?,
    tax_filing_mechanism = ?,
    tax_code = ?,
    tax_authority_code = ?,
    tax_authority_name = ?,
    province_city_code = ?,
    address = ?,
    manager_name = ?,
    chief_accountant = ?,
    internal_receivable_account = ?,
    internal_payable_account = ?,
    has_own_einvoice = ?,
    is_active = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;
