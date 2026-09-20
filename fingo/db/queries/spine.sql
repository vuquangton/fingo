-- Currency Queries

-- name: UpsertCurrency :exec
INSERT INTO currencies (
    code, company_profile_id, name, symbol, decimal_places, is_base, is_active
) VALUES (
    ?, ?, ?, ?, ?, ?, ?
) ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    symbol = VALUES(symbol),
    decimal_places = VALUES(decimal_places),
    is_base = VALUES(is_base),
    is_active = VALUES(is_active),
    updated_at = CURRENT_TIMESTAMP;

-- name: GetCurrencyByCode :one
SELECT code, company_profile_id, name, symbol, decimal_places, is_base, is_active, created_at, updated_at
FROM currencies
WHERE company_profile_id = ? AND code = ?
LIMIT 1;

-- name: GetBaseCurrency :one
SELECT code, company_profile_id, name, symbol, decimal_places, is_base, is_active, created_at, updated_at
FROM currencies
WHERE company_profile_id = ? AND is_base = TRUE
LIMIT 1;

-- name: ListCurrenciesByCompany :many
SELECT code, company_profile_id, name, symbol, decimal_places, is_base, is_active, created_at, updated_at
FROM currencies
WHERE company_profile_id = ?
ORDER BY is_base DESC, code ASC;

-- Exchange Rate Queries

-- name: UpsertExchangeRate :exec
INSERT INTO exchange_rates (
    id, company_profile_id, currency_code, rate_date, rate_type, rate, source_bank, created_by
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?
) ON DUPLICATE KEY UPDATE
    rate = VALUES(rate),
    source_bank = VALUES(source_bank);

-- name: GetEffectiveExchangeRate :one
SELECT id, company_profile_id, currency_code, rate_date, rate_type, rate, source_bank, created_at, created_by
FROM exchange_rates
WHERE company_profile_id = ?
  AND currency_code = ?
  AND rate_type = ?
  AND rate_date <= ?
ORDER BY rate_date DESC
LIMIT 1;

-- name: ListExchangeRatesByDate :many
SELECT id, company_profile_id, currency_code, rate_date, rate_type, rate, source_bank, created_at, created_by
FROM exchange_rates
WHERE company_profile_id = ? AND rate_date = ?
ORDER BY currency_code ASC, rate_type ASC;

-- Chart of Accounts Queries

-- name: UpsertAccount :exec
INSERT INTO accounts (
    id, company_profile_id, code, name, english_name, parent_id,
    account_level, nature, category, is_leaf, is_foreign_currency,
    requires_partner, requires_bank_account, requires_cost_center, requires_expense_item, is_active
) VALUES (
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?
) ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    english_name = VALUES(english_name),
    parent_id = VALUES(parent_id),
    account_level = VALUES(account_level),
    nature = VALUES(nature),
    category = VALUES(category),
    is_leaf = VALUES(is_leaf),
    is_foreign_currency = VALUES(is_foreign_currency),
    requires_partner = VALUES(requires_partner),
    requires_bank_account = VALUES(requires_bank_account),
    requires_cost_center = VALUES(requires_cost_center),
    requires_expense_item = VALUES(requires_expense_item),
    is_active = VALUES(is_active),
    updated_at = CURRENT_TIMESTAMP;

-- name: GetAccountByID :one
SELECT id, company_profile_id, code, name, english_name, parent_id,
       account_level, nature, category, is_leaf, is_foreign_currency,
       requires_partner, requires_bank_account, requires_cost_center, requires_expense_item, is_active,
       created_at, updated_at
FROM accounts
WHERE id = ?
LIMIT 1;

-- name: GetAccountByCode :one
SELECT id, company_profile_id, code, name, english_name, parent_id,
       account_level, nature, category, is_leaf, is_foreign_currency,
       requires_partner, requires_bank_account, requires_cost_center, requires_expense_item, is_active,
       created_at, updated_at
FROM accounts
WHERE company_profile_id = ? AND code = ?
LIMIT 1;

-- name: ListAccountsByCompany :many
SELECT id, company_profile_id, code, name, english_name, parent_id,
       account_level, nature, category, is_leaf, is_foreign_currency,
       requires_partner, requires_bank_account, requires_cost_center, requires_expense_item, is_active,
       created_at, updated_at
FROM accounts
WHERE company_profile_id = ?
ORDER BY code ASC;

-- name: ListChildAccounts :many
SELECT id, company_profile_id, code, name, english_name, parent_id,
       account_level, nature, category, is_leaf, is_foreign_currency,
       requires_partner, requires_bank_account, requires_cost_center, requires_expense_item, is_active,
       created_at, updated_at
FROM accounts
WHERE parent_id = ?
ORDER BY code ASC;

-- name: UpdateAccountLeafStatus :exec
UPDATE accounts
SET is_leaf = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- Fiscal Years Queries

-- name: UpsertFiscalYear :exec
INSERT INTO fiscal_years (
    id, company_profile_id, year, start_date, end_date, status
) VALUES (
    ?, ?, ?, ?, ?, ?
) ON DUPLICATE KEY UPDATE
    status = VALUES(status),
    updated_at = CURRENT_TIMESTAMP;

-- name: GetFiscalYearByYear :one
SELECT id, company_profile_id, year, start_date, end_date, status, created_at, updated_at
FROM fiscal_years
WHERE company_profile_id = ? AND year = ?
LIMIT 1;

-- name: ListFiscalYearsByCompany :many
SELECT id, company_profile_id, year, start_date, end_date, status, created_at, updated_at
FROM fiscal_years
WHERE company_profile_id = ?
ORDER BY year DESC;

-- Accounting Periods Queries

-- name: UpsertAccountingPeriod :exec
INSERT INTO accounting_periods (
    id, fiscal_year_id, company_profile_id, period_number, name,
    start_date, end_date, lock_date, status
) VALUES (
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?
) ON DUPLICATE KEY UPDATE
    lock_date = VALUES(lock_date),
    status = VALUES(status),
    updated_at = CURRENT_TIMESTAMP;

-- name: GetPeriodByDate :one
SELECT id, fiscal_year_id, company_profile_id, period_number, name,
       start_date, end_date, lock_date, status, created_at, updated_at
FROM accounting_periods
WHERE company_profile_id = ?
  AND start_date <= ?
  AND end_date >= ?
LIMIT 1;

-- name: ListPeriodsByFiscalYear :many
SELECT id, fiscal_year_id, company_profile_id, period_number, name,
       start_date, end_date, lock_date, status, created_at, updated_at
FROM accounting_periods
WHERE fiscal_year_id = ?
ORDER BY period_number ASC;

-- name: UpdatePeriodLock :exec
UPDATE accounting_periods
SET lock_date = ?, status = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- Cost Centers Queries

-- name: UpsertCostCenter :exec
INSERT INTO cost_centers (
    id, company_profile_id, branch_id, code, name, parent_id, is_leaf, is_active
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?
) ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    parent_id = VALUES(parent_id),
    is_leaf = VALUES(is_leaf),
    is_active = VALUES(is_active),
    updated_at = CURRENT_TIMESTAMP;

-- name: GetCostCenterByCode :one
SELECT id, company_profile_id, branch_id, code, name, parent_id, is_leaf, is_active, created_at, updated_at
FROM cost_centers
WHERE company_profile_id = ? AND code = ?
LIMIT 1;

-- name: ListCostCentersByCompany :many
SELECT id, company_profile_id, branch_id, code, name, parent_id, is_leaf, is_active, created_at, updated_at
FROM cost_centers
WHERE company_profile_id = ?
ORDER BY code ASC;

-- Expense Items Queries

-- name: UpsertExpenseItem :exec
INSERT INTO expense_items (
    id, company_profile_id, code, name, category, parent_id, is_leaf, is_active
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?
) ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    category = VALUES(category),
    parent_id = VALUES(parent_id),
    is_leaf = VALUES(is_leaf),
    is_active = VALUES(is_active),
    updated_at = CURRENT_TIMESTAMP;

-- name: GetExpenseItemByCode :one
SELECT id, company_profile_id, code, name, category, parent_id, is_leaf, is_active, created_at, updated_at
FROM expense_items
WHERE company_profile_id = ? AND code = ?
LIMIT 1;

-- name: ListExpenseItemsByCompany :many
SELECT id, company_profile_id, code, name, category, parent_id, is_leaf, is_active, created_at, updated_at
FROM expense_items
WHERE company_profile_id = ?
ORDER BY code ASC;
