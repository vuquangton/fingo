# 05 — Build, Run, Deploy

> All of this is a **read-only assessment**. I did not run a build, a migration, or the test suite: the task forbids side effects, integration tests create/drop databases, and a build writes into the repository. Where a command is stated, it is quoted from the repository, not executed.

## 1. How to build and run, according to the repository

There are **two mutually contradictory instruction sets** in the tree. The current one is the build script; the other is stale.

| Source | Says | Status |
| :--- | :--- | :--- |
| `fingo/build.cmd` / `fingo/build.ps1` | `build.cmd` → `go generate ./cmd/fingo` (manifest) → `go build -ldflags "-H windowsgui -s -w" -o dist\FinGo.exe ./cmd/fingo` → bundle MariaDB into `dist\server` | **Current** — `dist\FinGo.exe` dated 2026-10-12 21:31, one minute after the last source edit |
| `GEMINI.md:78-88` | "Dev Live Server: `wails dev`", "Production Build: `wails build`" | **Stale** — Wails was deleted; `fingo/wails.json` and `fingo/main.go` no longer exist in the working tree |
| `docs/plan-native-shell.md:226,381-382` | `rsrc` + `go build` wired into `build.cmd`, one command | Matches the build script |
| `fingo/README.md` | "use `wails build`" | **Stale** — untouched Wails template |
| Root `README.md:75-84` | `dotnet build Accounting.slnx -c Release` / `dotnet test` | **Stale** — the .NET solution was deleted in `689b20b` |

### 1.1 Exact commands (current)

```powershell
# from d:\accounting\fingo
.\build.cmd                  # builds dist\FinGo.exe and bundles the DB server
.\build.cmd -NoBundle        # executable only (machine without MariaDB)
.\build.cmd -MariaDbRoot "C:\Program Files\MariaDB 12.3"
go generate ./cmd/fingo      # manifest → cmd/fingo/rsrc.syso (done by build.cmd)
go build ./...               # compile check
go vet ./...                 # vet check
go test ./...                # unit + integration (integration skips without a DB)
```

`GEMINI.md:85` names the gate as `go build ./...` then `go vet ./...` then `go test ./...`. `GEMINI.md:82-84` also documents `sqlc generate` after query/schema changes and `db.Up(sqlDB)` (not the goose CLI) as the way to apply migrations, because the goose CLI leaves triggers behind.

### 1.2 Prerequisites, exactly

| Prerequisite | Version | Why | Evidence | Present on this machine? |
| :--- | :--- | :--- | :--- | :--- |
| Windows | x64 | `walk` is Win32-only; `app.manifest`, `rsrc.syso`, `-H windowsgui` | `cmd/fingo/main.go:9-17` | yes |
| Go | **1.26+** (`go.mod` declares `go 1.26.0`; spikes declare 1.27.0) | module minimum | `fingo/go.mod:3`; `spike/*/go.mod` | yes — `go1.27.0 windows/amd64` |
| `rsrc` | any (`go install github.com/akavel/rsrc@latest`) | compiles the Common Controls v6 manifest; **without it the app does not start at all** | `fingo/build.ps1:132-134`; `cmd/fingo/main.go:17` | **no — not on PATH** |
| MariaDB **install** (build-time only) | 12.3 (script also probes 11.4, 10.11) | source of the bundled server binaries | `fingo/build.ps1:63-67` | **yes** — `C:\Program Files\MariaDB 12.3` |
| MariaDB server (run-time, tests only) | reachable server + credentials | integration tests | `internal/adapter/mariadb/testsupport` | yes |
| `sqlc` | v2 | regenerating `internal/adapter/mariadb/sqlc` after SQL changes | `fingo/sqlc.yaml` | **no — not on PATH** |
| `goose` CLI | v3 | *creating* a new migration only | `GEMINI.md:83` | **no — not on PATH** |
| PowerShell | 5.1+ (`build.cmd` calls `powershell -NoProfile -ExecutionPolicy Bypass`) | build wrapper | `build.cmd` | yes |

**Reproducibility note:** `build.cmd` fails immediately on a machine without `rsrc` (`Require-Command` throws). `rsrc` is not vendored, has no version pin, and is not in `go.mod` (it is a `go install` tool, not a dependency). [HIGH]

### 1.3 Running the application

- **Shipped path:** run `dist\FinGo.exe`. On first run it creates `%LOCALAPPDATA%\FinGo`, initialises a MariaDB data directory, starts the bundled server on loopback, creates the database, runs 15 migrations + 12 triggers, seeds 123 accounts, then shows the shell. Verified by the project: "Empty directory → provisioned in **12.6 s**" (`docs/plan-native-shell.md:236`) and by screenshots `docs/gui-bundled-firstrun.png`, `docs/gui-bundled-ready.png`.
- **Portable path:** `FINGO_ROOT` relocates the whole install root (`cmd/fingo/main.go:672-686`).
- **Dev path:** `go run ./cmd/fingo` — but it needs a `config.json` or `.env` pointing at a real server, and it will spawn the *bundled* server if `dist\server` exists beside the binary.
- **HTML harness:** `go run ./cmd/webui` serves the server-rendered admin UI over **in-memory** books (`cmd/webui/main.go:38`: "The books are in memory: nothing entered here survives the process").

## 2. Deployment model

| Aspect | State | Evidence | Confidence |
| :--- | :--- | :--- | :--- |
| Model | **Portable directory**, self-provisioning | `build.ps1`; `docs/plan-native-shell.md:243-246` | HIGH |
| Output | `dist\FinGo.exe` (12.0 MB) + `dist\server\` (73 files, 36.3 MB) = **47.8 MB** | disk measure | HIGH |
| Installer (MSI/NSIS/Inno) | **None** | `[NOT FOUND]`; the only `.nsi` is Wails' template (`fingo/build/windows/installer/project.nsi`, tracked, unusable per `plan-native-shell.md:320-322`) | HIGH |
| Portable ZIP | **None produced** by the script; the directory itself is the portable artifact | `build.ps1` | HIGH |
| Elevation | `asInvoker` — deliberately not elevated; writes only under the user profile | `cmd/fingo/app.manifest`; `plan-native-shell.md:156-158` | HIGH |
| Service / scheduled task | **None** — the DB server is a child process, killed by a Windows job object when the app dies | `internal/dbhost`; `plan-native-shell.md:237` | HIGH |
| Uninstall | "deleting one directory"; registry untouched | `cmd/fingo/main.go:664-670` | HIGH |
| Docker / containers | **None** | `[NOT FOUND]` | HIGH |

### Environments

| Environment | How identified | Database | Notes |
| :--- | :--- | :--- | :--- |
| Developer | `fingo/.env` (gitignored) | a local MariaDB, database `fingo` | `.env` is the fallback source; ADR 0005 |
| Test | `testsupport.Config` walks up to 5 directories for `.env`; creates `fingo_test_*` / `fingo_migrate_test_*` scratch DBs | local server | skips when unreachable unless `REQUIRE_DB=1` |
| Production / installed | `%LOCALAPPDATA%\FinGo\config.json` (native app) or `%APPDATA%\FinGo\config.json` (documented in ADR 0005) | bundled MariaDB under `%LOCALAPPDATA%\FinGo` | plaintext password |
| CI | **does not exist** | — | `docs/testing-strategy.md:21,336` |

There is **no test/staging/production separation** as configuration. "Production" is whatever machine the `.exe` runs on. [HIGH]

## 3. CI/CD

| Item | State | Evidence |
| :--- | :--- | :--- |
| CI pipeline | **None** — no `.github/`, `.gitlab-ci.yml`, `azure-pipelines.yml`, `Makefile`, `Dockerfile` | directory scan; `docs/testing-strategy.md:21` |
| Linter config | **None** — no `.golangci.yml` | `docs/testing-strategy.md:22` |
| Pre-commit hooks | **None** | `[NOT FOUND]` |
| Release automation | **None** — `build.cmd` is manual | `build.ps1` |
| Proposed CI | Documented in full, unimplemented: lint → unit (`-short -race`) → database job with `mariadb:11.4` service → coverage ratchet | `docs/testing-strategy.md:336-500` |

## 4. Tests

| Metric | Value | How measured | Confidence |
| :--- | ---: | :--- | :--- |
| Test files | 118 | `*_test.go` | HIGH |
| Test functions (`Test`/`Fuzz`/`Benchmark`) | **848** | function-level grep | HIGH |
| Test LOC | 30,097 (46.9% of all Go) | line count | HIGH |
| `t.Skip` call sites | 18 | grep | HIGH |
| Test functions that need a database | ≥81 (project's own count at 92 test files) | `docs/testing-strategy.md:16-20` | MEDIUM (count predates the current suite) |
| Packages with the most tests | `internal/domain` 123, `internal/ui` 109, `internal/reports` 88, `internal/close` 70 | per-package grep | HIGH |
| Test framework | stdlib `testing` only — **no testify, no mock library** | `docs/testing-strategy.md:32-34`; import scan | HIGH |
| Coverage artifact | `fingo/$cov`, 21,961 bytes, stale | file exists | HIGH |
| Coverage percentage | **unknown / not measurable read-only** | no profile in the repo is current | — |

### Are the tests runnable?

- **Unit tests: yes, plausibly.** They are stdlib-only, pure-Go, no CGO.
- **Database tests: only with a reachable MariaDB and a `.env`.** They create and drop scratch databases. Without those they **skip**, and `go test ./...` reports `ok` for every package. The project documents this as its own highest-value defect: *"81 tests that can vanish … a green suite proves nothing about the database path"* (`docs/testing-strategy.md:495-524`).
- **Mitigation that exists:** `REQUIRE_DB=1` turns a missing database into a **failure** (`internal/adapter/mariadb/testsupport/testsupport.go:99-120`), and `testsupport.Skipped` is exported so a `TestMain` can report how many skipped. `docs/testing-strategy.md:527-535` records this as "Phase 0 steps 1–2 done, step 3 outstanding" — the CI assertion that would read that count does not exist.
- **End-to-end:** `fingo/web/e2e_test.go` (514 lines) drives the whole stack through `httptest.NewServer` and the HTML UI, with no `HX-Request` header (i.e. the no-JavaScript path). This is the only end-to-end suite, and it is what makes `web/` worth keeping after demotion (`docs/plan-native-shell.md:83-86`).
- **The native UI itself is not tested.** `walk` has no test harness; the design compensates by pushing all rules into pure models (`internal/ui/*`) with 109 tests, leaving the widget layer (`internal/ui/shell`) with only `header_test.go`. `shell.go` (13 KB), `voucher.go` (8.8 KB) and `table.go` have **no tests**. [HIGH]

## 5. Reproducibility risks

| # | Risk | Impact | Evidence | Confidence |
| ---: | :--- | :--- | :--- | :--- |
| B1 | **The code is not in version control.** A clean clone produces only the Wails skeleton and cannot build the application at all. | Blocking | `git ls-files` vs working tree; [04 §1](04-pending-work.md) | HIGH |
| B2 | `rsrc` is required but unpinned and absent; build fails at step 1 on a fresh machine | Blocking (build) | `build.ps1:132-134`; not on PATH here | HIGH |
| B3 | MariaDB install location is discovered from a hard-coded candidate list (`C:\Program Files\MariaDB 12.3/11.4/10.11`) or `FINGO_MARIADB_BIN` | Build produces an exe that cannot provision | `build.ps1:56-72` | HIGH |
| B4 | Bundled server layout is hand-assembled by name (`mariadbd.exe`, `mysqld.exe`, `mariadb-admin.exe`, `mariadb-install-db.exe`, `bin\*.dll`, `lib\plugin`, `share\english`, `share\charsets`); a MariaDB upgrade can silently move a required file | First run fails with a misleading error | `build.ps1:150-190`; `plan-native-shell.md:264-267` | HIGH |
| B5 | `.env` is gitignored, so no clone can run the database tests without hand-writing one | Silent test skip | `docs/testing-strategy.md:495-505` | HIGH |
| B6 | Generated code (`internal/adapter/mariadb/sqlc`) is committed in the working tree but untracked; `sqlc` is not installed here, so regeneration is not reproducible on this machine | Schema drift | `sqlc.yaml`; toolchain check | HIGH |
| B7 | `dist/` (47.8 MB) and three spike `.exe` files (33 MB) live inside the project; if ever committed the repo doubles in size | Bloat, accidental binary commits | disk measure | MEDIUM |
| B8 | `research/` and `_vat_research/` (131 MB, including 15 PDFs and Python extraction scripts) are untracked but referenced by `docs/legal-basis.md` as the provenance of every rate | Audit trail cannot be reproduced from a clone | `docs/legal-basis.md:296-298` | HIGH |
| B9 | Time-zone sensitivity: a non-UTC DB connection silently shifts every DATE column by a day; mitigated only by `DB_PARAMS` in `.env` and `config.json` | Wrong accounting period | `fingo/.env:8-18` (records a real occurrence) | HIGH |
| B10 | Dates in the repository are in 2026 (commits, docs "Last verified: 2026-10-08", legal instruments effective 2026). Any date-based logic, CI certificate, or licence will behave oddly on a real clock. | Confusion; time-bomb behaviour | commit log, `docs/legal-basis.md:3` | HIGH |

## 6. Buildability verdict

| Question | Answer | Basis |
| :--- | :--- | :--- |
| Does the current working tree build? | **Very likely yes.** | `dist\FinGo.exe` (12 MB) was produced 2026-10-12 21:31, one minute after the last source edit; 848 tests exist; the project's own gate is stated and small (`build`/`vet`/`test`) |
| Can it be built from a **fresh clone**? | **No.** | 96% of the code, all migrations after 00001, and every design doc are untracked or deleted relative to `HEAD` |
| Can it be built on a **fresh machine**? | **Not without manual steps** | `rsrc` missing, MariaDB path discovery, `.env` creation |
| Is there a deployable artifact? | `dist\` portable directory, **no installer** | §2 |
| Is there a rollback path for a bad deploy? | **No** — no installer, no versioning of the app itself, no migration rollback (`internal/migrate.Restore` exists but is unreachable) | §2, §5 |
