# tenancy — RLS, scope, test harness

**Packages:** `internal/store`, `internal/testdb`, `db/migrations`.
**Status:** WIRED, DONE (isolation tests are the Phase 1 gate).

## Purpose

Make "one client's books" a shape, not a convention: every business query runs in
a transaction that first sets `app.client_id`; PostgreSQL row-level security
filters on it. No scope ⇒ `ErrNoScope`; no setting ⇒ zero rows, never all rows.

## Public API

| Symbol | What |
| :--- | :--- |
| `store.Open(ctx, Config{DSN,MaxOpenConns,ConnMaxLifetime})` | pool that pings at startup |
| `store.NewScope(uuid)` | validates a client id; empty → `ErrNoScope` |
| `(*Store).InClient(ctx, scope, fn(pgx.Tx))` | begin → `set_config('app.client_id', $1, true)` → fn → commit |
| `(*Store).Read(...)` | same, for reads |
| `(*Store).Pool()` | escape hatch for migrations/tests only |
| `store.FromPool(pool)` | wrap an existing pool |
| `testdb.Load(t)` / `testdb.Open(t, cfg)` | real DB or fail under `REQUIRE_DB=1` |

## Invariants

1. `set_config(..., true)` is `SET LOCAL`: the scope dies with the transaction, so
   a pooled connection cannot leak it into the next request.
2. RLS is `ENABLE` **and** `FORCE` on 15 tables; the app role is
   `NOSUPERUSER NOBYPASSRLS` and owns nothing.
3. Policy: `client_id = app_current_client()` `FOR ALL … WITH CHECK` — a forged
   `client_id` is refused on write, not just filtered on read.
4. `app_current_client()` = `nullif(current_setting('app.client_id', true),'')::uuid`
   — the two-arg form turns a forgotten scope into zero rows, not a 500.
5. Five tables are deliberately outside RLS: `clients`, `chart_template`,
   `users`, `sessions`, `failed_logins` (practice-level, no financial data).
6. Tests connect as `fingo_app`, never the owner; a suite that runs as superuser
   proves nothing.

## DB tables

Scoped (15): `accounts`, `accounting_periods`, `accounting_period_locks`,
`vouchers`, `voucher_lines`, `documents`, `document_lines`, `document_credits`,
`allocations`, `inventory_policies`, `stock_movements`, `audit_log`,
`period_close_reports`, `partners`, `items`.
Unscoped (5): `clients`, `chart_template`, `users`, `sessions`, `failed_logins`.

## Tests

`internal/store/store_test.go` — 11 tests: scope required, unscoped sees zero,
app role cannot bypass RLS, clients cannot see each other, forged write refused,
unbalanced refused at COMMIT, posted immutable, allocation cap, lock
irreversible, every business table has `client_id`, every client gets 123
accounts. `$env:REQUIRE_DB='1'; go test ./internal/store -count=1`.

## Gotchas

- The suite shares one `fingo_test` database and **accumulates rows** between
  tests; per-test truncation is todo B3. The login test already had to scope
  itself to its own run.
- Schema is **not** applied at server startup: run
  `$env:PGADMIN_PASSWORD='…'; go run ./cmd/migrate -database fingo_test`.
- `internal/testdb` prints `.\scripts\migrate.cmd` in its failure message — that
  script does not exist.
- Migrations run as the owner; the owner is subject to `FORCE` RLS too, so seed
  writes must `set_config` first (`00001_init.sql` tail).
