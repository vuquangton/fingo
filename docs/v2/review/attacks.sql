-- Review fixture + direct-SQL attack suite. Run as the application role
-- (fingo_app) against a throwaway database. Every attempt is expected to be
-- refused by PostgreSQL, not by Go.
--
-- psql runs with ON_ERROR_STOP off so each failure is visible and the suite
-- continues: the output IS the evidence.

\set ON_ERROR_STOP off
\set A '11111111-1111-4111-8111-111111111111'
\set B '33333333-3333-4333-8333-333333333333'
\set P '22222222-2222-4222-8222-222222222222'
\set V1 '44444444-4444-4444-8444-444444444444'
\set V2 '55555555-5555-4555-8555-555555555555'
\set V3 '66666666-6666-4666-8666-666666666666'
\set D1 '77777777-7777-4777-8777-777777777777'

\echo '=== SETUP: two clients, charts, one period ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO clients (id, name, tax_id) VALUES (:'A', 'Review A', 'REV-A');
INSERT INTO clients (id, name, tax_id) VALUES (:'B', 'Review B', 'REV-B');
SELECT count(*) AS chart_rows FROM provision_client_chart(:'A');
COMMIT;

BEGIN;
SET LOCAL app.client_id = :'B';
SELECT count(*) AS chart_rows_b FROM provision_client_chart(:'B');
COMMIT;

BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO accounting_periods (id, client_id, name, start_date, end_date)
VALUES (:'P', :'A', 'FY2026', '2026-01-01', '2026-12-31');
COMMIT;

\echo '=== SETUP: one posted voucher in client A ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq, voucher_date, description, created_by)
VALUES (:'V1', :'A', 'GH2026-0001', 'GENERAL', 2026, 1, '2026-03-15', 'Hợp lệ', 'review');
INSERT INTO voucher_lines (client_id, voucher_id, line_order, account_code, side, amount)
VALUES (:'A', :'V1', 1, '1111', 'DEBIT', 1000000),
       (:'A', :'V1', 2, '4111', 'CREDIT', 1000000);
UPDATE vouchers SET posted_at = now(), posted_by = 'review' WHERE id = :'V1';
COMMIT;

\echo '=== ATTACK 1: unbalanced posted voucher, straight into the tables ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq, voucher_date, description, created_by)
VALUES (:'V2', :'A', 'GH2026-0002', 'GENERAL', 2026, 2, '2026-03-16', 'Lệch', 'review');
INSERT INTO voucher_lines (client_id, voucher_id, line_order, account_code, side, amount)
VALUES (:'A', :'V2', 1, '1111', 'DEBIT', 1000000),
       (:'A', :'V2', 2, '4111', 'CREDIT', 900000);
UPDATE vouchers SET posted_at = now() WHERE id = :'V2';
COMMIT;

\echo '=== ATTACK 2: update a posted voucher ==='
UPDATE vouchers SET description = 'Đã sửa' WHERE id = :'V1';

\echo '=== ATTACK 3: delete a posted line ==='
DELETE FROM voucher_lines WHERE voucher_id = :'V1' AND line_order = 1;

\echo '=== ATTACK 4: post into a locked period ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO accounting_period_locks (client_id, period_id, locked_by) VALUES (:'A', :'P', 'review');
COMMIT;
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq, voucher_date, description, created_by)
VALUES (:'V3', :'A', 'GH2026-0003', 'GENERAL', 2026, 3, '2026-03-17', 'Kỳ đã khoá', 'review');
INSERT INTO voucher_lines (client_id, voucher_id, line_order, account_code, side, amount)
VALUES (:'A', :'V3', 1, '1111', 'DEBIT', 500000),
       (:'A', :'V3', 2, '4111', 'CREDIT', 500000);
UPDATE vouchers SET posted_at = now() WHERE id = :'V3';
COMMIT;

\echo '=== ATTACK 5: allocate more than the document ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO documents (id, client_id, kind, document_date, total, allocated, status)
VALUES (:'D1', :'A', 'SALES_INVOICE', '2026-03-15', 100000, 0, 'OPEN');
INSERT INTO allocations (client_id, document_id, voucher_id, amount)
VALUES (:'A', :'D1', :'V1', 150000);
COMMIT;

\echo '=== ATTACK 6: post to a heading account (411 has children) ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq, voucher_date, description, created_by)
VALUES ('88888888-8888-4888-8888-888888888888', :'A', 'GH2026-0004', 'GENERAL', 2026, 4, '2026-03-18', 'TK tổng hợp', 'review');
INSERT INTO voucher_lines (client_id, voucher_id, line_order, account_code, side, amount)
VALUES (:'A', '88888888-8888-4888-8888-888888888888', 1, '411', 'DEBIT', 100000),
       (:'A', '88888888-8888-4888-8888-888888888888', 2, '4111', 'CREDIT', 100000);
UPDATE vouchers SET posted_at = now() WHERE id = '88888888-8888-4888-8888-888888888888';
COMMIT;

\echo '=== ATTACK 7: write a voucher into another client ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq, voucher_date, description, created_by)
VALUES ('99999999-9999-4999-8999-999999999999', :'B', 'GH2026-9001', 'GENERAL', 2026, 9001, '2026-03-19', 'Xuyên khách hàng', 'review');
COMMIT;

\echo '=== ATTACK 8: alter the audit log ==='
UPDATE audit_log SET actor = 'someone else' WHERE true;
DELETE FROM audit_log WHERE true;

\echo '=== ATTACK 9: read with no client scope (fresh session, no SET LOCAL) ==='
SELECT count(*) AS vouchers_visible_without_scope FROM vouchers;

\echo '=== ATTACK 10: read another client while scoped to A ==='
BEGIN;
SET LOCAL app.client_id = :'A';
SELECT count(*) AS b_vouchers_visible_from_a FROM vouchers WHERE client_id = :'B';
SELECT count(*) AS all_vouchers_visible FROM vouchers;
COMMIT;

\echo '=== ATTACK 11: scope set at session level survives? (SET vs SET LOCAL) ==='
BEGIN;
SET LOCAL app.client_id = :'A';
COMMIT;
SELECT coalesce(current_setting('app.client_id', true), '(unset)') AS after_commit;
