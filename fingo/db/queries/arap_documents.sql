-- name: CreateArapDocument :exec
INSERT INTO arap_documents (id, doc_type, partner_id, document_date, period_id, vat_rate_id,
                            total_amount, allocated_amount, currency, status)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetArapDocument :one
SELECT id, doc_type, partner_id, document_date, period_id, vat_rate_id,
       total_amount, allocated_amount, currency, status
FROM arap_documents
WHERE id = ? LIMIT 1;

-- name: GetArapDocumentForUpdate :one
-- The row lock every settlement goes through.
--
-- Reading the document without this and writing it back afterwards is the
-- over-allocation bug: two payments would each read the same outstanding
-- balance, each decide it may spend it, and each write.
SELECT id, doc_type, partner_id, document_date, period_id, vat_rate_id,
       total_amount, allocated_amount, currency, status
FROM arap_documents
WHERE id = ? LIMIT 1
FOR UPDATE;

-- name: ListArapDocuments :many
-- Oldest first, which is the order a payment settles them in and the order an
-- aging report reads them in.
SELECT id, doc_type, partner_id, document_date, period_id, vat_rate_id,
       total_amount, allocated_amount, currency, status
FROM arap_documents
ORDER BY document_date ASC, id ASC;

-- name: UpdateArapDocumentSettlement :execrows
-- Allocated and status move together, in one statement, so there is no moment
-- at which the row says one thing and means the other.
UPDATE arap_documents
SET allocated_amount = ?, status = ?
WHERE id = ?;

-- name: VoidArapDocument :execrows
UPDATE arap_documents
SET status = ?
WHERE id = ? AND status <> 'VOID';

-- name: CreateArapLine :exec
INSERT INTO arap_lines (document_id, line_no, account_id, debit, credit, vat_amount, base_amount)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListArapLines :many
SELECT document_id, line_no, account_id, debit, credit, vat_amount, base_amount
FROM arap_lines
WHERE document_id = ?
ORDER BY line_no ASC;

-- name: CreateArapAllocation :exec
INSERT INTO arap_allocations (id, document_id, payment_id, amount, currency, allocated_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ListArapAllocations :many
SELECT id, document_id, payment_id, amount, currency, allocated_at
FROM arap_allocations
WHERE document_id = ?
ORDER BY allocated_at ASC, id ASC;

-- name: CreateArapCredit :exec
INSERT INTO arap_credits (id, partner_id, document_id, payment_id, amount, currency, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListArapCreditsByPartner :many
SELECT id, partner_id, document_id, payment_id, amount, currency, created_at
FROM arap_credits
WHERE partner_id = ?
ORDER BY created_at ASC, id ASC;

-- name: CountArapAllocationsForPayment :one
-- Whether this payment has already been applied to this document. It is counted
-- in two places because an application that settled something is an allocation,
-- and one that settled nothing is a credit; a repeat of either is a double count.
SELECT COUNT(*) FROM arap_allocations
WHERE document_id = ? AND payment_id = ?;

-- name: CountArapCreditsForPayment :one
SELECT COUNT(*) FROM arap_credits
WHERE document_id = ? AND payment_id = ?;
