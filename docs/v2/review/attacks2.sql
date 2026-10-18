-- Second pass: the first pass ran some statements without a client scope, so RLS
-- made them update zero rows instead of reaching the trigger. A control is only
-- proven when the statement is refused for the right reason, so each attempt here
-- runs inside a scoped transaction.

\set ON_ERROR_STOP off
\set A '11111111-1111-4111-8111-111111111111'
\set V1 '44444444-4444-4444-8444-444444444444'

\echo '=== A: update a posted voucher, with scope ==='
BEGIN;
SET LOCAL app.client_id = :'A';
UPDATE vouchers SET description = 'Đã sửa' WHERE id = :'V1';
COMMIT;

\echo '=== B: update a posted line amount, with scope ==='
BEGIN;
SET LOCAL app.client_id = :'A';
UPDATE voucher_lines SET amount = 999 WHERE voucher_id = :'V1';
COMMIT;

\echo '=== C: delete a posted line, with scope ==='
BEGIN;
SET LOCAL app.client_id = :'A';
DELETE FROM voucher_lines WHERE voucher_id = :'V1' AND line_order = 1;
COMMIT;

\echo '=== D: delete a posted voucher, with scope ==='
BEGIN;
SET LOCAL app.client_id = :'A';
DELETE FROM vouchers WHERE id = :'V1';
COMMIT;

\echo '=== E: rewrite the audit log, with scope ==='
BEGIN;
SET LOCAL app.client_id = :'A';
UPDATE audit_log SET actor = 'someone else' WHERE true;
COMMIT;

\echo '=== F: delete audit rows, with scope ==='
BEGIN;
SET LOCAL app.client_id = :'A';
DELETE FROM audit_log WHERE true;
COMMIT;

\echo '=== G: unlock a period (locks are meant to be irreversible) ==='
BEGIN;
SET LOCAL app.client_id = :'A';
UPDATE accounting_period_locks SET locked_by = 'nobody' WHERE true;
DELETE FROM accounting_period_locks WHERE true;
COMMIT;

\echo '=== H: app role attempts DDL (disable the triggers) ==='
ALTER TABLE vouchers DISABLE TRIGGER ALL;
ALTER TABLE voucher_lines DISABLE TRIGGER ALL;

\echo '=== I: app role attempts to read as superuser-ish ==='
SELECT current_user, session_user, current_setting('row_security', true) AS row_security;

\echo '=== J: can the app role create its own client? (would let it mint a tenant) ==='
BEGIN;
INSERT INTO clients (name, tax_id) VALUES ('Rogue client', 'ROGUE-1');
COMMIT;

\echo '=== K: period lock vs a voucher dated inside the locked period (again, scoped) ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq, voucher_date, description, created_by)
VALUES ('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb', :'A', 'GH2026-0005', 'GENERAL', 2026, 5, '2026-03-20', 'Sau khi khoá', 'review');
INSERT INTO voucher_lines (client_id, voucher_id, line_order, account_code, side, amount)
VALUES (:'A', 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb', 1, '1111', 'DEBIT', 1000),
       (:'A', 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb', 2, '4111', 'CREDIT', 1000);
UPDATE vouchers SET posted_at = now() WHERE id = 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb';
COMMIT;

\echo '=== L: inventory: receive without a source document, and issue more than on hand ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO stock_movements (client_id, item_code, warehouse_code, kind, quantity, cost, unit_cost, source_doc_id, moved_at)
VALUES (:'A', 'ITEM-X', 'MAIN', 'ISSUE', 5, 0, 0, '', '2026-03-21');
COMMIT;
