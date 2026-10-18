# FinGo v2 — fixed decisions

**Status:** accepted · 2026-10-18 · the plan of record for the rewrite. Every other v2 document defers to this one, and to ADRs 0006–0008.

## 1. What v2 is

An **on-premise, multi-user, browser-based Vietnamese accounting system** for a practice that serves many SME clients. Regime: **TT133/2016/TT-BTC as amended by TT 46/2025**. One Go binary serves the UI and the API over HTTPS; PostgreSQL 16 is the only external process.

**Strategy.** Keep the tested engine. Replace the shell: UI, database, tenancy, deployment. Cut scope hard. The v1 engine is the asset — 848 tests, correct double-entry, immutable posting, a close that carries 911→4212 — and rewriting it would discard the only part of the project that is demonstrably right.

## 2. The stack (fixed)

| Layer | Choice |
| :--- | :--- |
| Language | Go (toolchain 1.27.0 installed; `go.mod` → `go 1.27.0`) |
| HTTP | Gin |
| DB | PostgreSQL 16 — `fingo` (prod), `fingo_dev`, `fingo_test` |
| DB access | `pgx` v5 + `sqlc` (pgx/v5 output) |
| Migrations | `goose`, embedded; triggers live **in** migrations |
| Tenancy | one database; `client_id NOT NULL` on every business table; RLS forced; `app.client_id` per transaction |
| Money | `int64` đồng / `bigint` / JSON string |
| UI kit | Tabler (Bootstrap 5, MIT), vendored |
| Interaction | htmx + Alpine.js, vendored |
| Entry grid | Tabulator — journal entry screen only |
| Pickers | Tom Select (accounts, partners), flatpickr (`vi`), hotkeys-js |
| Sessions / auth | `scs` (pgx store), argon2id, roles `admin\|accountant\|viewer` |
| CSRF | Go 1.27 `net/http.CrossOriginProtection` (verified available: `go doc net/http.CrossOriginProtection`) |
| Excel | `excelize` |
| Backup | `robfig/cron` + `pg_dump` |
| Deploy | one binary with embedded templates/static + PostgreSQL service. No Docker, no Node, no CDN |

### Dependencies and licences

Verified at the point each is added; the upstream `LICENSE` is committed beside any vendored asset. [Tabulator 6.5.0 is MIT — confirmed from its licence page on 2026-10-18](https://www.tabulator.info/docs/6.x/license/).

| Dependency | Licence | Where |
| :--- | :--- | :--- |
| `gin-gonic/gin` | MIT | Go module |
| `jackc/pgx/v5` | MIT | Go module |
| `sqlc-dev/sqlc` (tool) | MIT | dev tool, not linked |
| `pressly/goose/v3` | MIT | Go module |
| `alexedwards/scs/v2` + `scs/pgxstore` | MIT | Go module |
| `golang.org/x/crypto` (argon2) | BSD-3-Clause | Go module |
| `qax-os/excelize/v2` | BSD-3-Clause | Go module |
| `robfig/cron/v3` | MIT | Go module |
| Tabler | MIT | `web/static/vendor/tabler/` |
| htmx | BSD-2-Clause | `web/static/vendor/htmx.min.js` |
| Alpine.js | MIT | `web/static/vendor/alpine.min.js` |
| Tabulator | MIT | `web/static/vendor/tabulator/` |
| Tom Select | Apache-2.0 | `web/static/vendor/tom-select/` |
| flatpickr | MIT | `web/static/vendor/flatpickr/` |
| hotkeys-js | MIT | `web/static/vendor/hotkeys.min.js` |

**Refused:** any GPL/AGPL library; any dependency that needs Node at build time; any CDN at run time; `gorilla/csrf` (superseded by the stdlib); `shopspring/decimal` (removed).

## 3. Tenancy and security model

- `client_id uuid NOT NULL` + FK on every business table; composite uniqueness `(client_id, …)`.
- `ENABLE` + **`FORCE`** row-level security, policy `client_id = current_setting('app.client_id', true)::uuid`, `WITH CHECK` identical.
- Scope set **per transaction** with `set_config('app.client_id', $1, true)`. No setting ⇒ zero rows, never all rows.
- App role: `LOGIN NOSUPERUSER NOBYPASSRLS`; it owns nothing. Migrations run as the owner.
- `clients` (the registry) is the only table without RLS; it holds no financial data.
- Authentication: argon2id, `scs` session in PostgreSQL, `Secure`/`HttpOnly`/`SameSite=Lax`.
- Authorization: `admin` (users, clients, period lock, close), `accountant` (documents, post), `viewer` (read-only). **Creator ≠ poster** for post and lock.
- `audit_log` is append-only (trigger denies UPDATE/DELETE): who, when (UTC), action, entity, before/after JSON.

## 4. Database-level rules (must exist as constraints, not conventions)

| Rule | Mechanism |
| :--- | :--- |
| Posted voucher and its lines are immutable | `BEFORE UPDATE/DELETE` triggers, `RAISE EXCEPTION` |
| A voucher must balance at commit | `CONSTRAINT TRIGGER ... DEFERRABLE INITIALLY DEFERRED`, sums the lines at COMMIT |
| A locked period cannot be reopened | trigger on `accounting_period_locks` |
| An allocation cannot exceed its document | `BEFORE INSERT/UPDATE` trigger |
| Stock movements and allocations are append-only | triggers |
| Every business row belongs to a client | `NOT NULL` + FK + RLS |
| TT133 chart exists | seeded **by migration** (123 accounts), not from Go |

## 5. Scope

**In v2:** login and roles · chart of accounts · journal entry and posting · journal list · ledger (drill-down) · trial balance · P&L · balance sheet · sales invoice + COGS · receipt and allocation · purchase order → receipt → invoice with three-way match · supplier payment · stock list · period close (911 → 4212/4211) with depreciation and bad-debt provisions where the engine already has them · Excel export · scheduled backup and a scripted restore drill · opening-balance import · per-client handover export · Vietnamese operator guide.

**Out of v2 (explicitly):** payroll · fixed assets · CCDC/TK 242 · e-invoice · bank reconciliation · B03 cash flow · landed cost · statutory filing (`mã số`) · PDF diacritics · multi-currency · cross-client reporting · mobile UI · Docker.

**Legal guardrails.** `docs/legal-basis.md` governs. A figure marked `?` stays a warning. The Circular 48/2019 provisioning policy becomes a **parameter with an effective end date of 2026-12-31**, and the UI warns after it.

## 6. Keep / replace / drop

| Keep and port | Replace | Drop (archive tag only) |
| :--- | :--- | :--- |
| `internal/{domain/gl, money, vat, posting, period, document, partner, inventory, trading, close, reports}` + tests | MariaDB adapters, migrations, triggers | Wails/Vue, .NET remnants, Next.js |
| `docs/**`, ADRs (0006–0008 added; 0001/0003/0004 marked superseded) | `appconfig`, `boot`, `dbhost`, `singleton` | `spike/`, `fingo/dist/`, `fingo/$cov` |
| `docs/legal-basis.md`, `requirements-v1.md` | `walk` UI, `cmd/fingo` desktop | 11 dead `internal/domain/*` packages |
| `web/e2e_test.go` (as the model for the new e2e suite) | `web/` templates → Tabler | `internal/migrate`, `internal/idempotency` (re-implemented simply) |
| `internal/ui/format` (formatting rules only) | plaintext config, `%USERNAME%` identity | `research/` PDFs (text and scripts kept) |
| `internal/db/triggers/*.sql` (logic, re-expressed as PostgreSQL) | | `backup-before-alpha-rollback` (copied to `docs/archive/`, never merged) |

## 7. Port order (Phase 1)

1. `internal/money` → `int64` (already), `internal/vat`, `internal/document`, `internal/partner`, `internal/inventory`, `internal/posting`, `internal/period`, `internal/trading`, `internal/close`, `internal/reports` — **unchanged**, adapt imports only.
2. `internal/domain/gl` — **changed**: `decimal.Decimal` → `int64` (ADR 0008). Its tests change only literal types; the behaviour assertions are unchanged. Every changed test is listed in `docs/v2/test-report.md`.
3. New: `internal/client` (Scope), `internal/store` (pgx pool + scoped tx), `internal/web` (Gin), `cmd/fingo`.

**If a ported test fails:** fix the adapter, not the test. The only exception is a test that asserts a MariaDB mechanism (e.g. an `information_schema` query or a `DECIMAL(18,4)` column) or a decimal literal; each such change is listed and justified in `test-report.md`.

## 8. Database provisioning (decided after the credential was supplied)

The target is the **system PostgreSQL 16.15** service (`postgresql-x64-16`) on `127.0.0.1:5432`, not a private cluster. Migration/DDL runs as `postgres`; the application and the test suite connect as **`fingo_app`** (`LOGIN NOSUPERUSER NOBYPASSRLS`), because a superuser bypasses row-level security and a suite that runs as one proves nothing about it.

`scripts/pgsetup.ps1` (run once per machine) creates `fingo_dev` and `fingo_test`, creates `fingo_app` with a generated password, grants schema usage and default table privileges, and writes the credentials to a **gitignored** `.env`. The admin password is read from `PGADMIN_PASSWORD`, never written to disk and never echoed. The script is idempotent; `-Rotate` rotates the app password.

`scripts/devdb.ps1` remains as a fallback for a machine where the system service is unavailable: it initialises a throwaway cluster under `.local\pgdata` on port 65432 with trust auth. It is not the default and its data is disposable. (Port 55432 was the first choice and is unusable here: Windows reserves 55302–55501, and a bind there fails with "Permission denied" that looks like a firewall fault.)

Tests read `PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE` and fall back to `.env` at the repository root. `REQUIRE_DB=1` turns a missing database into a failure.
## 9. Definition of done, per phase

A phase is done only when, on branch `v2`:

1. `go build ./...` clean.
2. `go vet ./...` clean.
3. `REQUIRE_DB=1 go test ./...` green against the local PostgreSQL — **with `REQUIRE_DB=1`, so no test may skip silently.**
4. `docs/v2/plan-status.md` updated with the counts.
5. Work committed; `git status` clean.
6. At phase end: re-run the gate, confirm `archive/pre-rewrite` still exists, confirm no secret was committed.

## 10. Phase gates

| Phase | Gate |
| :--- | :--- |
| 0 | archive tag + clean tree + ADRs 0006–0008 |
| 1 | `REQUIRE_DB=1 go test ./...` green on PostgreSQL; tenant isolation, immutability and balance tests pass |
| 2 | restore drill succeeds; unauthenticated request rejected; audit rows written for every post/lock |
| 3 | a month of vouchers posted; trial balance totals match the source |
| 4 | credit sale → receipt → COGS end to end, covered by an e2e test |
| 5 | a month closes, locks, every P&L account ends at zero |
| 6 | parallel run against current software on one client; differences listed |

## 11. Known risks carried into Phase 1

| Risk | Mitigation |
| :--- | :--- |
| `gl` decimal→int64 touches `reports`, `close`, `posting`, `usecase/ledger` | One slice, tests as the gate, changed tests enumerated |
| RLS + `FORCE` breaks the owner's own queries | Seed data written by migration as owner with an explicit `client_id`; tests connect as the app role |
| sqlc/pgx output differs from the MySQL generator | Regenerate from `db/queries`; adapter is the only place types cross |
| Tabulator keyboard flow may be unacceptable | Time-boxed spike in Phase 3 with a documented fallback |
| The statutory `mã số` is still unresolved | Filing stays out of scope; warnings preserved |

## 12. Deviations from the mission, and why

1. **`fingo/dist/` is not committed.** It is 48 MB of reproducible build output (`*.exe` was already ignored in v1). It stays on disk and in the local copy; the archive tag holds the source and the build script.
2. **`research/` PDFs are not committed** (they are ignored by policy and weigh 49 MB); the extracted text and scripts are, force-added, because `docs/legal-basis.md` cites them as provenance.
3. **A second archive tag `archive/wails-skeleton`** marks `f9193d3`, the skeleton that `main` was before this archive, so the pre-Go state is reachable even after the working-tree commit.
4. **The local PostgreSQL cluster is workspace-local on port 55432** (§8) instead of the system service, because its password is unavailable.
