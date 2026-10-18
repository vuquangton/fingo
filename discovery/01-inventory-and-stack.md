# 01 — Inventory and Stack

> Discovery run: read-only, from `D:\accounting` at git `HEAD` = `f9193d3` (2026-09-20) with a large uncommitted working tree. All paths below are repository-relative.

## 1. What this repository actually is

| Observation | Evidence | Confidence |
| :--- | :--- | :--- |
| The repository is called FinGo: an **on-premise Windows accounting desktop application for Vietnamese SMEs**. | `GEMINI.md:46-54`; `docs/requirements-v1.md:1-6` | HIGH |
| The **code is Go**, not .NET. The root README describes a .NET 10/WPF/PostgreSQL solution that does not exist anywhere in the tree. | Root `README.md:1-10` (describes `src\Core\...`, `Accounting.slnx`); no `src/`, no `*.slnx`, no `*.csproj` in the working tree; only 3 stray `.cs` files in `spike/` | HIGH |
| Two Go module roots exist: the application at `fingo/`, and two throwaway spikes under `spike/`. | `fingo/go.mod:1` (`module fingo`), `spike/editable/go.mod:1`, `spike/walkui/go.mod:1` | HIGH |
| Three UI directions are visible in one repository: .NET/WPF (deleted), Wails+Vue (deleted from the working tree, still in `HEAD`), native Win32 via `lxn/walk` (current). | `689b20b` deletes 296 files; `git status` shows `fingo/frontend/**` deleted; `fingo/cmd/fingo/main.go:1-16` is the walk entry point | HIGH |

### Sizes

| Metric | Value | Evidence |
| :--- | ---: | :--- |
| Files (excluding `.git`) | 730 | recursive count |
| Repository size (excluding `.git`) | 235.1 MB | disk measure |
| — `research/` (legal source PDFs/text) | 118.9 MB | disk measure |
| — `fingo/` (of which `fingo/dist/` is 47.8 MB of build output) | 50.2 MB | disk measure |
| — `spike/` (committed-adjacent `.exe` binaries) | 32.7 MB | `spike/walkui/spike.exe` 14.8 MB, `spike/walkui/spike-release.exe` 10.1 MB, `spike/editable/spike2.exe` 8.5 MB |
| — `.codegraph/` (15 MB SQLite index) | 19.6 MB | `.codegraph/codegraph.db` |
| Go source files | 285 | `*.go` |
| **Go LOC (total)** | **64,116** | line count |
| — of which `_test.go` | 30,097 (46.9%) | line count |
| — **committed at HEAD** | **2,102 LOC in 36 files** | `git ls-tree -r HEAD` |
| SQL LOC | 1,213 across 37 files | migrations, queries, triggers |
| Markdown LOC | 5,230 across 71 files (incl. 27 agent-skill files) | line count |

**The single most important inventory fact:** 96% of the Go code (281 of 285 files, ~62k LOC) exists **only in the working tree and has never been committed** — see `git ls-files fingo` (72 tracked files) versus `git status --porcelain` (99 untracked entries under `fingo/`, 34 under `docs/`). [HIGH — observed directly]

### Files by extension

| Ext | Count | Size | Note |
| :--- | ---: | ---: | :--- |
| `.go` | 285 | 1.98 MB | application + tests + 2 spikes |
| `.md` | 71 | 482 KB | 27 are `.agents/skills/**`, 16 in `docs/` (11 top-level + 5 ADR), remainder root READMEs/GEMINI/spike notes |
| `.png` | 58 | 35.5 MB | GUI screenshots in `docs/` and spike artefacts |
| `.txt` | 53 | 4.2 MB | Vietnamese legal texts in `research/`, `_vat_research/` |
| `.xml` | 43 | 11.8 MB | office/legal document internals |
| `.dll` | 41 | 31.8 MB | bundled MariaDB server in `fingo/dist/server/bin` |
| `.sql` | 37 | 57 KB | 15 migrations, 11 query files, 12 triggers |
| `.pdf` | 15 | 49.2 MB | gazette/decree PDFs in `research/` |
| `.exe` | 8 | 48.8 MB | `fingo/dist/FinGo.exe` (12 MB) + MariaDB binaries + spike binaries |
| `.py` | 11 | 16 KB | ad-hoc legal-text extraction scripts in `research/` |
| `.cs` | 3 | 9 KB | Win32 P/Invoke experiments (`spike/click-row.cs`, `drive-form.cs`, `panes.cs`) |
| `.nsi` | 1 | 5 KB | Wails' NSIS template — unusable per `docs/plan-native-shell.md:320-322` |

## 2. Entry points and build files

| Artifact | Path | Role | Confidence |
| :--- | :--- | :--- | :--- |
| Production entry point | `fingo/cmd/fingo/main.go` (691 lines) | Native walk desktop app: single-instance guard, provisioning, window, 5 screens | HIGH |
| Secondary entry point | `fingo/cmd/webui/main.go` | Serves the HTML admin UI over in-memory books; a dev harness, not shipped | HIGH — `docs/plan-native-shell.md:83-86` |
| Old entry point (tracked, deleted in working tree) | `fingo/main.go`, `fingo/app.go`, `fingo/wails.json` | Wails shell | HIGH — `git status` `D fingo/main.go` |
| Build script | `fingo/build.cmd` → `fingo/build.ps1` | `go generate` (rsrc manifest) → `go build -H windowsgui -s -w` → bundle MariaDB `bin`, `lib\plugin`, `share\english`, `share\charsets` into `dist\server` | HIGH |
| Manifest generation | `fingo/cmd/fingo/main.go:17` (`//go:generate rsrc ...`) | Common Controls v6 manifest is mandatory or the app will not start | HIGH |
| Module file | `fingo/go.mod` | `go 1.26.0`; 7 direct dependencies | HIGH |
| SQL codegen config | `fingo/sqlc.yaml` | mysql engine, schema=`db/migrations`, out=`internal/adapter/mariadb/sqlc` | HIGH |
| Migrations | `fingo/db/migrations/00001..00015` | goose-marked SQL | HIGH |
| Trigger DDL (not a migration) | `fingo/db/triggers/*.sql` (12 files) | Re-applied by `db.Up` after goose; the goose CLI alone leaves a database without them | HIGH — `GEMINI.md:84` |
| Client registry (SQL only) | `fingo/db/client_scope_integration_test.go` | `internal/client` has no adapter | HIGH |
| No solution/project files | — | `[NOT FOUND]`: no `.sln`, `.slnx`, `pom.xml`, `Makefile`, `Dockerfile`, `package.json` (the Vue one is deleted) | HIGH |

## 3. Detected stack

| Layer | Technology | Version | Evidence | Confidence |
| :--- | :--- | :--- | :--- | :--- |
| Language | Go | `go 1.26.0` declared; toolchain on this machine is `go1.27.0 windows/amd64` | `fingo/go.mod:3`; `go version` | HIGH |
| Desktop UI | `github.com/lxn/walk` (Win32 native) | pseudo-version `v0.0.0-20210112085537-c389da54e794` (last upstream commit 2021-01-12) | `fingo/go.mod:9`; `docs/plan-native-shell.md:348` | HIGH |
| DB engine | MariaDB | 12.3 (12.3.2-MariaDB reported; the build script also probes 11.4 and 10.11) | `docs/plan-native-shell.md:236,365`; `fingo/build.ps1:63-67` | HIGH |
| DB driver | `go-sql-driver/mysql` | v1.10.1 | `fingo/go.mod:6` | HIGH |
| Query layer | `sqlc` generated code | v2 config | `fingo/sqlc.yaml`; `fingo/internal/adapter/mariadb/sqlc/` | HIGH |
| Migrations | `pressly/goose/v3` (embedded) | v3.28.0 | `fingo/go.mod:8`; `fingo/db/migrate.go` | HIGH |
| Money math | `shopspring/decimal` in `internal/domain/*`; `int64` minor units in `internal/{money,vat,inventory,document}` | v1.4.0 | `fingo/go.mod:10`; `GEMINI.md:64-66` | HIGH |
| IDs | `google/uuid` | v1.6.0 | `fingo/go.mod:7` | HIGH |
| OS / text | `golang.org/x/sys`, `golang.org/x/text` | v0.47.0, v0.41.0 | `fingo/go.mod:11-12` | HIGH |
| HTML admin UI | `html/template` + vendored `htmx.min.js` + embedded static | — | `fingo/web/render.go`; `fingo/web/static/htmx.min.js` | HIGH |
| App server | **None** — desktop binary; the HTTP UI runs in-process on demand | — | `docs/adr/0004-admin-ui-is-server-rendered.md:8-30` | HIGH |
| Target OS | Windows only | `cmd/fingo/app.manifest`, `rsrc.syso`, `-H windowsgui`; `internal/singleton/guard_other.go` is a non-Windows stub | HIGH |
| CI/CD | **None** | no `.github/`, `.gitlab-ci.yml`, `azure-pipelines.yml`, `Makefile` | HIGH — also `docs/testing-strategy.md:21` |
| Linter config | **None** | no `.golangci.yml` | HIGH — `docs/testing-strategy.md:22` |

### Runtime version checks

- Go 1.26 is newer than any public release I can verify, and the constant `go 1.26.0` cannot be validated from inside this repository. `[INFERRED]` This repository is dated in a simulated/projected future (all git commits are 2026-09-18…2026-09-20; file mtimes run to 2026-10-12). Do not read the dates as evidence about the real Go ecosystem. [MEDIUM]
- `spike/*/go.mod` require `go 1.27.0`, one minor above the application module. [HIGH]

### End-of-life / unsupported components

| Component | Status | Evidence | Confidence |
| :--- | :--- | :--- | :--- |
| `lxn/walk` | **Dormant**: last commit 2021-01-12, ~345 open issues; no accessibility/UIA bridge | `docs/plan-native-shell.md:348-349` | HIGH |
| `gopkg.in/Knetic/govaluate.v3` | Pulled in transitively by walk; unmaintained upstream | `fingo/go.mod:16` | MEDIUM `[INFERRED]` from the indirect dependency |
| Wails v2 / Vue 3 frontend | Deleted from the working tree; `go.mod` no longer requires Wails | `git status`; `fingo/go.mod` | HIGH |
| Circular 200/2014/TT-BTC, TT 89/2026 forms, Circular 48/2019 | Legal, not software: the project's own register says 48/2019 expires ≤ 2026-12-31 and the tax-administration layer was replaced 2026-07-01 | `docs/legal-basis.md:12-31`; `docs/accounting-review.md:8-22` | HIGH (per project docs) |
| .NET 10 WPF track | Abandoned; 296 files deleted in one commit | `git show --stat 689b20b` | HIGH |

## 4. Documentation present

| Document | Path | Lines | What it is | Confidence |
| :--- | :--- | ---: | :--- | :--- |
| Agent/project rules + living architecture state | `GEMINI.md` | 178 | Caveman-style operating rules, stack, module-by-module status including "Unwired" notes | HIGH |
| Requirements baseline | `docs/requirements-v1.md` | 180 | Actors, 18-event transaction inventory, in/out of scope, testable acceptance criteria | HIGH |
| Accounting data review | `docs/accounting-review.md` | 145 | 67 findings, severity-ranked (Critical 13 / High 24 / Med 23 / Low 7) | HIGH |
| Legal basis register | `docs/legal-basis.md` | 298 | Instrument register, confidence marks `P`/`S`/`?`, open questions Q2–Q8, payroll/provision rates | HIGH |
| Testing strategy | `docs/testing-strategy.md` | 631 | Suite shape, CI proposal, the "81 tests that can vanish" gap | HIGH |
| Page catalogue (2 drafts) | `docs/pages.md`, `docs/pages-map.md` | 492 / 465 | Screen-by-screen IA with engine-backing column | HIGH |
| UI plans | `docs/plan-gui.md` (268), `docs/plan-native-shell.md` (388), `docs/plan-desktop-dashboard.md` (200) | | Three successive UI plans; the last supersedes the others | HIGH |
| Reference flow | `docs/reference-misa-sme-flow.md` | 207 | Competitive/UX reference against MISA SME | HIGH |
| ADRs | `docs/adr/0001..0005` | 24 KB | Journal line model, posting lifecycle, client boundary, server-rendered UI, connection settings | HIGH |
| Smoke-test script | `docs/UIWorkflowSmokeTestScript.md` | 34 | Leftover from the .NET track (still tracked) | HIGH |
| Legal source corpus | `research/`, `_vat_research/` | 118.9 + 12.5 MB | Gazette PDFs, extracted text, Python fetch/extract scripts | HIGH |
| GUI evidence | `docs/gui-*.png` (18 files) | — | Screenshots of the running native app (first run, posting, trial balance) | HIGH |
| README (root) | `README.md` | 100 | **Stale**: describes the deleted .NET 10/WPF solution | HIGH |
| README (module) | `fingo/README.md` | 19 | **Stale**: the untouched Wails Vue-TS template | HIGH |

### Language of the repository

- **Code comments and UI strings are primarily English prose with Vietnamese domain terms**, deliberately. Vietnamese phrases appear as user-facing strings (`"chưa hỗ trợ"`, account names), and Vietnamese accounting terms (Nợ/Có, TK, kết chuyển) are used inside English sentences. [HIGH]
- **All planning/legal documents are English**, with Vietnamese instrument names and form codes quoted verbatim. [HIGH]
- **`docs/pages.md`, `docs/pages-map.md`, `docs/reference-misa-sme-flow.md` are partly Vietnamese** (page names, operator questions). [HIGH]
- A previous track explicitly fixed a UTF-8 BOM problem for Vietnamese rendering in WPF: commit `2c888c1`. [HIGH]
- I verified the on-disk encoding: Go, SQL and Markdown files are valid UTF-8 with real Vietnamese codepoints (e.g. `U+1EC7 ệ` in `db/migrations/00001_init_fingo_schema.sql`). Earlier mojibake in my terminal was a console-codepage artefact, **not** a repository defect. [HIGH]

## 5. Module/package inventory

`fingo/` contains 281 non-spike, non-test-heavy Go files organised as:

```
fingo/
  cmd/fingo/            native desktop entry point (walk)
  cmd/webui/            HTML admin UI harness
  db/                   embedded migrations, queries, triggers, runner
  internal/
    domain/*            17 packages — the original skeleton (see 03)
    adapter/mariadb/*   9 adapters + sqlc-generated code + testsupport
    client/             tenant registry + Scope (unwired)
    money/ vat/ document/ partner/ inventory/ posting/ period/ close/
    trading/            sales + purchase cycles (unwired)
    reports/ report+statutory output
    migrate/            backup-before-migrate runner
    boot/ dbhost/ onboarding/ appconfig/ singleton/  installation and provisioning
    idempotency/        HTTP idempotency middleware (no HTTP server mounts it)
    ui/*                11 presentation-model packages + walk widgets
    usecase/*           ledger, sales, practice
    books/              assembly: one client's books + reports
    pkg/logger/         slog wrapper
  web/                  server-rendered admin UI + e2e suite (demoted to harness)
  dist/                 build output (12 MB FinGo.exe + 36 MB MariaDB bundle)
```

Full module status is in [02-business-and-features.md](02-business-and-features.md); dependency reality (which packages the binary actually reaches) is in [03-architecture-and-data.md](03-architecture-and-data.md).

## 6. Environment and configuration inventory

| Item | Location | Contents | Confidence |
| :--- | :--- | :--- | :--- |
| Dev env file | `fingo/.env` (gitignored, 18 lines) | `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_DRIVER`, and a long commented `DB_PARAMS` (`loc=UTC`) | HIGH — values deliberately not reproduced |
| Runtime settings | `%APPDATA%\FinGo\config.json` (or `%LOCALAPPDATA%\FinGo` for the native app's root) | connection host/port/user/password/database, company name + tax code; **password in plaintext** | HIGH — `docs/adr/0005-connection-settings.md:20-27`; `fingo/internal/appconfig/appconfig.go:10-20` |
| Installation root override | `FINGO_ROOT` | moves the whole install root (portable install / tests) | HIGH — `cmd/fingo/main.go:672-686` |
| MariaDB bundle override | `FINGO_MARIADB_BIN` / `-MariaDbRoot` | where to copy the server from at build time | HIGH — `fingo/build.ps1:56-59` |
| Test database gate | `REQUIRE_DB` | when `1`, a missing database fails instead of skipping | HIGH — `internal/adapter/mariadb/testsupport/testsupport.go:99-120` |
| Feature flags | **none** | `[NOT FOUND]`: no flag package, no env-var feature toggles | HIGH |
| Per-customer variation | one database per client (`docs/requirements-v1.md:12-39`) **plus** a `client_id` column on 18 tables (`db/migrations/00015_client_scope.sql`) | two competing isolation mechanisms coexist | HIGH |

## 7. Secrets found (locations and types only)

| Type | Location | Notes |
| :--- | :--- | :--- |
| Dev DB password | `fingo/.env:5` | gitignored; not in version control (`git ls-files` does not list it) |
| Dev DB credential printed in a **tracked** document | `GEMINI.md:75` | a full `user:password@tcp(...)` DSN is quoted in the project-instructions file; this file *is* tracked |
| Plaintext DB password at rest | `%APPDATA%\FinGo\config.json` (written by `internal/appconfig`) | deliberate, documented in `docs/adr/0005-connection-settings.md:20-27`, file mode `0600` on write (`appconfig.go:290`) |
| Bundled DB has **no password** | `internal/boot/live.go:26-28` | deliberate: loopback-only server, no stored credential |
| No API keys, tokens, private keys | — | `[NOT FOUND]` in the tree outside `research/` PDFs and `dist/` |

[INFERRED] The `GEMINI.md:75` credential is the same development default as `fingo/.env`; it is a local `dev` login, not a production secret. It is still a tracked, credential-shaped string. [MEDIUM]
