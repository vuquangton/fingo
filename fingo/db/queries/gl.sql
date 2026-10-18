-- name: GetAccountByCode :one
SELECT id, code, name, parent_id, account_type, nature, is_active, created_at, updated_at
FROM accounts
WHERE code = ? LIMIT 1;

-- name: ListActiveAccounts :many
SELECT id, code, name, parent_id, account_type, nature, is_active, created_at, updated_at
FROM accounts
WHERE is_active = TRUE
ORDER BY code ASC;

-- name: ListAllAccounts :many
-- Includes deactivated accounts: postability is decided by the active flag, and
-- a chart missing a deactivated parent would look as though its children were
-- orphans.
SELECT id, code, name, parent_id, account_type, nature, is_active, created_at, updated_at
FROM accounts
ORDER BY code ASC;

-- name: CreateAccount :exec
INSERT INTO accounts (id, code, name, parent_id, account_type, nature, is_active)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: CreateVoucher :exec
INSERT INTO vouchers (id, book, fiscal_year, seq, voucher_no, voucher_date, posted_date, posted_at, posted_by, created_by, voucher_type, description, is_posted)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: MarkVoucherPosted :exec
-- The posting transition, as a separate statement.
--
-- A voucher is stored as a draft, gets its lines, and is then marked posted —
-- the lifecycle the immutability trigger enforces. Writing the header as already
-- posted would make adding its lines impossible, which is the trigger working
-- correctly.
UPDATE vouchers
SET is_posted = TRUE, book = ?, fiscal_year = ?, seq = ?, voucher_no = ?,
    posted_date = ?, posted_at = ?, posted_by = ?
WHERE id = ?;

-- name: GetVoucherByID :one
SELECT id, book, fiscal_year, seq, voucher_no, voucher_date, posted_date, posted_at, posted_by, created_by, voucher_type, description, is_posted, created_at
FROM vouchers
WHERE id = ? LIMIT 1;

-- name: CreateVoucherLine :exec
INSERT INTO voucher_lines (id, voucher_id, line_order, account_id, partner_id, side, amount, note)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListVoucherLinesByVoucherID :many
-- Joins accounts so the caller gets the account CODE, which is what the domain
-- speaks. Storage keys lines by accounts.id for referential integrity; the
-- domain never sees that identity.
SELECT vl.id, vl.voucher_id, vl.line_order, a.code AS account_code, vl.partner_id, vl.side, vl.amount, vl.note
FROM voucher_lines vl
JOIN accounts a ON a.id = vl.account_id
WHERE vl.voucher_id = ?
ORDER BY vl.line_order ASC;

-- name: ListPostedLinesInRange :many
-- Every posted line whose voucher falls inside a date range, both ends included.
--
-- One query for the whole range rather than one per voucher: a report sums every
-- line it is given, and a round trip per voucher would make the cost of a report
-- a function of how many vouchers a client has ever posted.
--
-- Both arguments are named, because both are compared against the same column
-- and the generator names an unnamed argument after the column it is compared
-- to — which would give this query two parameters called VoucherDate.
--
-- is_posted is the filter that matters. A draft is not in the books, and
-- including one would state a figure nobody has signed off.
--
-- The range has no lower bound in practice: the reports ask for everything up to
-- a period's end, so that an account shows what it carried in as well as what
-- the period did to it. A caller with nothing carried in passes the beginning of
-- the books rather than NULL, because a comparison against NULL is never true
-- and would silently report an empty ledger.
SELECT vl.id, vl.voucher_id, v.voucher_date, a.code AS account_code, vl.side, vl.amount
FROM voucher_lines vl
JOIN vouchers v ON v.id = vl.voucher_id
JOIN accounts a ON a.id = vl.account_id
WHERE v.is_posted = TRUE
  AND v.voucher_date >= sqlc.arg(from_date)
  AND v.voucher_date <= sqlc.arg(to_date)
ORDER BY v.voucher_date ASC, vl.line_order ASC;
