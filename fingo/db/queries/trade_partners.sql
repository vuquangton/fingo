-- name: CreateTradePartner :exec
INSERT INTO trade_partners (id, name, partner_type, tax_id, payment_terms_days, is_active)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetTradePartner :one
SELECT id, name, partner_type, tax_id, payment_terms_days, is_active
FROM trade_partners
WHERE id = ? LIMIT 1;

-- name: ListTradePartners :many
-- Ordered by name, because that is how an operator looks for one.
SELECT id, name, partner_type, tax_id, payment_terms_days, is_active
FROM trade_partners
ORDER BY name ASC, id ASC;

-- name: ListTradePartnersByTaxID :many
-- The rows already carrying a tax id, for the duplicate warning.
--
-- Only ever called with a non-empty tax id: NULL never matches, and a partner
-- with no tax id cannot duplicate one.
SELECT id, name, partner_type, tax_id, payment_terms_days, is_active
FROM trade_partners
WHERE tax_id = ?
ORDER BY id ASC;

-- name: GetTradePartnerForUpdate :one
-- The locking read an update goes through.
--
-- Without it, "does this partner exist" would have to be answered by the row
-- count of the UPDATE — and MySQL counts the rows an UPDATE CHANGED, not the
-- rows it matched, so re-saving a partner under the values it already has would
-- report zero and look like a missing row.
SELECT id, name, partner_type, tax_id, payment_terms_days, is_active
FROM trade_partners
WHERE id = ? LIMIT 1
FOR UPDATE;

-- name: UpdateTradePartner :execrows
UPDATE trade_partners
SET name = ?, partner_type = ?, tax_id = ?, payment_terms_days = ?, is_active = ?
WHERE id = ?;

-- name: DeleteTradePartner :execrows
DELETE FROM trade_partners
WHERE id = ?;
