# FinGo v1 — Requirements Baseline

**Status:** draft for correction. **Author:** lead BA. **Date:** 2026-10-08.
**Inputs:** regime = TT133 · users = one operator serving N client companies · quality = "works perfectly in production".

> **Read this first.** Section 1.1 changes the data model. If I have misread the business, stop here and correct me before anything downstream is built.

---

## 1. What your three answers actually changed

### 1.1 One client = one database. This is a practice, not a company.

Your clarification — *one accountant, N customers, one customer has only one database* — settles the isolation model, and **it corrects my previous draft.**

I had recommended a `company_id` column on every financially-scoped table. **That was wrong for this model.** If each client's books live in their own database, the database **is** the tenant boundary. Adding row-level scoping on top would be a weaker, more error-prone duplicate of a boundary you already have.

| Current state | What the business requires |
| :--- | :--- |
| `GetCompanyProfile(ctx)` returns **one** company ([`system.go:91`](../fingo/internal/domain/system/system.go#L91)) | **One database per client**, each with its own profile, tax code, COA, periods and lock dates |
| `CREATE DATABASE IF NOT EXISTS fingo; USE fingo;` — one hardcoded schema ([`migration:4-6`](../fingo/db/migrations/00001_init_fingo_schema.sql#L4-L6)) | **N client databases** with identical schema, plus a registry naming them |
| `AccountingStandard` global and inert | **Per-database regime** — per-client configuration, not a per-row column |
| Nothing knows a client list exists | A control layer: which clients exist, where each database lives, which regime each uses |

**Database-per-client is the better design for this business**, and it's worth stating why:

- **Isolation is structural.** Cross-client leakage would require connecting to the wrong database. It cannot result from a forgotten `WHERE` clause — the failure mode simply doesn't exist.
- **Handover is trivial.** A client leaving takes their database with them.
- **Backup and restore are per client.** One corrupt database does not endanger the other forty-nine.
- **Blast radius is one client.** A migration that fails fails on the database it ran against.
- **Different regimes coexist cleanly** — one client on TT133, another on TT99, another micro, each in their own schema.

**What it costs — and these become requirements:**

- **Client switching is a connection concern**, not a filter. That is the whole safety model, so it must be the *only* way to reach books.
- **Every migration runs across N databases** and must be version-tracked per database, with a dry run and a per-database failure report.
- **The connection layer is now the critical control.** A report pointed at the wrong connection is still a confidentiality event. Structural isolation means the wrong connection is *hard to reach*, not that it is impossible — so the access path must be narrow and enforced in one place.

> **One thing to confirm.** I read "1 customer has only 1 db" as one database per client. If you *also* mean there is no shared or aggregate database at all — no practice-level rollup across clients — then say so, because that permanently removes any cross-client reporting from the product.

### 1.2 "TT133" is a default, not a constant

I'll take TT133 as the target regime. **But a service practice's client list rarely observes that boundary.** TT133 covers small and medium enterprises. Your clients may include:

- **micro enterprises** → **TT 58/2026/TT-BTC** (in force from 01/07/2026, requires **no chart of accounts at all** — books only, two statements)
- **general-regime entities** → **TT 99/2025/TT-BTC**
- **household businesses** → TT 152/2025 and NĐ 68/2026

**Confirm: do you serve only TT133 SMEs?** If not, regime must be a **per-company attribute from the first migration**, even if v1 only *implements* TT133. The field and the seam cost nothing now; the retrofit costs a data migration and a rewrite of every statutory output.

### 1.3 "Works perfectly in PROD" is not yet testable

As stated it can't be verified, so it can't be built to. Section 5 translates it into testable requirements. The one clause I need you to settle: it does **not** answer *which fiscal year you must be live for*. "Production-ready" and "ready before 31/12/2026" are very different projects — the second pulls year-end provisions into v1 and races a statutory deadline.

---

## 2. Actors and context

| Actor | Description |
| :--- | :--- |
| **Operator** *(the only human user)* | The accountant running the practice. Switches between clients; does all data entry, review, posting, closing and reporting for every client. |
| **Client** *(the unit of books)* | A separate legal entity with its own tax code, regime, periods and books — stored in **its own database**. The operator serves N of these. |
| **Client registry** *(the control layer)* | The practice-level list: which clients exist, where each database is, which regime each uses, engagement status. Holds no financial data of its own. |
| **Tax authority** | Consumer of e-invoices and periodic declarations. |
| **Bank** | Source of statements and payment confirmations. |
| **Client's auditor / successor accountant** | Receives the client's database on handover. |

**Deployment:** on-premise Windows desktop, single operator, no concurrent users. Local database server; one schema per client.

---

## 3. Transaction inventory — the trading cycle

This is the core deliverable the journal-model decision must be derived from. Every row below is a real business event, and **every posting is N:M** — three or more accounts per entry. That is what makes [`VoucherLine`](../fingo/internal/domain/gl/gl.go#L21) unusable, and it is a *finding from the business*, not an architectural preference.

> **Account numbers are provisional.** They are pre-TT46/2025 TT133 references and are recorded as open question `Q3` in [`legal-basis.md`](legal-basis.md). Verify the TT133 COA as amended before any of this is hard-coded.

| # | Event | Trigger | Document | Posting (TT133, provisional) | Dependency |
| ---: | :--- | :--- | :--- | :--- | :--- |
| 1 | **Client onboarding** | New client engaged | Profile + opening balance sheet | Opening balances → COA; must balance | Per-client COA seed; regime |
| 2 | **Cash purchase** | Goods bought, paid now | Hóa đơn + phiếu chi | Nợ 156 / Nợ 1331 / Có 111 | VAT rate; e-invoice |
| 3 | **Credit purchase** | Goods received, on terms | Hóa đơn + phiếu nhập kho | Nợ 156 / Nợ 1331 / Có 331 | AP allocation; payment terms |
| 4 | **Import purchase with landed cost** | Customs cleared | Tờ khai + freight + duty | Nợ 156 *(cost + freight + duty)* / Nợ 1331 / Có 331 / Có 111 | Landed-cost allocation method; FX rate |
| 5 | **Purchase return** | Goods rejected | Adjustment invoice | Nợ 331 / Có 156 / Có 1331 | Adjustment, **not** cancellation |
| 6 | **Cash sale with VAT** | Customer pays now | Phiếu thu + e-invoice | Nợ 111 / Có 511 / Có 33311 | VAT rate; **COGS** Nợ 632 / Có 156 |
| 7 | **Credit sale with VAT** | Goods delivered | Hóa đơn + phiếu xuất kho | Nợ 131 / Có 511 / Có 33311 | VAT rate; **COGS**; credit limit |
| 8 | **Sales return / adjustment** | Customer rejects | Adjustment invoice | Nợ 521 / Nợ 33311 / Có 131; goods back to stock | Adjustment only — "hủy" abolished |
| 9 | **Customer receipt** | Bank credit / cash | Giấy báo có / phiếu thu | Nợ 112 or 111 / Có 131 | **Allocation to specific invoices** |
| 10 | **Supplier payment** | Payment instruction | Ủy nhiệm chi | Nợ 331 / Có 112 | Allocation; **idempotency** |
| 11 | **Warehouse transfer** | Internal movement | Phiếu điều chuyển | No GL entry — stock ledger only | Per-warehouse valuation |
| 12 | **Stocktake adjustment** | Physical count differs | Biên bản kiểm kê | Nợ/Có 632 ↔ 156 | Approval trail |
| 13 | **Expense payment** | Cost incurred | Phiếu chi / UNC | Nợ 642 *(or 641/635)* / Nợ 1331 / Có 111 or 112 | Expense account mapping |
| 14 | **Payroll** | Month end | Bảng lương | Nợ 642 / Có 334; Nợ 334 / Có 3383/3384/3386/3382/3335; Nợ 334 / Có 111 | Insurance base + caps; **PIT 5 brackets** |
| 15 | **Fixed asset** | Asset acquired | Hóa đơn | Nợ 211 / Nợ 1331 / Có 331; monthly Nợ 642 / Có 214 | Depreciation cap at cost |
| 16 | **Period close** | Month end | — | Nợ 511 / Có 911; Nợ 911 / Có 632, 642…; balance → 4211/4212 | Per-company lock date |
| 17 | **Year-end provisions** | FY close | Bảng trích lập | Nợ 642 / Có 2293 *(receivables)*; Nợ 632 / Có 2294 *(inventory)* | **Circular 48/2019 expires ≤31/12/2026** |
| 18 | **VAT declaration prep** | Period end | 01/GTGT + 8% appendix | Derived from posted vouchers only | TT 89/2026 forms; 8% window to 31/12/2026 |

**Observation that settles ADR-0001.** Of the eighteen events, **sixteen post three or more accounts**. There is no version of this business that fits a debit-account/credit-account pair with one shared amount. The journal model is not a judgement call — it is forced by rows 2, 3, 4, 6, 7, 8, 9, 10, 13, 14, 15, 16 and 17.

## 4. Scope — v1 in / out

**In scope (v1):**

- **Practice container**: a client registry, one operator, **N client databases** — each with its own COA, regime, periods and lock dates
- **Client switching** through the connection layer, with isolation enforced in exactly one place
- Client onboarding: company profile + balanced opening balances
- The trading cycle: purchase, import purchase with landed cost, sale (cash and credit), returns, receipts and payments **with invoice-level allocation**
- Inventory: perpetual valuation with COGS posted on sale; stocktake adjustment
- Payroll: insurance with correct base and caps, PIT on 5 brackets, net pay
- Expense capture, fixed-asset purchases and depreciation
- Period close, year-end provisions
- Statutory outputs: B01-DN / B02-DN / B03-DN equivalents under the elected regime, plus the statutory books
- E-invoice issue, adjust and replace
- Per-company backup, restore, and **client data export on handover**

**Explicitly out of scope (v1):**

| Excluded | Reason |
| :--- | :--- |
| `costing` — TK 621/622/627 production cost (the whole module) | A trading SME does not run it. Currently built, unused, and it is the clearest example of the scope inversion in this repo. |
| Multi-user, concurrent access, approval workflow by different people | Single operator by definition. *(Audit trail still required — see 5.6.)* |
| Multi-currency / FX revaluation | Only if a client imports on non-VND terms. **Confirm** — row 4 assumes a rate exists. |
| Consolidation, intercompany elimination | One operator, separate legal entities; no group reporting stated. |
| TT58/2026 micro and TT152/2025 household **implementation** | Regime **field** present from day one; implementation deferred. |
| IFRS (TT 118/2026) | Not applicable. |
| Cloud / multi-device sync | On-premise desktop. |

## 5. "Perfect in production" — translated

Each of these is testable. This is what "perfect" has to mean, or it means nothing.

| # | Requirement | Acceptance test |
| ---: | :--- | :--- |
| 5.1 | **No partial postings.** A voucher and all its lines commit or none do | Kill the process mid-post; ledger contains no orphan header or unbalanced voucher |
| 5.2 | **No fractional đồng.** All VND amounts are integers | Round-trip every posting; no fractional amount stored |
| 5.3 | **Payments are idempotent** | Submit the same receipt twice; ledger unchanged, second attempt reported as duplicate |
| 5.4 | **Client isolation is structural** — a client's books are reachable *only* through that client's database connection | Automated test: no code path can issue a financial query without an explicit client connection; there is exactly one access route, and it cannot be bypassed |
| 5.5 | **Client switching is fast** | Switch client in <1 s with 50 clients registered |
| 5.6 | **Per-client audit trail.** Every create/post/void attributed and timestamped in UTC | Any voucher's full history reconstructable; entries immutable once posted |
| 5.7 | **Backup and restore are proven, per client database** | Restore one client's database into a clean install and reproduce their trial balance exactly; every other client is untouched |
| 5.8 | **Client handover export** | Hand over one client's complete database, usable by a successor accountant |
| 5.9 | **Migrations apply across all client databases, version-tracked per database** | With N clients: upgrade all; a failure on one leaves it on the old version, corrupts nothing, and is reported. Dry run available before commit. |
| 5.10 | **Deployable on a clean Windows machine** | Installer runs; no manual database steps; single-command migration |
| 5.11 | **Deterministic close** | Running period close twice produces no duplicate entries |
| 5.12 | **Errors are recoverable and legible** | Every failure surfaces an actionable message; no silent catch-and-continue |
| 5.13 | **Registry integrity** | A registry entry pointing at a missing or unreadable database degrades that one client only — the practice remains usable |

## 6. What this determines for ADR-0001

The journal model is now **derived, not chosen**:

1. One line = **one account**, one side, one amount — because sixteen of the eighteen events post three or more accounts each.
2. **No `company_id` column.** Tenant scope is the **database connection**, not a row filter. Duplicating the boundary in every row would add a second, weaker source of truth.
3. Money as **integer đồng** — because VND has no subunit.
4. A `posted_at` / `posted_by` / immutability guarantee — because a practice is professionally liable for the books it keeps.
5. An idempotency key on payments — because row 10 will be retried.
6. **Schema identical across all client databases**, so one migration set applies to every client and no schema drift is possible.
7. **The connection layer is the single access route to books** — a capability that cannot be passed to code that has no business holding it.

## 7. Open questions

| # | Question | Blocks |
| ---: | :--- | :--- |
| 1 | **Is there any practice-level rollup at all?** One database per client is confirmed — but do you ever need a combined view across clients (fees, workload, total revenue)? | Whether the registry stays purely administrative or must carry financial aggregates |
| 2 | **Database topology:** one database server holding N client schemas, or N separate instances/files? | Installer design, backup tooling, portability, and whether a client's database can be copied to a USB stick |
| 3 | **Do you serve only TT133 SMEs?** Or micro / general-regime / household clients too? | Depth of the per-client regime seam and the statutory output set |
| 4 | **Which fiscal year must this be live for?** "Works perfectly in production" is not a date | Whether year-end provisions, and the expiring Circular 48/2019 rules, are v1 |
| 5 | **Any import or non-VND client?** | Whether multi-currency is v1 or deferred (inventory row 4 assumes a rate exists) |
| 6 | **How many clients, and how many vouchers per client per month?** | Backup strategy, switching performance, migration batching |
| 7 | **Handover format** — when a client leaves, what exactly do they receive? A database file? A printed set? An export? | Scope of 5.8 |
| 8 | **TT133 COA as amended by TT 46/2025** — still unresolved (`Q3` in legal-basis.md) | Every account number in §3 is provisional until this is closed |

---

## What this document does *not* do

It does not specify the journal model, the schema, or any interface — that is ADR-0001, and this is its input. It also does not close `Q3`; every TT133 account number above remains provisional until the amended chart of accounts is read from primary text.

**Nothing has been implemented.** This is a requirements baseline for your correction.
