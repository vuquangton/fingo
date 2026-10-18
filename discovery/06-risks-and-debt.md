# 06 — Risks and Technical Debt

> Severity is business impact if the risk materialises. Effort is S (hours) / M (days) / L (1–2 weeks) / XL (2+ weeks) for **one experienced Go developer with the domain documents in hand**.

## 1. Code-quality hotspots

### 1.1 Largest files (Go)

| File | Lines | Assessment |
| :--- | ---: | :--- |
| `internal/close/provisions_test.go` | 1,406 | Test-only; appropriate for the riskiest arithmetic in the system |
| `internal/usecase/practice/client_integration_test.go` | 1,241 | Test-only; integration |
| **`internal/trading/purchase.go`** | **1,019** | **Non-test hotspot.** One file holds PO, goods receipt, three-way match, invoice posting, payment and allocation. Largest unit of production logic in the tree. |
| `internal/close/close_test.go` | 909 | test |
| `internal/reports/statutory/statutory_test.go` | 826 | test |
| `internal/trading/purchase_test.go` | 802 | test |
| **`internal/trading/sales.go`** | **799** | Invoice, payment, credit note in one type |
| **`internal/close/provisions.go`** | **768** | Depreciation + bad debt + chart validation |
| `internal/usecase/practice/client.go` | 718 | One client's books; overlapping with `books` and `close` |
| `internal/adapter/mariadb/sqlc/models.go` | 699 | generated |
| `cmd/fingo/main.go` | 691 | composition root + screens + provisioning; acceptable for a `main`, but it also carries the product's `openable` truth |
| `internal/migrate/backup.go` | 655 | logical dump writer |

No god class of 3,000 lines exists. The hot spots are **two cycle packages that each do four jobs** (`sales.go`, `purchase.go`) and **`close/provisions.go`**, which mixes two provision types in one file. [HIGH — line counts]

### 1.2 Duplication

| Duplication | Scale | Evidence |
| :--- | :--- | :--- |
| **Two generations of domain packages** with the same names and different semantics | 11 unreachable `internal/domain/*` packages vs their engine equivalents | [03 §2](03-architecture-and-data.md); `GEMINI.md:100-111` |
| **Two AR/AP models and two table families** (`documents`/`allocations` in 00009 vs `arap_*` in 00014) | ~10 tables | migrations 00009, 00014 |
| **Two partner models** (`partners` 00008 vs `trade_partners` 00013) | 2 tables, 2 repositories | migrations |
| **Two report implementations** (`internal/reports` live; `internal/domain/report` a hand-filled stub) | 1 package | `GEMINI.md:111` |
| **Two period-closing implementations** (`internal/close` vs `domain/closing` + `usecase/practice.ClosePeriod`) | 3 code paths | `GEMINI.md:109,130,146` |
| **Two migration runners** | `db.Up` and `internal/migrate.Runner` | `GEMINI.md:116` |
| **Two money regimes** (decimal đồng vs int64 minor units) | codebase-wide; conversion happens at the voucher | `GEMINI.md:64-66` |
| Circular/report restatements | `posting`, `reports`, `close` all define a `Periods`/`Journal` port | ports declared where used — a deliberate, healthy pattern |
| Stale README vs reality | 100 lines describing a deleted .NET app | `README.md` |

### 1.3 Naming and consistency

- Package names and Vietnamese domain terms are used consistently; comments are unusually good (they explain *why*, cite ADRs, and record bugs found). This is above the median legacy codebase by a wide margin. [HIGH]
- Inconsistent identifier language: `internal/domain/*` uses exported Go names (`Voucher`, `EntrySide`) while `internal/*` uses the same names with different shapes. A reader cannot tell from an import path which generation they are in. [HIGH]
- Two spellings of the same idea: `internal/document` (engine) vs `internal/domain/document`; `internal/inventory` vs `internal/domain/inventory`. [HIGH]

## 2. Security

| # | Finding | Severity | Evidence | Note |
| ---: | :--- | :--- | :--- | :--- |
| S1 | **No authentication anywhere.** No login, no password hash, no session. Anyone who launches the binary has full access to every client's books reachable from that machine. | **Critical** | `[NOT FOUND]`; `cmd/fingo/main.go:136-140` | Justified only by "single operator, her own machine" |
| S2 | **No authorization.** No role check on any mutating path; no segregation of duties. | **Critical** | `docs/accounting-review.md:100-101`; `internal/domain/system` unreachable | An accounting practice is professionally liable for its books |
| S3 | **Client boundary not enforced in code.** Two mechanisms exist, neither applied at the repository layer; `client.Middleware` is never mounted. | **Critical** | [03 §4.5](03-architecture-and-data.md) | Confirmed by zero `client.Scope` call sites |
| S4 | DB password stored in plaintext in `%APPDATA%\FinGo\config.json` | Medium | ADR 0005:20-27; `appconfig.go:10-20` | Deliberate and documented; DPAPI deferred |
| S5 | A dev `user:password@tcp(...)` DSN is quoted in a **tracked** file | Medium | `GEMINI.md:75` | Value intentionally not reproduced here; still a credential in git |
| S6 | Bundled MariaDB runs with **no password**, loopback only | Low | `internal/boot/live.go:26-28` | Threat: another local process/user on the same machine |
| S7 | No transport encryption (loopback only) | Low | same | — |
| S8 | SQL injection | **Not found** | sqlc-generated parameterised queries; a search for `fmt.Sprintf` into SQL returned nothing | Good |
| S9 | Log file may contain DB errors and paths | Low | `cmd/fingo/main.go:265-274`, mode `0600` | — |
| S10 | Dormant third-party GUI toolkit with no security backports | Medium | `lxn/walk`, last release 2021 | — |
| S11 | PDF/statutory output has no integrity/authenticity mark | Low | `internal/reports/statutory/pdf.go` | Filing authenticity is the taxpayer's signature/invoice system |

**Security verdict:** there is no application security model. There is a **process isolation model** (one Windows user, one machine) that is being asked to carry a multi-client confidentiality requirement. That mismatch is the repository's most serious non-code risk. [HIGH]

## 3. Data risks

| # | Risk | Severity | Evidence |
| ---: | :--- | :--- | :--- |
| D1 | **No scheduled backup.** Backup runs only before a migration, and that runner (`internal/migrate`) is unreachable from the binary. `backups/` is empty. | **Critical** | `internal/migrate/backup.go`; `backups/.` empty; import closure |
| D2 | **No automated migration across N client databases**, no dry run, no per-DB failure report — while the product's premise is N clients | High | `db.Up` migrates one DB; requirement 5.9 unimplemented |
| D3 | `client_id` nullable with no composite natural keys: two clients in one schema will collide on `accounts.code`, `vouchers.voucher_no` | High | ADR 0003 "Deferred"; migrations 00005, 00008 |
| D4 | No audit-log table; only `created_by/posted_by` columns. Who changed what, before/after, is unrecoverable | High | `00003`; requirement 5.6 |
| D5 | Identity is `%USERNAME%`, which is spoofable and not a person | Medium | `cmd/fingo/main.go:141-147` |
| D6 | Triggers live outside migrations and are re-applied every open; a broken trigger file breaks **startup**, not a migration | Medium | `db/migrate.go:41-70` |
| D7 | `goose` keeps package-level state; `db.Up` is documented as unsafe to call concurrently | Medium | `db/migrate.go:23-26` |
| D8 | Dual money regimes (decimal vs int64) invite a silent unit mix-up at a boundary | Medium | `GEMINI.md:64-66` |
| D9 | No index on `vouchers(voucher_date)` / `voucher_lines(voucher_id, line_order)` beyond FKs | Low | `docs/accounting-review.md:130` |
| D10 | A settings file that cannot be read refuses to start (correct) — but there is **no repair path in the UI**, only on disk | Low | ADR 0005 |
| D11 | Immutability is genuinely enforced in two layers (domain + triggers). This is a strength, not a risk. | — | triggers 10–22, 30–32, 40–41, 50–51 |

## 4. Operational risks

| # | Risk | Severity | Evidence |
| ---: | :--- | :--- | :--- |
| O1 | **Bus factor 1.** One author, one machine, no pushed code. Everything built after 2026-09-20 exists in one working tree. | **Critical** | `git log` authors; `git status`; [04 §1](04-pending-work.md) |
| O2 | **Work not committed for ~3 weeks** (last commit 2026-09-20; last code 2026-10-12) | **Critical** | same |
| O3 | The instructions a new engineer would follow (`GEMINI.md`) describe a build (`wails build`) that cannot work, because the tool was deleted | High | `GEMINI.md:78-82`; `fingo/frontend` gone |
| O4 | `walk` is dormant; there is no accessibility bridge, which matters for a filing instrument | High | `docs/plan-native-shell.md:348-349` |
| O5 | No CI means "green" is an opinion; database tests skip silently by default | High | `docs/testing-strategy.md:495-524` |
| O6 | No installer and no uninstall story beyond "delete the folder" | Medium | `plan-native-shell.md:243-246` |
| O7 | Bundled server brings antivirus false positives, port conflicts, ~48 MB distribution | Medium | `plan-native-shell.md:351` |
| O8 | Domain knowledge held outside the repo: which law applies, which form, which mã số. The repo's own rule is "do not build on `?`". | High | `docs/legal-basis.md:131-163,288-298` |
| O9 | `research/` (131 MB of legal PDFs and ad-hoc Python scrapers) is the provenance for every rate and is **untracked** | High | `docs/legal-basis.md:296-298` |
| O10 | Dead code volume (30 unreachable packages) makes every grep and onboarding harder and misleads readers into thinking features exist | Medium | [03 §2](03-architecture-and-data.md) |
| O11 | The statutory PDF transliterates Vietnamese because standard PDF fonts lack glyphs — legally readable but not faithful | Medium | `internal/reports/statutory/pdf.go` |

## 5. Legal / compliance risks (business-level)

| # | Risk | Severity | Evidence |
| ---: | :--- | :--- | :--- |
| L1 | **Circular 48/2019 expires ≤ 2026-12-31** — the provisioning rules the close engine implements are on a statutory countdown | **Critical** | `docs/legal-basis.md:12-23` |
| L2 | **No mã số chỉ tiêu on any statutory line** — the returns are generated without the indicator numbers a filing requires | **Critical** for filing | `internal/reports/statutory/statutory.go:18-31`; `docs/legal-basis.md:133,163` |
| L3 | The targeted regime (TT133 as amended by TT 46/2025) is assumed, not confirmed; four regimes now coexist | High | `docs/legal-basis.md:35-51` |
| L4 | Payroll arithmetic is wrong on three independent grounds (KPCĐ unmodelled, base = gross instead of insurable salary, caps absent) | High if payroll is in scope | `docs/legal-basis.md:279-286`; `docs/accounting-review.md:77` |
| L5 | The accounting review that leaders may consult is **partly stale**: its legal basis was superseded, and several engineering findings are now fixed | Medium | `docs/accounting-review.md:8-22` (its own warning); migrations 00002/00003 |

## 6. Technical debt register

| ID | Debt | Severity | Effort | Evidence | Recommended action |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **T1** | 96% of the codebase uncommitted; no remote copy; no backup | **Critical** | **S** | [04 §1](04-pending-work.md) | Commit the working tree today, in coherent slices; push. Add a pre-commit `build+vet+test` gate |
| **T2** | Two divergent implementations with **colliding migration numbers** (`main` vs `backup-before-alpha-rollback`) | **Critical** | **L** | `git diff main backup-before-alpha-rollback` (130 A / 21 M) | Decide: mine the branch for `docs/BRD-*`/`SPEC-*`, then delete it. Never merge the SQL |
| **T3** | No authentication or authorization | **Critical** | **L** | S1, S2 | Minimum: OS-user identity + append-only audit; then a real login if >1 operator |
| **T4** | No scheduled backup / no restore drill | **Critical** | **M** | D1; `internal/migrate` dead | Wire `migrate.Backup` on close-of-day; test a restore into a clean install |
| **T5** | Client isolation unenforced (two mechanisms, zero call sites) | **Critical** | **L** | S3, D3, ADR 0003 | Choose one mechanism; make repository signatures require it; add a test that no unscoped query compiles |
| **T6** | Statutory output has no mã số; TT133/TT99/58 regime undecided | **Critical** | **L** | L2, L3 | Read Phụ lục I–IV from the gazette; or descope filing from v1 explicitly |
| **T7** | 30 unreachable packages incl. the whole sales/purchase/statutory engine | High | **M** | [03 §2](03-architecture-and-data.md) | Wire `trading` to a screen (sales first) or move to `internal/experimental` to stop misleading readers |
| **T8** | Duplicate domain generations (11 packages, 5 concepts) | High | **L** | §1.2 | Retire `internal/domain/{asset,cash,catalog,closing,costing,opening,payroll,purchase,report,sales,system}` or promote them; do not leave both |
| **T9** | Two migration runners; the safer one is dead | High | **M** | D2, `GEMINI.md:116` | One runner: backup → apply → verify → per-DB report |
| **T10** | Payroll/PIT/insurance wrong and unimported | High (if in v1) | **M** | L4 | Implement from `docs/legal-basis.md` figures, or cut payroll from v1 |
| **T11** | No CI; DB tests skip silently | High | **S** | O5 | Adopt the CI in `docs/testing-strategy.md:349-440`; set `REQUIRE_DB=1` in the DB job |
| **T12** | `walk` dormant, no accessibility bridge | High | **XL** to replace | O4 | Accept and contain (thin screens over models); revisit if WCAG is a requirement |
| **T13** | Root README and `fingo/README.md` describe software that does not exist | Medium | **S** | O3; `README.md` | Rewrite README from `GEMINI.md`'s verified state; delete the Wails template |
| **T14** | Legal provenance (131 MB `research/`) untracked | Medium | **S** | O9 | Commit the extracted text and the extraction scripts (not the PDFs) |
| **T15** | No installer / no versioning of the app | Medium | **M** | O6 | Inno Setup or portable ZIP + a version stamp in the shell header |
| **T16** | Chart seeded from Go, not a migration — reference data has no version history | Medium | **S** | `internal/domain/gl/coa_tt133.go` | Seed via a versioned migration, or version `TT133Chart()` explicitly |
| **T17** | Debug `TRACE` print in the shipped read path | Low | **S** | `cmd/fingo/main.go:351` | Remove or move behind a debug flag |
| **T18** | Two hot-spot files do four jobs each (`trading/sales.go`, `trading/purchase.go`) | Medium | **M** | §1.1 | Split invoice/payment/allocation into separate types within the package |
| **T19** | Missing report indexes at volume | Low | **S** | D9 | Add when profiling shows it; do not guess now |
| **T20** | Stale review document that a reader will trust | Medium | **S** | L5 | Add a dated "superseded by" banner with a per-finding status column |

### Debt totals

| Severity | Count |
| :--- | ---: |
| Critical | 6 (T1–T6, plus O1/O2/S1/S2/S3/L1/L2 at risk level) |
| High | 6 |
| Medium | 7 |
| Low | 2 |
| **Cheapest high-value action** | **T1: commit and push.** One hour of work removes the only *unrecoverable* risk in the repository. |
