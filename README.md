# FinGo v2

On-premise, multi-user, browser-based accounting for Vietnamese SMEs (TT133/2016/TT-BTC, as amended by TT 46/2025), built for an accounting practice that serves many client companies.

**Status: Phases 0–2 and 5 done; phases 3, 4 and 6 partial.** The v1 application (a Windows desktop binary) is archived at tag `archive/pre-rewrite` and is not built from this branch. See [`docs/v2/plan-status.md`](docs/v2/plan-status.md) for the live phase status, and [`CONTEXT.md`](CONTEXT.md) for the one-file briefing every contributor and agent should read first.

Known gaps: there is no login page yet (login is `POST /login` only), the three-way match is not built, and the entry grid is a plain server-rendered form.

## Stack

| Layer | Choice |
| :--- | :--- |
| Language | Go 1.27 |
| HTTP | Gin |
| Database | PostgreSQL 16 |
| DB access | `pgx` v5, hand-written SQL (no sqlc) |
| Migrations | `goose`, embedded; triggers live in migrations |
| Tenancy | one database, `client_id NOT NULL`, PostgreSQL row-level security |
| Money | `int64` đồng in Go, `bigint` in PostgreSQL, string on the wire |
| UI | server-rendered `html/template` + Tabler 1.0.0-beta21 (vendored), htmx (vendored, currently unused) |
| Entry grid | plain server-rendered rows; a Tabulator spike is still an open decision |
| Sessions | `scs` in PostgreSQL, argon2id passwords |
| Deploy | one Go binary with embedded templates and assets + PostgreSQL. No Docker, no Node, no CDN |

## Quick start (development)

```powershell
# 1. One-time: databases + app role on the local PostgreSQL 16 service
$env:PGADMIN_PASSWORD = '<postgres password>'   # read from the environment, never stored
.\scripts\pgsetup.cmd                              # writes a gitignored .env

# 2. Apply the schema (the server does not migrate itself)
$env:PGADMIN_PASSWORD = '<postgres password>'
go run ./cmd/migrate -database fingo_dev

# 3. Build, certificate, and run
go build ./...
go run ./cmd/devcert -dir .local/tls          # development certificate
$env:FINGO_DSN = 'host=127.0.0.1 port=5432 user=fingo_app password=<...> dbname=fingo_dev sslmode=disable'
go run ./cmd/fingo                            # serves https://127.0.0.1:8443

# 4. Tests (never skip the database path)
$env:REQUIRE_DB='1'
go test ./...
```

The gate for every change is:

```powershell
go build ./... ; go vet ./... ; $env:REQUIRE_DB='1' ; go test ./...
```

## Layout

```
cmd/fingo/            the server: config, router, startup
internal/
  domain/gl/          journal model, TT133 chart, posting rules
  money/ vat/ posting/ period/ document/ domain/document/
  partner/ inventory/ trading/ close/ reports/   the accounting engine
  store/              pgx pool, scoped transactions, RLS (store.Scope is the tenant scope)
  ledger/             the only posting path
  books/              assembly of reports/close/trading over one scoped store
  web/                Gin handlers, middleware, templates, sidecar assets
db/migrations/        goose migrations (schema, triggers, policies, COA seed)
scripts/              pgsetup, devdb, backup, restore-drill, backup task install
docs/                 requirements, legal basis, ADRs, v2 plan and reports
```

## Scope

**In:** login and roles · chart of accounts · journal entry and posting · journal list · ledger with drill-down · trial balance · P&L · balance sheet · sales invoice with COGS · receipt and allocation · supplier bill (cost path) and supplier payment · stock list, goods receipt and stocktake · period close (911 → 4212/4211) · reversal · Excel export · scheduled backup and a scripted restore drill · opening-balance import · AR/AP aging · per-client handover export.

**Not yet built (in scope, outstanding):** purchase order → goods receipt → supplier invoice (three-way match, engine-only today) · bill from a goods receipt · stock transfer between warehouses · the Tabulator entry-grid decision.

**Out:** payroll · fixed assets · CCDC/TK 242 · e-invoice · bank reconciliation · B03 cash flow · landed cost · statutory filing (`mã số`) · multi-currency · cross-client reporting.

## Documents

| Document | What it is |
| :--- | :--- |
| [`CONTEXT.md`](CONTEXT.md) | **Start here.** One-file briefing: stack, commands, layout, hard rules, traps |
| [`docs/v2/context/`](docs/v2/context/) | Module and reference context, generated where possible |
| [`docs/v2/decisions.md`](docs/v2/decisions.md) | The plan of record: stack, tenancy, scope, port order, gates |
| [`docs/v2/plan-status.md`](docs/v2/plan-status.md) | Phase status and gate evidence |
| [`docs/v2/open-questions.md`](docs/v2/open-questions.md) | Open questions, defaults taken, who can answer |
| [`docs/v2/operator-guide.md`](docs/v2/operator-guide.md) | Hướng dẫn sử dụng cho kế toán (tiếng Việt) |
| [`docs/v2/installer-notes.md`](docs/v2/installer-notes.md) | Install, run, back up, upgrade, and what never to do |
| [`docs/v2/todo.md`](docs/v2/todo.md) | Engineering todo: P0/P1/P2, owners, done tests, release gates |
| [`docs/requirements-v1.md`](docs/requirements-v1.md) | Business requirements (still the baseline for *what*) |
| [`docs/legal-basis.md`](docs/legal-basis.md) | Which legal instrument governs which figure; confidence marks |
| [`docs/adr/`](docs/adr/) | Architecture decisions; 0006–0008 are v2 |
| [`discovery/`](discovery/) | The read-only discovery report on v1 |

## A note on v1

v1 is preserved at `archive/pre-rewrite` with its full history: a Go accounting engine with 848 tests, a Win32 desktop shell, a bundled MariaDB, and a legal register. The engine is being ported; the shell is being replaced. Nothing from v1 is deleted from history.
