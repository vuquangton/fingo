# FinGo v2

On-premise, multi-user, browser-based accounting for Vietnamese SMEs (TT133/2016/TT-BTC, as amended by TT 46/2025), built for an accounting practice that serves many client companies.

**Status: Phase 0 complete — v2 in progress.** The v1 application (a Windows desktop binary) is archived at tag `archive/pre-rewrite` and is not built from this branch. See [`docs/v2/plan-status.md`](docs/v2/plan-status.md) for the live phase status.

## Stack

| Layer | Choice |
| :--- | :--- |
| Language | Go 1.27 |
| HTTP | Gin |
| Database | PostgreSQL 16 |
| DB access | `pgx` v5 + `sqlc` |
| Migrations | `goose`, embedded; triggers live in migrations |
| Tenancy | one database, `client_id NOT NULL`, PostgreSQL row-level security |
| Money | `int64` đồng in Go, `bigint` in PostgreSQL, string on the wire |
| UI | server-rendered `html/template` + Tabler (vendored), htmx + Alpine.js |
| Entry grid | Tabulator, on the journal-entry screen only |
| Sessions | `scs` in PostgreSQL, argon2id passwords |
| Deploy | one Go binary with embedded templates and assets + PostgreSQL. No Docker, no Node, no CDN |

## Quick start (development)

```powershell
# 1. A local PostgreSQL cluster, no administrator rights needed (port 55432)
.\scripts\devdb.ps1

# 2. Build and run
cd fingo
go build ./...
go run ./cmd/fingo            # serves https://localhost:8443

# 3. Tests (never skip the database path)
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
  money/ vat/ posting/ period/ document/ partner/ inventory/
  trading/ close/ reports/            the accounting engine
  client/             tenant scope
  store/              pgx pool, scoped transactions, RLS
  web/                Gin handlers, middleware, templates
db/migrations/        goose migrations (schema, triggers, policies, COA seed)
db/queries/           sqlc source
web/                  templates, static, vendored assets
scripts/              devdb.ps1, backup.ps1, restore-drill.ps1, service install
docs/                 requirements, legal basis, ADRs, v2 plan and reports
```

## Scope

**In:** login and roles · chart of accounts · journal entry and posting · journal list · ledger with drill-down · trial balance · P&L · balance sheet · sales invoice with COGS · receipt and allocation · purchase order → receipt → invoice · supplier payment · stock list · period close (911 → 4212/4211) · Excel export · scheduled backup and a scripted restore drill · opening-balance import · per-client handover export.

**Out:** payroll · fixed assets · CCDC/TK 242 · e-invoice · bank reconciliation · B03 cash flow · landed cost · statutory filing (`mã số`) · multi-currency · cross-client reporting.

## Documents

| Document | What it is |
| :--- | :--- |
| [`docs/v2/decisions.md`](docs/v2/decisions.md) | The plan of record: stack, tenancy, scope, port order, gates |
| [`docs/v2/plan-status.md`](docs/v2/plan-status.md) | Phase status and gate evidence |
| [`docs/v2/open-questions.md`](docs/v2/open-questions.md) | Open questions, defaults taken, who can answer |
| [`docs/requirements-v1.md`](docs/requirements-v1.md) | Business requirements (still the baseline for *what*) |
| [`docs/legal-basis.md`](docs/legal-basis.md) | Which legal instrument governs which figure; confidence marks |
| [`docs/adr/`](docs/adr/) | Architecture decisions; 0006–0008 are v2 |
| [`discovery/`](discovery/) | The read-only discovery report on v1 |

## A note on v1

v1 is preserved at `archive/pre-rewrite` with its full history: a Go accounting engine with 848 tests, a Win32 desktop shell, a bundled MariaDB, and a legal register. The engine is being ported; the shell is being replaced. Nothing from v1 is deleted from history.
