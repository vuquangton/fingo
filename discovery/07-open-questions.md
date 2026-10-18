# 07 — Open Questions

> Questions only a human can close. Each carries **who** can answer it and **why it matters** (what work is blocked or wrongly built until it is answered). Grouped by urgency: 🔴 blocks the next slice · 🟠 blocks a phase · 🟡 shapes design · ⚪ hygiene.

---

## A. Repository and process (answerable today)

### 🔴 Q1. Is the uncommitted working tree the real codebase, and where should it land?
`HEAD` is a 72-file Wails skeleton; 285 Go files / ~62k LOC, all 14 migrations after `00001`, and 15 of the 16 documents in `docs/` exist only in the working tree (`git status`; [04 §1](04-pending-work.md)). Nothing is pushed.

- **Who could answer:** the repository owner / the only committer (`VAS Accounting <dev@accounting.local>`).
- **Why it matters:** this is the only *unrecoverable* risk in the project. A wrong `git checkout`, a disk failure, or an IDE clean wipes the engine and the native UI. Every other question is cheaper than this one. Also: is there a private backup outside this machine?

### 🔴 Q2. What is `backup-before-alpha-rollback` — a goldmine or a corpse?
The branch carries 25 commits / 130 files not on `main`, including `docs/BRD-06/07/08`, `docs/SPEC-01..08`, `docs/TECHNICAL_STANDARDS.md`, RBAC/SoD, company profile with tax-code check digit, branch/org-unit, and four subledgers (`git log backup-before-alpha-rollback`; [04 §3.2](04-pending-work.md)). Its migrations `00002..00013` **collide by number** with the working tree's different migrations.

- **Who could answer:** the author. A business owner cannot.
- **Why it matters:** (a) those BRD/SPEC documents are the only requirement record for a whole layer and exist nowhere else; (b) any attempt to merge the branch will silently produce a schema that is neither design. Until answered, the branch must be read-only and its SQL must not be merged.

### 🟠 Q3. What is the intended commit discipline going forward?
The repository has 44 commits in 3 days (2026-09-18…20) and **zero for the following three weeks of work**.

- **Who:** the author, or whoever now owns delivery.
- **Why:** it determines whether a new engineer may commit at all, whether `main` is a release branch, and whether the `GEMINI.md` "change control gate" (`GEMINI.md:8-24`) is a real process or a personal note.

### ⚪ Q4. Is this repository a training/simulation artefact or a product with a real customer?
Every date in the tree is 2026 (commits, "Last verified: 2026-10-08", legal instruments effective 2026-07-01), and the Go version declared is 1.26. The domain documents read as real practice requirements.

- **Who:** the project sponsor.
- **Why:** it changes the acceptable risk profile completely. If a real accounting practice will file real returns from this binary, T3–T6 are release blockers. If it is a construction exercise, they are architectural notes.

---

## B. Business model and tenancy

### 🔴 Q5. One database per client, or one database with a `client_id` column — which is the design?
Both are implemented. `requirements-v1.md §1.1` argues for database-per-client and **explicitly rejects** a `company_id` column; ADR 0003 reverses that and added `client_id` to 18 tables; the code enforces neither (`client.Scope` has zero call sites). See [03 §4.5](03-architecture-and-data.md).

- **Who:** the author + the accountant who will operate it. This is partly a deployment question (one MariaDB instance with N schemas vs N instances) that affects backup and handover.
- **Why:** every repository signature, every migration strategy, the backup design and the client-switching screen depend on it. `requirements-v1.md` open question 2 asks the same thing and is unanswered.

### 🔴 Q6. Is there exactly one operator, forever?
The current answer in code is "yes": identity is `%USERNAME%`, there is no login, no roles, no audit table (`cmd/fingo/main.go:136-147`; `[NOT FOUND]` for auth).

- **Who:** the practice owner.
- **Why:** if a second person ever touches the books, S1–S3 move from "acceptable trade-off" to "cannot ship". Also decides whether segregation of duties (creator ≠ poster) is required, which the old review called a High finding.

### 🟠 Q7. Do you serve only TT133 SMEs?
TT133/2016 (as amended by TT 46/2025) is the declared regime; TT 99/2025, TT 58/2026 (micro, no chart of accounts), TT 152/2025 (household) and TT 118/2026 (IFRS) all coexist. `AccountingStandard` is a free-form string read by nothing.

- **Who:** the practice owner, with the accountant.
- **Why:** determines whether the regime must become a per-client attribute from the first migration, and whether the statutory output set is one form family or four. `docs/requirements-v1.md:41-49` flags the retrofit cost.

### 🟠 Q8. Which fiscal year must the system be live for?
"Works perfectly in production" has no date. If it is FY2026 close, year-end provisions and the Circular 48 expiry are in v1 and race a statutory deadline.

- **Who:** the practice owner.
- **Why:** sizes v1. `docs/requirements-v1.md:51-53` and open question 4.

### 🟡 Q9. How many clients, and how many vouchers per client per month?
- **Who:** the accountant.
- **Why:** drives backup strategy, migration batching, report indexing, and whether the bundled single server is adequate. `requirements-v1.md` open question 6.

### 🟡 Q10. Is there any practice-level rollup across clients?
`requirements-v1.md` open question 1 is unanswered; database-per-client permanently removes cross-client reporting if the answer is "no shared database".

- **Who:** the practice owner.
- **Why:** decides whether the client registry stays administrative (no financial data) or must carry aggregates — which conflicts with the isolation model.

### 🟡 Q11. What exactly does a client receive on handover?
A database dump? A printable set of books? Both?
- **Who:** the practice owner / a successor accountant.
- **Why:** requirement 5.8 ("handover export") is entirely unimplemented, and `internal/migrate.Backup` produces a logical dump that may or may not be the right artefact. `requirements-v1.md` open question 7.

### 🟡 Q12. Any client with import purchases or non-VND currency?
`requirements-v1.md` transaction row 4 (import purchase with landed cost) is in v1 scope but has **no code**; landed cost and FX are absent.

- **Who:** the accountant.
- **Why:** landed cost is a High-severity accounting gap (inventory and COGS understated) and multi-currency is currently excluded but assumed present in row 4.

---

## C. Legal and statutory (the project's own blockers)

### 🔴 Q13. What are the `mã số chỉ tiêu` numbers for TT 89/2026 forms and the TT133 book forms?
The engine deliberately emits **no indicator numbers** — every statutory line carries a warning instead (`internal/reports/statutory/statutory.go:18-31`). `Q2` and `Q8` in `docs/legal-basis.md:133,163` record why: the appendices (Phụ lục I–IV) could not be read from a machine-readable official source.

- **Who:** a Vietnamese licensed tax accountant or tax agent with the gazette appendices; a Big-4/Vietnamese audit firm's technical desk.
- **Why:** statutory filing (rail entry `tax`, plan phase P4) is hard-blocked on it. The author's rule "never build on `?`" (`docs/legal-basis.md:288-294`) is correct and should not be relaxed — but it must be unblocked by a human with the documents.

### 🔴 Q14. What replaces Circular 48/2019 for provisions after 2026-12-31?
Decision 1760/QĐ-BTC keeps 45/2013 and 48/2019 in force "no later than 31/12/2026". The close engine implements the 48/2019 bands (`internal/close/provisions.go`).

- **Who:** tax adviser / the practice owner.
- **Why:** a statutory provision rule with an expiry date baked into the close. If unaddressed, the FY2027 close computes provisions under a repealed instrument.

### 🟠 Q15. Are the payroll figures — 15,500,000 / 6,200,000 deductions, 5 PIT brackets, employer 17%+3%+1%+0.5%+2% KPCĐ — the ones to implement?
`docs/legal-basis.md:192-286` records the current figures with confidence marks, and explicitly corrects the old review's 11M/4.4M/7-bracket claim. The code in `internal/domain/payroll` still applies the old, wrong arithmetic and is unimported.

- **Who:** a payroll/tax specialist.
- **Why:** decides whether payroll is in v1 at all. If it is, it must be rebuilt from the register, not patched.

### 🟠 Q16. Is payroll in v1? Are fixed assets? CCDC/TK 242?
All three are in the requirements' "in scope" list, are `STUBBED` in code, and are 3 of the 12 rail entries that cannot open.

- **Who:** the practice owner.
- **Why:** the honest v1 could be "trading cycle + close + trial balance" and ship far sooner. Cutting them is a product decision, not a technical one.

### 🟡 Q17. Should statutory PDFs transliterate Vietnamese, or must they carry real diacritics?
The PDF writer folds and transliterates because standard PDF base-14 fonts carry no Vietnamese glyphs (`internal/reports/statutory/pdf.go`).

- **Who:** the accountant (is the archive legally acceptable?) — and an engineer, for the embedding-a-font cost.
- **Why:** affects whether the PDF is an archive artefact or a filing artefact.

---

## D. Product and UX

### 🟠 Q18. Is `lxn/walk` acceptable, given it has been dormant since 2021 and has no accessibility (UIA) bridge?
Four UI directions have already been tried (Next.js → WPF → Wails/Vue → walk). The plan calls this out as its highest non-technical risk.

- **Who:** the sponsor, plus anyone with an accessibility requirement (a filing instrument may be procured under one).
- **Why:** the mitigation is "keep screens thin over tested models" — which is being done — but the decision should be *made*, not drifted into. Replacing it later is XL; replacing it now is L.

### 🟡 Q19. Does the operator need the HTML admin UI at all, or is it purely a test harness?
ADR 0004 makes server-rendered Go "the admin UI"; `plan-native-shell.md` supersedes exactly that clause and demotes `web/` to a harness. `cmd/webui` still exists.

- **Who:** the author/product owner.
- **Why:** two UIs means two wiring efforts. `web/e2e_test.go` is the only end-to-end suite and must be kept regardless.

### 🟡 Q20. What is the closing *order* the operator must follow, and must the UI enforce it?
`docs/plan-gui.md` and `plan-native-shell.md:297-302` argue the work-cycle home should become a **control** (kết chuyển disabled until depreciation and costing have run), modelled on MISA.

- **Who:** the accountant.
- **Why:** decides whether the home screen is a dashboard or a workflow gate — a significant design fork, and the stated reason the UI exists at all.

### ⚪ Q21. Which MISA screens should FinGo match, and which should it deliberately not?
`docs/reference-misa-sme-flow.md` and `docs/pages.md` are informed by MISA SME/AMIS.

- **Who:** the accountant who currently uses MISA.
- **Why:** the page catalogue is 492 lines of assumed IA; validating it with one operator is cheaper than building it.

---

## E. Engineering decisions still open in the docs

| # | Question | Who | Why it matters |
| ---: | :--- | :--- | :--- |
| Q22 | Is `internal/domain/*` being retired or promoted? 11 packages have no caller. | Author | Leaving both generations is the main maintenance tax ([06 T8](06-risks-and-debt.md)). |
| Q23 | Which migration runner survives: `db.Up` (goose) or `internal/migrate`? | Author | The safer one is dead; the live one has no backup and no per-client reporting. |
| Q24 | Backup cadence and retention — daily? on close? before every migration? Where do backups go? | Author + accountant | T4; `backups/` is empty and `internal/migrate` is unreachable. |
| Q25 | Does the bundled-MariaDB licence (GPLv2) permit the intended distribution, and who owns that review? | Sponsor/legal | 36 MB of GPL binaries ship beside a proprietary app ([03 §7](03-architecture-and-data.md)). |
| Q26 | Is `rsrc` allowed to be a `go install` prerequisite, or must the manifest be committed as a `.syso` with a pinned version? | Author/DevOps | Build fails on a clean machine today ([05 §1.2](05-build-run-deploy.md)). |
| Q27 | What is the app's version number, and where is it displayed? | Product | No version anywhere in the binary or the UI; needed for support and for E2E risk. |
| Q28 | Should `GEMINI.md:75` keep a development credential in a tracked file? | Author | Secret hygiene; see [01 §7](01-inventory-and-stack.md). |
| Q29 | Is the `%APPDATA%` vs `%LOCALAPPDATA%` root inconsistency intentional? ADR 0005 says `%APPDATA%`; the native app uses `%LOCALAPPDATA%`. | Author | Two locations for settings is a support trap. |
| Q30 | What does "posted_by" mean when there is no login — is `%USERNAME%` acceptable as an audit attribution? | Accountant/auditor | Requirement 5.6 (per-client audit trail) claims attribution; today it is a spoofable OS string. |

---

## F. Things I could not determine (recorded, not guessed)

| # | Unknown | Why it could not be resolved |
| ---: | :--- | :--- |
| U1 | Whether `go build ./...`, `go vet ./...`, `go test ./...` pass **right now** | Read-only brief forbids building/testing; integration tests create databases. The last artifact (`dist\FinGo.exe`, 2026-10-12 21:31) is one minute younger than the last source edit, which is strong but indirect evidence. |
| U2 | Actual test coverage | The only coverage file (`fingo/$cov`) is stale, and re-running coverage is a build/test side effect. |
| U3 | Whether the shipped binary opens the same 5 screens as the source implies | `dist\FinGo.exe` could be stale by one build; the `openable` map in source is the best available evidence. |
| U4 | Which client(s), if any, the author was targeting | No named client, no sample data set, no migration of real data appears anywhere. |
| U5 | The author's intent for the `cline checkpoint` dangling commits (`50d820f`, `f7379d8`) | They are unreachable stash-like commits with no message beyond the tool's default. |
| U6 | Whether `research/`'s legal PDFs are authentic/official | I did not open 49 MB of PDFs; `docs/legal-basis.md` records its own provenance limits and rates sources `P`/`S`/`?`. |
