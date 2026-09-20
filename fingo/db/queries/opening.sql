-- name: CreateOpeningBatch :exec
INSERT INTO opening_batches (
    id, company_profile_id, as_of_date, status, total_debit, total_credit, notes, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetOpeningBatchByID :one
SELECT * FROM opening_batches WHERE id = ? LIMIT 1;

-- name: GetOpeningBatchByDate :one
SELECT * FROM opening_batches 
WHERE company_profile_id = ? AND as_of_date = ? 
LIMIT 1;

-- name: UpdateOpeningBatchStatus :exec
UPDATE opening_batches 
SET status = ?, committed_at = ?, committed_by = ?, updated_at = ?
WHERE id = ?;

-- name: UpdateOpeningBatchTotals :exec
UPDATE opening_batches 
SET total_debit = ?, total_credit = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteAccountBalancesByBatch :exec
DELETE FROM account_opening_balances WHERE batch_id = ?;

-- name: InsertAccountBalance :exec
INSERT INTO account_opening_balances (
    id, batch_id, account_id, currency_code, debit_amount_fc, credit_amount_fc,
    exchange_rate, debit_amount_vnd, credit_amount_vnd, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListAccountBalancesByBatch :many
SELECT aob.*, a.code as account_code, a.name as account_name
FROM account_opening_balances aob
JOIN accounts a ON aob.account_id = a.id
WHERE aob.batch_id = ?
ORDER BY a.code ASC;

-- name: DeleteCustomerBalancesByBatch :exec
DELETE FROM customer_opening_balances WHERE batch_id = ?;

-- name: InsertCustomerBalance :exec
INSERT INTO customer_opening_balances (
    id, batch_id, customer_id, invoice_no, invoice_date, due_date, currency_code,
    debit_amount_fc, credit_amount_fc, exchange_rate, debit_amount_vnd, credit_amount_vnd,
    notes, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListCustomerBalancesByBatch :many
SELECT cob.*, c.code as customer_code, c.name as customer_name
FROM customer_opening_balances cob
JOIN customers c ON cob.customer_id = c.id
WHERE cob.batch_id = ?
ORDER BY c.code ASC;

-- name: DeleteVendorBalancesByBatch :exec
DELETE FROM vendor_opening_balances WHERE batch_id = ?;

-- name: InsertVendorBalance :exec
INSERT INTO vendor_opening_balances (
    id, batch_id, vendor_id, bill_no, bill_date, due_date, currency_code,
    debit_amount_fc, credit_amount_fc, exchange_rate, debit_amount_vnd, credit_amount_vnd,
    notes, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListVendorBalancesByBatch :many
SELECT vob.*, v.code as vendor_code, v.name as vendor_name
FROM vendor_opening_balances vob
JOIN vendors v ON vob.vendor_id = v.id
WHERE vob.batch_id = ?
ORDER BY v.code ASC;

-- name: DeleteInventoryBalancesByBatch :exec
DELETE FROM inventory_opening_balances WHERE batch_id = ?;

-- name: InsertInventoryBalance :exec
INSERT INTO inventory_opening_balances (
    id, batch_id, warehouse_id, item_id, uom_id, quantity, unit_cost, total_amount_vnd,
    batch_number, expiry_date, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListInventoryBalancesByBatch :many
SELECT iob.*, w.code as warehouse_code, w.name as warehouse_name,
       i.code as item_code, i.name as item_name, u.code as uom_code
FROM inventory_opening_balances iob
JOIN warehouses w ON iob.warehouse_id = w.id
JOIN items i ON iob.item_id = i.id
JOIN unit_of_measures u ON iob.uom_id = u.id
WHERE iob.batch_id = ?
ORDER BY w.code, i.code ASC;

-- name: DeleteAssetBalancesByBatch :exec
DELETE FROM asset_opening_balances WHERE batch_id = ?;

-- name: InsertAssetBalance :exec
INSERT INTO asset_opening_balances (
    id, batch_id, asset_code, asset_name, asset_account_id, depreciation_account_id,
    cost_account_id, department_id, acquisition_date, start_depreciation_date,
    original_cost, accumulated_depreciation, net_book_value, useful_life_months,
    remaining_life_months, monthly_depreciation, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListAssetBalancesByBatch :many
SELECT aob.*, aa.code as asset_account_code, da.code as depreciation_account_code, ca.code as cost_account_code
FROM asset_opening_balances aob
JOIN accounts aa ON aob.asset_account_id = aa.id
JOIN accounts da ON aob.depreciation_account_id = da.id
JOIN accounts ca ON aob.cost_account_id = ca.id
WHERE aob.batch_id = ?
ORDER BY aob.asset_code ASC;
