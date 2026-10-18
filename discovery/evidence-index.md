# Evidence Index

Every finding in this discovery maps to a repository artefact. Paths are repository-relative; `:n` is a verified line anchor; commit hashes are verified against `git log --all`. Claims that are inferences are marked `[INFERRED]` in their report file as well as here.

**Verification method, in order of strength:** (1) git object inspection (`git show`, `git diff`, `git ls-tree`), (2) file/line reads, (3) static import-closure analysis over `import` blocks, (4) recursive counting and disk measurement, (5) project documents (treated as claims, cross-checked against code wherever the code was reachable). No build, test, migration or write command was run against the repository.

---

## A. Inventory and stack

| ID | Finding | File / commit | Line | Conf. |
| :--- | :--- | :--- | :--- | :--- |
| INV-1 | Repository is a Go desktop accounting app, not the .NET app the README describes | `README.md` vs `fingo/go.mod:1` | `README.md:1-10` | HIGH |
| INV-2 | Application module declares `go 1.26.0`; two spike modules declare 1.27.0 | `fingo/go.mod`, `spike/editable/go.mod`, `spike/walkui/go.mod` | `fingo/go.mod:3` | HIGH |
| INV-3 | 7 direct dependencies: mysql driver, uuid, walk, goose, decimal, x/sys, x/text | `fingo/go.mod` | 5-12 | HIGH |
| INV-4 | 285 Go files / 64,116 LOC; 30,097 test LOC (46.9%) | whole tree | — | HIGH |
| INV-5 | Only 36 Go files / 2,102 LOC are committed at HEAD | `git ls-tree -r HEAD` | — | HIGH |
| INV-6 | 730 files, 235.1 MB excluding `.git`; `research/` is 118.9 MB, `dist/` 47.8 MB, `spike/` 32.7 MB | disk measure | — | HIGH |
| INV-7 | Production entry point is `cmd/fingo` (691 lines); `cmd/webui` is a harness | `fingo/cmd/fingo/main.go`, `fingo/cmd/webui/main.go` | — | HIGH |
| INV-8 | One-command build: rsrc manifest → `go build -H windowsgui -s -w` → bundle MariaDB | `fingo/build.ps1`, `fingo/build.cmd` | `build.ps1:130-190` | HIGH |
| INV-9 | Manifest generation is mandatory or the binary will not load | `fingo/cmd/fingo/main.go` | 9-17 | HIGH |
| INV-10 | Bundled MariaDB version probe list: 12.3, 11.4, 10.11 | `fingo/build.ps1` | 63-67 | HIGH |
| INV-11 | sqlc v2 config, mysql engine, schema = `db/migrations` | `fingo/sqlc.yaml` | 1-10 | HIGH |
| INV-12 | Triggers are applied outside goose because sqlc cannot parse `CREATE TRIGGER` | `fingo/db/migrate.go` | 41-52 | HIGH |
| INV-13 | No CI, no linter config, no installer, no Makefile/Dockerfile | tree scan; `docs/testing-strategy.md` | `testing-strategy.md:21-22` | HIGH |
| INV-14 | `lxn/walk` last commit 2021-01-12, ~345 open issues | `docs/plan-native-shell.md` | 348 | HIGH |
| INV-15 | Dev env file has DB host/port/user/password/name/driver/params | `fingo/.env` | 2-7, 18 | HIGH |
| INV-16 | `fingo/.env` is gitignored and untracked | root `.gitignore`; `git ls-files` | `.gitignore:7` | HIGH |
| INV-17 | A dev `user:password@tcp(...)` DSN is printed in a tracked document | `GEMINI.md` | 75 | HIGH |
| INV-18 | DB password stored plaintext at `%APPDATA%\FinGo\config.json`, mode 0600 | `docs/adr/0005-connection-settings.md`; `internal/appconfig/appconfig.go` | `0005:27-29`; `appconfig.go:10-20,290` | HIGH |
| INV-19 | Bundled DB runs with no password, loopback only | `internal/boot/live.go` | 26-28 | HIGH |
| INV-20 | Files are valid UTF-8 with real Vietnamese codepoints; no encoding defect | `db/migrations/00001_init_fingo_schema.sql`, `internal/ui/nav/rail.go` | U+1EC7 observed | HIGH |
| INV-21 | Only 3 stray `.cs` files remain (Win32 P/Invoke spikes) | `spike/click-row.cs`, `drive-form.cs`, `panes.cs` | — | HIGH |
| INV-22 | `dist/FinGo.exe` = 12,041,728 bytes, built 2026-10-12 21:31 | `fingo/dist/FinGo.exe` | mtime | HIGH |
| INV-23 | `dist/server` = 73 files, 36.3 MB; `dist/` total 47.8 MB | disk measure | — | HIGH |

## B. Business and features

| ID | Finding | File / commit | Line | Conf. |
| :--- | :--- | :--- | :--- | :--- |
| BIZ-1 | Product = one accountant serving N SME clients, on-premise Windows | `docs/requirements-v1.md`; `GEMINI.md` | `requirements-v1.md:1-6,57-70`; `GEMINI.md:46-54` | HIGH |
| BIZ-2 | Only actor is the operator; identity is `%USERNAME%` / `"ketoan"` | `fingo/cmd/fingo/main.go` | 136-147 | HIGH |
| BIZ-3 | 18 transaction events defined as the business core | `docs/requirements-v1.md` | 78-98 | HIGH |
| BIZ-4 | 13 testable acceptance criteria for production readiness | `docs/requirements-v1.md` | 133-147 | HIGH |
| BIZ-5 | Journal line model = one account, one side, one amount | `docs/adr/0001-journal-line-model.md`; `db/migrations/00002_journal_line_side.sql` | ADR 1; SQL 15-24 | HIGH |
| BIZ-6 | Posting is one-way; corrections are reversing entries | `docs/adr/0002-posting-lifecycle.md`; `internal/domain/gl/gl.go` | `0002:7-11` | HIGH |
| BIZ-7 | Sales posts Dr 131 / Cr 5111 / Cr 33311 and COGS Dr 632 / Cr 156 | `internal/posting/posting.go` | 120-142 | HIGH |
| BIZ-8 | Purchase postings use configured accounts (331, 1331, 156/expense), not posting rules | `internal/trading/purchase.go` | 221-238 | HIGH |
| BIZ-9 | VAT computed in basis points, per line, half away from zero, big.Int | `internal/vat/vat.go` | 147-180 | HIGH |
| BIZ-10 | Money allocation is largest-remainder so parts sum to total | `internal/money/allocate.go` | 9-21 | HIGH |
| BIZ-11 | Close carries P&L through 911 into 4212; P&L must end at zero | `internal/close/close.go`; `internal/close/close_test.go` | `close.go:351`; test 394-457 | HIGH |
| BIZ-12 | Provisions: straight-line depreciation + bad-debt age bands, folded into one voucher | `internal/close/provisions.go` | 1-40 | HIGH |
| BIZ-13 | Statutory output emits **no mã số**, warnings instead | `internal/reports/statutory/statutory.go` | 18-31, 96-118 | HIGH |
| BIZ-14 | Report rows always carry source voucher count; no cached balances | `GEMINI.md`; `internal/reports/reports.go` | `GEMINI.md:133` | HIGH |
| BIZ-15 | 17 rail destinations declared; exactly 5 openable | `internal/ui/nav/rail.go`; `cmd/fingo/main.go` | `rail.go:69-113`; `main.go:426` | HIGH |
| BIZ-16 | 12 unavailable entries each carry a Vietnamese "not built" reason | `internal/ui/nav/rail.go` | 75-112 | HIGH |
| BIZ-17 | No e-invoice, bank, tax-submission, Excel, or print integration exists | `[NOT FOUND]`; rail entries | `rail.go:79-92` | HIGH |
| BIZ-18 | Landed cost, purchase returns, stocktake, warehouse transfer, credit limit: no code | `[NOT FOUND]`; `plan-native-shell.md` | 292-294 | HIGH |
| BIZ-19 | Payroll arithmetic wrong on 3 grounds (KPCĐ, base, caps) | `docs/legal-basis.md`; `docs/accounting-review.md` | `legal-basis.md:279-286` | HIGH |
| BIZ-20 | PDF writer is hand-rolled and transliterates Vietnamese | `internal/reports/statutory/pdf.go` | — | HIGH |
| BIZ-21 | 01/GTGT, 03/TNDN, B01, B02 implemented; B03 not | `internal/reports/statutory/`; `plan-native-shell.md` | 306 | HIGH |

## C. Architecture and data

| ID | Finding | File / commit | Line | Conf. |
| :--- | :--- | :--- | :--- | :--- |
| ARC-1 | Onion/modular-monolith layering declared | `GEMINI.md` | 50-54 | HIGH |
| ARC-2 | Business logic lives in Go, not SQL; no procedures/views/jobs | `[NOT FOUND]` in SQL; `db/migrations/*` only `CREATE TABLE/ALTER` | — | HIGH |
| ARC-3 | 12 triggers enforce immutability and allocation rules | `fingo/db/triggers/10..51` | — | HIGH |
| ARC-4 | 19 tables across 15 migrations | `fingo/db/migrations/` | — | HIGH |
| ARC-5 | `voucher_lines` reshaped to one side per row | `db/migrations/00002_journal_line_side.sql` | 15-24 | HIGH |
| ARC-6 | Audit columns added; no users table, no audit-log table | `db/migrations/00003_voucher_audit_columns.sql` | 1-16 | HIGH |
| ARC-7 | 18 tables get nullable `client_id`; `clients` table created | `db/migrations/00015_client_scope.sql` | 36 ALTERs; 18 tables | HIGH |
| ARC-8 | `client.Scope` has zero call sites outside `internal/client` | grep across all `.go` | — | HIGH |
| ARC-9 | ADR 0003 explicitly supersedes ADR 0001's tenant clause and requirements §1.1 | `docs/adr/0003-client-data-boundary.md` | 5 | HIGH |
| ARC-10 | Composite natural keys deferred | `docs/adr/0003-client-data-boundary.md` | 44 | HIGH |
| ARC-11 | 30 packages unreachable from `cmd/fingo` (incl. trading, vat, posting, statutory, migrate, client) | static import closure | — | HIGH |
| ARC-12 | Two migration runners with different bookkeeping tables; `internal/migrate` is dead | `db/migrate.go`; `internal/migrate/bookkeeping.go` | `bookkeeping.go:30-39` | HIGH |
| ARC-13 | Chart of accounts seeded from Go (123 accounts), not a migration | `internal/domain/gl/coa_tt133.go` | 65-67, 216 | HIGH |
| ARC-14 | VAT rates live in memory; no table | `GEMINI.md` | 119 | HIGH |
| ARC-15 | No authentication, authorization, session, or password hashing | `[NOT FOUND]`; `cmd/fingo/main.go` | 136-140 | HIGH |
| ARC-16 | `client.Middleware` exists but nothing mounts it | `internal/client/middleware.go`; ADR 0003 | `0003:62` | HIGH |
| ARC-17 | Config precedence: saved settings beat `.env`; unreadable settings is an error | `docs/adr/0005-connection-settings.md` | 13-33 | HIGH |
| ARC-18 | `%APPDATA%` (ADR 0005) vs `%LOCALAPPDATA%` (native app) inconsistency | `docs/adr/0005-connection-settings.md`; `cmd/fingo/main.go` | `0005:15`; `main.go:672-686` | HIGH |
| ARC-19 | `DB_DRIVER` is dead configuration | `docs/adr/0005-connection-settings.md`; `fingo/.env:7` | `0005:78-84` | HIGH |
| ARC-20 | Startup order: data dir → server → DB → migrations → chart | `internal/boot/boot.go` | 7-11, 62-104 | HIGH |
| ARC-21 | DB location pinned to UTC; local-time connection shifts dates by a day | `fingo/.env` comment | 8-18 | HIGH |
| ARC-22 | `books` assembly: one source satisfies three report ports | `internal/books/books.go` | 43-65 | HIGH |
| ARC-23 | Goose keeps package-level state; `db.Up` not concurrency-safe | `fingo/db/migrate.go` | 23-26 | HIGH |
| ARC-24 | Two generations of duplicated domain models (money, partner, document, inventory, period, close, sales) | `GEMINI.md` | 100-111 | HIGH |
| ARC-25 | `openable{...}` literal in `main` is the real definition of "done" | `cmd/fingo/main.go` | 426 | HIGH |

## D. Pending work

| ID | Finding | File / commit | Line | Conf. |
| :--- | :--- | :--- | :--- | :--- |
| PEND-1 | 96% of Go code uncommitted; 99 untracked `fingo/` entries, 34 `docs/` | `git status --porcelain` | — | HIGH |
| PEND-2 | `origin/main` identical to `HEAD`; nothing pushed | `git rev-list --left-right --count` | `0 0` | HIGH |
| PEND-3 | 15 of 16 documents in `docs/` untracked; only the stale .NET smoke-test script is tracked | `git ls-files docs`; `git status` | — | HIGH |
| PEND-4 | No stash; two dangling stash-like commits (`50d820f`, `f7379d8`) | `git log --all --graph` | — | HIGH |
| PEND-5 | Zero TODO/FIXME/HACK/XXX markers in Go source | word-boundary grep | — | HIGH |
| PEND-6 | 18 `t.Skip` sites, all database-gated | grep `t.Skip` | — | HIGH |
| PEND-7 | 848 test functions, 118 test files | grep `^func Test` | — | HIGH |
| PEND-8 | Debug `TRACE` print in shipped read path | `cmd/fingo/main.go` | 351 | HIGH |
| PEND-9 | Stray coverage file `fingo/$cov` (21,961 bytes) | file | — | HIGH |
| PEND-10 | 44 commits, one author (`VAS Accounting <dev@accounting.local>`), 3 commit-days | `git log --all` | — | HIGH |
| PEND-11 | 296 files / 33,621 lines deleted in a single "reset" commit | `689b20b` | — | HIGH |
| PEND-12 | Next.js added then reverted one commit later | `f204026`, `26e968e` | — | HIGH |
| PEND-13 | Abandoned branch: 25 commits, 130 added files, last 2026-09-20 20:59 | `backup-before-alpha-rollback` = `f04a09c` | — | HIGH |
| PEND-14 | Branch migrations `00002..00013` collide by number with the working tree's | `00002_create_company_profile.sql` vs `00002_journal_line_side.sql` | — | HIGH |
| PEND-15 | `docs/BRD-*` and `SPEC-*` exist only on the abandoned branch | `git diff --name-status main backup-…` | — | HIGH |
| PEND-16 | Last code change 2026-10-12 21:30; no commit since 2026-09-20 | mtimes; `git log` | — | HIGH |
| PEND-17 | 18-day quiet period between last commit and the document burst | commit dates | — | HIGH |
| PEND-18 | 5.1–5.13 acceptance criteria: 2 done, 6 partial, 5 not met (4 absent, 1 broken) | `docs/requirements-v1.md` vs code | report 04 §4 | HIGH |
| PEND-19 | 0 of 18 business events usable end-to-end; 12 have engine code, 4 none, 2 stubs | `docs/requirements-v1.md:78-98` vs import closure | — | HIGH |
| PEND-20 | Half-done tenant migration, journal migration, migration runners, UI platform | report 04 §5 | — | HIGH |
| PEND-21 | Root README claims features that do not exist (FIFO, PIT, B01-B03, HTKK, reversals) | `README.md` | 60-92 | HIGH |
| PEND-22 | Stall hypothesis: legal reading wall + uncommitted work + four UI pivots | `[INFERRED]` from PEND-10..17 and `legal-basis.md:131-163` | — | MEDIUM |
| PEND-23 | "This is UI direction number four" is stated by the project itself | `docs/plan-native-shell.md` | 354 | HIGH |
| PEND-24 | P0 verified done 2026-10-11 by running the app (12.6 s provisioning, 123 accounts) | `docs/plan-native-shell.md` | 232-251 | HIGH |

## E. Build, run, deploy

| ID | Finding | File / commit | Line | Conf. |
| :--- | :--- | :--- | :--- | :--- |
| BLD-1 | Current build path is `build.cmd` → `build.ps1` | `fingo/build.cmd`, `fingo/build.ps1` | — | HIGH |
| BLD-2 | `GEMINI.md` still instructs `wails dev` / `wails build` (deleted tool) | `GEMINI.md` | 78-82 | HIGH |
| BLD-3 | `fingo/README.md` is the untouched Wails template | `fingo/README.md` | 1-19 | HIGH |
| BLD-4 | Root README instructs `dotnet build Accounting.slnx` | `README.md` | 75-84 | HIGH |
| BLD-5 | `rsrc` is a required, unpinned, `go install` prerequisite; absent here | `fingo/build.ps1`; PATH check | 132-134 | HIGH |
| BLD-6 | MariaDB install discovery is a hard-coded candidate list | `fingo/build.ps1` | 56-72 | HIGH |
| BLD-7 | Server bundle assembled by explicit file names | `fingo/build.ps1` | 150-190 | HIGH |
| BLD-8 | `sqlc` and `goose` CLIs are named in docs but absent from PATH here | `GEMINI.md`; PATH check | 82-84 | HIGH |
| BLD-9 | No installer; NSIS template is Wails' and unusable | `fingo/build/windows/installer/project.nsi`; `plan-native-shell.md` | 322 | HIGH |
| BLD-10 | Elevation is `asInvoker`; writes only under the user profile | `fingo/cmd/fingo/app.manifest` | — | HIGH |
| BLD-11 | Install root overridable via `FINGO_ROOT`; default `%LOCALAPPDATA%\FinGo` | `cmd/fingo/main.go` | 672-686 | HIGH |
| BLD-12 | DB server is a child process killed by a Windows job object | `internal/dbhost`; `plan-native-shell.md` | 237 | HIGH |
| BLD-13 | `REQUIRE_DB=1` turns a missing database into a failure | `internal/adapter/mariadb/testsupport/testsupport.go` | 99-120 | HIGH |
| BLD-14 | 81 database tests can silently skip without `.env` | `docs/testing-strategy.md` | 16-20, 495-524 | MEDIUM (count predates current suite) |
| BLD-15 | `web/e2e_test.go` is the only end-to-end suite, via `httptest` | `fingo/web/e2e_test.go` | — | HIGH |
| BLD-16 | `shell.go`/`voucher.go`/`table.go` have no tests (walk has no harness) | `internal/ui/shell/` | — | HIGH |
| BLD-17 | No CI, no golangci config, no coverage gate | tree scan; `testing-strategy.md` | 21-22 | HIGH |
| BLD-18 | Build likely works in-place; impossible from a fresh clone | `dist/FinGo.exe` mtime vs PEND-1 | — | HIGH |
| BLD-19 | Locale/timezone sensitive: all repo dates are 2026 | commits; `legal-basis.md:3` | — | HIGH |

## F. Risk, security, debt

| ID | Finding | File / commit | Line | Conf. |
| :--- | :--- | :--- | :--- | :--- |
| RSK-1 | Uncommitted/unpushed work: single point of total loss | PEND-1, PEND-2 | — | HIGH |
| RSK-2 | Parallel implementation with colliding migrations | PEND-13, PEND-14 | — | HIGH |
| RSK-3 | No authentication | ARC-15 | — | HIGH |
| RSK-4 | No authorization / segregation of duties | `docs/accounting-review.md` | 100-101 | HIGH |
| RSK-5 | Client isolation not enforced; two competing mechanisms | ARC-8, ARC-9 | — | HIGH |
| RSK-6 | No scheduled backup; `backups/` empty; backup code unreachable | `internal/migrate/backup.go`; `backups/`; ARC-11 | — | HIGH |
| RSK-7 | Statutory output has no mã số | BIZ-13 | — | HIGH |
| RSK-8 | Circular 48/2019 expires ≤ 2026-12-31 | `docs/legal-basis.md` | 12-23 | HIGH |
| RSK-9 | Regime (TT133) assumed, not confirmed; four regimes coexist | `docs/legal-basis.md` | 35-51 | HIGH |
| RSK-10 | Payroll/PIT/insurance not implemented and currently wrong | BIZ-19 | — | HIGH |
| RSK-11 | Bus factor 1; domain knowledge outside the repo | `git log`; `docs/legal-basis.md` | 288-298 | HIGH |
| RSK-12 | `walk` dormant; no accessibility bridge | `docs/plan-native-shell.md` | 348-349 | HIGH |
| RSK-13 | Duplicate domain generations / 30 dead packages | ARC-11, ARC-24 | — | HIGH |
| RSK-14 | Two migration runners; safer one dead | ARC-12 | — | HIGH |
| RSK-15 | SQL injection: **not found** — sqlc parameterised; no `fmt.Sprintf` into SQL | grep | — | HIGH |
| RSK-16 | Plaintext DB password, deliberate and documented | INV-18 | — | HIGH |
| RSK-17 | Dev credential printed in a tracked file | INV-17 | — | HIGH |
| RSK-18 | `walk` / bundled MariaDB licensing (GPLv2) not reviewed anywhere | `[INFERRED]`; `plan-native-shell.md` | 351 | MEDIUM |
| RSK-19 | Legal provenance (`research/`, 131 MB) untracked | `git status`; `legal-basis.md:296-298` | — | HIGH |
| RSK-20 | Stale review document still presented as current | `docs/accounting-review.md` | 8-22 | HIGH |

## G. Open questions

| ID | Question | Source of the question | Conf. |
| :--- | :--- | :--- | :--- |
| Q1 | Is the uncommitted tree the real codebase, and where must it land? | PEND-1, PEND-2 | HIGH |
| Q2 | Mine or delete `backup-before-alpha-rollback`? | PEND-13, PEND-14, PEND-15 | HIGH |
| Q3 | Commit discipline going forward? | PEND-10, PEND-16 | HIGH |
| Q4 | Real product or simulation (all dates 2026)? | BLD-19 | MEDIUM |
| Q5 | Database-per-client or `client_id` scope? | ARC-8, ARC-9; `requirements-v1.md:12-39` | HIGH |
| Q6 | Exactly one operator forever? | BIZ-2; ARC-15 | HIGH |
| Q7 | Only TT133 SMEs? | `requirements-v1.md:41-49`; `legal-basis.md:35-51` | HIGH |
| Q8 | Which fiscal year must it be live for? | `requirements-v1.md:51-53` | HIGH |
| Q9 | How many clients / vouchers? | `requirements-v1.md:170` | HIGH |
| Q10 | Any practice-level rollup? | `requirements-v1.md:165` | HIGH |
| Q11 | Handover format? | `requirements-v1.md:171` | HIGH |
| Q12 | Import purchases / non-VND currency? | `requirements-v1.md:169`; BIZ-18 | HIGH |
| Q13 | Who supplies the `mã số` from TT 89/2026? | BIZ-13; `legal-basis.md:133,163` | HIGH |
| Q14 | What replaces Circular 48/2019 after 2026-12-31? | RSK-8 | HIGH |
| Q15 | Confirm payroll figures (15.5M/6.2M, 5 brackets, employer rates)? | `legal-basis.md:192-286` | HIGH |
| Q16 | Is payroll / fixed assets / CCDC in v1? | BIZ-19; `requirements-v1.md:101-127` | HIGH |
| Q17 | Must statutory PDFs carry Vietnamese diacritics? | BIZ-20 | HIGH |
| Q18 | Is `lxn/walk` acceptable given dormancy and no accessibility? | RSK-12 | HIGH |
| Q19 | Is the HTML admin UI shipped or harness-only? | `docs/adr/0004:7,25,86`; `plan-native-shell.md:77-86` | HIGH |
| Q20 | Must the home screen enforce closing order? | `plan-native-shell.md:297-302` | HIGH |
| Q21 | Which MISA screens to match? | `docs/reference-misa-sme-flow.md`; `docs/pages.md` | HIGH |
| Q22 | Retire or promote `internal/domain/*`? | ARC-24 | HIGH |
| Q23 | Which migration runner survives? | ARC-12 | HIGH |
| Q24 | Backup cadence, retention, destination? | RSK-6 | HIGH |
| Q25 | Bundled-MariaDB GPLv2 distribution review? | RSK-18 | MEDIUM |
| Q26 | Pin `rsrc`, or commit the `.syso`? | BLD-5 | HIGH |
| Q27 | Where is the app version number? | `[NOT FOUND]` | HIGH |
| Q28 | Keep a dev credential in `GEMINI.md`? | INV-17 | HIGH |
| Q29 | `%APPDATA%` vs `%LOCALAPPDATA%` — intentional? | ARC-18 | HIGH |
| Q30 | Is `%USERNAME%` acceptable audit attribution? | BIZ-2 | HIGH |

## H. Document claims I could not verify read-only

| ID | Claim | Source | Why unresolved | Conf. |
| :--- | :--- | :--- | :--- | :--- |
| U1 | `go build`/`go vet`/`go test` currently pass | `docs/accounting-review.md:6`; `GEMINI.md:85` | Read-only brief forbids building/testing; integration tests create databases | — |
| U2 | Coverage percentages (e.g. "96.2%" for the singleton guard) | `docs/plan-native-shell.md:237` | The only profile is stale (`fingo/$cov`) | — |
| U3 | The shipped `FinGo.exe` opens the same 5 screens as the source | `dist/FinGo.exe` | Binary could predate a source edit by seconds; source map is the best evidence | — |
| U4 | Legal PDFs in `research/` are authentic gazette copies | `docs/legal-basis.md:296-298` | I did not open 49 MB of PDFs; the register rates its own sources `P`/`S`/`?` | — |
| U5 | Author intent for dangling `cline checkpoint` commits | `50d820f`, `f7379d8` | No message beyond the tool default | — |

---

## Cross-reference: report → finding IDs

| Report | Finding ID prefixes |
| :--- | :--- |
| `00-executive-summary.md` | Synthesises INV/BIZ/ARC/PEND/BLD/RSK; no new claims |
| `01-inventory-and-stack.md` | INV-* |
| `02-business-and-features.md` | BIZ-* |
| `03-architecture-and-data.md` | ARC-* |
| `04-pending-work.md` | PEND-* |
| `05-build-run-deploy.md` | BLD-* |
| `06-risks-and-debt.md` | RSK-* (register rows T1–T20 map to these) |
| `07-open-questions.md` | Q1–Q30, U1–U6 |
| `08-glossary.md` | Definitions, no new claims |

## Self-audit performed after writing

1. **Every quantitative claim re-derived** from the working tree in the same session (`git ls-tree`, `git ls-files`, `git rev-list`, line counts, disk measures).
2. **Every "not found" is a real negative**: searched for auth/hashing/session symbols, `CREATE PROCEDURE|VIEW|EVENT`, installer scripts, CI configs, `.sln`/`.csproj`, TODO markers, and Excel/CSV writers.
3. **No secret value reproduced.** The `.env` password, the `GEMINI.md:75` DSN value, and the bundled-DB credential appear only as file paths and secret types.
4. **Inferences labelled.** Two findings are `[INFERRED]`: the stall hypothesis (PEND-22) and the GPLv2 licensing consequence (RSK-18). One count is `MEDIUM` because it predates the current suite (BLD-14).
5. **A near-miss corrected:** Vietnamese text first appeared as mojibake in console output; direct byte inspection showed valid UTF-8, so no encoding defect was reported.
