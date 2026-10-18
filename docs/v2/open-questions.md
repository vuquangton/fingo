# FinGo v2 — open questions

> Rule: if a question blocks a single, reversible decision, take the documented default and continue. If it is irreversible, stop. Each row records the default actually taken, so the answer can later contradict a decision rather than a memory.

**Status legend:** OPEN · ANSWERED · DEFAULTED (a default was taken; revisit when answered).

| # | Question | Blocks | Default taken | Who can answer | Status |
| ---: | :--- | :--- | :--- | :--- | :--- |
| Q1 | **Which PostgreSQL instance is the real target?** Resolved: the system PostgreSQL 16.15 service on 5432. | Phase 1 gate (tests), Phase 2 (backup/service) | **ANSWERED** — the system service is used. `scripts/pgsetup.ps1` creates `fingo_dev`/`fingo_test` and the `fingo_app` role; credentials live in a gitignored `.env`. | whoever administers the server | ANSWERED |
| Q2 | **Production host: Windows service or Linux systemd?** Both install scripts are required by Phase 2, but which is the real deployment decides the installer notes, the certificate path and the backup destination. | Phase 2, Phase 6 | Build both; test the Windows path on this machine, ship the systemd unit untested with a note. | Practice owner / ops | DEFAULTED |
| Q3 | **Who supplies the statutory `mã số chỉ tiêu`?** `docs/legal-basis.md` Q2/Q8 are unresolved and the engine emits warnings instead of numbers. | Statutory filing — **out of v2 scope** | Filing stays out; warnings preserved verbatim. | Licensed tax practitioner with TT 89/2026 appendices | OPEN (known blocker, descoped) |
| Q4 | **Which regime(s) must v2 serve?** TT133 is assumed (chart transcribed, forms planned). TT 99/2025, TT 58/2026 and TT 152/2025 are not implemented. | Chart seed, report layout, form set | TT133/2016 as amended by TT 46/2025 only. | Practice owner | DEFAULTED |
| Q5 | **Is the keyboard flow on a Tabulator grid acceptable to a real operator?** | Phase 3 screens beyond the spike | Spike first (≤2 days); fallback = server-rendered rows with per-row forms, documented if the spike fails. | Accountant who will use it | OPEN (spike pending) |
| Q6 | **Confirm v2 scope cuts.** Payroll, fixed assets, CCDC/TK 242, e-invoice, bank reconciliation, B03, landed cost are out. | Phase planning | Out of v2; the engine stubs stay in the archive only. | Practice owner | DEFAULTED |
| Q7 | **How many users, and which roles?** Three roles are defined (`admin`, `accountant`, `viewer`) and creator≠poster is enforced. | Phase 2 | Three roles; creator≠poster for post and lock; `admin` may override with an audit row. | Practice owner | DEFAULTED |
| Q8 | **Client handover format.** A filtered `COPY` per client is the plan; is that what a successor accountant can actually receive? | Phase 6 | Per-client `COPY` export + a README describing the schema. | Practice owner / successor accountant | DEFAULTED |
| Q9 | **Backup retention and destination.** | Phase 2 | Daily `pg_dump -Fc` at 01:00, 30 days kept, under `%PROGRAMDATA%\FinGo\backups` (Windows) / `/var/backups/fingo` (Linux), mode 0600. | Practice owner / ops | DEFAULTED |
| Q10 | **May vendored front-end assets be fetched from the network at build time?** The target machine has no Node. If `Invoke-WebRequest` is blocked in this environment, vendoring must be done another way (hand-copied files or a committed archive). | Phase 1 and Phase 3 UI work | Fetch pinned versions from unpkg/jsDelivr **once**, commit the files and their `LICENSE`, then never fetch again. If fetching is blocked, report it as a Phase 1 blocker rather than silently shipping a UI with no CSS. | whoever controls the build network | OPEN |
| Q11 | **Is the decimal→int64 port of `internal/domain/gl` acceptable as the one "port unchanged" exception?** ADR 0008 requires it; it changes test literals in `gl`, `reports`, `close`, `posting`. | Phase 1 | Do it, list every changed test in `docs/v2/test-report.md`, keep every behavioural assertion. | Author / team lead | DEFAULTED (approved by mission's int64 rule) |
| Q12 | **Should `research/` PDFs be committed for legal provenance?** They are 49 MB and currently ignored. | Repository policy | Text and scripts committed; PDFs stay local, path recorded in `docs/legal-basis.md`. | Author | DEFAULTED |
| Q13 | **Does the 8% VAT window (NQ 204/2025) still apply on the go-live date, and does the 2026 regime change alter any rate in the seed?** | VAT defaults and statutory warnings | Rates come from the legal register; the 8% window ends 2026-12-31 and is a warning after that. | Tax practitioner | OPEN |
| Q14 | **Is exempting the `clients` registry from RLS acceptable?** It holds names and tax codes, no financial data, and is readable by every authenticated user. | Phase 1 migration | Exempt; document that it is a trusted, practice-level table. | Security reviewer / author | DEFAULTED |
| Q15 | **What is the app version number and where is it shown?** | Phase 2 (support), Phase 6 | `v2.0.0-dev` in the binary (`-ldflags`), shown in the footer and `/healthz`. | Product owner | DEFAULTED |
| Q16 | **Opening balances: import format and control-account reconciliation.** The v1 engine has `internal/domain/opening` only as a stub. | Phase 6 | Excel template with ten groups, each reconciled to its control account (131/331/156/…); refuse an unbalanced import. | Accountant | DEFAULTED |
| Q17 | **What happens to the 11 dead `internal/domain/*` packages and the abandoned branch's BRDs?** | Phase 1 cleanup | Packages dropped from `v2` (kept in `archive/pre-rewrite`); BRD/SPEC copied to `docs/archive/` as reference, not as commitments. | Author | DEFAULTED |
| Q18 | **Time zone for period boundaries.** v1 recorded UTC instants but dated documents by local day. | Reporting boundaries | Store `timestamptz` UTC; interpret document dates in `Asia/Ho_Chi_Minh` (UTC+7); one helper, tested. | Accountant / DBA | DEFAULTED |
| Q19 | **Does the practice need a combined view across clients?** (v1 open question 1.) RLS makes it a deliberate, privileged feature. | Cross-client reporting | Not in v2. | Practice owner | OPEN |
| Q20 | **Who owns the backup restore drill sign-off?** | Phase 2 gate | The drill is scripted and recorded in `docs/v2/restore-drill.md`; sign-off by whoever runs production. | Ops | OPEN |

## Resolved during Phase 0

| # | Question | Answer | Evidence |
| ---: | :--- | :--- | :--- |
| R1 | Is Go's built-in CSRF protection available? | Yes — `net/http.CrossOriginProtection` exists in the installed Go 1.27. `gorilla/csrf` is not needed. | `go doc net/http.CrossOriginProtection` |
| R2 | Is PostgreSQL 16 present? | Yes, 16.15, service running; `psql`/`pg_dump` at `C:\Program Files\PostgreSQL\16\bin`. | `psql --version`; `pg_dump --version` |
| R3 | Is Tabulator's licence acceptable? | MIT, version 6.5.0 current. | [Tabulator licence page](https://www.tabulator.info/docs/6.x/license/) |
| R4 | Does the v1 tree build and pass its tests before the port? | Recorded in `docs/v2/plan-status.md` §2. | baseline run |
