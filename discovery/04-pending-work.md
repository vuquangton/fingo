# 04 — Pending Work

> This is the phase that matters. Everything below is measured from the repository, not from the project's own claims, and where the two disagree the repository wins.

## 1. The headline

The project is **not abandoned — it is uncommitted**. As of the working tree at 2026-10-12 21:30:

| Fact | Evidence | Confidence |
| :--- | :--- | :--- |
| Git `HEAD` is `f9193d3` (2026-09-20 15:56), a 72-file Wails skeleton. | `git log -1`; `git ls-tree -r HEAD` | HIGH |
| The working tree contains 285 Go files / 64,116 LOC. **96% of the code has never been committed** (2,102 LOC in 36 files are at HEAD). | `git ls-files`; LOC counts | HIGH |
| 15 of the 16 Markdown documents in `docs/` are untracked; only `docs/UIWorkflowSmokeTestScript.md` is committed, and it is a leftover from the .NET track. | `git ls-files docs` (1 file) vs `git status` (34 untracked `docs/` entries: 16 `.md` + 18 `.png`) | HIGH |
| `origin/main` is identical to `HEAD` (`0 0` ahead/behind), so the remote has **only** the skeleton too. | `git rev-list --left-right --count origin/main...main` | HIGH |
| No stash, no WIP commit, no backup branch of this work. | `git stash list` empty; `git for-each-ref refs/stash` empty | HIGH |
| Last source change: `fingo/cmd/fingo/main.go` and `internal/ui/journal/*`, 2026-10-12 21:30. | mtimes | HIGH |

**Risk: one `git checkout .` or one lost disk destroys three weeks of engine and UI work.** See [06-R1](06-risks-and-debt.md).

## 2. Unfinished-work markers

### 2.1 The usual markers are absent

| Marker | Hits in Go source | Note |
| :--- | ---: | :--- |
| `TODO` / `FIXME` / `HACK` / `XXX` / `WIP` in comments | **0** | word-boundary search across all 285 `.go` files |
| `panic("not implemented")` | **0** | — |
| `ErrNotSupported` / `ErrNotImplemented` | **0** | — |
| `t.Skip` call sites | 18 | all in database-backed tests, gated by `testsupport` |
| `unimplemented` | 1 | `internal/singleton/guard_other.go:10` — the non-Windows stub of the single-instance guard |
| Commented-out code lines (heuristic) | 16 | sampled; all read as prose comments, not disabled code |

This is unusual and worth stating plainly: **the codebase carries no TODO debt.** It carries *design* debt and *wiring* debt, and both are written down in prose in `GEMINI.md` and `docs/` rather than scattered as markers. [HIGH]

### 2.2 The real "unfinished" markers are runtime refusals

The product's own way of saying "not built" is a visited rail entry that refuses in Vietnamese:

```go
// internal/ui/nav/rail.go:54
func (e Entry) Open(a Available) error {
    if a != nil && a.CanOpen(e.ID) { return nil }
    return fmt.Errorf("%s chưa hỗ trợ — %s", e.Label, strings.TrimSpace(e.Reason))
}
```

`Rail()` declares **17 destinations**; the shell's `openable` map opens **5**:

| Rail section | Entry | Opens? | Declared reason if not |
| :--- | :--- | :---: | :--- |
| HÔM NAY | `today` | **yes** | — |
| NGHIỆP VỤ | `sales` | no | "chứng từ bán hàng, giá vốn và công nợ phải thu" |
| | `purchase` | no | "cần nối internal/trading: đơn mua, nhập kho và đối chiếu ba chiều" |
| | `cash` | no | "cần dựng internal/domain/cash" |
| | `bank` | no | "cần dựng internal/domain/cash và đối chiếu sổ phụ" |
| | `inventory` | no | "cần nối internal/inventory: nhập, xuất và điều chuyển" |
| | `tools` | no | "cần dựng internal/domain/asset và phân bổ TK 242" |
| | `assets` | no | "cần dựng internal/domain/asset và khấu hao hàng tháng" |
| | `payroll` | no | "cần dựng internal/domain/payroll: bảo hiểm, thuế TNCN và lương net" |
| | `einvoice` | no | "cần dựng internal/domain/tax và kết nối nhà cung cấp hóa đơn" |
| | `general` | **yes** (entry form) | — |
| SỔ & BÁO CÁO | `books` | **yes** (general journal) | — |
| | `reports` | **yes** (trial balance) | — |
| | `tax` | no | "01/GTGT và 03/TNDN — chờ chốt mã số chỉ tiêu" |
| DANH MỤC | `masterdata` | **yes** (chart of accounts) | — |
| HỆ THỐNG | `practice` | no | "cần nối internal/client: sổ khách hàng và chuyển đổi" |
| | `system` | no | "kỳ kế toán, khóa sổ, sao lưu và người dùng" |

Evidence: `internal/ui/nav/rail.go:69-113`; `cmd/fingo/main.go:426`. These 12 reasons are the most honest and the most useful backlog in the repository. [HIGH]

### 2.3 Developer leftovers

| Item | Location | Severity |
| :--- | :--- | :--- |
| `fmt.Fprintln(os.Stderr, "TRACE ledger loading", account)` — debug trace in the shipped read path | `cmd/fingo/main.go:351` | Low |
| `fingo/$cov` — 21,961-byte coverage profile written to a path that never interpolated | `fingo/$cov` | Low — also noted `docs/testing-strategy.md:23` |
| Two committed-adjacent spike binaries (`spike.exe` 14.8 MB, `spike2.exe` 8.5 MB, `spike-release.exe` 10.1 MB) | `spike/**` | Low (untracked, but part of the 235 MB footprint) |
| Stale `docs/UIWorkflowSmokeTestScript.md` from the .NET track — the only tracked doc | `docs/UIWorkflowSmokeTestScript.md` | Low |
| Dead config key `DB_DRIVER` | `fingo/.env:7`; ADR 0005 says so | Low |

## 3. Git history: what happened

### 3.1 Timeline

```mermaid
timeline
    title FinGo — three UI pivots in five days, then a quiet rebuild
    2026-09-18 : .NET 10 / WPF / EF Core track
               : 10 commits, 343 files added, incl. MISA journey test suite
    2026-09-19 : Next.js scaffold committed (f204026)
               : reverted one commit later (26e968e)
               : WPF SetupWizard, UTF-8 BOM fix
    2026-09-20 : "reset" deletes 296 files / 33,621 lines (689b20b)
               : Go + Wails scaffold (f9193d3)
               : 25 further commits on a side branch: BRDs, SPECs, RBAC,
                 subledgers (f04a09c, 20:59)
    2026-09-21 : README rewritten on the side branch
    2026-10-08 : requirements-v1, legal-basis, accounting-review, ADR 0001/0002
               : 154 files touched
    2026-10-09 : ADRs 0003/0004/0005, testing-strategy, migrations 00002-00015,
                 engine packages
    2026-10-10 : native shell spikes
    2026-10-11 : P0 walk shell (bundled MariaDB, single instance, first run)
    2026-10-12 : P1 screens: today, journal, trial balance, ledger, entry form
    2026-10-13 : nothing (only .codegraph index mtime)
```

| Metric | Value | Evidence |
| :--- | :--- | :--- |
| Commits, all refs | 44 (+2 dangling stash commits) | `git log --all` |
| Commits on `main` | 17 | `git log main --oneline` |
| First commit | 2026-09-18 14:xx `143c5af` | `git log --reverse` |
| Last commit | 2026-09-20 20:59 `f04a09c` (side branch) | `git log -1 backup-before-alpha-rollback` |
| Commit-days | 3 (2026-09-18: 10, 09-19: 5, 09-20: 29) | `git log --all --date=short` |
| Authors | **1**: `VAS Accounting <dev@accounting.local>` | `git log --pretty='%an <%ae>'` |
| Commit sizes | large: 19 files/5,888 insertions; 74 files/3,812; 343 files/31,808; deletion 296 files/33,621 | `--shortstat` |
| Longest gap | 2026-09-20 → 2026-10-08 (18 days, no commits) | — |
| Branches | `main` (`f9193d3`), `backup-before-alpha-rollback` (`f04a09c`) | `git branch -v -a` |
| Stashes | none (`refs/stash` absent) but **two dangling stash-like commits** exist: `50d820f` ("On master: cline checkpoint"), `f7379d8` ("index on master") | `git log --all --graph` |

### 3.2 The abandoned branch

`backup-before-alpha-rollback` = 25 commits **on top of** `f9193d3`, last one 2026-09-20 20:59. It adds **130 files** and modifies 21. [HIGH]

| What it adds | Examples |
| :--- | :--- |
| Requirements/spec documents | `docs/BRD-06/07/08`, `docs/SPEC-01..08`, `docs/TECHNICAL_STANDARDS.md`, `docs/TESTING_STRATEGY.md` |
| A **different** migration series | `00002_create_company_profile.sql` … `00013_create_sales_subledger.sql` |
| A repository layer with a different shape | `internal/adapter/mariadb/repository/{bank,cash,catalog,company_profile,opening,org_unit,purchase,sales,spine,system_option,user,voucher}_repo.go` |
| RBAC / SoD, company profile, branch/org-unit, opening balances, cash/bank/purchase/sales subledgers | commit subjects `8ff523b`, `c427de3`, `74590c9`, `9fad503`, `f04a09c`, `5355751` |

**The branch is unmerged and unreachable from `main`.** Its migration file `00002_create_company_profile.sql` collides by number with the working tree's `00002_journal_line_side.sql` (different content). Merging later is a manual reconciliation, not a merge. [HIGH — `git diff --name-status main backup-before-alpha-rollback`]

Critically: the branch's `docs/BRD-*` and `docs/SPEC-*` **do not exist in the working tree at all**. Requirement history for the RBAC/subledger work lives only in that commit range. [HIGH]

### 3.3 The deleted .NET application

`689b20b` "reset" removed 296 files / 33,621 lines: `Accounting.slnx`, `Directory.Build.props`, `Accounting.Application.csproj` (MediatR/CQRS), WPF views, EF Core context, xUnit tests. The root `README.md` that describes it **is still the repository's README and was never updated**. [HIGH — `git show --stat 689b20b`; `README.md`]

## 4. Requirements gap analysis

`docs/requirements-v1.md §5` states 13 testable acceptance criteria for "works perfectly in production". Assessed against the code:

| # | Requirement | Status | Evidence | Why |
| ---: | :--- | :--- | :--- | :--- |
| 5.1 | No partial postings | **DONE** | `postingrepo.Store.Save` (one transaction); `glrepo.SaveWithin` | — |
| 5.2 | No fractional đồng | **DONE** | `00002` `DECIMAL(19,0)`; `internal/money` int64; inventory float-ban test | — |
| 5.3 | Payments idempotent | **PARTIAL** | `internal/idempotency` + `00007`; but the package is unreachable and no payment screen exists | No HTTP server mounts the middleware |
| 5.4 | Client isolation structural | **BROKEN** | `client.Scope` has zero call sites outside its package; no auth; two tenant mechanisms | The stated guarantee is not enforced anywhere |
| 5.5 | Client switch < 1 s with 50 clients | **NOT IMPLEMENTED** | no registry adapter, no switching UI | — |
| 5.6 | Per-client audit trail | **PARTIAL** | `created_by`/`posted_by`/`posted_at` (`00003`); no audit-log table; identity is `%USERNAME%` | No users, no append-only log |
| 5.7 | Backup/restore proven per client | **PARTIAL** | `internal/migrate/{backup,restore}.go` + tests; `backups/` is empty; package unreachable | Backup exists as code, never runs |
| 5.8 | Handover export | **NOT IMPLEMENTED** | `[NOT FOUND]` | — |
| 5.9 | Migrations across N client DBs, dry-run, per-DB report | **NOT IMPLEMENTED** | `db.Up` migrates **one** database; `internal/migrate` is single-DB and dead | — |
| 5.10 | Deployable on a clean Windows machine | **PARTIAL** | `build.ps1` + bundled server verified by screenshots; no installer | `docs/plan-native-shell.md:243-246` |
| 5.11 | Deterministic close | **PARTIAL** | `close.Service` is retry-safe (`period-close:<id>`, snapshot before entries); **unwired** | `usecase/practice` closes months a different way |
| 5.12 | Errors recoverable and legible | **PARTIAL** | Vietnamese operator-facing messages in `cmd/fingo`; `TRACE` leftover; `web` renders errors | No error taxonomy/telemetry beyond `fingo.log` |
| 5.13 | Registry integrity degrades one client only | **NOT IMPLEMENTED** | `internal/client.Registry` is in-memory; no persistence | — |

### The 18 transaction events (`requirements-v1.md §3`)

| # | Event | Engine | Screen | Overall |
| ---: | :--- | :--- | :--- | :--- |
| 1 | Client onboarding + opening balances | profile: yes; opening balances: **stub** (`internal/domain/opening`) | first-run company form | **PARTIAL** |
| 2 | Cash purchase | yes (`trading.Purchasing`) | — | **PARTIAL** |
| 3 | Credit purchase | yes | — | **PARTIAL** |
| 4 | Import purchase + landed cost | **no** | — | **NOT IMPLEMENTED** |
| 5 | Purchase return | **no** (sales has `Credit`; purchase does not) | — | **NOT IMPLEMENTED** |
| 6 | Cash sale + VAT + COGS | yes | — | **PARTIAL** |
| 7 | Credit sale + VAT + COGS | yes | — | **PARTIAL** |
| 8 | Sales return | yes (`Credit`) | — | **PARTIAL** |
| 9 | Customer receipt + allocation | yes | — | **PARTIAL** |
| 10 | Supplier payment + allocation | yes | — | **PARTIAL** |
| 11 | Warehouse transfer | **no** | — | **NOT IMPLEMENTED** |
| 12 | Stocktake adjustment | **no** | — | **NOT IMPLEMENTED** |
| 13 | Expense payment | generic journal only | `general` | **PARTIAL** |
| 14 | Payroll (insurance, PIT) | **stub with wrong arithmetic** | — | **STUBBED** |
| 15 | Fixed asset + depreciation | **stub** | — | **STUBBED** |
| 16 | Period close | yes (`close`) | — | **PARTIAL** |
| 17 | Year-end provisions | yes (`close/provisions.go`) | — | **PARTIAL** |
| 18 | VAT declaration prep | 01/GTGT JSON **without mã số** | — | **PARTIAL** |

**Tally: 0 of 18 events are usable end-to-end by an operator. 12 have engine code. 4 have no code. 2 have stubs whose arithmetic the project's own review calls wrong.**

## 5. Half-done migrations and structural inconsistencies

| # | Half-done thing | Detail | Evidence |
| :--- | :--- | :--- | :--- |
| H1 | **Tenant boundary migration** | `client_id` added to 18 tables (00015) but no repository passes a `Scope`; `clients` table lives in the same per-client database it is meant to register | ADR 0003 "Not decided here"; migration 00015 |
| H2 | **Journal model migration** | `voucher_lines` reshaped by 00002, but `internal/domain/gl` and the older `internal/domain/{sales,purchase,report}` still carry pair-shaped concepts | `00002`; `GEMINI.md:100-111` |
| H3 | **Migration system split** | goose (`goose_db_version`) vs `internal/migrate` (`schema_migrations`), the latter dead | `GEMINI.md:116`; import closure |
| H4 | **UI platform migration** | Wails/Vue deleted in the working tree but still in `HEAD`; ADR 0004 says the HTML admin UI is the answer, `plan-native-shell.md` supersedes exactly that clause | `git status`; ADR 0004; `plan-native-shell.md:4-9` |
| H5 | **Domain generation split** | 11 `internal/domain/*` packages have no non-test importer; engine packages duplicate them | §2 of [03](03-architecture-and-data.md) |
| H6 | **Requirements vs ADR contradiction** | `requirements-v1.md §1.1` says "no `company_id` column"; ADR 0003 reverses it; the schema follows the ADR and the requirements file was never amended | `docs/requirements-v1.md:16-23` vs ADR 0003 |
| H7 | **Documented vs implemented README** | root README describes .NET/WPF/PostgreSQL and claims COA seeding, FIFO, PIT, B01/B02/B03, HTKK XML, reversals, audit trail | `README.md` vs [02](02-business-and-features.md) |
| H8 | **Two inventory semantics** | engine: integer unit average; old domain: exact total value — "opposite answers to the same rounding question" | `GEMINI.md:107` |
| H9 | **Statutory numbering unresolved** | The engine emits warnings instead of mã số for every line; `Q2`/`Q8` open | `internal/reports/statutory/statutory.go:18-31`; `docs/legal-basis.md:133-163` |

## 6. Why did the project stall? — hypothesis with evidence

I do **not** think it stalled. I think it is a **solo, high-discipline, high-context effort that repeatedly restarted its UI before committing anything**, and the repository state is the scar tissue. The evidence:

| Signal | Evidence | Reading |
| :--- | :--- | :--- |
| Four UI directions in five days (Next.js → WPF → Wails/Vue → walk) | `f204026` then `26e968e`; `689b20b`; `fingo/frontend` deleted; `plan-native-shell.md:354` ("This is UI direction **number four**") | Each pivot was made *before* the previous one was committed, so no pivot was ever paid for by a revert — which made pivoting cheap and therefore frequent |
| 296 files deleted in one commit, then 25 commits of a different design, then `main` rolled back to before them | `689b20b`; `backup-before-alpha-rollback`; branch name says "before alpha rollback" | A rollback decision was made *after* building a full BRD/SPEC/RBAC/subledger layer — the layer is preserved but abandoned |
| 18-day silence with no commits, then 154 files in one day | commit dates; mtimes | Work resumed in a new, document-first mode (`requirements-v1`, `legal-basis`, `accounting-review`) |
| The 18-day-quiet period produced the three best documents in the repo | `docs/legal-basis.md` is sourced to gazette PDFs; `docs/testing-strategy.md` correctly diagnoses the skip problem | The pause was productive: the second phase is far better grounded than the first |
| The newest phase is entirely uncommitted | `git status` | The commit habit was lost after the reset; **the process, not the code, is the pending item** |
| Legal blockage is real and self-imposed by the project's own rule | `docs/legal-basis.md:131-163` (Q2 mã số, Q8 appendices); P4 is explicitly "gated on mã số" (`plan-native-shell.md:304-307`) | The statutory phase **cannot** proceed without reading Phụ lục I–IV; the author refused to guess. That is a correctness win and a schedule wall |

**Stall hypothesis:** the project did not stop from technical failure. It stopped at a **decision boundary dressed as a legal one**: to ship statutory filing it needs the indicator numbering (`mã số`) from TT 89/2026 appendices plus the TT133 amended book forms, which the author could not read from primary text and refused to invent. Meanwhile the UI work it *could* do was moving fast and being committed nowhere, so the visible artifact (`HEAD`) looks like a three-week-old skeleton while the real system lives in an uncommitted directory.

Confidence: **MEDIUM**. It is an inference from commit topology, doc contents, and file mtimes — no commit message or note states "blocked". [INFERRED]

## 7. What a successor must do first (pending-work order)

1. **Commit the working tree** before touching anything (one commit, or a small series). This is the only irreversible risk in the repository.
2. **Decide the fate of `backup-before-alpha-rollback`** — mine it for `docs/BRD-*`/`SPEC-*` (unique content) or delete the branch. Do **not** merge its migrations.
3. Resolve the tenant question (H1) before adding another repository method.
4. Wire one vertical end-to-end (sales is closest: engine + model exist, only the screen and the rail flag are missing) to prove the flow.
5. Close `Q2`/`Q8` from the gazette appendices, or explicitly descope statutory filing from v1.
