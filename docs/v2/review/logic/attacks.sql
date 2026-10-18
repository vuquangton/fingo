-- Business-logic attacks not covered by the earlier infrastructure review.
-- Throwaway database fingo_logic, application role fingo_app.
\set ON_ERROR_STOP off
\set A '11111111-1111-4111-8111-111111111111'
\set L '22222222-2222-4222-8222-222222222222'

\echo '=== SETUP ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO clients (id, name, tax_id) VALUES (:'A', 'Logic A', 'LOGIC-A');
INSERT INTO clients (id, name, tax_id) VALUES ('33333333-3333-4333-8333-333333333333', 'Logic B', 'LOGIC-B');
SELECT provision_client_chart(:'A');
COMMIT;
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO accounting_periods (id, client_id, name, start_date, end_date)
VALUES (:'L', :'A', 'FY2026', '2026-01-01', '2026-12-31');
COMMIT;

\echo '=== A1: voucher with zero lines, posted ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq, voucher_date, description, created_by)
VALUES ('aaaaaaaa-0000-4000-8000-000000000001', :'A', 'GH2026-9001', 'GENERAL', 2026, 9001, '2026-03-01', 'zero lines', 'x');
UPDATE vouchers SET posted_at = now() WHERE id = 'aaaaaaaa-0000-4000-8000-000000000001';
COMMIT;

\echo '=== A2: voucher with a single line, posted ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq, voucher_date, description, created_by)
VALUES ('aaaaaaaa-0000-4000-8000-000000000002', :'A', 'GH2026-9002', 'GENERAL', 2026, 9002, '2026-03-01', 'one line', 'x');
INSERT INTO voucher_lines (client_id, voucher_id, line_order, account_code, side, amount)
VALUES (:'A', 'aaaaaaaa-0000-4000-8000-000000000002', 1, '1111', 'DEBIT', 100);
UPDATE vouchers SET posted_at = now() WHERE id = 'aaaaaaaa-0000-4000-8000-000000000002';
COMMIT;

\echo '=== A3: zero-amount line pair (balanced at zero) ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq, voucher_date, description, created_by)
VALUES ('aaaaaaaa-0000-4000-8000-000000000003', :'A', 'GH2026-9003', 'GENERAL', 2026, 9003, '2026-03-01', 'zero amount', 'x');
INSERT INTO voucher_lines (client_id, voucher_id, line_order, account_code, side, amount)
VALUES (:'A', 'aaaaaaaa-0000-4000-8000-000000000003', 1, '1111', 'DEBIT', 0),
       (:'A', 'aaaaaaaa-0000-4000-8000-000000000003', 2, '4111', 'CREDIT', 0);
UPDATE vouchers SET posted_at = now() WHERE id = 'aaaaaaaa-0000-4000-8000-000000000003';
COMMIT;

\echo '=== A4: negative-amount line (a credit written as a negative debit) ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq, voucher_date, description, created_by)
VALUES ('aaaaaaaa-0000-4000-8000-000000000004', :'A', 'GH2026-9004', 'GENERAL', 2026, 9004, '2026-03-01', 'negative', 'x');
INSERT INTO voucher_lines (client_id, voucher_id, line_order, account_code, side, amount)
VALUES (:'A', 'aaaaaaaa-0000-4000-8000-000000000004', 1, '1111', 'DEBIT', -500),
       (:'A', 'aaaaaaaa-0000-4000-8000-000000000004', 2, '4111', 'CREDIT', -500);
UPDATE vouchers SET posted_at = now() WHERE id = 'aaaaaaaa-0000-4000-8000-000000000004';
COMMIT;

\echo '=== A5: duplicate voucher number in the same series ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq, voucher_date, description, created_by)
VALUES ('aaaaaaaa-0000-4000-8000-000000000005', :'A', 'GH2026-0001', 'GENERAL', 2026, 5001, '2026-03-01', 'dup number', 'x');
COMMIT;

\echo '=== A6: post to an inactive account ==='
BEGIN;
SET LOCAL app.client_id = :'A';
UPDATE accounts SET active = false WHERE code = '5111';
COMMIT;
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq, voucher_date, description, created_by)
VALUES ('aaaaaaaa-0000-4000-8000-000000000006', :'A', 'GH2026-9006', 'GENERAL', 2026, 9006, '2026-03-01', 'inactive account', 'x');
INSERT INTO voucher_lines (client_id, voucher_id, line_order, account_code, side, amount)
VALUES (:'A', 'aaaaaaaa-0000-4000-8000-000000000006', 1, '1111', 'DEBIT', 100),
       (:'A', 'aaaaaaaa-0000-4000-8000-000000000006', 2, '5111', 'CREDIT', 100);
UPDATE vouchers SET posted_at = now() WHERE id = 'aaaaaaaa-0000-4000-8000-000000000006';
COMMIT;

\echo '=== A7: allocate the same payment twice against one document ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO documents (id, client_id, kind, document_date, total, allocated, status)
VALUES ('dddddddd-0000-4000-8000-000000000001', :'A', 'SALES_INVOICE', '2026-03-01', 1000, 0, 'OPEN');
COMMIT;
BEGIN;
SET LOCAL app.client_id = :'A';
UPDATE documents SET allocated = 1000 WHERE id = 'dddddddd-0000-4000-8000-000000000001';
INSERT INTO allocations (client_id, document_id, voucher_id, amount)
VALUES (:'A', 'dddddddd-0000-4000-8000-000000000001', 'aaaaaaaa-0000-4000-8000-000000000005', 1000);
COMMIT;
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO allocations (client_id, document_id, voucher_id, amount)
VALUES (:'A', 'dddddddd-0000-4000-8000-000000000001', 'aaaaaaaa-0000-4000-8000-000000000005', 1000);
COMMIT;

\echo '=== A8: stock movement pointing at a draft voucher ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq, voucher_date, description, created_by)
VALUES ('aaaaaaaa-0000-4000-8000-000000000008', :'A', 'GH2026-9008', 'GENERAL', 2026, 9008, '2026-03-01', 'draft', 'x');
COMMIT;

\echo '=== A9: stock issue beyond on-hand (no stock at all) ==='
BEGIN;
SET LOCAL app.client_id = :'A';
INSERT INTO stock_movements (client_id, item_code, warehouse_code, kind, quantity, cost, unit_cost, source_doc_id, moved_at)
VALUES (:'A', 'ITEM-Z', 'MAIN', 'ISSUE', 3, 0, 0, 'GH2026-9008', '2026-03-01');
COMMIT;
