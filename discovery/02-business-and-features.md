# 02 — Business and Features

## 1. What problem the system solves

FinGo is a **desktop bookkeeping application for a Vietnamese accounting practice**: one accountant ("the operator") keeps the statutory books of **N small/medium enterprise clients** under the Vietnamese Accounting System (VAS), on a Windows machine, on-premise. [HIGH — `docs/requirements-v1.md:1-6,57-70`; `GEMINI.md:46-54`]

It is not a generic ERP. It is aimed at the trading-SME cycle: buy goods (often imported), hold and value stock, sell on credit, collect, close the month, file VAT and corporate-income-tax returns and the statutory financial statements, and survive a tax audit.

### Actors

| Actor | Description | Where modelled | Confidence |
| :--- | :--- | :--- | :--- |
| **Operator** (the only human user) | The accountant. Switches between clients, does all entry, posting, closing, reporting. | `docs/requirements-v1.md:61`; string identity in `cmd/fingo/main.go:136-147` (`%USERNAME%`) | HIGH |
| **Client** (unit of books) | A separate legal entity with its own tax code, regime, periods, books. | `internal/client`, `internal/usecase/practice` | HIGH |
| **Practice / client registry** | The list of clients and where each database lives. Holds no financial data. | `internal/client/registry.go` (in-memory only) | HIGH |
| **Tax authority** | Consumer of declarations and e-invoices. | `internal/reports/statutory` | HIGH |
| **Bank** | Statement source / payment confirmation. | **not modelled** — `internal/domain/cash` is a stub | HIGH |
| **Auditor / successor accountant** | Receives the client database on handover. | `docs/requirements-v1.md:66,141-142` | HIGH |

### Deployment model

On-premise Windows desktop, single operator, no concurrent users, local MariaDB, **one schema per client** — with a `client_id` row-scope layer added on top (the two mechanisms are unresolved; see [04](04-pending-work.md)). [HIGH — `docs/requirements-v1.md:68`; `db/migrations/00015_client_scope.sql`]

### Domain glossary

See [08-glossary.md](08-glossary.md). The working language of the domain is Vietnamese accounting terminology (TK, Nợ/Có, kết chuyển, BCTC); the code is English with Vietnamese identifiers only in user-facing strings.

---

## 2. Feature status

Status legend: `DONE` = code + caller + tests and reachable from the shipped binary · `PARTIAL` = engine exists but is not wired to a screen, or a screen exists over an incomplete engine · `STUBBED` = a domain type with no arithmetic and no caller · `BROKEN` = built and observably wrong · `ABANDONED` = deleted/rolled back · `UNKNOWN` = cannot determine from the repository.

**The reachability test used here** is the transitive `fingo/...` import closure of `fingo/cmd/fingo` (the shipped binary). Packages outside that closure are, by definition, not reachable by a user. See [03 §5](03-architecture-and-data.md).

### 2.1 General ledger and posting

| Feature | Status | Evidence | Notes / gaps |
| :--- | :--- | :--- | :--- |
| One-account-one-side journal model (N:M entries) | **DONE** | `db/migrations/00002_journal_line_side.sql`; `internal/domain/gl/gl.go` | Replaces the original debit/credit pair; ADR `docs/adr/0001` |
| Double-entry validation (`Σ Dr == Σ Cr`) | **DONE** | `internal/domain/gl/gl.go` (`ValidateBalance`, `Post`); tests `gl_test.go` | ADR `docs/adr/0002` |
| One-way posting, reversing entries, no edit/delete | **DONE** | `internal/domain/gl/gl.go`; ADR 0002; DB triggers `db/triggers/10..22` | Immutability enforced in domain **and** database |
| Transactional post (voucher + lines + stock + document + idempotency) | **DONE** | `internal/adapter/mariadb/postingrepo/store.go` (`Save`); `internal/adapter/mariadb/glrepo/voucher_store.go` | `sqlc.Queries.WithTx` is used; the review's "not transactional" finding is fixed |
| Idempotent posting (retry-safe) | **DONE** | `internal/idempotency`; `db/migrations/00007_idempotency_keys.sql`; `usecase/ledger` takes a `RequestKey` | `cmd/fingo/main.go:109-134` names the request after the voucher |
| Statutory voucher-number series (`PT`, `PC`, `BH`, …) with gap detection | **PARTIAL** | `db/migrations/00005_voucher_number_series.sql`, `00012_voucher_series.sql`; `db/queries/voucher_numbers.sql` | Tables and queries exist; gap detection not demonstrated |
| Manual journal entry screen (5 free lines, keyboard-first) | **DONE** | `internal/ui/voucher/grid.go` (10 KB + 12 KB tests); `internal/ui/shell/voucher.go`; openable at `cmd/fingo/main.go:426` | Posting path: grid → `usecase/ledger` |
| Chart of accounts (TT133, 123 accounts incl. cấp-2) | **DONE** | `internal/domain/gl/coa_tt133.go`; seeded at boot (`internal/boot`); read-only screen `internal/ui/accounts` | Transcribed from gazette, per file header |
| Chart maintenance (add/rename/deactivate accounts) | **PARTIAL** | `gl.ChartOfAccounts.Add/Update/Deactivate` exist and are tested | No screen; `[NOT FOUND]` production caller |
| Period lock / irreversible close | **PARTIAL** | `internal/period`; `db/migrations/00004,00011`; triggers `50`, `51` | No reopen in v1; no UI to lock; app auto-opens a calendar year (`cmd/fingo/main.go:78-101`) |

### 2.2 The trading cycle (sales / purchase / inventory)

| Feature | Status | Evidence | Notes / gaps |
| :--- | :--- | :--- | :--- |
| Sales invoice: Dr 131 / Cr 511 / Cr 33311, plus COGS 632/156 | **PARTIAL** | `internal/trading/sales.go`; `internal/posting/posting.go:120-142` (`TT133Defaults`) | Engine built and tested; **no screen** — rail entry `sales` is not openable (`cmd/fingo/main.go:426` lacks `sales`) |
| Customer payment + allocation to invoices, overpayment → credit | **PARTIAL** | `internal/trading/sales.go` (`Pay`), `internal/document` (`Settle`), `db/migrations/00009` | No receipt screen; no cash/bank module |
| Credit note (returns) mirroring the invoice | **PARTIAL** | `internal/trading/sales.go` (`Credit`), `00014_arap_documents.sql` | Engine only |
| Purchase order → goods receipt → invoice (three-way match) | **PARTIAL** | `internal/trading/purchase.go` (1,019 lines); `PurchaseAccounts{Payable, InventoryOrExpense, VATInput}` | Engine only; no screen. Purchase accounts are configured, **not** derived from posting rules (documented limitation `GEMINI.md:127`) |
| Landed cost (freight/duty/customs) allocation | **NOT IMPLEMENTED** | `docs/requirements-v1.md:83,121` (in scope); `[NOT FOUND]` in `internal/trading` | A v1-scope item with no code |
| Inventory valuation: moving average + COGS entries | **DONE (engine)** / **PARTIAL (wired)** | `internal/inventory/valuation.go`, `cogs.go`, `internal/domain/inventory`; tests incl. a float-ban test | No warehouse screen; rail `inventory` unavailable |
| FIFO / periodic weighted average | **NOT IMPLEMENTED** | `[NOT FOUND]`; `docs/accounting-review.md:73` flagged it; README claimed it | README claim is false |
| Stocktake adjustment / warehouse transfer | **NOT IMPLEMENTED** | `docs/plan-native-shell.md:292-294` ("no stocktake exists") | Triggers exist for stock movement immutability only |
| Supplier payment / AP allocation | **PARTIAL** | `internal/trading/purchase.go`; `00014` tables | No screen |
| Credit-limit enforcement | **NOT IMPLEMENTED** | `docs/accounting-review.md:111`; `[NOT FOUND]` in `internal/trading` | — |

### 2.3 Cash, bank, payroll, assets

| Feature | Status | Evidence | Notes / gaps |
| :--- | :--- | :--- | :--- |
| Cash receipts/payments (phiếu thu/chi) | **STUBBED** | `internal/domain/cash/cash.go` (no importer outside tests) | Rail says "cần dựng internal/domain/cash" (`internal/ui/nav/rail.go:79-82`) |
| Bank reconciliation / statement import / UNC | **NOT IMPLEMENTED** | rail `bank`; `docs/accounting-review.md:87` | — |
| Payroll: insurance, PIT 5 brackets, net pay | **STUBBED** | `internal/domain/payroll/payroll.go` (unimported); `docs/accounting-review.md:76-77,108-109` | Not merely unwired — the arithmetic was found wrong. See [06](06-risks-and-debt.md) |
| Fixed assets + monthly depreciation | **STUBBED** | `internal/domain/asset/asset.go` (unimported) | Depreciation leakage finding open |
| CCDC / prepaid allocation (TK 242) | **NOT IMPLEMENTED** | `docs/plan-native-shell.md:299-301`; `[NOT FOUND]` | — |
| Expenses / other vouchers | **DONE (generic)** | the "general" journal screen (`cmd/fingo/main.go:383-390`) | Any account pair can be entered; not a typed expense form |

### 2.4 Period close, provisions, reports

| Feature | Status | Evidence | Notes / gaps |
| :--- | :--- | :--- | :--- |
| Close: check outstanding docs → carry 511/6xx/8xx through 911 → 4212 → lock | **PARTIAL (engine)** | `internal/close/close.go` (`ClosePeriod:351`, `provisions.go` 768 lines, 2,300 lines of tests) | No screen, no production adapter ports wired (`GEMINI.md:130`). The app's *practice* client keeps its own month-based close (`usecase/practice`) |
| Bad-debt provision (% AR age bands) | **PARTIAL** | `internal/close/provisions.go`, `DefaultBadDebtPolicy`, `Circular48BadDebtPolicy` | Engine only; Circular 48 expires ≤ 2026-12-31 |
| Inventory write-down (TK 2294) | **NOT IMPLEMENTED** | `[NOT FOUND]` | — |
| Trial balance, per-row voucher trace | **DONE** | `internal/reports/trialbalance.go`; screen `internal/ui/trialbalance`; e2e `fingo/web/e2e_test.go` | Drill-through to the account ledger (`cmd/fingo/main.go:389-400`) |
| Profit & loss | **DONE (engine)** | `internal/reports/profitandloss.go` | Not openable as its own screen; reports screen shows trial balance |
| Balance sheet | **DONE (engine)** | `internal/reports/balancesheet.go` | Same |
| Statutory statements 01/GTGT, 03/TNDN, B01-DN, B02-DN as JSON + PDF | **PARTIAL** | `internal/reports/statutory/{catalogue,service,json,pdf}.go`; `schema.json` | **Deliberately emits no mã số** — warnings instead (`internal/reports/statutory/statutory.go:18-31`) |
| B03-DN cash flow, Sổ nhật ký chung / Sổ cái / Sổ quỹ print forms | **NOT IMPLEMENTED** | `docs/plan-native-shell.md:306-307`; journal *screen* exists but no statutory book print | — |

### 2.5 System, security, multi-client

| Feature | Status | Evidence | Notes / gaps |
| :--- | :--- | :--- | :--- |
| Bundled MariaDB, first-run provisioning, single-instance | **DONE** | `internal/dbhost`, `internal/boot`, `internal/singleton`; `build.ps1`; screenshots `docs/gui-bundled-*.png` | 47.8 MB `dist/` proves it built and ran |
| Connection settings UI (structured fields, saved to `%APPDATA%`) | **DONE** | `internal/appconfig`, `internal/ui/company`, ADR 0005 | — |
| Native shell: rail, header, status bar, 5 sections | **DONE** | `internal/ui/shell`, `internal/ui/nav` | 5 of 17 rail entries openable |
| Authentication / login | **NOT IMPLEMENTED** | `[NOT FOUND]`: no bcrypt/argon2, no session, no user table in the schema; `cmd/fingo/main.go:136-140` says "there is no login" | Reviewer finding open |
| Authorization / RBAC / segregation of duties | **NOT IMPLEMENTED** | `internal/domain/system` has types but is unimported; `docs/accounting-review.md:100-101` | — |
| Audit trail (who posted, when, immutable) | **PARTIAL** | `db/migrations/00003` adds `created_by/posted_by/posted_at`; `cmd/fingo/main.go:141-147` records `%USERNAME%` | No append-only audit-log table; no before/after payload |
| Multi-client registry + switching | **STUBBED** | `internal/client` (in-memory `Registry`, `Scope`, `Middleware`); ADR 0003 | **Unwired**: `client.Scope` appears in no code outside its own package |
| `client_id` on 18 tables | **DONE (schema only)** | `db/migrations/00015_client_scope.sql`; enforced by `db/client_scope_integration_test.go` reading `information_schema` | No repository passes a `Scope` |
| Backup / restore | **PARTIAL** | `internal/migrate/backup.go` (logical dump + manifest + checksums), `restore.go`; `backups/` directory exists and is empty | Backup runs *before a migration*, not on a schedule; no UI |
| Packaging / installer | **NOT IMPLEMENTED** | `docs/plan-native-shell.md:243-246,317-322`; `[NOT FOUND]` installer script | `build.ps1` emits a portable `dist\` only |

### 2.6 Abandoned work

| Thing | Status | Evidence |
| :--- | :--- | :--- |
| .NET 10 / WPF / EF Core / PostgreSQL application (claimed by root README) | **ABANDONED** | `689b20b` "reset" deletes 296 files / 33,621 lines; root `README.md` never updated |
| Next.js web app | **ABANDONED** | `f204026` adds it, `26e968e` reverts it next commit |
| Wails v2 + Vue 3 desktop frontend | **ABANDONED** | `fingo/frontend/**` deleted in the working tree; `fingo/wails.json`, `fingo/app.go` deleted; plan `docs/plan-native-shell.md:80-86` |
| 25-commit Go subledger/RBAC/BRD line | **ABANDONED (branch preserved)** | `backup-before-alpha-rollback` = `f04a09c`, 25 commits, 130 added files, last dated 2026-09-20 20:59 |
| C# / Win32 P/Invoke spikes | **ABANDONED** | `spike/click-row.cs`, `drive-form.cs`, `panes.cs` |

### Feature completion summary

For the shipped application (not the engine), **5 of the 17 destinations the rail advertises can be opened**: `today`, `masterdata` (chart of accounts), `books` (general journal), `reports` (trial balance), `general` (manual journal entry) — `cmd/fingo/main.go:426`. Everything else answers "chưa hỗ trợ" with a reason (`internal/ui/nav/rail.go:54-60`).

---

## 3. Core business flows

### 3.1 First launch → provisioned → books open (`DONE`)

1. Compute install root (`%LOCALAPPDATA%\FinGo` or `FINGO_ROOT`) — `cmd/fingo/main.go:672-686`.
2. Redirect stderr to `fingo.log` inside it, because a GUI has no console — `main.go:256-274`.
3. Acquire a named single-instance kernel object; refuse a second copy — `internal/singleton`.
4. `boot.Ensure` probes then creates, in order: data directory → start bundled `mariadbd` → database → migrations → TT133 chart — `internal/boot/boot.go:62-104`.
5. `appconfig.Store` decides the connection: saved `config.json` **wins**, `.env` is the fallback; an unreadable settings file is an error, never a silent fallback — ADR 0005.
6. `books.Open` assembles one client's ledger + reports from one source — `internal/books/books.go:43-65`.
7. If no accounting period exists, open the calendar year — `main.go:78-101`.
8. Show the shell; status line reports provisioning progress.

### 3.2 Manual journal entry → posted voucher (`DONE`)

1. Rail `general` opens the fixed-row entry form — `main.go:383-390`.
2. `voucher.EntryGrid` holds the rules: whole đồng, account must be a postable leaf in the chart, side, running totals, posting gate — `internal/ui/voucher/grid.go:79`.
3. `grid.Voucher(id, date, description)` builds a domain voucher.
4. `usecase/ledger.Post` is the only path to a posted voucher; the posting request key is the voucher id — `main.go:109-134`.
5. `gl.Voucher.Post` refuses an unbalanced voucher and refuses a second post — ADR 0002.
6. `postingrepo.Store.Save` writes voucher, lines, stock and document in **one transaction**.
7. DB triggers reject UPDATE/DELETE of the posted row — `db/triggers/10..22`.

### 3.3 Sales cycle (`PARTIAL` — engine complete, no screens)

1. Price lines: one pricing path for both cycles, VAT resolved through a `vat.Registry` by `VATRateID` (never a percentage), integer minor units — `internal/trading/pricing.go:47`.
2. `Sales.Invoice` posts Dr 131 (AR) / Cr 5111 (revenue) / Cr 33311 (output VAT) — `internal/posting/posting.go:120-142`.
3. COGS is posted in the same transaction: Dr 632 / Cr 156, cost from the inventory valuation engine.
4. Records an AR document (`arap_documents`), so the receivable is an object with `Allocated` and `Status`.
5. `Pay` posts Dr cash-or-bank / Cr 131 and allocates; overpayment becomes a **credit**, never an error — `internal/trading/sales.go` + `internal/document.Settle`.
6. `Credit` mirrors the invoice's postings, posts them, and allocates the credit back.

### 3.4 Purchase cycle (`PARTIAL`)

1. `Order` prices and records the promise — **posts nothing** ("a promise is not an event").
2. `Receive` moves goods into inventory at the ordered price — posts nothing ("the bill is what debits inventory").
3. `Invoice` posts Dr inventory-or-expense / Dr 1331 input VAT / Cr 331 AP, and reports a three-way mismatch when PO ≠ GRN ≠ invoice.
4. `Pay` posts Dr 331 / Cr bank and allocates against the bill.

### 3.5 Period close (`PARTIAL` — engine, no screen)

1. Refuse to close a period with documents still owed, or force it with a recorded reason — `internal/close/close.go:343-360`.
2. Refuse out-of-order closes (`ErrEarlierPeriodOpen`); refuse a 911 that already carries a balance.
3. Optional provisions: straight-line depreciation `(cost − residual)/useful_life_months`, capped at the base; bad-debt `%` of AR age bands; the target delta is **not** modelled (documented, not hidden) — `internal/close/provisions.go`.
4. Every account is checked against the chart **before** anything is written; the expense must be a P&L leaf, the accumulation must not be.
5. Carry revenue/expense through **TK 911** into **TK 4212** (profit) or 4211 (loss) — `close_test.go:394-419`.
6. Reduce every P&L account to zero; write the report snapshot **before** the entries so a retry is safe; request key `period-close:<id>`.
7. Lock the period range; the lock is irreversible, enforced by triggers 50/51.

### 3.6 Reporting (`DONE` for 3 reports)

Every figure is summed from **posted voucher lines**; there are no cached balances. A row exists only because a line touched the account, and carries `SourceVoucherCount` so no figure is stated without a voucher behind it — `GEMINI.md:133`; `internal/reports/`.

- Trial balance rejects: an account not in the chart, a result account not in the chart, a line with no voucher behind it, a line with neither debit nor credit, a negative amount.
- P&L excludes closing vouchers **whole** and reports how many it excluded.
- Balance sheet carries an unclosed period's result into equity so `Assets = Liabilities + Equity` holds before and after a close.

---

## 4. Key business rules found in code

| # | Rule | Where | Confidence |
| :--- | :--- | :--- | :--- |
| R1 | A journal line is **one account, one side, one amount**; a voucher must net to zero | `internal/domain/gl/gl.go`; `00002` | HIGH |
| R2 | VND is integer đồng; no float ever touches an amount | `internal/money` (minor units), `internal/domain/*` (decimal); an inventory test asserts no `float32/float64` in the package | HIGH — `GEMINI.md:64-66` |
| R3 | VAT is computed per line, rounded **half away from zero**, in basis points, using `big.Int` to avoid overflow; the document total is the sum of rounded lines | `internal/vat/vat.go:147-180` | HIGH |
| R4 | Allocation is largest-remainder so the parts sum to the total exactly | `internal/money/allocate.go:21` | HIGH |
| R5 | Posted vouchers are immutable; corrections are counter-entries; there is no unpost/edit/delete | ADR 0002; triggers 10–22 | HIGH |
| R6 | A locked period accepts no posting, and the lock cannot be undone in the database | `00011`; triggers 50/51 | HIGH |
| R7 | A payment cannot settle more than a document owes (domain **and** trigger 30) | `db/triggers/30_allocations_do_not_exceed_document.sql` | HIGH |
| R8 | Negative stock is an error, not a warning | `internal/inventory` (`ErrNegativeStock`) | HIGH |
| R9 | A duplicate partner tax code is a **warning**, not a refusal | `internal/partner`; `internal/trading` `Warning` | HIGH — `GEMINI.md:120` |
| R10 | Closing accounts: 911 → 4212 on profit, 4211 on loss; P&L accounts must end at zero | `internal/close/close.go`; `close_test.go:394-457` | HIGH |
| R11 | Boot refuses contradictory posting rules rather than guessing | `internal/posting` `Load` | HIGH — `GEMINI.md:128` |
| R12 | A settings file that exists but cannot be read is an **error, not a fallback** | ADR 0005; `internal/appconfig` | HIGH |
| R13 | All instants are recorded in UTC; the DB connection is pinned to `loc=UTC` because a local-time connection silently shifts every DATE column by a day | `fingo/.env:8-18` comment (documents a real found bug) | HIGH |
| R14 | A `client.Scope` zero value means *nobody*, never *everybody* | `internal/client`; ADR 0003 | HIGH |
| R15 | A statutory line the legal register marks `?` is **never emitted**; it becomes a warning | `internal/reports/statutory/statutory.go:18-31`; `docs/legal-basis.md:131-163` | HIGH |

---

## 5. Reports, printing, import/export, integrations

| Capability | State | Evidence | Confidence |
| :--- | :--- | :--- | :--- |
| Trial balance (bảng cân đối số phát sinh) | **Implemented**, screen + e2e | `internal/reports/trialbalance.go`; `internal/ui/trialbalance`; `fingo/web/e2e_test.go` | HIGH |
| P&L (B02-DN equivalent) | Implemented as service, rendered in the HTML harness | `internal/reports/profitandloss.go`; `web/templates/profit_and_loss.html` | HIGH |
| Balance sheet (B01-DN equivalent) | Implemented as service, rendered in the HTML harness | `internal/reports/balancesheet.go`; `web/templates/balance_sheet.html` | HIGH |
| 01/GTGT VAT return JSON | Implemented, **without mã số** | `internal/reports/statutory/{catalogue,service}.go` | HIGH |
| 03/TNDN CIT return JSON | Implemented, **without mã số** | same | HIGH |
| PDF archive of statements | Implemented, hand-rolled PDF, Vietnamese transliterated (standard PDF fonts lack Vietnamese) | `internal/reports/statutory/pdf.go`; test `pdf_test.go` | HIGH |
| Print templates / Excel export | **NOT FOUND** | `[NOT FOUND]`: no xlsx/csv writer, no print templates for statutory books. `docs/plan-native-shell.md:371-373` plans `ShellExecute` on the PDF | HIGH |
| E-invoice integration | **NOT IMPLEMENTED** | rail `einvoice`; `internal/domain/tax` stub only | HIGH |
| Bank statement import | **NOT IMPLEMENTED** | `[NOT FOUND]` | HIGH |
| Tax-authority submission (HTKK XML) | **NOT IMPLEMENTED** | root `README.md:87-89` claims it; no code | HIGH |
| Excel/CSV opening-balance import | **ABANDONED** | `README` and old .NET track mention `IOpeningBalanceExcelParser`; no Go equivalent | HIGH — `[NOT FOUND]` in `fingo/` |
| HTTP API for third parties | **None shipped** | `internal/idempotency` is middleware with no server; `web.Server` is not mounted (`GEMINI.md:117`) | HIGH |

**Integration conclusion:** the system integrates with nothing. It is a self-contained desktop binary plus a locally bundled database. [HIGH]
