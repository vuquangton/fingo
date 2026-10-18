-- name: CreateDocument :exec
INSERT INTO documents (id, partner_id, voucher_id, kind, reference, document_date, due_date, amount)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetDocumentByID :one
SELECT id, partner_id, voucher_id, kind, reference, document_date, due_date, amount, created_at
FROM documents
WHERE id = ? LIMIT 1;

-- name: CreateAllocation :exec
INSERT INTO allocations (id, document_id, voucher_id, amount, allocated_at)
VALUES (?, ?, ?, ?, ?);

-- name: ListDocumentsByPartner :many
-- Every document for a partner, oldest first — the order a payment settles them
-- in.
--
-- Fully settled documents come back too: the caller decides what "open" means
-- rather than the query quietly hiding a paid invoice.
SELECT id, partner_id, voucher_id, kind, reference, document_date, due_date, amount, created_at
FROM documents
WHERE partner_id = ?
ORDER BY document_date ASC, reference ASC;

-- name: ListAllocationsByPartner :many
-- What has been settled against this partner's documents.
--
-- Returned raw rather than summed in SQL: sqlc cannot type the aggregate, and
-- money is added with exact decimals in Go everywhere else. A report that
-- totals differently from the ledger is worse than no report.
SELECT a.document_id, a.voucher_id, a.amount, a.allocated_at
FROM allocations a
JOIN documents d ON d.id = a.document_id
WHERE d.partner_id = ?
ORDER BY a.allocated_at ASC, a.id ASC;

