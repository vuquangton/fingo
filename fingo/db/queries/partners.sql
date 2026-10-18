-- name: CreatePartner :exec
INSERT INTO partners (id, code, name, tax_code, address, kind, is_active)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetPartnerByCode :one
SELECT id, code, name, tax_code, address, kind, is_active, created_at, updated_at
FROM partners
WHERE code = ? LIMIT 1;

-- name: ListPartners :many
SELECT id, code, name, tax_code, address, kind, is_active, created_at, updated_at
FROM partners
ORDER BY code ASC;

-- name: ListPostablePartners :many
-- A partner who is both is a counterparty to a sale and to a purchase, so the
-- filter is by side rather than by exact kind.
SELECT id, code, name, tax_code, address, kind, is_active, created_at, updated_at
FROM partners
WHERE is_active = TRUE AND (kind = ? OR kind = 'BOTH')
ORDER BY code ASC;

-- name: ListPartnerLedgerLines :many
-- The lines of Sổ chi tiết TK 131, for the caller to total.
--
-- Returned raw rather than summed in SQL: money is added with exact decimals in
-- Go everywhere else, and a report that totals differently from the ledger is
-- worse than no report.
--
-- The LIKE covers any prescribed detail accounts under 131 as well as 131
-- itself, so this does not depend on which level a client posts at. The join
-- drops lines whose partner_id is NULL — everything that belongs to nobody in
-- particular.
SELECT p.id AS partner_id, p.code AS partner_code, p.name AS partner_name,
       vl.side, vl.amount
FROM voucher_lines vl
JOIN vouchers v ON v.id = vl.voucher_id
JOIN accounts a ON a.id = vl.account_id
JOIN partners p ON p.id = vl.partner_id
WHERE v.is_posted = TRUE AND a.code LIKE '131%'
ORDER BY p.code ASC, vl.voucher_id ASC, vl.line_order ASC;


