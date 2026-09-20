# Caveman Mode Rules

- Use caveman style: ultra-terse, blunt, high signal, zero fluff.
- Drop corporate pleasantries, apologies, and chit-chat.
- Short sentences. Direct answers first.
- Keep technical precision 100%. Code and commands remain complete, typed, and battle-tested.

# FinGo Project Context

- **App Name**: FinGo (SME Accounting Desktop Application).
- **Target OS**: Windows (On-premise).
- **Architecture**: Onion Architecture (Modular Monolith).
  - `internal/domain`: Core business models, pure Go structs, double-entry rules. Zero external dependencies.
  - `internal/usecase`: Application services and business workflows.
  - `internal/adapter`: Database implementations (`mariadb`), UI handlers (`wails`), PDF/Excel exports.
  - `frontend`: Wails UI (Vue 3 + Vite + TypeScript).

# Tech Stack

- **Backend**: Go (v1.25+)
- **Desktop UI**: Wails v2 (`github.com/wailsapp/wails/v2`)
- **Database**: MariaDB 12.3 (Localhost / LAN on-premise)
  - Driver: `github.com/go-sql-driver/mysql`
  - Generator: `sqlc` (raw SQL to type-safe Go code)
  - Migrations: `goose` (`db/migrations`)
- **Math Precision**: `github.com/shopspring/decimal` (NEVER use float for currency/amounts).

# Database & Environment

- **Database Name**: `fingo`
- **Default Dev Credential**: `dev:123456@tcp(127.0.0.1:3306)/fingo?parseTime=true`
- **Config**: `.env` in `d:\accounting\fingo`

# Essential Commands (Run from `d:\accounting\fingo`)

- **Dev Live Server**: `wails dev`
- **Production Build**: `wails build`
- **Generate SQLC**: `sqlc generate`
- **Run Migrations**: `goose -dir db/migrations mysql "dev:123456@tcp(127.0.0.1:3306)/fingo?parseTime=true" up`
- **Create Migration**: `goose -dir db/migrations create <migration_name> sql`
- **Run Domain Tests**: `go test -v ./internal/domain/...`
- **Run Logger Tests**: `go test -v ./pkg/logger/...`
- **CodeGraph Sync**: `codegraph sync` (run from `d:\accounting`)

# Implemented Modules & Domain Packages

- **`pkg/logger`**: Structured context-aware logger (`log/slog` standard library wrapper). Injects `trace_id` and `user_id`.
- **`internal/domain/system`**: `User`, `Role`, `Permission`, `CompanyProfile`, `AuditLog`, `SystemRepositoryStub`.
- **`internal/domain/catalog`**: `Customer`, `Vendor`, `Item`, `BankAccount`, `Warehouse`, `CatalogRepositoryStub`.
- **`internal/domain/gl`**: `Voucher`, `VoucherLine` (double-entry validation), `GLRepositoryStub`.
- **`internal/domain/cash`**: `CashReceipt`, `CashPayment`, `BankTransaction`, `CashRepositoryStub`.
- **`internal/domain/sales`**: `SalesInvoice`, `SalesInvoiceLine` (VAT math), `SalesRepositoryStub`.
- **`internal/domain/purchase`**: `PurchaseInvoice`, `PurchaseInvoiceLine`, `PurchaseRepositoryStub`.
- **`internal/domain/inventory`**: `StockInward`, `StockOutward`, `StockMovementLine`, `InventoryRepositoryStub`.
- **`internal/domain/asset`**: `FixedAsset` (monthly straight-line depreciation), `AssetRepositoryStub`.
- **`internal/domain/opening`**: `OpeningBatch`, `AccountOpeningBalance`, `CustomerOpeningBalance`, `VendorOpeningBalance`, `InventoryOpeningBalance`, `OpeningRepositoryStub`.
- **`internal/domain/tax`**: `EInvoice`, `VATDeclaration` (Box 40/43 calculation), `TaxRepositoryStub`.
- **`internal/domain/closing`**: `PeriodClosing` (P&L transfer 511/632/642 -> 911 -> 421), `SystemLockConfig` (Lock date checks), `ClosingRepositoryStub`.
- **`internal/domain/report`**: `TrialBalance` (Double-entry balance check), `FinancialReportType`, `ReportRepositoryStub`.
- **`internal/domain/payroll`**: `Employee`, `PayrollItem` (Statutory BHXH 10.5% / 21.5% calculation), `PayrollRepositoryStub`.
- **`internal/domain/costing`**: `ProductionCostCard` (WIP + 621/622/627 allocation, unit cost), `CostingRepositoryStub`.


# Skill Selection Decision Table (5W1H Framework)

| Skill | Who (Trigger Role) | What (Task / Outcome) | When (Timing / Phase) | Where (Target Area) | Why (Goal) | How (Execution Method) |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
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

