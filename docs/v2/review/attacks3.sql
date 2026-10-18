-- Third pass: integrity between client_id and parent rows, unknown accounts,
-- and the practice-level registry.
\set ON_ERROR_STOP off
\echo '=== M: attach lines to another client voucher (client_id=A, voucher_id=B) ==='
BEGIN;
SET LOCAL app.client_id = '11111111-1111-4111-8111-111111111111';
INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq, voucher_date, description, created_by)
VALUES ('cccccccc-cccc-4ccc-8ccc-cccccccccccc','33333333-3333-4333-8333-333333333333','GH2026-7777','GENERAL',2026,7777,'2026-03-15','B voucher','review');
INSERT INTO voucher_lines (client_id, voucher_id, line_order, account_code, side, amount)
VALUES ('11111111-1111-4111-8111-111111111111','cccccccc-cccc-4ccc-8ccc-cccccccccccc',1,'1111','DEBIT',100),
       ('11111111-1111-4111-8111-111111111111','cccccccc-cccc-4ccc-8ccc-cccccccccccc',2,'4111','CREDIT',100);
UPDATE vouchers SET posted_at = now() WHERE id='cccccccc-cccc-4ccc-8ccc-cccccccccccc';
COMMIT;

\echo '=== N: post to an account that does not exist ==='
BEGIN;
SET LOCAL app.client_id = '11111111-1111-4111-8111-111111111111';
INSERT INTO vouchers (id, client_id, voucher_no, book, fiscal_year, seq, voucher_date, description, created_by)
VALUES ('dddddddd-dddd-4ddd-8ddd-dddddddddddd','11111111-1111-4111-8111-111111111111','GH2026-0006','GENERAL',2026,6,'2026-03-15','TK ma','review');
INSERT INTO voucher_lines (client_id, voucher_id, line_order, account_code, side, amount)
VALUES ('11111111-1111-4111-8111-111111111111','dddddddd-dddd-4ddd-8ddd-dddddddddddd',1,'9999','DEBIT',100),
       ('11111111-1111-4111-8111-111111111111','dddddddd-dddd-4ddd-8ddd-dddddddddddd',2,'4111','CREDIT',100);
UPDATE vouchers SET posted_at = now() WHERE id='dddddddd-dddd-4ddd-8ddd-dddddddddddd';
COMMIT;

\echo '=== O: allocate against another client document ==='
INSERT INTO allocations (client_id, document_id, voucher_id, amount)
VALUES ('11111111-1111-4111-8111-111111111111','77777777-7777-4777-8777-777777777777','44444444-4444-4444-8444-444444444444',1000);

\echo '=== P: deactivate every client (tenant lockout as the app role) ==='
UPDATE clients SET active = false WHERE tax_id IN ('REV-A','REV-B');

\echo '=== Q: read the practice-level tables ==='
SELECT count(*) AS clients_visible FROM clients;
SELECT count(*) AS users_visible FROM users;
