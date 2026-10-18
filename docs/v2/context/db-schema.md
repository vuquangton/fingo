# db-schema — tables, triggers, policies

> Generated from `db/migrations/*.sql`. Regenerate with:
> `Select-String -Path db/migrations/*.sql -Pattern 'CREATE TABLE|CREATE TRIGGER|CREATE POLICY|ENABLE ROW LEVEL|FORCE ROW LEVEL'`
> Last generated: 2026-10-18 at `986e572`. 7 migrations, 20 tables.

## Migrations

| Version | File | Adds |
| ---: | :--- | :--- |
| 1 | `00001_init.sql` | 13 tables, functions, triggers, RLS, 123-account chart seed, dev client |
| 2 | `00002_sessions.sql` | `sessions`, `failed_logins` (scs + login abuse) |
| 3 | `00003_period_close_reports.sql` | `period_close_reports` |
| 4 | `00004_partners.sql` | `partners` |
| 5 | `00005_document_lines_and_credits.sql` | `document_lines`, `document_credits` |
| 6 | `00006_document_period.sql` | `documents.period_id` + index |
| 7 | `00007_items_and_stock_costs.sql` | `items`; `stock_movements.unit_cost`, `source_doc_id` |

Applied by `cmd/migrate` (owner role). **Not** applied at server startup.

## Tables

| Table | Key columns | RLS |
| :--- | :--- | :--- |
| `clients` | id uuid PK, name, tax_id UNIQUE, active, created_at | no |
| `chart_template` | code PK, name, parent_code, account_type, nature | no |
| `users` | id, email UNIQUE, display_name, password_hash, role CHECK, active | no |
| `sessions` | scs schema (token, data, expiry) | no |
| `failed_logins` | id, email, remote_ip, at | no |
| `audit_log` | id identity, client_id, at, actor, action, entity, entity_id, before/after jsonb | **yes** |
| `accounts` | PK `(client_id, code)`, name, parent_code, type, nature, active | yes |
| `accounting_periods` | id, client_id, name, start_date, end_date, UNIQUE (client,start,end) | yes |
| `accounting_period_locks` | id, client_id, period_id UNIQUE, locked_at, locked_by, reason | yes |
| `vouchers` | id, client_id, voucher_no, book, fiscal_year, seq, date, posted_at/by, created_by; UNIQUE (client,book,year,seq), (client,voucher_no) | yes |
| `voucher_lines` | id, client_id, voucher_id, line_order, account_code, side, amount bigint > 0, note | yes |
| `documents` | id, client_id, kind CHECK, partner_id, dates, total, allocated, status, period_id | yes |
| `document_lines` | document_id, line_order, account_code, debit/credit/vat/base bigint ≥ 0 | yes |
| `document_credits` | document_id, partner_id, payment_id, amount > 0 | yes |
| `allocations` | document_id, voucher_id, amount > 0, UNIQUE (document,voucher) | yes |
| `inventory_policies` | PK (client_id, fiscal_year), method CHECK | yes |
| `stock_movements` | seq identity, item_code, warehouse_code, kind RECEIPT/ISSUE, quantity numeric(19,4) ≠ 0, cost bigint, unit_cost, voucher_id, source_doc_id | yes |
| `items` | id, client_id, sku, name, unit, reorder_point, active, UNIQUE (client,sku) | yes |
| `partners` | id, client_id, name, tax_id, kind; UNIQUE (client,name), partial UNIQUE tax_id | yes |
| `period_close_reports` | client_id, period_id, report payload, closed_at | yes |

Money is `bigint`; quantities are `numeric(19,4)`; instants are `timestamptz`.

## Functions

| Function | Purpose |
| :--- | :--- |
| `app_current_client() → uuid` | `nullif(current_setting('app.client_id', true),'')::uuid` |
| `app_require_client() → uuid` | same, but raises when unset (used by ledger reads) |
| `provision_client_chart(uuid) → int` | copy `chart_template` into `accounts` for a client |
| `voucher_assert_balanced(uuid)` | sum debits = sum credits for one voucher |
| `vouchers_guard`, `voucher_lines_guard` | refuse update/delete of posted rows |
| `allocations_guard` | allocation total ≤ document total |
| `append_only` | raise on UPDATE/DELETE (shared) |

## Triggers

| Trigger | Table | Fires | Effect |
| :--- | :--- | :--- | :--- |
| `vouchers_balanced` | `vouchers` | AFTER INSERT/UPDATE, **DEFERRABLE INITIALLY DEFERRED** | balance checked at COMMIT |
| `voucher_lines_balanced` | `voucher_lines` | AFTER INSERT/UPDATE/DELETE, deferred | balance checked at COMMIT |
| `vouchers_immutable` | `vouchers` | BEFORE UPDATE/DELETE | exception if posted |
| `voucher_lines_immutable` | `voucher_lines` | BEFORE INSERT/UPDATE/DELETE | exception if parent posted |
| `allocations_within_document` | `allocations` | BEFORE INSERT/UPDATE | exception when over-allocated |
| `audit_log_append_only` | `audit_log` | BEFORE UPDATE/DELETE | exception |
| `period_locks_append_only` | `accounting_period_locks` | BEFORE UPDATE/DELETE | exception |
| `allocations_append_only` | `allocations` | BEFORE UPDATE/DELETE | exception |
| `stock_movements_append_only` | `stock_movements` | BEFORE UPDATE/DELETE | exception |

There is **no** trigger that refuses a posting into a locked period. That rule is
enforced in Go from the lock table (`internal/ledger/post.go:246-260`).

## RLS

`ENABLE` + `FORCE` on the 15 tables marked yes; one `FOR ALL` policy per table:

```sql
USING (client_id = app_current_client())
WITH CHECK (client_id = app_current_client())
```

No scope ⇒ zero rows. `fingo_app` is `NOSUPERUSER NOBYPASSRLS` and owns nothing;
migrations run as the owner, which is also subject to `FORCE`, so seeding sets
`app.client_id` first.

## Seed

`chart_template`: 123 TT133 accounts (`00001_init.sql` `INSERT`). One development
client, fixed id `00000000-0000-0000-0000-000000000001`, tax id `0101234567`,
with its chart provisioned. Real deployments create their own clients.
