# FinGo v2 — Context
> Last verified: 2026-10-18 at commit `986e572` by Context Curator

## 1. What this is

On-premise, multi-user, browser-based Vietnamese accounting for a practice that
serves many SME client companies. One Go binary over HTTPS plus PostgreSQL 16;
no Docker, no Node, no CDN. Regime: **TT133/2016 as amended by TT 46/2025**.
Three roles: `admin`, `accountant`, `viewer`. UI copy is Vietnamese.

## 2. Stack (fixed — code wins over prose)

| Layer | Choice | Version | Evidence |
| :--- | :--- | :--- | :--- |
| Language | Go | 1.27.0 | `go.mod:3` |
| HTTP | Gin | 1.12.0 | `go.mod:8` |
| DB | PostgreSQL | 16.x (16.15 here) | `docs/v2/decisions.md:17` |
| DB access | `pgx/v5` **hand-written SQL, no sqlc** | 5.11.0 | `go.mod:10`; no `sqlc.yaml` |
| Migrations | `goose`, embedded, applied by `cmd/migrate` | 3.29.0 | `db/db.go:18-35` |
| Tenancy | one DB, `client_id NOT NULL`, forced RLS | — | `db/migrations/00001_init.sql:407-424` |
| Money | `int64` đồng in Go, `bigint` in PG, string in JSON | — | `internal/money/dong.go` |
| UI | `html/template` + Tabler **1.0.0-beta21** + htmx, embedded | — | `internal/web/static/vendor/` |
| Sessions/auth | `scs` in PG, argon2id | 2.9.0 | `internal/web/auth.go:53-66` |
| CSRF | `net/http.CrossOriginProtection` | stdlib | `internal/web/auth.go:82` |
| Excel | `excelize/v2` | 2.11.0 | `go.mod:12` |
| Backup | `pg_dump` + Windows scheduled task | — | `scripts/backup.ps1` |

Absent despite older docs: sqlc, Alpine.js, Tabulator, Tom Select, flatpickr,
hotkeys-js, `robfig/cron`.

## 3. Commands

Run from the repository root. `✅` = executed green on 2026-10-18.

| Task | Command | Status |
| :--- | :--- | :--- |
| Build | `go build ./...` | ✅ rc=0 |
| Vet | `go vet ./...` | ✅ rc=0 |
| Test (all, DB required) | `$env:REQUIRE_DB='1'; go test ./... -count=1` | ✅ 21 pkg ok, 0 fail, 0 skip |
| One package | `$env:REQUIRE_DB='1'; go test ./internal/ledger -count=1` | ✅ pattern |
| DB setup (once) | `$env:PGADMIN_PASSWORD='…'; .\scripts\pgsetup.cmd` | [UNVERIFIED — not re-run] |
| Apply schema | `$env:PGADMIN_PASSWORD='…'; go run .\cmd\migrate -database fingo_test` | [UNVERIFIED — CI path] |
| Dev cert | `go run .\cmd\devcert -dir .local\tls` | [UNVERIFIED — code read] |
| Run server | `$env:FINGO_DSN='…'; go run .\cmd\fingo` → `https://127.0.0.1:8443` | [UNVERIFIED — not started] |
| Fallback cluster | `.\scripts\devdb.cmd` (port **65432**, trust) | [UNVERIFIED] |
| Backup / drill | `.\scripts\backup.ps1` · `.\scripts\restore-drill.ps1` | [UNVERIFIED] |
| sqlc | — | **does not exist** |

`REQUIRE_DB=1` is mandatory: without it DB tests skip and `ok` is meaningless.

## 4. Layout

| Path | Purpose | Status |
| :--- | :--- | :--- |
| `cmd/fingo` | server: TLS, sessions, 41 routes | WIRED |
| `cmd/migrate`, `cmd/devcert` | DDL as owner; TLS cert | WIRED |
| `internal/web` | Gin handlers, middleware, embedded templates/assets (19 screens) | WIRED |
| `internal/books` | assembly layer: reports/close/trading over one scoped store | WIRED |
| `internal/ledger` | **the only posting path** (`Post`, `PostVoucher`) | WIRED |
| `internal/store` | pool, `Scope`, `InClient` (sets `app.client_id`) | WIRED |
| `internal/domain/gl` | journal model, TT133 chart, numbering, periods | WIRED |
| `internal/{money,format,vat,posting,period}` | primitives and rules | WIRED |
| `internal/{trading,inventory,document,domain/document}` | documents, pricing, stock valuation | WIRED |
| `internal/{reports,close,audit,auth}` | report engine, close, audit, passwords | WIRED |
| `internal/usecase/ledger` | domain posting service used via books ports | WIRED |
| `internal/partner` | partner directory — books has its own model | **UNWIRED** |
| `internal/reports/statutory` | statutory form set, deliberately out of scope | **UNWIRED** |
| `pkg/logger` | only imported by statutory | **DEAD** |
| `internal/testdb` | real-DB-or-fail harness | TEST-ONLY |
| `db/migrations` | 7 migrations: schema, triggers, RLS, 123-account seed | WIRED |

## 5. Hard rules

1. Money is `int64` đồng everywhere. No `float64`, no `numeric` for money, no
   decimal conversion without an explicit error on a fraction.
2. Every business read/write goes through `store.InClient`/`Read`. No scope ⇒
   `ErrNoScope`; no setting ⇒ zero rows, never all rows.
3. Posted vouchers and lines are immutable (DB trigger). Corrections are mirror
   vouchers (`POST /reverse`), never edits.
4. A figure marked `?` is never emitted. Never build on `?`
   (`docs/legal-basis.md`).
5. No secrets in git. `.env` is gitignored; DSN comes from `FINGO_DSN`.
6. The gate is build + vet + `REQUIRE_DB=1` tests, all green. A run with skips is
   a failed run.
7. One posting path: `internal/ledger`. Do not add a second way to the ledger.
8. Migrations are an administrative step (`cmd/migrate`), not startup work.

## 6. Current state

Phase 0, 1, 2, 5 **DONE**; 3 and 4 **PARTIAL**; 6 **PARTIAL — gate unrun**
(`docs/v2/plan-status.md:9-15`). Release gates G1–G3 **not met**
(`docs/v2/todo.md:172-174`). Gate today: build/vet/test green, 21 packages,
514 test functions, 0 skips. Open blockers: pilot client + parallel month (A1,
A2), CI green on GitHub (A4), production host (A5), per-test DB isolation (B3).
[INFERRED] ~75% of the in-scope feature list is routed; missing: three-way
match, bill-from-receipt, stock transfer, entry-grid spike.

## 7. Do not

1. Do not run `sqlc generate` or look for `db/queries/` — no sqlc; SQL is in Go.
2. Do not assume Alpine/Tabulator/pickers exist — only Tabler CSS + inert htmx.
3. Do not tell a user to open `/` and log in — no login page; `POST /login` only.
4. Do not import `internal/client` or write to `web/` — use `store.Scope` and
   `internal/web`.
5. Do not expect the schema to apply at startup — run `cmd/migrate` first.
6. Do not look for a `migrate.cmd` helper (a test error names one) — none exists; use `cmd/migrate`.
7. Do not wire `internal/partner` or `reports/statutory` by assumption — both
   are unwired; `books` owns the partner model.
8. Do not claim creator ≠ poster or "every lock is audited" — neither is
   implemented (`internal/ledger/post.go:283-300`, `internal/books/close.go:60-69`).
9. Do not trust the fallback cluster on 55432 — it is 65432, and it is fallback.
10. Do not add a `numeric` money column or a second balance table — reports sum
    the lines.

## 8. Where to read next

| File | Read this when… |
| :--- | :--- |
| [`docs/v2/context/gl.md`](docs/v2/context/gl.md) | touching vouchers, chart, numbering, period rules |
| [`docs/v2/context/posting.md`](docs/v2/context/posting.md) | adding a posting path, numbering, audit, reversal |
| [`docs/v2/context/trading.md`](docs/v2/context/trading.md) | sales, purchase, receipt, payment, stock |
| [`docs/v2/context/inventory.md`](docs/v2/context/inventory.md) | valuation, COGS, stocktake |
| [`docs/v2/context/reports.md`](docs/v2/context/reports.md) | trial balance, P&L, balance sheet, aging, export |
| [`docs/v2/context/close.md`](docs/v2/context/close.md) | period close, 911 → 4212, locks |
| [`docs/v2/context/document.md`](docs/v2/context/document.md) | AR/AP documents, allocations (two models!) |
| [`docs/v2/context/auth.md`](docs/v2/context/auth.md) | login, roles, sessions, CSRF |
| [`docs/v2/context/tenancy.md`](docs/v2/context/tenancy.md) | RLS, `Scope`, test harness |
| [`docs/v2/context/web.md`](docs/v2/context/web.md) | routes, forms, templates, assets |
| [`docs/v2/context/routes.md`](docs/v2/context/routes.md) | exact method/path/handler/role table |
| [`docs/v2/context/db-schema.md`](docs/v2/context/db-schema.md) | tables, columns, triggers, policies |
| [`docs/v2/context/domain-glossary.md`](docs/v2/context/domain-glossary.md) | Vietnamese accounting terms in code |
| [`docs/v2/context/decisions-index.md`](docs/v2/context/decisions-index.md) | which ADR governs what, and what is superseded |
| [`docs/v2/context/legal-status.md`](docs/v2/context/legal-status.md) | rates, sunset dates, `?` items |
| [`docs/v2/context/drift-report.md`](docs/v2/context/drift-report.md) | auditing a doc claim against code |
| `docs/v2/open-questions.md` | blocked on a human decision |

## 9. Top risks

1. Prose and code disagree on the stack — an agent may add sqlc/Alpine/Tabulator.
2. No login UI; the operator guide says otherwise — a pilot cannot sign in.
3. Legal clock: Circular 48 expires ≤ 2026-12-31 and only 10% VAT is coded.
4. Tests share one database and accumulate — cross-test pollution (todo B3).
5. Three security guarantees are documented but not enforced (see §7.8).
