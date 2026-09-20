-- ============================================================================
-- UnitOfMeasure Queries
-- ============================================================================

-- name: CreateUOM :exec
INSERT INTO unit_of_measures (
    id, company_profile_id, code, name, description, is_active, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    description = VALUES(description),
    is_active = VALUES(is_active),
    updated_at = VALUES(updated_at);

-- name: GetUOMByID :one
SELECT * FROM unit_of_measures
WHERE id = ? LIMIT 1;

-- name: GetUOMByCode :one
SELECT * FROM unit_of_measures
WHERE company_profile_id = ? AND code = ? LIMIT 1;

-- name: ListUOMs :many
SELECT * FROM unit_of_measures
WHERE company_profile_id = ?
ORDER BY code ASC;

-- ============================================================================
-- UOMConversion Queries
-- ============================================================================

-- name: CreateUOMConversion :exec
INSERT INTO uom_conversions (
    id, company_profile_id, item_id, from_uom_id, to_uom_id, multiplier, conversion_type, is_active, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    multiplier = VALUES(multiplier),
    conversion_type = VALUES(conversion_type),
    is_active = VALUES(is_active),
    updated_at = VALUES(updated_at);

-- name: GetUOMConversion :one
SELECT * FROM uom_conversions
WHERE company_profile_id = ?
  AND ((item_id = ? OR (item_id IS NULL AND ? IS NULL)))
  AND from_uom_id = ?
  AND to_uom_id = ?
LIMIT 1;

-- name: ListUOMConversionsByItem :many
SELECT * FROM uom_conversions
WHERE company_profile_id = ? AND (item_id = ? OR item_id IS NULL)
ORDER BY created_at ASC;

-- ============================================================================
-- Warehouse Queries
-- ============================================================================

-- name: CreateWarehouse :exec
INSERT INTO warehouses (
    id, company_profile_id, branch_id, code, name, address, default_account_id, is_active, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    branch_id = VALUES(branch_id),
    name = VALUES(name),
    address = VALUES(address),
    default_account_id = VALUES(default_account_id),
    is_active = VALUES(is_active),
    updated_at = VALUES(updated_at);

-- name: GetWarehouseByID :one
SELECT * FROM warehouses
WHERE id = ? LIMIT 1;

-- name: GetWarehouseByCode :one
SELECT * FROM warehouses
WHERE company_profile_id = ? AND code = ? LIMIT 1;

-- name: ListWarehouses :many
SELECT * FROM warehouses
WHERE company_profile_id = ?
ORDER BY code ASC;

-- ============================================================================
-- BankAccount Queries
-- ============================================================================

-- name: CreateBankAccount :exec
INSERT INTO bank_accounts (
    id, company_profile_id, branch_id, account_number, bank_name, bank_code, branch_name, currency_code, gl_account_id, is_active, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    branch_id = VALUES(branch_id),
    bank_name = VALUES(bank_name),
    bank_code = VALUES(bank_code),
    branch_name = VALUES(branch_name),
    currency_code = VALUES(currency_code),
    gl_account_id = VALUES(gl_account_id),
    is_active = VALUES(is_active),
    updated_at = VALUES(updated_at);

-- name: GetBankAccountByID :one
SELECT * FROM bank_accounts
WHERE id = ? LIMIT 1;

-- name: GetBankAccountByNumber :one
SELECT * FROM bank_accounts
WHERE company_profile_id = ? AND account_number = ? LIMIT 1;

-- name: ListBankAccounts :many
SELECT * FROM bank_accounts
WHERE company_profile_id = ?
ORDER BY account_number ASC;

-- ============================================================================
-- Customer Queries
-- ============================================================================

-- name: CreateCustomer :exec
INSERT INTO customers (
    id, company_profile_id, code, name, tax_code, address, phone, email, contact_person,
    payment_term_days, credit_limit, enforce_credit_limit, default_ar_account_id, is_active, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    tax_code = VALUES(tax_code),
    address = VALUES(address),
    phone = VALUES(phone),
    email = VALUES(email),
    contact_person = VALUES(contact_person),
    payment_term_days = VALUES(payment_term_days),
    credit_limit = VALUES(credit_limit),
    enforce_credit_limit = VALUES(enforce_credit_limit),
    default_ar_account_id = VALUES(default_ar_account_id),
    is_active = VALUES(is_active),
    updated_at = VALUES(updated_at);

-- name: GetCustomerByID :one
SELECT * FROM customers
WHERE id = ? LIMIT 1;

-- name: GetCustomerByCode :one
SELECT * FROM customers
WHERE company_profile_id = ? AND code = ? LIMIT 1;

-- name: GetCustomerByTaxCode :one
SELECT * FROM customers
WHERE company_profile_id = ? AND tax_code = ? LIMIT 1;

-- name: ListCustomers :many
SELECT * FROM customers
WHERE company_profile_id = ?
ORDER BY code ASC;

-- ============================================================================
-- Vendor Queries
-- ============================================================================

-- name: CreateVendor :exec
INSERT INTO vendors (
    id, company_profile_id, code, name, tax_code, address, phone, email, contact_person,
    bank_account_number, bank_name, bank_branch, payment_term_days, default_ap_account_id, is_active, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    tax_code = VALUES(tax_code),
    address = VALUES(address),
    phone = VALUES(phone),
    email = VALUES(email),
    contact_person = VALUES(contact_person),
    bank_account_number = VALUES(bank_account_number),
    bank_name = VALUES(bank_name),
    bank_branch = VALUES(bank_branch),
    payment_term_days = VALUES(payment_term_days),
    default_ap_account_id = VALUES(default_ap_account_id),
    is_active = VALUES(is_active),
    updated_at = VALUES(updated_at);

-- name: GetVendorByID :one
SELECT * FROM vendors
WHERE id = ? LIMIT 1;

-- name: GetVendorByCode :one
SELECT * FROM vendors
WHERE company_profile_id = ? AND code = ? LIMIT 1;

-- name: GetVendorByTaxCode :one
SELECT * FROM vendors
WHERE company_profile_id = ? AND tax_code = ? LIMIT 1;

-- name: ListVendors :many
SELECT * FROM vendors
WHERE company_profile_id = ?
ORDER BY code ASC;

-- ============================================================================
-- Item Queries
-- ============================================================================

-- name: CreateItem :exec
INSERT INTO items (
    id, company_profile_id, code, name, barcode, item_type, base_uom_id,
    default_warehouse_id, inventory_account_id, cogs_account_id, revenue_account_id,
    default_vat_rate, standard_cost_price, standard_sale_price, is_active, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    barcode = VALUES(barcode),
    item_type = VALUES(item_type),
    base_uom_id = VALUES(base_uom_id),
    default_warehouse_id = VALUES(default_warehouse_id),
    inventory_account_id = VALUES(inventory_account_id),
    cogs_account_id = VALUES(cogs_account_id),
    revenue_account_id = VALUES(revenue_account_id),
    default_vat_rate = VALUES(default_vat_rate),
    standard_cost_price = VALUES(standard_cost_price),
    standard_sale_price = VALUES(standard_sale_price),
    is_active = VALUES(is_active),
    updated_at = VALUES(updated_at);

-- name: GetItemByID :one
SELECT * FROM items
WHERE id = ? LIMIT 1;

-- name: GetItemByCode :one
SELECT * FROM items
WHERE company_profile_id = ? AND code = ? LIMIT 1;

-- name: ListItems :many
SELECT * FROM items
WHERE company_profile_id = ?
ORDER BY code ASC;

-- ============================================================================
-- Employee Queries
-- ============================================================================

-- name: CreateEmployee :exec
INSERT INTO employees (
    id, company_profile_id, branch_id, code, full_name, department, position,
    citizen_id, tax_code, social_insurance_no, base_salary, salary_coefficient,
    bank_account_number, bank_name, default_advance_acc, default_payroll_acc,
    is_active, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    branch_id = VALUES(branch_id),
    full_name = VALUES(full_name),
    department = VALUES(department),
    position = VALUES(position),
    citizen_id = VALUES(citizen_id),
    tax_code = VALUES(tax_code),
    social_insurance_no = VALUES(social_insurance_no),
    base_salary = VALUES(base_salary),
    salary_coefficient = VALUES(salary_coefficient),
    bank_account_number = VALUES(bank_account_number),
    bank_name = VALUES(bank_name),
    default_advance_acc = VALUES(default_advance_acc),
    default_payroll_acc = VALUES(default_payroll_acc),
    is_active = VALUES(is_active),
    updated_at = VALUES(updated_at);

-- name: GetEmployeeByID :one
SELECT * FROM employees
WHERE id = ? LIMIT 1;

-- name: GetEmployeeByCode :one
SELECT * FROM employees
WHERE company_profile_id = ? AND code = ? LIMIT 1;

-- name: GetEmployeeByCitizenID :one
SELECT * FROM employees
WHERE company_profile_id = ? AND citizen_id = ? LIMIT 1;

-- name: ListEmployees :many
SELECT * FROM employees
WHERE company_profile_id = ?
ORDER BY code ASC;
