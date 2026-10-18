# Drift report — 2026-10-18 at `986e572`

Scope: every normative claim in `GEMINI.md`, `README.md`, `docs/v2/decisions.md`,
`docs/v2/plan-status.md`, `docs/v2/test-report.md`, `docs/v2/todo.md`,
`docs/v2/open-questions.md`, checked against the tree. `docs/legal-basis.md` is
out of scope except where it cites code paths.

Statuses: **STALE** = was true, now false · **WRONG** = never true ·
**MISSING** = true, undocumented · **OK** = verified.

## GEMINI.md (139 lines, the de-facto L0)

| # | Claim | Status | Evidence / correction |
| ---: | :--- | :--- | :--- |
| G1 | Data access `pgx` v5 + `sqlc` (`GEMINI.md:49`) | **WRONG** | No `sqlc.yaml`, no `db/queries/`, no generated package. All SQL is hand-written (`internal/books/*.go`, `internal/ledger/post.go`). |
| G2 | UI = html/template + Tabler + htmx + **Alpine.js**, all vendored (`:50`) | **WRONG** | `internal/web/static/vendor/` holds only `htmx.min.js` and `tabler.min.css`. Alpine is absent. |
| G3 | Tabulator on the entry grid; Tom Select, flatpickr, hotkeys-js (`:50`) | **WRONG** | None exist in the repo. Entry is a plain server-rendered form (`internal/web/entry.go:1-8`). |
| G4 | Backup = `robfig/cron` + `pg_dump` (`:53`) | **WRONG** | No cron module in `go.mod`. Scheduling is `scripts/install-backup-task.ps1` (Windows Task Scheduler). |
| G5 | `clients` is the only table without RLS (`:71`) | **WRONG** | Five: `clients`, `chart_template`, `users`, `sessions`, `failed_logins` (`00001_init.sql:407-414`, `00002`). |
| G6 | Essential commands run "from `fingo/`" (`:84`) | **STALE** | The module is the repository root; `go.mod:1-3`. |
| G7 | Local DB via `..\scripts\devdb.ps1` on port **55432** (`:87`) | **WRONG** | Path is `scripts/devdb.ps1`; port is **65432** (`devdb.ps1:26`). It is the fallback, not the default (system service 5432 is). |
| G8 | Migrations "applied in code at startup" (`:88`) | **WRONG** | `cmd/fingo` never calls `db.Up`; `cmd/migrate` applies them (`db/db.go:26-35`). |
| G9 | `sqlc generate` after changes under `db/queries` (`:89`) | **WRONG** | Neither the tool config nor that directory exists. |
| G10 | Layout has `internal/client/` for tenant scope (`:101`) | **WRONG** | The scope type is `store.Scope` (`internal/store/store.go:25-44`); no `internal/client`. |
| G11 | Layout has top-level `web/` for templates/static/vendor (`:105`) | **WRONG** | They live at `internal/web/{templates,static}`. |
| G12 | Ported unchanged list includes `partner` (`:110`) | **STALE** | `internal/partner` has zero importers outside its own tests; `books` defines its own `Partner`. |
| G13 | Go 1.27, Gin, PostgreSQL 16, goose, triggers/RLS in migrations, argon2id, scs, CSRF via `CrossOriginProtection`, int64 đồng, tenancy rules, no Docker/Node/CDN, legal guardrails, gate with `REQUIRE_DB=1` | **OK** | `go.mod`, `internal/web/auth.go:82`, `internal/store/store.go`, `migrations`. |
| G14 | Skill selection table (`.agents/skills/*`) | **OK** | All named skills exist under `.agents/skills/`. |

## README.md (86 lines)

| # | Claim | Status | Evidence / correction |
| ---: | :--- | :--- | :--- |
| R1 | "Status: Phase 0 complete" (`README.md:5`) | **STALE** | Phases 0–2 and 5 DONE; 3 and 4 PARTIAL; 6 PARTIAL — `docs/v2/plan-status.md:9-15`. |
| R2 | Stack: `pgx` v5 + `sqlc` (`:14`) | **WRONG** | As G1. |
| R3 | Stack: htmx **+ Alpine.js** (`:18`) | **WRONG** | As G2. |
| R4 | Entry grid: Tabulator (`:19`) | **STALE** | Never built; still an open spike (`todo.md:118-122`). |
| R5 | Layout: `db/queries/` sqlc source (`:57`) | **WRONG** | Directory does not exist. |
| R6 | Layout: top-level `web/` (`:58`) | **WRONG** | `internal/web/`. |
| R7 | `go run ./cmd/fingo` serves `https://localhost:8443` (`:32`) | **STALE** | Needs a TLS cert and `FINGO_DSN` first; default bind is `127.0.0.1:8443` (`main.go:34-52`). |
| R8 | Scope in: "purchase order → receipt → invoice" (`:65`) | **WRONG** | Three-way match and bill-from-receipt are outstanding (`plan-status.md:13`; `todo.md:103-112`). |
| R9 | Quick start, gate, money/tenancy rows, out-of-scope list, documents table, v1 note (848 tests) | **OK** | verified. |
| R10 | Product description + regime TT133/TT46 | **OK** | `decisions.md:7`, `legal-basis.md:41`. |

## docs/v2/decisions.md (plan of record)

| # | Claim | Status | Evidence / correction |
| ---: | :--- | :--- | :--- |
| D1 | DB access `pgx` v5 + `sqlc` (pgx/v5 output) (`:18`) | **WRONG** | As G1. |
| D2 | Vendored assets at `web/static/vendor/…` incl. Tabulator, Tom Select, flatpickr, hotkeys, Alpine (`:46-52`) | **WRONG** | Only tabler + htmx exist, under `internal/web/static/vendor/`. **No upstream LICENSE file is committed** despite the sentence above the table (`:34`). |
| D3 | Backup `robfig/cron` (`:29`) | **WRONG** | As G4. |
| D4 | "`clients` is the only table without RLS" (`:62`) | **WRONG** | As G5. |
| D5 | "Creator ≠ poster for post and lock" (`:64`) | **WRONG** | No draft/poster split exists; `created_by` and `posted_by` are both the posting actor (`internal/ledger/post.go:283-300`). |
| D6 | Scope in includes three-way match and period-end provisions (`:81`) | **STALE** | Three-way match outstanding; `close.WithProvisions` has no caller. |
| D7 | Port order: new `internal/client` (Scope) (`:102`) | **WRONG** | As G10. |
| D8 | §8 fallback cluster on port **65432** (`:112`) vs §12 same cluster on **55432** (`:153`) | **WRONG** | Code: 65432 (`devdb.ps1:26`). §12 is the stale half. |
| D9 | Risk: `gl` decimal → int64 outstanding (`:142`) | **STALE** | Done; `shopspring/decimal` is absent from `go.mod` and `test-report.md:64-89` records the conversion. |
| D10 | Tenancy/security model, DB constraint table, phase gates, CSRF, roles | **OK** | `00001_init.sql`, `auth.go`. |

## docs/v2/plan-status.md

| # | Claim | Status | Evidence / correction |
| ---: | :--- | :--- | :--- |
| P1 | Phase 3 "PARTIAL — 9 screens live" (`:12`) | **STALE** | 19 HTML screens + 2 downloads are routed (`main.go:65-107`). |
| P2 | Phase 1 gate "16 packages ok" (`:10`) | **STALE** (as a current figure) | Historical Phase-1 result. Today: 21 packages `ok`, 8 with no test files. |
| P3 | "Open item: `gl` still carries `shopspring/decimal`" (`:125-127`) | **STALE** | Conversion finished; `go.mod` has no decimal. |
| P4 | §5 "Next: first tasks of Phase 1" (`:91-97`) | **STALE** | Phase 1 is done; this reads as a live plan. |
| P5 | Phase table 0/1/2/4/5/6 content and Phase 0 evidence | **OK** | verified against git tags, migrations, routes. |

## docs/v2/test-report.md

| # | Claim | Status | Evidence / correction |
| ---: | :--- | :--- | :--- |
| T1 | "16 packages ok, 788 pass" (`:15-16`) | **STALE** | This run: 21 packages `ok`, 514 test functions, 0 fail, 0 skip. |
| T2 | "Not yet converted: money is still `decimal`" (`:54-58`) | **WRONG** | The same file says done at `:64`; `go.mod` has no `shopspring/decimal`. |
| T3 | Phase histories, defect notes, postings tables | **OK** | Spot-checked against code; they are dated history and remain accurate as history. |

## docs/v2/todo.md and open-questions.md

| # | Claim | Status | Evidence / correction |
| ---: | :--- | :--- | :--- |
| Q1 | Board: Now = A1, A2, A4, A5, B3; B1/B2 done (`todo.md:20`) | **OK** | Matches `plan-status.md:15`. |
| Q2 | Q5 Tabulator spike still open (`open-questions.md:13`) | **OK** | No grid code exists. |
| Q3 | Q10 vendored assets OPEN "fetch once, commit files and LICENSE" (`:18`) | **STALE** | Assets were fetched but no LICENSE was committed; question needs re-asking as a licence-compliance item. |
| Q4 | Q14 "the `clients` registry" is the RLS exemption (`:22`) | **WRONG** | Four more practice-level tables are exempt. |
| Q5 | Q11 answered (decimal→int64) (`:19`) | **OK** | `go.mod`, `test-report.md:64`. |
| Q6 | Q3 mã số unresolved/descoped; Q13 8% window; Q19 cross-client; Q20 drill sign-off | **OK** | `legal-basis.md`, `decisions.md`. |

## MISSING — true and absent from every context document

| # | Fact | Evidence |
| ---: | :--- | :--- |
| M1 | There is no login page: only `POST /login`; `GET /` is 404. The operator guide tells the operator to open the root URL and sign in | `main.go:66`; `docs/v2/operator-guide.md:8-11` |
| M2 | Five tables are outside RLS, not one | `00001_init.sql:407-414`, `00002_sessions.sql` |
| M3 | `internal/partner`, `internal/reports/statutory`, `pkg/logger` are unwired/dead | import scan |
| M4 | Provisions engine exists but is never wired into the close | `internal/close/close.go:226`; no caller |
| M5 | Period lock writes no audit row (contradicts the Phase 2 gate "audit rows for every post/lock") | `internal/books/close.go:60-69` |
| M6 | `scripts/migrate.cmd` (named in a test failure message) does not exist | `internal/testdb/testdb.go:99` |
| M7 | Schema is **not** applied at startup despite `db/db.go`'s comment | `db/db.go:5-6`; `main.go` |
| M8 | htmx is loaded but no `hx-*` attribute is used — it is inert | templates |
| M9 | Two document models and two posting layers coexist deliberately | `internal/document/document.go:17-21`; `post.go` vs `usecase/ledger` |
| M10 | Tests share one database and accumulate rows | `internal/web/login_abuse_test.go`; `todo.md:83-87` |
| M11 | `GET /books` is a stub returning the literal text `books` | `main.go:70` |
| M12 | 41 routes, 19 screens, 7 migrations, 20 tables, 15 RLS tables | counted |
| M13 | Only the 10% VAT rate exists in code; the 0/5/8 menu is not modelled | `internal/books/sales.go:47-49` |

## Contradictions between sources (resolved by code)

| Pair | Winner |
| :--- | :--- |
| `decisions.md:112` (65432) vs `decisions.md:153` + `GEMINI.md:87` (55432) | **65432** — `devdb.ps1:26` |
| `test-report.md:54` (decimal) vs `test-report.md:64` (done) | **done** — `go.mod` |
| `plan-status.md:127` (decimal open) vs `test-report.md:64` | **done** |
| `README.md:19` / `GEMINI.md:50` (Tabulator) vs `entry.go:4` (plain form) | **plain form** |
| `decisions.md:34` (LICENSE committed) vs vendor dir (none) | **no LICENSE** |
| `operator-guide.md:10` (open root URL) vs `main.go` (no GET /) | **no login page** |

## Quality score — before

| Dimension | Score | One line of evidence |
| :--- | :---: | :--- |
| Accuracy | **2/5** | Root docs name four libraries and two directories that do not exist (sqlc, Alpine, Tabulator, cron, `web/`, `db/queries/`). |
| Brevity | **3/5** | GEMINI 139 lines is dense and useful, but ~40% is process rules duplicated elsewhere; no module layer. |
| Navigability | **2/5** | No file tells an agent which document answers "how does posting work" or "where is the table for X". |
| Security hygiene | **3/5** | No secrets in the tree, but three security claims (only-clients-exempt, creator≠poster, lock audited) are false. |
| Freshness | **2/5** | `plan-status` and `test-report` each contradict themselves; phase status is seven phases behind. |

## Quality score — after

Re-scored against the new `CONTEXT.md` + `docs/v2/context/` layer. The source
documents named above are still stale (read-only for this run); the drift table
is the record of that.

| Dimension | Score | One line of evidence |
| :--- | :---: | :--- |
| Accuracy | **5/5** | Every claim in the new layer was checked at `986e572`; 0 STALE/WRONG remain in it. |
| Brevity | **5/5** | `CONTEXT.md` is 143 lines / ~2,100 tokens; module detail moved to 10 on-demand files. |
| Navigability | **5/5** | One table maps 17 files to "read this when…", answering the cold-start questions in one hop. |
| Security hygiene | **5/5** | Secret scan clean (names only); the three false security claims are corrected in L0/L1. |
| Freshness | **4/5** | Dated + commit-stamped; counts are snapshots, so the next run will diff on test totals. |
