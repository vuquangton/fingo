-- name: GetActiveCompanyProfile :one
SELECT id, tax_code, legal_name, trade_name, english_name, address, province_city, district_ward,
       phone, email, website, legal_representative, representative_position, chief_accountant,
       tax_authority_code, tax_authority_name, state_budget_chapter, regime, base_currency,
       fiscal_year_start_month, vat_method, costing_method, business_type, registered_banks,
       einvoice_config, is_active, lock_date, created_at, updated_at
FROM company_profile
WHERE is_active = TRUE
LIMIT 1;

-- name: GetCompanyProfileByTaxCode :one
SELECT id, tax_code, legal_name, trade_name, english_name, address, province_city, district_ward,
       phone, email, website, legal_representative, representative_position, chief_accountant,
       tax_authority_code, tax_authority_name, state_budget_chapter, regime, base_currency,
       fiscal_year_start_month, vat_method, costing_method, business_type, registered_banks,
       einvoice_config, is_active, lock_date, created_at, updated_at
FROM company_profile
WHERE tax_code = ?
LIMIT 1;

-- name: UpsertCompanyProfile :exec
INSERT INTO company_profile (
    id, tax_code, legal_name, trade_name, english_name, address, province_city, district_ward,
    phone, email, website, legal_representative, representative_position, chief_accountant,
    tax_authority_code, tax_authority_name, state_budget_chapter, regime, base_currency,
    fiscal_year_start_month, vat_method, costing_method, business_type, registered_banks,
    einvoice_config, is_active, lock_date
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?
) ON DUPLICATE KEY UPDATE
    legal_name = VALUES(legal_name),
    trade_name = VALUES(trade_name),
    english_name = VALUES(english_name),
    address = VALUES(address),
    province_city = VALUES(province_city),
    district_ward = VALUES(district_ward),
    phone = VALUES(phone),
    email = VALUES(email),
    website = VALUES(website),
    legal_representative = VALUES(legal_representative),
    representative_position = VALUES(representative_position),
    chief_accountant = VALUES(chief_accountant),
    tax_authority_code = VALUES(tax_authority_code),
    tax_authority_name = VALUES(tax_authority_name),
    state_budget_chapter = VALUES(state_budget_chapter),
    regime = VALUES(regime),
    vat_method = VALUES(vat_method),
    costing_method = VALUES(costing_method),
    business_type = VALUES(business_type),
    registered_banks = VALUES(registered_banks),
    einvoice_config = VALUES(einvoice_config),
    is_active = VALUES(is_active),
    lock_date = VALUES(lock_date);

-- name: UpdateLockDate :exec
UPDATE company_profile
SET lock_date = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;
