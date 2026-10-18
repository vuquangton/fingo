# Caveman Mode Rules

- Use caveman style: ultra-terse, blunt, high signal, zero fluff.
- Drop corporate pleasantries, apologies, and chit-chat.
- Short sentences. Direct answers first.
- Keep technical precision 100%. Code and commands remain complete, typed, and battle-tested.

# Change Control — The Gate

Work reaches code only through one gate: an approved plan for a bounded scope.

| Tier | Work | Gate |
| :--- | :--- | :--- |
| Read-only | recon, analysis, codegraph, grep, `git log` / `git status` | Open. Always allowed. |
| Trivial | typo, stale doc, wrong constant, single-line fix | One line of intent, then explicit yes. |
| Structural | schema, migration, domain model, interface seam, generated code | Written plan, then explicit approval. |

**Proposal stops the turn.** Recommending a fix or proposing a plan ends my turn; I hold for your answer. A question I asked is not a mandate to build.

**Approval binds to presented scope.** "Approved" covers the plan on the table, exactly. Wider work re-enters through the gate.

**Plan breaks mid-flight → stop.** Re-plan, re-approve. I never improvise scope.

**Authorization is your yes, not a document.** A plan nobody approved unlocks nothing; a one-line yes to bounded work unlocks that work.

# Slice Discipline — One Kind of Change at a Time

Cost is set by how many existing call sites a change touches, not by how much new code it adds. A slice that changes a signature costs roughly 5x one that only adds — and nearly all of that is mechanical edit churn, which is exactly where mistakes hide undetected.

| Size | What it is | Ends when |
| :--- | :--- | :--- |
| Slice | Adds a domain type, query or rule. **No existing signature changes.** | Red to green in one pass. |
| Vertical | Domain + migration + repository + caller + integration test. **At most one** signature change. | One acceptance test proves it through real MariaDB. |
| Refactor | A signature change across the codebase. **Zero new behaviour.** | Green before, green after, same tests. |

1. **Never mix a refactor with a feature.** Land the new shape green first, then build on it. Mixed, a broken edit is indistinguishable from a broken design.
2. **Signature change gets its own slice.** If a feature needs a new shape, ship the shape first.
3. **A refactor may be wide.** It is allowed to be, because it is the only thing in it.
4. **Every slice ends green.** Nothing is left broken between slices.
5. **State the acceptance evidence up front:** the one test that proves the slice worked.

**Slice size is not the goal; one kind of change per slice is.** A wide refactor is correct. A small slice that also changes an interface is not.

**Plan the phase as slices before starting it.** A "phase" that contains five features is five slices, not one.

# FinGo Project Context

- **App Name**: FinGo (SME Accounting Desktop Application).
- **Target OS**: Windows (On-premise).
- **Architecture**: Onion Architecture (Modular Monolith).
  - `internal/domain`: pure models and double-entry rules. Imports no other FinGo package; the one outside dependency is `github.com/shopspring/decimal`.
  - `internal/usecase`: application services and business workflows.
  - `internal/adapter`: storage adapters, **all of them `mariadb`** — sqlc-generated queries plus hand-written stores. No Wails adapter, no PDF/Excel export exists yet.
  - `frontend`: Wails UI (Vue 3 + Vite + TypeScript). The Wails app itself is `fingo/app.go` + `fingo/main.go`, at the module root.

# Tech Stack

- **Backend**: Go 1.26
- **Desktop UI**: Wails v2 (`github.com/wailsapp/wails/v2`)
- **Database**: MariaDB 12.3 (Localhost / LAN on-premise)
  - Driver: `github.com/go-sql-driver/mysql`
  - Generator: `sqlc` (raw SQL to type-safe Go code)
  - Migrations: `goose` (`db/migrations`)
- **Math Precision**: NEVER use float for currency/amounts. Two integer regimes, and they do not mix:
  - `internal/domain/*` + `gl` — `github.com/shopspring/decimal`, whole đồng.
  - `internal/money`, `vat`, `inventory`, `document` — `int64` minor units (with an explicit `Currency`). `internal/inventory` has a test asserting the package contains no `float32`/`float64` at all.

# Database & Environment

- **One database per client** (see `docs/requirements-v1.md` §1.1) **and** a `client_id` column on every table holding a client's data — 18 tables, migration `00015_client_scope.sql`. See `docs/adr/0003-client-data-boundary.md`, which supersedes ADR 0001's "no company_id column" clause.
  - The connection decides which database is *reachable*; the column plus `client.Scope` decides which client a query *is for*. Both, not either.
  - A financial repository method takes a `client.Scope` first. An unscoped query is refused (`ErrNoScope`), not widened. `internal/client` is the reference; existing repositories do not take a `Scope` yet.
- **Config — two sources, in this order.** `%APPDATA%\FinGo\config.json` (written by the connection screen, **plaintext password**) wins; a `.env` is the fallback and is reported in the UI as a development source. `internal/appconfig` owns the file, `internal/onboarding` probes and prepares a database, and `Status.source` says which one is in use. A settings file that exists but cannot be read is an **error, not a fallback** — quietly opening a different database is the one failure an accounting app must not have. See `docs/adr/0005-connection-settings.md`.
- **Dev database**: `fingo` (one client of many; not a shared schema)
- **Default Dev Credential**: `dev:123456@tcp(127.0.0.1:3306)/fingo?parseTime=true`
- **Config**: `.env` in `d:\accounting\fingo` — the development fallback only. The application's own settings live in `%APPDATA%\FinGo\config.json` and win over it.

# Essential Commands (Run from `d:\accounting\fingo`)

- **Dev Live Server**: `wails dev`
- **Production Build**: `wails build`
- **Generate SQLC**: `sqlc generate`
- **Create Migration**: `goose -dir db/migrations create <migration_name> sql`
- **Apply Migrations**: in code, via `db.Up(sqlDB)`. It runs goose **and then** re-applies `db/triggers/*.sql`. The goose CLI applies the versioned migrations only, so it leaves a database without triggers.
- **Full gate — run after every slice**: `go build ./...` then `go vet ./...` then `go test ./...`
- **Format**: `gofmt -w <paths>`. Never in the same command as an edit — it rewrites the file and invalidates the edit tool's read snapshot.
- **Never rewrite source with PowerShell**: `Set-Content`/`-replace` on a `.go` file collapses newlines and destroys it. Use the edit tool.
- **CodeGraph Sync**: `codegraph sync` (run from `d:\accounting`) after adding packages or files — the index is otherwise stale and silently misses them.

Integration tests talk to a real MariaDB and create/drop scratch databases named
`fingo_test_*` and `fingo_migrate_test_*`. They skip when no `.env` or no server is
reachable, so a green run with no database proves nothing about the database path.

# Implemented Modules & Domain Packages

Two generations. `internal/domain/*` is the original skeleton; `internal/*` is the engine built slice by slice since. Some domain packages are live, the rest are scaffold.

## Duplicated models — the main debt

Same idea, two packages, neither retired. Resolve before building on either side of a row.

| Concept | Engine (`internal/`) | Older (`internal/domain/`) | Difference that matters |
| :--- | :--- | :--- | :--- |
| Money | `money` — int64 minor unit + `Currency` | `money` — `decimal` đồng | The engine uses `internal/money` |
| Partner | `partner` — `trade_partners` (00013); duplicate `TaxID` warns | `partner` — `partners` (00008); `code` UNIQUE, `tax_code` UNIQUE | `voucher_lines.partner_id` and `usecase/practice` still use the old one |
| Document | `document` — `arap_*` (00014); stores `Allocated`/`Status` | `document` — `documents`/`allocations` (00009); derives settlement | `record_sale` writes the old one |
| Inventory | `inventory` — integer unit average, plus COGS entries | `inventory` — exact total value, `UnitCost` for display only | Opposite answers to the same rounding question |
| Period | `period` — date ranges, irreversible lock (00011) | `gl.Periods` — closed (year, month) rows (00004) | Ranges express strictly more |
| Period close | `close` — P&L through 911 into 4212, then the range lock; provisions in the same voucher | `domain/closing` — `PeriodClosing`, net profit only, posts nothing | `usecase/practice.Client.ClosePeriod` closes a month and `ReopenPeriod` reopens it; v1 has no reopen |
| Sales | `usecase/sales` | `domain/sales` | The old one carries a decimal `VATRate` |
| Trial balance | `reports` — three reports summed from voucher lines, each row tracing to its vouchers | `domain/report` — `TrialBalance` rows filled in by hand, `ValidateBalance` | The old one is a stub with no reader; nothing computes it |

## Engine — `internal/`

- **`client`**: the practice's client registry and the tenant boundary. `Client` (`ID`, `Name`, `TaxID`, `CreatedAt`, `Active`), `Registry` + `MemoryRegistry` (duplicate tax code refused; provisioning is part of `Create`), `Scope` — the zero value means *nobody*, never *everybody* — `ChartProvisioner` (seeds the TT133 chart on registration), `Document`/`DocumentRepository` (every method takes a `Scope`; no unscoped query is expressible), and `Middleware` (reads the client off the authenticated principal, refuses 401/403, puts the scope on the request context). **Unwired**: nothing outside its own tests imports it, there is no MariaDB `Registry` adapter, and its `Document` is the reference shape for the scoping rule — not `internal/document`, which is the real model and does not take a `Scope` yet.
- **`migrate`**: backup-before-migrate. `Load` reads numbered `.sql` files — plain, or goose-marked, of which it takes the Up half — and refuses a set that is not 1, 2, 3 … with nothing missing. `Runner.Migrate` backs the database up and verifies it before applying anything, then applies the batch; a failure anywhere restores the whole batch and clears what the run applied. `Backup` is a logical dump written in Go — `schema.sql` from `SHOW CREATE TABLE`, `data.sql` as INSERTs, `manifest.json` with per-table row counts and checksums — into `backups/YYYY-MM-DD_HH-MM-SS/`, because mysqldump is a client binary a Windows install is not guaranteed to have. `VerifyBackup` checks the file checksum and the live row counts; `Restore` refuses to run against a database the backup does not name. **Overlaps `db/`**: goose still owns `db/migrations` and `goose_db_version`, this owns `schema_migrations`, and nothing wires the two together yet.
- **`web`**: the admin UI — server-rendered Go, `html/template`, HTMX as progressive enhancement only. Pages: dashboard, documents (list/create/view/pay), chart of accounts, reports (trial balance, P&L, balance sheet), settings (periods, VAT rates, posting rules). Everything is embedded (`//go:embed templates static`), including a vendored `htmx.min.js`, because the deployment is an offline desktop binary. The pages depend on one port, `web.Books`, declared where it is used; `web/books` is the in-memory implementation composing `gl`, `internal/document`, `internal/vat`, `internal/posting` and `internal/reports`. Writes answer with a page rather than a 303, so internal/idempotency's record-and-replay applies; the idempotency key travels in a hidden form field and a middleware promotes it to the header. **Unwired**: nothing mounts `web.Server`; `cmd/webui` serves it over in-memory books to look at. See `docs/adr/0004-admin-ui-is-server-rendered.md`.
- **`money`**: `Money` (minor unit + currency), `Allocate` (largest remainder; parts sum to the total exactly). The only place a legal amount is decided.
- **`vat`**: `VATRate` (`ID`, `Rate` in basis points, `EffectiveFrom`, `EffectiveTo`, `Active`), `Tax` (integer, half away from zero), `Registry` (append-only; one active rate per date; **no update path**), `Line`/`Charge` — a line that touches VAT names a `VATRateID`, never a percentage. No table yet; the registry is in memory.
- **`partner`**: `Partner` (`ID`, `Name`, `Type` customer/supplier/both, `TaxID`, `PaymentTerms`, `Active`), `Warning` — a duplicate `TaxID` is a warning, not a refusal. `Repository` + `MemoryRepository`.
- **`document`**: `Document` (`Type`, `PartnerID`, `Date`, `PeriodID`, `VATRateID`, `Total`, `Allocated`, `Status`), `Line`, `Application`/`Allocation`/`Credit`, `Settle` — an overpayment becomes a credit, never an error. `Repository.Apply` is the one method that must be transactional.
- **`inventory`**: `Item`, `StockMove`, `Valuation` (weighted average, integer minor units per unit, truncated), `Ledger` (level is the sum of the moves; negative stock is `ErrNegativeStock`), `cogs.go` — `Entry`, `IssueCost`, `ReverseCost`, `Sale`/`Poster`.
- **`idempotency`**: `Middleware` wraps an `http.Handler`, keyed on `Idempotency-Key` — the same request replays, the same key with a different request is 409, `TTL` 24h with a `Reaper`. `Store` + `MemoryStore`. Transport, not business logic; **no HTTP server exists yet**, so this is the seam.
- **`trading`**: the two cycles, and the first thing that uses `vat` + `document` + `inventory` + `posting` together. Shared: `pricing.go` (one line-pricing path for both cycles), `Poster`/`Posting` (the ledger seam), and `money.Money` throughout — the decimal conversion happens once, at the voucher.
  - **Sales — `Service`**: `Invoice` posts Dr AR / Cr revenue / Cr VAT out and records the receivable; `Pay` posts Dr cash-or-bank / Cr AR and allocates; `Credit` mirrors what the invoice recorded and allocates it back.
  - **Purchases — `Purchasing`**: `Order` prices and records (posts nothing — a promise is not an event); `Receive` moves goods into `inventory` at the ordered price (posts nothing — the bill is what debits inventory); `Invoice` posts Dr inventory-or-expense / Dr VAT input / Cr AP and reports a three-way mismatch; `Pay` posts Dr AP / Cr bank and allocates.
  - Purchase accounts are a configured `PurchaseAccounts`, **not** posting rules: `posting.Key` cannot tell inventory (156) from input VAT (1331) — both are assets debited by one document. `posting` says so itself; it needs a line-role dimension before purchases can be keyed that way.
- **`posting`**: `RuleSet` maps a decision (`DocType`/`Side`/`AccountType`) to an account code; `Requirements` is the contract, `TT133Defaults()` the sales-cycle defaults. `Load` turns a missing or contradictory rule into a boot error.
- **`period`**: `Period` (date range), `Calendar`, `Guard` (`CheckLocked` for a posting, `CheckPosting` for a report), `Repository`.
- **`close`**: `Service.ClosePeriod` — refuses a period with documents still owed (or forces with a recorded reason), carries revenue and expenses through 911 into 4212 (`closingEntries`), records the `Report` snapshot, then locks via `period.Repository.Lock`. Periods close in order (`ErrEarlierPeriodOpen`); a 911 already carrying a balance is refused. Retry-safe: the snapshot is written before the entries, and the request key `period-close:<id>` is stable per period. No reopen. **Unwired**: no production `Ledger`/`Reports`/`Assets`/`Receivables` adapter, and `usecase/practice.Client` keeps its own month-based close. Ports: `Periods`, `Ledger`, `Poster`, `Documents`, `Reports`, and the provision ports below (`Assets`, `Receivables`, `ledger.ChartSource` — the one already implemented, by `usecase/practice.Client`).
  - **Provisions** (`provisions.go`, opt-in via `Service.WithProvisions`): straight-line depreciation `(cost − residual) / useful_life_months`, one rate per calendar month the period covers, never past the base (`depreciationFor`); bad debt as `%` of AR age bands (`BadDebtPolicy`, contiguous by construction — `DefaultBadDebtPolicy` ages, `Circular48BadDebtPolicy` provides). Ports `Assets` + `Receivables` (an amount owed + its due date) + `ledger.ChartSource`. Every account is checked against the chart **before** anything is written: the expense must be a P&L leaf and the accumulation must not be one (an accumulation into 6421 would leave a P&L account standing after the close), neither may be 911/4212, and an asset may name its own pair. Entries and the closing entries are **one voucher**, so the provision expenses are folded into the balances the close carries (`withProvisions`). TT133 defaults 6422/2141/6422/2293 — all leaves; 214/229/642 are headings. Charges each close; a target-vs-held delta is not modelled.
- **`voucher`**: `Voucher`, `Line`, `ValidateBalance`, `Prefix`/`Series`/`Format`.
- **`reports`**: `Service.TrialBalance` / `ProfitAndLoss` / `BalanceSheet`, every figure summed from posted lines (`Journal.Lines`) — **no cached balances anywhere**. A row exists only because a voucher line touched the account, and every row carries `SourceVoucherCount`, so no figure is stated without a voucher behind it. The P&L excludes the closing vouchers (the ones touching 911) **whole** and says how many it left out — a trading line bundled into a closing voucher is lost from the report, so keep closing entries to what they close; the balance sheet carries an unclosed period's result into equity, which is what makes `Assets = Liabilities + Equity` hold before a close as well as after. Refuses (rather than mis-states): an account the chart does not hold, a result account the chart does not hold (the P&L would otherwise report a profit of nothing), a line with no voucher behind it, neither a debit nor a credit, or a negative amount. Ports: `Periods`, `Journal`, `ledger.ChartSource`, plus `close.ClosingAccounts` (aliased, so "the result account is 911" has one definition).
  - **Statutory output** (`statutory/`, `Service.Generate`): 01/GTGT, its reduced-VAT appendix (NQ 204/2025, cited under the name the file gives it), 03/TNDN, and the B01-DN/B02-DN financial statements, as JSON for filing and as a PDF for the archive. **Every line carries its legal basis** (instrument + provision) and the confidence `docs/legal-basis.md` gives it; a line the file marks `?` is **never emitted** — it becomes a warning, in the output and in the log. **No line is filed with a mã số**: the file records the numbering against a secondary source and leaves it open (Q2, Q5, Q8), and its rule is not to build on what it has not resolved — what it does record is named in each statement's warning. `TestCatalogue_CitesOnlyWhatTheLegalBasisHolds` reads the file and checks every instrument, `Điều` article and form code cited is a row in it. Figures come from `internal/reports`; the VAT rates and the year's turnover come from a `Facts` port. The PDF is hand-rolled, folded to the page and transliterated (standard PDF fonts carry no Vietnamese; the JSON keeps the names as written). Refuses to file rather than mis-state: an unknown regime, and a period outside the 8% window states no reduction.

## Domain — `internal/domain/`

Reached by the engine today (non-test importers): **`gl`** (`Voucher`, `VoucherLine`, `ChartOfAccounts`, `Periods`, `Book`/`Number`, `PostingRules`, `Rehydrate`, `TT133Chart()`), **`document`**, **`inventory`**, **`tax`**, **`money`**, **`partner`**. Three of those — `document`, `money`, `partner` — are the older half of the duplicated pairs above, so they are in use *and* on the way out.

Wired into nothing — no non-test importer at all: `asset`, `cash`, `catalog`, `closing`, `costing`, `opening`, `payroll`, `purchase`, `report`, `sales`, and **`system`** (`CompanyProfile`). (`report` is superseded by `internal/reports`.)

## Use cases

- **`usecase/ledger`**: `Posting`, `Service.Post` — the only path to a posted voucher. `Posting.Cost` carries the inventory-side voucher so both commit in one transaction.
- **`usecase/sales`**: `RecordSale` — credit sale. COGS is resolved through a `StockSource` it declares itself; nothing routes it to `internal/inventory` yet.
- **`usecase/practice`**: `Client` — one client's books, opened.

**`internal/trading` is the flow the sales use case should become**: it is wired to the new packages, while `usecase/sales` still predates them.

## Adapters

- **`mariadb/postingrepo`**: `Store.Save` — one transaction spanning the voucher, its `Cost` voucher, lines, stock and document.
- **`mariadb/glrepo`** `SaveWithin` · **`periodstore`**/`periodrepo` · **`documentstore`**/`documentrepo` · **`partnerstore`** · **`stockrepo`** · **`voucherstore`** · **`conn`** · **`testsupport`**
- **`db`**: embedded migrations plus the trigger runner. 00013 adds `trade_partners`; 00014 adds `arap_documents`/`arap_lines`/`arap_allocations`/`arap_credits`; 00015 adds `clients` and puts `client_id` on the other 18 tables. `TestEveryFinancialTableIsClientScoped` derives the table list from `information_schema`, so a table added later without a `client_id` fails it. After changing `db/queries/*.sql` or a migration: `sqlc generate`.
- **`pkg/logger`**: structured logger (`log/slog`) injecting `trace_id` / `user_id`.


# Skill Selection Decision Table (5W1H Framework)

Installed skills are `.agents/skills/*`. This table covers the ones this project reaches for.

| Skill | Who (Trigger Role) | What (Task / Outcome) | When (Timing / Phase) | Where (Target Area) | Why (Goal) | How (Execution Method) |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `codegraph` | Recon / Impact Analyst | Symbol lookup, call paths, callers/callees, blast radius | Before touching a symbol; checking what a change reaches | `internal/`, `.codegraph/codegraph.db` | Facts about callers instead of assumptions | `codegraph sync` after adding files, then `query` / `callees` / `impact`. Prefer `impact`: some edges are mis-attributed to the enclosing method, so check a surprising `callers` result in the file. |
| `tdd` | Backend Dev | Unit & integration tests before/with code | Adding business logic, accounting rules, bug fixing | `internal/domain`, `internal/usecase` | Ensure 100% accounting math accuracy, zero balance drift | Red-Green-Refactor, table-driven tests |
| `codebase-design` | Lead Architect | Deep module interfaces & abstraction boundaries | Creating new module or refactoring bloated layers | `internal/` package seams, domain interfaces | High cohesion, loose coupling, leak-free Onion layers | Evaluate surface area vs depth, hide MariaDB/Wails details |
| `domain-modeling` | Business Analyst / Architect | Ubiquitous language, entities, ADRs, CONTEXT.md | Clarifying accounting terms (VAS/Circular 133/200), schema shifts | `docs/`, `internal/domain/` | Avoid semantic confusion between dev & domain terms | Define entities, value objects, write ADRs |
| `diagnosing-bugs` | Debugger | Root cause diagnosis & fix | Panic, calculation discrepancy, DB lock, unexpected state | App-wide, DB queries, logs | Eliminate root defect, prevent regression | Reproduce with minimal test case, trace call path, patch |
| `code-review` | Reviewer | Rigorous diff review against specs & standards | Before merging, finishing PR, or after large refactor | Modified files vs main/base branch | Enforce Onion Architecture, no float usage, style rules | Parallel spec & standard checklist verification |
| `research` | Tech Lead / Researcher | External library evaluation, protocol specs, regulations | Unfamiliar domain / third-party integration (e.g. e-invoice) | Documentation, POC files | Grounded technical decisions based on primary docs | Fetch official docs, extract constraints, summarize |
| `prototype` | UI / Product Engineer | Throwaway UI or workflow spike | Testing UX flow or visual layout with user | `frontend/src/views`, standalone scratch | Fast feedback before committing to full architecture | Build fast throwaway UI in Vue/Vite, validate, discard/adapt |
| `grilling` | Lead Architect | Stress-test architectural plan or domain decisions | Before executing complex/high-risk changes | Implementation plans, ADR drafts | Catch edge cases, double-entry flaws, perf bottlenecks | Relentlessly challenge assumptions with hard Socratic questions |
| `karpathy-guidelines` | Core Dev | Disciplined step-by-step implementation | Building non-trivial algorithms or complex pipeline | Accounting engine, calculation routines | Prevent premature complexity, stay grounded in real data | Inspect data first -> trivial baseline -> overfit -> scale |
| `resolving-merge-conflicts` | Git Operator | Clean git resolution | In-progress rebase or merge conflict | Git working tree | Prevent accidental code clobbering | Analyze both sides, preserve invariants, verify build |
| `wizard` | DevOps / Operator | Interactive guided walkthrough | Manual on-prem DB install, certificate config, env setup | Local OS, Windows services, MariaDB | Guide human through steps agent cannot automate | Interactive step-by-step CLI execution |
| `writing-for-agents` | Meta / Agent Ops | Maintain AGENTS.md, GEMINI.md, custom skills | Modifying agent rules or creating new team skills | `GEMINI.md`, `.agents/skills/` | High instruction adherence, low token bloat | Terse, structured, concrete examples, clear triggers |
| `diagnose-windows-sandbox-acl` | Ops / Debugger | Distinguish a real sandbox denial from a broken ACL, and repair the ACL | A workspace write or listing fails, or an ordinarily readable path will not read | Windows paths and every ancestor | Unblock file work without loosening confinement | One bundled command inspects the path and its ancestors and repairs only what it proves |

