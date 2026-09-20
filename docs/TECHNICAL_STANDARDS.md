# FinGo Enterprise Technical Standards and Architecture Guidelines
**Vietnamese Accounting & Statutory FinTech Platform**

---

## Document Control Block

| Metadata | Specification |
| :--- | :--- |
| **Document ID** | FIN-STD-2026-V1 |
| **Version** | 1.0.0 (Formal Release) |
| **Status** | APPROVED / NORMATIVE |
| **Author** | Senior Solution Architect (FinGo Accounting Platform) |
| **Effective Date** | 2026-01-01 |
| **Review Cycle** | Quarterly / Regulatory Event-Driven |
| **Target Audience** | Backend Engineers, Frontend Engineers, DevOps, QA/SDET, Chief Accountants, Security & Compliance Auditors |

### Document History
| Version | Date | Author | Description |
| :--- | :--- | :--- | :--- |
| `1.0.0` | 2026-09-20 | Solution Architecture Team | Baseline formalization incorporating Go 1.25+, Onion Architecture, Circular 99/2025/TT-BTC, Circular 32/2025/TT-BTC, Decree 70/2025/ND-CP, Resolution 204/2025/QH15, and Law 91/2025/QH15 (PDPL). |

### Governance & Approvals
| Role | Name / Title | Status |
| :--- | :--- | :--- |
| **Lead Architect** | Solution Architecture Board | Approved |
| **Accounting Domain Lead** | Chief Accounting Specialist (VAS / Circular 99 Lead) | Approved |
| **Head of Engineering** | VP of Engineering | Approved |
| **Information Security** | Head of Cyber Security & Compliance | Approved |

---

## Step 0 — Repository Context Extraction & Operational Assumptions

The standards in this document are grounded in the authoritative system definition established in `GEMINI.md`:

### 1. Extracted Facts from GEMINI.md
* **Language & Tooling**: Go `v1.25+` (`go 1.25.0` pinned in `go.mod`), Wails v2 desktop runtime (`github.com/wailsapp/wails/v2`), Vue 3 + TypeScript frontend.
* **Architecture Style**: Strict Onion Architecture (Modular Monolith). Zero external dependencies in `internal/domain`.
* **Data Storage**: MariaDB 12.3 on-premise/local LAN (`github.com/go-sql-driver/mysql`). Type-safe code generation via `sqlc`, versioned database migrations via `goose`.
* **Financial Math**: Mandatory `github.com/shopspring/decimal`. Floating point primitives (`float32`/`float64`) are categorically forbidden.
* **Logging Foundation**: Structured contextual logging using `log/slog` stdlib via `fingo/pkg/logger` with mandatory `trace_id` and `user_id`.
* **Domain Modules**: 14 distinct packages (`system`, `catalog`, `gl`, `cash`, `sales`, `purchase`, `inventory`, `asset`, `opening`, `tax`, `closing`, `report`, `payroll`, `costing`).

### 2. Explicit Baseline Assumptions (`[ASSUMPTION]`)
Where `GEMINI.md` does not specify broader enterprise infrastructure details, the following normative assumptions govern this specification:
* `[ASSUMPTION-01] Multi-Tenancy`: FinGo operates as an on-premise single-tenant instance (dedicated MariaDB database `fingo` per legal entity) with provision for logical `tenant_id` / `org_unit_id` schema segregation for multi-branch consolidated holding setups.
* `[ASSUMPTION-02] In-Process & Inter-Service Communication`: As a modular monolith, modules interact via deep in-process Go interfaces. For external integrations (E-invoice providers, Bank APIs, Tax Authority portals), outbound communication MUST use TLS 1.3 HTTPS REST/JSON or signed XML.
* `[ASSUMPTION-03] Background Task Execution`: Asynchronous workloads (e.g., e-invoice transmission, bulk depreciation runs) execute via an in-memory transactional outbox worker within the desktop daemon process.

---

# Section 1: Go Language & Code Standards

## 1.1 Language & Tooling

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-GO-01** | Compiler & Toolchain Pinning | All Go packages | Engineering teams MUST compile all binaries using Go `1.25.0` or higher. The minimum Go directive in `go.mod` MUST remain pinned. Upgrades to newer minor Go releases MUST be evaluated within 30 days of release and validated against the full regression suite. | Guarantees deterministic builds, language compatibility, memory allocator improvements, and runtime security patches. | CI pipeline step verifying `go version` against `.go-version` / `go.mod`. | RFC submitted to Lead Architect with benchmark and runtime validation. |
| **VN-GO-02** | Dependency Hygiene & Checksum Locking | `go.mod`, `go.sum` | Dependencies MUST be pinned to exact semantic versions. Indirect dependencies MUST NOT be altered manually. `go.sum` MUST be checked into VCS. Running `go mod tidy -diff` in CI MUST return zero diffs. Vendoring (`go mod vendor`) SHOULD be maintained for air-gapped on-premise distribution. | Prevents software supply-chain attacks, phantom dependency upgrades, and broken builds on offline customer environments. | CI check: `go mod verify && go mod tidy -diff`. | None. `go.sum` tampering is rejected automatically. |
| **VN-GO-03** | Mandatory Static Analysis & Linter Suite | All Go code | All code MUST pass `golangci-lint` without warnings. The following linters MUST be active: `govet`, `staticcheck`, `errcheck`, `gosec`, `revive`, `ineffassign`, `unused`, and custom float-detector `forbidigo`. | Eliminates concurrency hazards, uncaught errors, memory leaks, and non-idiomatic Go patterns prior to human code review. | Automated CI gate blocking merge if `golangci-lint run` fails. | `//nolint` comments MUST cite an approved Jira/Issue ID (e.g. `//nolint:gosec // Issue #142`). |
| **VN-GO-04** | CGO & Unsafe Package Prohibition | All Go packages | Go packages MUST NOT utilize CGO (`CGO_ENABLED=0`) or import `unsafe`, unless explicitly authorized for low-level OS native window handles in desktop adapters. `internal/domain` and `internal/usecase` MUST NEVER contain CGO or `unsafe`. | Guarantees cross-compilation reliability, eliminates memory corruption, and prevents panic-induced app crashes on client machines. | CI build with `CGO_ENABLED=0` and linter rule prohibiting `import "unsafe"`. | Architecture Board waiver required for native Windows Win32 API interop. |

## 1.2 Code Structure & Conventions

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-GO-05** | Onion Architecture Seam Isolation | `internal/domain/**` | Core domain packages MUST NOT import outer layers (`internal/usecase`, `internal/adapter`, `database/sql`, Wails runtime). Domain models MUST be pure Go structs containing business rules and repository interfaces only. | Preserves domain model integrity; allows 100% in-memory unit testing without database or GUI infrastructure. | Architecture unit test verifying import ASTs and `codegraph impact`. | Zero exceptions permitted. |
| **VN-GO-06** | Structured Error Handling & Wrapping | All Go code | Errors MUST be handled explicitly. When propagating errors across layers, errors MUST be wrapped using `fmt.Errorf("...: %w", err)`. Sentinels MUST be defined as exported `var ErrX = errors.New("...")`. Error types MUST be categorized into: (a) Domain Invariant Error, (b) Infrastructure Error, (c) Validation Error. | Prevents silent failures, preserves stack diagnostics, and allows deterministic error checking with `errors.Is()` and `errors.As()`. | Linter rule `errorlint` and code review gate. | None. |
| **VN-GO-07** | Context Propagation & Lifetime Discipline | All service & repo seams | Every I/O-bound, database, or long-running function MUST accept `ctx context.Context` as its first parameter. Methods MUST respect cancellation via `ctx.Done()`. Goroutines spawned in background MUST be owned by a supervised worker pool. Unbounded `go func()` calls are FORBIDDEN. | Prevents goroutine leaks, handles OS cancellation cleanly, and propagates `trace_id` and audit metadata. | Linter check (`contextcheck`) and PR code review. | Pure domain calculation methods without I/O do not take context. |
| **VN-GO-08** | Consumer-Driven Deep Interfaces | Application seams | Interfaces MUST be defined by the consumer (use case layer), NOT the producer (adapter layer). Interfaces SHOULD be small (1 to 3 methods) and deep (hiding high internal complexity behind minimal method signatures). | Minimizes coupling, prevents bloated mocks, and satisfies Interface Segregation Principle. | Codebase-design skill review against `codebase-design/SKILL.md`. | Legacy adapter wrappers require tech lead approval. |

## 1.3 Financial-Critical Code Rules

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-FIN-01** | Absolute Prohibition of Floating-Point Math | Entire codebase | Monetary values, quantities, VAT percentages, exchange rates, and depreciation rates MUST be represented using `github.com/shopspring/decimal`. Floating-point types (`float32`, `float64`) MUST NOT be used anywhere for financial state. | Binary floating-point representation induces rounding drift, corrupting general ledger balance sheets ($0.1 + 0.2 \neq 0.3$). | Custom `forbidigo` linter rule blocking `float32` and `float64` in domain and database models. | None. Strict zero-tolerance rule. |
| **VN-FIN-02** | VND Monetary Precision & Rounding Policy | All financial calculations | Vietnamese Dong (VND) transactions MUST be stored as whole integers (`decimal` with scale `0` on base currency). Intermediate calculations (e.g. Unit Price $\times$ Qty $\times$ VAT rate) MUST maintain at least 4 decimal places before applying `RoundBanker(0)` (half-even) on finalized voucher lines. Foreign currencies (USD, EUR, etc.) MUST retain 4 decimal places. | Aligns with Law on Accounting 2015. Half-even rounding minimizes statistical bias across multi-line vouchers. | Automated table-driven unit tests checking rounding edge cases (`0.50`, `1.50`, `2.50`). | Custom customer contractual rounding agreements documented in `SystemOption`. |
| **VN-FIN-03** | Mutation Idempotency & Replay Protection | Usecase & UI Handlers | All voucher posting, payment creation, and e-invoice generation commands MUST accept an `Idempotency-Key` (UUIDv4/v7). Re-submitting the same key within 24 hours MUST return the cached original response without creating duplicate ledger entries. | Desktop users repeatedly clicking "Submit" or network retries must never double-post financial vouchers. | Integration tests simulating rapid concurrent duplicate submissions. | Read-only queries do not require idempotency keys. |
| **VN-FIN-04** | Explicit Transactional Scoping | Adapters & Usecases | All voucher creations involving headers and lines MUST execute within an explicit MariaDB ACID transaction (`sql.Tx`). Transactions MUST set isolation level `READ COMMITTED` or `REPEATABLE READ`. Auto-commit mode for multi-row ledger mutations is FORBIDDEN. | Prevents orphaned voucher headers without lines or unbalanced ledger entries upon system crash. | Unit & integration tests asserting transactional rollback on injected failure. | None. |
| **VN-FIN-05** | Append-Only Ledger Immutability | `domain/gl`, DB schema | Posted journal vouchers (`is_posted = TRUE`) MUST NEVER be modified or deleted (`UPDATE` / `DELETE` SQL queries on posted vouchers are forbidden). Corrections MUST be executed via formal Reversing Entries (*Ghi âm* or *Ghi đỏ*) producing a new balanced voucher referencing the original. | Required by Law on Accounting 2015 (Article 16 & 28) and Circular 99/2025/TT-BTC. Ensures forensic auditability. | Database triggers / MariaDB row privileges blocking `UPDATE`/`DELETE` on posted rows. | Reversal approved by Chief Accountant. |

## 1.4 Testing Standards

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-TST-01** | Coverage Threshold & Table-Driven Tests | `internal/domain/**` | Core domain packages MUST maintain $\ge 90\%$ line and branch test coverage. Unit tests MUST follow Go table-driven testing idioms covering zero, boundary, negative, and invalid inputs. | Critical financial engine must not suffer regressions during code refactoring. | CI gate: `go test -coverprofile=coverage.out ./internal/domain/...` fails if coverage $< 90\%$. | Temporary waiver signed off by Lead Architect for newly drafted experimental modules. |
| **VN-TST-02** | Property-Based Invariant Verification | GL, Tax, Costing | The General Ledger double-entry engine ($\sum \text{Debit} == \sum \text{Credit}$) and VAT calculations MUST be tested using property-based random generation (`testing/quick` or `gopter`) over at least 10,000 generated voucher shapes. | Proves that balance invariant holds true across unpredictable transaction graphs and rounding variations. | Executed in CI test run: `TestDoubleEntry_PropertyBalancing`. | None. |
| **VN-TST-03** | Real-Database Integration Testing | `internal/adapter/mariadb` | Database repository tests MUST execute against real MariaDB 12.3 instances (using `testcontainers-go` or local test database). Mocking `database/sql` driver internals is FORBIDDEN. | MariaDB SQL syntax, lock semantics, unique constraints, and foreign keys must be validated against real database engine behavior. | CI integration test stage executing migrations and queries against test MariaDB container. | Offline standalone dev runs may use local dev MariaDB. |
| **VN-TST-04** | Golden-File Output Validation | `domain/report`, Tax, E-Invoice | All statutory Vietnamese tax forms (Mẫu 01/GTGT, B01-DNN, B02-DNN) and e-invoice XML outputs MUST be tested against verified "Golden Files" byte-for-byte or XML-canonicalized. | Guarantees that generated tax outputs match the strict schemas mandated by the General Department of Taxation without silent schema drift. | Golden file tests in `internal/domain/tax` and `internal/domain/report`. | Updates to golden files require Chief Accountant written sign-off. |
| **VN-TST-05** | Concurrency Race Detection | Entire codebase | All tests MUST pass clean execution under the Go race detector: `go test -race ./...`. | Unsynchronized memory access causes data corruption and catastrophic non-deterministic panics in desktop environments. | CI mandatory check: `go test -race ./...`. | None. Race conditions must be fixed immediately. |

## 1.5 CI/CD Pipeline

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-CICD-01**| Automated Quality Gates | Repository CI | Pull Requests MUST pass all sequential gates before merge: (1) `golangci-lint`, (2) `go vet`, (3) `go test -race`, (4) `govulncheck`, (5) `wails build`. PRs require at least 2 approvals, including 1 Domain Lead for accounting packages. | Guarantees that unverified code, security vulnerabilities, or broken desktop UI builds never enter `main`. | GitHub Actions branch protection rule enforcing status checks. | Hotfix emergency bypass requires CTO + Lead Architect dual override. |
| **VN-CICD-02**| Expand-Contract Database Migrations | `db/migrations/**` | All database migrations written in `goose` MUST be backward-compatible. Schema changes MUST follow the Expand-Migrate-Contract pattern: never drop a column or rename a table in the same release as application code changes. | Ensures zero-downtime updates and safe rollback without data loss on customer on-premise installations. | Migration verification test running `goose up` then `goose down` then `goose up`. | Schema breaking changes in major release (v2.0) with formal data migration tool. |

---

# Section 2: Vietnamese Accounting Domain Standards

## 2.1 Double-Entry Ledger (Circular 99/2025/TT-BTC & VAS)

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-ACC-01** | Chart of Accounts Hierarchy & Versioning | `domain/gl`, `domain/catalog` | The Chart of Accounts (COA) MUST comply with **Circular 99/2025/TT-BTC** (effective 1 Jan 2026, superseding Circular 200/2014/TT-BTC) and Circular 133/2016/TT-BTC for SMEs. Parent-child relationships MUST be strictly validated (sub-accounts inherit nature and type from parent). COA MUST be versioned per tenant. | Mandated by Ministry of Finance. Account numbers and reporting categories must match statutory tax classifications. | Automated validation in `domain/catalog` ensuring child account prefix matches parent code exactly. | Non-standard internal management sub-accounts permitted beyond Level 3. |
| **VN-ACC-02** | Dual Accounting Mode: VAS & IFRS Convergence | `domain/gl`, `domain/closing` | The accounting engine MUST support dual-reporting configurations: primary statutory VAS (Circular 99) and parallel IFRS adjustments for Foreign-Invested Enterprises (FIEs). Reconciliation between VAS historical cost and IFRS fair-value adjustments MUST be maintained via dedicated adjustment voucher types. | Circular 99 establishes roadmap for IFRS convergence in Vietnam. Multinationals operate under both frameworks. | Automated dual-ledger reconciliation tests verifying independent balance sheet integrity. | Pure domestic SMEs may operate in VAS-only mode. |
| **VN-ACC-03** | Mathematical Balance Invariant | `domain/gl` | Every journal voucher MUST enforce strict double-entry equality: $\sum \text{Debit Amounts} == \sum \text{Credit Amounts}$. Vouchers where debits do not equal credits MUST be rejected with `ErrUnbalancedVoucher`. Single-entry journal persistence is FORBIDDEN. | Foundational axiom of Vietnamese and international accounting. Prevents corrupted financial statements. | Unit test `TestVoucher_DoubleEntryBalanceCheck` and database constraint. | None. Unbalanced vouchers can never be saved. |
| **VN-ACC-04** | Sub-ledger to General Ledger Synchronization | Subledgers (AR, AP, Inventory, Cash, Bank) | All sub-ledger operational vouchers (Sales Invoice, Stock Inward, Cash Payment) MUST atomically generate corresponding GL journal entries within the same database transaction. Periodic reconciliation jobs MUST verify that $\text{Sum}(\text{Subledger Lines}) == \text{Balance}(\text{GL Control Account})$. | Eliminates reconciliation discrepancy between warehouse stock balances (TK 156) and inventory ledger, or AR balances (TK 131) and sales ledger. | Automated end-of-day reconciliation check raising `ErrSubledgerGLMismatch` if variance $> 0$. | None. |
| **VN-ACC-05** | Fiscal Period Closure & Re-Opening Governance | `domain/closing` | Fiscal years follow the calendar year (Jan 1 to Dec 31). Monthly and annual closing routines MUST lock the period. Re-opening a closed accounting period MUST require Chief Accountant authentication, generate an immutable audit log entry, and notify the compliance auditor. | Prevents retroactive manipulation of reported financial data prior to tax inspection. | Unit test `TestClosing_LockDateValidation` and permission gate check. | Written approval by Board of Directors and Chief Accountant. |

## 2.2 Data Model & Integrity

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-DAT-01** | Vietnamese Primary Currency & Multi-Currency Valuation | All transactions | The functional accounting currency MUST be Vietnamese Dong (VND). Foreign currency transactions (USD, EUR, etc.) MUST record: (a) Original currency amount, (b) Exchange rate (Tỷ giá giao dịch thực tế), (c) Equivalent converted VND amount. Period-end exchange rate revaluation (TK 413) MUST follow State Bank of Vietnam (SBV) transfer buying/selling rates. | Mandated by Law on Accounting 2015 (Article 10). Tax filings must be presented in VND. | Validation tests asserting foreign transactions maintain original currency and exchange rate. | Enterprises approved by Ministry of Finance to use foreign currency as accounting unit. |
| **VN-DAT-02** | Ten-Year Data Retention & Archival Invariant | Database & Backup | All accounting vouchers, invoices, general ledgers, sub-ledgers, and audit logs MUST be retained in accessible format for a minimum of **ten (10) years** per Article 36 of the Law on Accounting 2015. Soft deletes MUST use operational statuses (`POSTED`, `VOIDED`, `REVERSED`). Physical table truncation or hard deletion of historical records is FORBIDDEN. | Statutory compliance during tax audits, state inspections, and legal proceedings. | Schema verification asserting absence of `CASCADE DELETE` on ledger tables; backup retention policy. | None. |
| **VN-DAT-03** | Mandatory Vietnamese Language Locale for Official Output | UI & Reporting | All official financial reports, ledgers, tax declarations, and e-invoices MUST be generated in Vietnamese (*tiếng Việt* with standard Unicode UTF-8) compliant with Ministry of Finance layout templates. English/bilingual views MAY be provided for management dashboards. | Statutory tax filing rejection occurs if documents are not submitted in legal Vietnamese terminology. | Automated PDF/HTML golden file tests asserting correct Vietnamese diacritics and accounting headers. | None for statutory submissions. |

## 2.3 Vietnamese Tax Standards

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-TAX-01** | VAT Calculation & Special Regime Rules | `domain/tax`, `domain/sales`, `domain/purchase` | The VAT engine MUST support statutory rates: `0%`, `5%`, `10%`, and the temporary reduced `8%` rate pursuant to **Resolution 204/2025/QH15** and **Decree 174/2025/ND-CP** (effective through 31 Dec 2026). The engine MUST support non-taxable goods (*Không chịu thuế*), VAT exemptions, and direct calculation method (1% revenue for household businesses). | Prevents tax under-reporting or unlawful VAT deduction resulting in severe administrative tax penalties. | Table-driven tests validating VAT deduction across all statutory rates and item categories. | Legislative updates managed via versioned tax rule engine. |
| **VN-TAX-02** | Corporate Income Tax (CIT / TNDN) Calculations | `domain/tax`, `domain/closing` | The standard CIT rate of 20% MUST be configured. The tax calculation engine MUST categorize expenses into Deductible (*Chi phí hợp lý*) and Non-Deductible (*Chi phí không được trừ* - e.g. lack of legal e-invoices, cash payments $> 20$ million VND violating cash limitation rules). Quarterly provisional CIT payments and annual final returns (Mẫu 03/TNDN) MUST be calculated. | Law on Corporate Income Tax. Ensures accurate determination of taxable profit and corporate tax liabilities. | Unit tests asserting non-deductible expense lines do not reduce taxable income for CIT. | Preferential tax rates (e.g. 10%, 15% for high-tech zones) configured per tenant profile. |
| **VN-TAX-03** | Personal Income Tax (PIT / TNCN) Withholding Engine | `domain/payroll` | Payroll withholding MUST apply the 7-bracket progressive tax schedule (5% to 35%) for resident employees with labor contracts $\ge 3$ months. Personal deduction (11,000,000 VND/month) and dependent deduction (4,400,000 VND/month/dependent) MUST be applied before tax calculation. Ad-hoc payments to non-contract workers $\ge 2,000,000$ VND MUST withhold 10%. | Law on Personal Income Tax and Circular 111/2013/TT-BTC. | Property-based tests verifying progressive tax brackets across varied salary levels. | Legislative updates to family deductions configured via versioned rules. |
| **VN-TAX-04** | Electronic Invoicing Architecture (Nghị định 123 & Circular 32/2025) | `domain/tax` | E-invoices MUST comply with **Decree 123/2020/ND-CP** (as amended by **Decree 70/2025/ND-CP**) and **Circular 32/2025/TT-BTC** (effective 1 Jun 2025, superseding Circular 78/2021/TT-BTC). E-invoices MUST generate compliant XML payloads with digital signature tags, transmit to the General Department of Taxation (TCT), and obtain the Tax Authority Code (*Mã CQT*) or handle secure batch transmission without code. Invoice lifecycle MUST enforce: `Draft` $\rightarrow$ `Signed` $\rightarrow$ `Issued` $\rightarrow$ `Canceled` or `Replaced`. | Mandatory legal requirement for commercial transactions in Vietnam. Unsigned or unregistered invoices are legally invalid. | XML schema validation test against official TCT XSD specifications. | None. |
| **VN-TAX-05** | Form 01/GTGT Tax Position & Offset Engine | `domain/tax` | Period-end VAT offset routines MUST compare Total Deductible Input VAT (TK 133) and Total Output VAT (TK 33311). The engine MUST compute: (a) If Output $>$ Input: Tax Payable (*Chỉ tiêu [40]*), (b) If Input $>$ Output: Tax Carried Forward (*Chỉ tiêu [43]*). An automated offset entry ($Nợ\ TK\ 33311 / Có\ TK\ 1331$) MUST be generated for $\min(\text{Input}, \text{Output})$. | Circular 156/2013/TT-BTC and Circular 80/2021/TT-BTC tax filing mechanics. | Unit test `TestVATDeclaration_TaxPayableCalculation` and automated entry balancing. | None. |

## 2.4 Audit & Compliance

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-AUD-01** | Immutable Cryptographic Audit Trail | `domain/system`, `pkg/logger` | Every financial voucher creation, edit, reversal, approval, period close, and configuration change MUST record an immutable audit log capturing: `timestamp` (UTC), `user_id`, `client_ip`, `machine_mac`, `action`, `entity_name`, `entity_id`, and exact JSON `before_value` and `after_value`. Audit logs MUST be write-only and tamper-evident. | Mandated by Law on Accounting 2015 and cybersecurity audit standards for financial institutions. | Automated integration tests validating log generation upon voucher posting. | None. |
| **VN-AUD-02** | Segregation of Duties (Four-Eyes Principle) | `domain/system`, Usecases | Critical accounting operations MUST enforce segregation of duties: (a) The creator of a voucher MUST NOT be the approver/poster, (b) The person entering bank payments MUST NOT authorize the digital signature, (c) Cashier (*Thủ quỹ*) MUST NOT perform general ledger reconciliation. | Mandated by Circular 99/2025/TT-BTC mandatory internal control regulations to prevent internal fraud. | RBAC permission test asserting `ErrSelfApprovalForbidden` when user attempts to approve own voucher. | Sole proprietorships with $\le 2$ staff members may operate under documented single-user admin waiver. |
| **VN-AUD-03** | Deterministic Financial Report Reproducibility | `domain/report` | Re-running a financial statement (Balance Sheet B01, Income Statement B02, Trial Balance) for a closed past accounting period with the same parameters MUST yield byte-for-byte identical financial figures. | Accounting credibility during independent external financial audits and tax inspections. | Automated regression test checking report snapshot generation against historical periods. | Retroactive audit adjustments must be posted in the period of discovery. |

## 2.5 Multi-Tenancy & Isolation

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-TEN-01** | Complete Database-Level Tenant Isolation | Database & Adapters | Each tenant (enterprise / legal entity) MUST have a dedicated, separate MariaDB database (e.g. `fingo_tenant_<MST>`). In consolidated multi-branch deployments, cross-tenant data queries MUST be physically impossible without explicit consolidation service credentials. | Prevents accidental data cross-contamination between distinct corporate legal entities and satisfies client confidentiality. | Automated integration test asserting database connection routing per tenant session. | Consolidated holding company reporting engine using designated read-only views. |
| **VN-TEN-02** | Tenant-Specific Financial Configuration | Domain & Catalog | System configuration MUST be strictly tenant-scoped: (a) Tax ID (Mã số thuế), (b) Applicable circular (TT133 vs TT99/TT200), (c) Depreciation methods, (d) E-invoice digital certificate and provider credentials, (e) Bank account mappings. | Different legal entities within the same corporate group may operate under different tax incentives and accounting standards. | Unit tests asserting tenant isolation on configuration loading. | None. |

---

# Section 3: Architecture & Integration Standards

## 3.1 Service Boundaries & Integration Mechanics

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-ARC-01** | Modular Monolith In-Process Seams | Entire application | Modules MUST interact via well-defined Go interfaces. Direct inter-package state mutation or circular dependencies between domain packages are FORBIDDEN. Cross-domain workflows (e.g. Sales Invoice $\rightarrow$ Stock Outward $\rightarrow$ GL Voucher) MUST be coordinated by application usecases in `internal/usecase`. | Preserves single-binary desktop deployment simplicity while retaining strict microservice-level modularity. | Linter check (`depguard`) and CodeGraph dependency cycle analysis. | None. |
| **VN-ARC-02** | Outbox Pattern for Asynchronous Integration | External Integrations | External network calls (E-invoice issuance to Tax Authority, Bank API transactions) MUST NOT execute directly within the interactive database transaction. The system MUST persist an event in a `transactional_outbox` table, processed by a background worker with exponential backoff and jitter. | Prevents distributed transaction failures, MariaDB lock contention, and desktop UI freezes when external government portals experience latency. | Failure injection integration tests verifying outbox replay. | Synchronous queries (e.g. checking live tax code validity). |

## 3.2 API & Desktop Communication Design

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-API-01** | Type-Safe Desktop IPC Contract | `app.go`, Frontend | Communication between the Wails desktop frontend (Vue 3/TS) and the Go backend MUST utilize generated type-safe bindings (`wailsjs`). All input payloads MUST be validated using struct validation tags (`go-playground/validator/v10`) before usecase execution. | Eliminates runtime serialization crashes and prevents malformed input from breaching domain boundaries. | TypeScript compilation check (`npm run build`) and Go validation unit tests. | None. |
| **VN-API-02** | Standardized Error Response Structure | IPC & APIs | Errors exposed to the user interface MUST conform to RFC 9457 (Problem Details). Error responses MUST contain: `error_code` (machine-readable, e.g. `ERR_VOUCHER_UNBALANCED`), `message` (localized Vietnamese user message), `field_errors` (optional field validation breakdown), and `trace_id`. | Enables desktop UI to display actionable error dialogs in Vietnamese and allows developers to trace logs via `trace_id`. | Unit tests checking JSON error response structure on usecase failures. | None. |
| **VN-API-03** | Vietnamese Formatting & Locale Standards | Frontend & Export | The system presentation layer MUST adhere to Vietnamese locale standards: (a) Date format: `DD/MM/YYYY` (or `DD/MM/YYYY HH:mm:ss`), (b) Number format: Dot (`.`) for thousands separator and Comma (`,`) for decimal separator (e.g. `1.234.567,89 VND`), (c) Standard currency suffix `đ` or `VND`. | Matches standard Vietnamese accounting conventions and expectations of domestic bookkeepers. | UI component snapshot tests and Excel export validation. | English locale setting for international managers. |

## 3.3 External Integration Patterns

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-INT-01** | Banking Integration & Automated Reconciliation | `domain/cash`, Adapters | Bank integrations (Vietcombank, BIDV, Techcombank, VietinBank, MB) MUST support: (a) Secure statement ingestion (CAMT.053, MT940, and direct open banking APIs), (b) Automated three-way reconciliation matching Transaction Date, Reference/Voucher Number, and Amount against Account 112 ledger records. Mismatches MUST be routed to an unresolved suspense queue. | Eliminates manual statement reconciliation errors and detects unrecorded bank fees or unauthorized transfers. | Table-driven parser tests across bank-specific statement formats. | Manual reconciliation workflow for non-integrated banks. |
| **VN-INT-02** | Statutory Social Insurance Calculation Engine | `domain/payroll` | Payroll statutory deduction calculations MUST strictly enforce mandatory contribution rates: **Employee: 10.5%** (BHXH 8%, BHYT 1.5%, BHTN 1%), **Employer: 21.5%** (BHXH 17%, BHYT 3%, BHTN 1%) plus **Trade Union Fee: 2%** (Kinh phí công đoàn - employer paid). Contributions MUST be capped at 20 times the Statutory Base Wage (*Mức lương cơ sở* per Government Decree). | Vietnam Social Insurance Law and trade union statutory regulations. | Table-driven unit tests verifying insurance deduction caps and employer/employee splits. | Expatriate employees exempt from unemployment insurance configured in employee profile. |

---

# Section 4: Security & Compliance Standards

## 4.1 Authentication & Authorization

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-SEC-01** | Secure Password Hashing & Key Derivation | `domain/system` | User passwords MUST be hashed using `bcrypt` with a minimum cost factor of `12` (or Argon2id). Passwords MUST NEVER be stored in plaintext or reversible encryption. Plaintext passwords MUST be cleared from memory buffers after validation. | Prevents credential exposure in the event of offline database theft. | Unit test verifying hash cost factor and rejection of plaintext passwords. | None. |
| **VN-SEC-02** | Vietnamese Role-Based Access Control (RBAC) | Entire application | System authorization MUST enforce Vietnamese statutory accounting roles: (1) `Kế toán viên` (Bookkeeper - Entry only), (2) `Kế toán trưởng` (Chief Accountant - Approval, Closing, Re-opening), (3) `Giám đốc` (Director - Final financial approval), (4) `Thủ quỹ` (Cashier - Cash register only), (5) `Kiểm toán viên` (Auditor - Read-only all), (6) `Quản trị hệ thống` (Administrator - System & user config only; no voucher posting). | Mandated by internal accounting control regulations (Circular 99/2025/TT-BTC) and Law on Accounting 2015. | Integration tests asserting role permission matrices across all usecase handlers. | Small-business single-user mode documented in system setup. |

## 4.2 Data Protection & Privacy (PDPL & Law 91/2025/QH15)

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-SEC-03** | Compliance with Law 91/2025/QH15 on Personal Data Protection | Entire application | Employee, customer, and counterparty Personally Identifiable Information (PII) MUST be processed in compliance with **Law No. 91/2025/QH15 on Personal Data Protection (PDPL)** (effective 1 Jan 2026). In case of conflict between Right to Erasure and statutory 10-year accounting retention, financial voucher lines MUST retain legal counterparty tax codes while personal contact details (personal phone, private email) MAY be anonymized upon formal request. | Resolves statutory conflict between privacy law and financial accounting retention laws. | Unit tests validating PII masking and redaction in logs and customer registers. | None. |
| **VN-SEC-04** | Cryptographic Storage & Network Security | Database & Transport | Database connections MUST enforce TLS 1.3 encryption in transit. Sensitive fields (bank credentials, digital certificate private keys, tax portal tokens) MUST be encrypted at rest using AES-GCM-256 with keys managed via the OS secure vault (Windows Credential Manager / DPAPI). | Protects corporate financial secrets from unauthorized local extraction or packet sniffing. | Automated test asserting credential vault encryption round-trip. | Local development environment using encrypted `.env` secrets. |
| **VN-SEC-05** | On-Premise Data Sovereignty & Network Security | Deployment & Infra | All customer accounting databases, backups, and e-invoices MUST reside within the territory of Vietnam in compliance with **Decree 53/2022/ND-CP** guiding the Law on Cybersecurity. Data MUST NOT be exfiltrated to external telemetry servers without explicit enterprise consent. | National data sovereignty and cybersecurity legal compliance. | Network boundary audit confirming desktop app transmits data exclusively to approved local LAN DB and domestic tax endpoints. | None. |

## 4.3 Compliance Mapping Matrix

| Standard ID | National Legislation / Circular | Regulatory Requirement | System Enforcement Mechanism |
| :--- | :--- | :--- | :--- |
| **VN-ACC-01** | **Circular 99/2025/TT-BTC** & TT 133/2016/TT-BTC | Standardized Chart of Accounts (Hệ thống TKKT) | `internal/domain/catalog`: Validated hierarchical COA structure. |
| **VN-ACC-02** | **Circular 99/2025/TT-BTC** (IFRS Convergence) | Dual VAS and IFRS accounting records | `internal/domain/gl`: Dual-ledger adjustment voucher pipeline. |
| **VN-ACC-03** | **Luật Kế toán 2015** (Law on Accounting, Art. 16) | Balanced double-entry recording | `internal/domain/gl`: `ValidateBalance()` mandatory check. |
| **VN-FIN-05** | **Luật Kế toán 2015** (Art. 28) & TT 99/2025 | Immutability of posted accounting records | Database privileges & triggers blocking `UPDATE`/`DELETE` on posted rows. |
| **VN-DAT-02** | **Luật Kế toán 2015** (Article 36) | 10-year statutory document retention | Soft deletes only; automated 10-year backup retention schedule. |
| **VN-TAX-01** | **Resolution 204/2025/QH15** & **Decree 174/2025/ND-CP** | Reduced 8% VAT rate application through Dec 2026 | `internal/domain/tax`: Statutory rate matrix and item category rules. |
| **VN-TAX-04** | **Decree 123/2020/ND-CP**, **Decree 70/2025/ND-CP**, **Circular 32/2025/TT-BTC** | E-invoice XML format, digital signing, TCT code | `internal/domain/tax`: Compliant XML canonicalization and lifecycle state machine. |
| **VN-INT-02** | **Luật Bảo hiểm xã hội 2024** | Statutory insurance withholding (10.5% / 21.5%) | `internal/domain/payroll`: `CalculateEmployeeInsurance()` with statutory wage caps. |
| **VN-SEC-03** | **Law No. 91/2025/QH15 (PDPL)** | Personal data privacy & statutory reconciliation | PII masking and redaction pipelines in audit logs and user catalog. |
| **VN-SEC-05** | **Decree 53/2022/ND-CP** | Domestic data residency | On-premise MariaDB storage and local backup architectures. |

---

# Section 5: Observability & Operations

## 5.1 Logging Standards

| Standard ID | Name | Scope | Requirement | Rationale | Verification | Exception Process |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-OBS-01** | Context-Aware Structured JSON Logging | Entire application | All application logging MUST use `fingo/pkg/logger` (wrapping standard library `log/slog`) in JSON format. Every log record MUST include: `time` (RFC3339), `level`, `msg`, `trace_id`, and `user_id`. Financial mutation logs MUST additionally include `entity_id`, `amount`, and `voucher_no`. | Enables centralized log ingestion, audit reconstruction, and incident debugging across distributed client installations. | Unit tests in `pkg/logger/logger_test.go` and code review gate. | Plaintext console output allowed in interactive development mode. |
| **VN-OBS-02** | Sensitive Data & PII Redaction in Logs | All logging | Passwords, PINs, secret keys, bank account full card numbers, and individual personal identification numbers (CCCD) MUST be redacted or masked before logging. Tax codes (*Mã số thuế*) and voucher numbers are NOT classified as secret and SHOULD be logged for auditability. | Cybersecurity compliance and protection against unauthorized credential leaks into diagnostic logs. | Automated log inspection tests asserting sensitive field masking. | None. |

## 5.2 Monitoring, Health & Service Level Objectives (SLOs)

| Standard ID | Name | Scope | Target Metric / SLO | Rationale | Verification |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **VN-SLO-01** | General Ledger Voucher Posting Latency | Use cases | **P99 $\le$ 200 ms** for vouchers up to 100 lines. | Desktop accountants processing fast keyboard-driven data entry expect instantaneous voucher commitment. | Automated benchmark tests: `BenchmarkVoucherPosting`. |
| **VN-SLO-02** | Period-End Automatic Closing Latency | Closing engine | **Execution time $\le$ 30 seconds** for 100,000 journal lines. | Monthly and annual financial closures must execute smoothly without desktop app timeout or database locks. | Performance benchmark testing in `internal/domain/closing`. |
| **VN-SLO-03** | Trial Balance & Statutory Report Generation | Reporting engine | **Execution time $\le$ 15 seconds** for annual trial balance. | Accountants require rapid iterative report previews during tax closing season. | Benchmark gate: `BenchmarkGenerateTrialBalance`. |
| **VN-SLO-04** | E-Invoice XML Signing & Dispatch Latency | E-invoice adapter | **P95 $\le$ 5 seconds** (excluding third-party government network latency). | Point-of-sale checkout and invoice issuance require swift turnaround. | Outbox worker metrics and integration telemetry. |

## 5.3 Incident Response & Disaster Recovery

| Incident Severity | Classification & Definition | Immediate Action Runbook | Post-Incident Target |
| :--- | :--- | :--- | :--- |
| **P1 — Critical** | Financial corruption; unbalanced general ledger; database lockup; system-wide failure to issue e-invoices. | 1. Freeze period mutations.<br>2. Revert to latest verified MariaDB binary snapshot.<br>3. Replay transactional outbox.<br>4. Executive communication within 30 min. | Blameless Root Cause Analysis (RCA) and financial reconciliation report within 24 hours. |
| **P2 — Major** | Tax Authority API endpoint down; bank feed reconciliation failure; statutory report generation error. | 1. Switch e-invoice to queue buffer mode.<br>2. Alert accounting squad on-call.<br>3. Provide manual bank statement fallback parser. | Resolution within 4 hours; patch deployed within 24 hours. |
| **P3 — Minor** | UI layout discrepancy; export formatting flaw; non-critical search performance degradation. | 1. Log diagnostic ticket.<br>2. Route to standard sprint backlog. | Resolved in regular release cycle. |

---

# Section 6: Documentation, Governance & Regulatory Watch

## 6.1 Architectural Decision Records (ADRs)
* All architectural changes, data model modifications, or new technology adoptions MUST be documented as an **Architecture Decision Record (ADR)** under `docs/adr/`.
* ADRs MUST follow the standard schema: `Title`, `Status` (Draft, Accepted, Deprecated, Superseded), `Context`, `Decision`, `Consequences` (Positive & Negative), and `Compliance Impact`.

## 6.2 Vietnamese Accounting Glossary & Technical Mapping

| Vietnamese Accounting Term | Official English Term | Domain Struct / Concept | Technical Definition |
| :--- | :--- | :--- | :--- |
| **Sổ Cái** | General Ledger (GL) | `domain/gl/Voucher` | Master financial record containing all debits and credits posted to accounts. |
| **Chứng từ ghi sổ** | Journal Voucher | `domain/gl/Voucher` | Document authorizing entry into accounting books; header with balanced lines. |
| **Hệ thống tài khoản** | Chart of Accounts (COA) | `domain/catalog/Account` | Hierarchical coding structure for assets, liabilities, equity, revenue, and expenses. |
| **Bảng cân đối số phát sinh**| Trial Balance | `domain/report/TrialBalance` | Statement verifying $\text{Total Debits} == \text{Total Credits}$ across all accounts for a period. |
| **Bảng cân đối kế toán** | Balance Sheet (B01-DNN) | `domain/report/BalanceSheet` | Snapshot of financial position: $\text{Assets} = \text{Liabilities} + \text{Equity}$. |
| **Báo cáo kết quả HĐKD** | Income Statement (B02-DNN) | `domain/report/IncomeStatement`| Profit and loss statement: $\text{Revenue} - \text{Expenses} = \text{Net Profit}$. |
| **Lưu chuyển tiền tệ** | Cash Flow Statement (B03-DNN)| `domain/report/CashFlow` | Statement tracking operating, investing, and financing cash movements. |
| **Số dư đầu kỳ** | Opening Balance | `domain/opening/OpeningBatch` | Baseline account, debt, and stock balances at the start of an accounting period. |
| **Khóa sổ kế toán** | Period Closing / Book Locking | `domain/closing/PeriodClosing`| Procedural freeze preventing further voucher entry prior to a lock date. |
| **Kết chuyển lãi lỗ** | Profit & Loss Transfer | `domain/closing/PeriodClosing`| Automated transfer clearing TK 511/632/642 into TK 911 then into TK 421. |
| **Hóa đơn điện tử** | Electronic Invoice (E-Invoice)| `domain/tax/EInvoice` | Digitally signed invoice XML registered with the General Department of Taxation. |
| **Khấu trừ thuế GTGT** | Deductible VAT Offset | `domain/tax/VATDeclaration` | Offsetting input VAT (TK 133) against output VAT (TK 3331) for tax settlement. |
| **Hao mòn lũy kế** | Accumulated Depreciation | `domain/asset/FixedAsset` | Contra-asset account (TK 214) reducing original cost of fixed assets over time. |
| **Công nợ phải thu (131)** | Accounts Receivable (AR) | `domain/sales`, `Customer` | Subledger tracking customer invoices, debt balances, and payment collections. |
| **Công nợ phải trả (331)** | Accounts Payable (AP) | `domain/purchase`, `Vendor` | Subledger tracking supplier bills, liabilities, and disbursements. |
| **Giá thành sản xuất (154)** | Cost of Production / WIP | `domain/costing/CostCard` | Accumulation of direct materials (621), direct labor (622), and overhead (627). |

## 6.3 Regulatory Watch & Legal Change Management
* **Assigned Watch Role**: Lead Solution Architect & Chief Accounting Specialist.
* **Monitoring Scope**: Ministry of Finance (*Bộ Tài chính*), General Department of Taxation (*Tổng cục Thuế*), State Bank of Vietnam (*Ngân hàng Nhà nước*), and Vietnam Social Insurance (*Bảo hiểm xã hội Việt Nam*).
* **Time-Limited Measures Watchlist**:
  - **8% VAT Rate**: Established under Resolution 204/2025/QH15 & Decree 174/2025/ND-CP. **Expires on 31 December 2026**. The system tax engine MUST automatically transition back to 10% on 2027-01-01 00:00:00 ICT unless extended by the National Assembly.
  - **Circular 99/2025/TT-BTC**: Effective 1 January 2026. Engineering teams MUST ensure full schema deprecation of legacy Circular 200 mapping by Q4 2025.
  - **Decree 70/2025/ND-CP & Circular 32/2025/TT-BTC**: Ongoing audit of e-invoice XML schema conformance.
* **Change Protocol**:
  1. Issuance of official circular/decree $\rightarrow$ Formal technical impact assessment within 5 business days.
  2. RFC authored and reviewed by Solution Architecture Board.
  3. Tax rule engine updated via versioned configuration files with effective start/end timestamps.
  4. Golden file test cases updated and verified by Chief Accounting Specialist.
  5. Deployment of automated database migration and application patch.
