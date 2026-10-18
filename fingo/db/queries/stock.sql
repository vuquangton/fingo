-- name: CreateStockMovement :exec
INSERT INTO stock_movements (id, item_code, warehouse_code, kind, quantity, cost, voucher_id, moved_at, note)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListStockMovements :many
-- The whole history of one position, in insertion order — the order the balance
-- is replayed in.
SELECT id, item_code, warehouse_code, kind, quantity, cost, voucher_id, moved_at, note
FROM stock_movements
WHERE item_code = ? AND warehouse_code = ?
ORDER BY seq ASC;

-- name: ListStockPositions :many
-- Every item and warehouse that has ever moved, so a caller can enumerate the
-- stock it has without a separate register that could disagree.
SELECT DISTINCT item_code, warehouse_code FROM stock_movements
ORDER BY item_code ASC, warehouse_code ASC;

-- name: SetInventoryPolicy :exec
INSERT INTO inventory_policies (fiscal_year, method, chosen_at)
VALUES (?, ?, ?);

-- name: ListInventoryPolicies :many
SELECT fiscal_year, method FROM inventory_policies ORDER BY fiscal_year ASC;
