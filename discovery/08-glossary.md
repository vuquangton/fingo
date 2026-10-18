# 08 — Glossary

Two vocabularies meet in this repository: **Vietnamese statutory accounting** and **the project's own architectural shorthand**. Both are used in code comments, often inside the same sentence. This file exists so a new engineer can read either.

---

## 1. Vietnamese accounting terms (the domain)

| Term | Literal / expansion | Meaning in this system | Where it appears |
| :--- | :--- | :--- | :--- |
| **VAS** | Vietnamese Accounting System (Chế độ kế toán Việt Nam) | The statutory accounting framework. | `README.md:2`; `GEMINI.md:47` |
| **TK** | Tài khoản (account) | An account code, e.g. "TK 131". The chart has 123 entries in 7 classes. | `internal/domain/gl/coa_tt133.go` |
| **Nợ / Có** | Debit / Credit | The two sides of a journal line. A line is one account, one side, one amount. | `gl.EntrySide` `DEBIT`/`CREDIT` |
| **Bút toán** | Journal entry / posting | One accounting movement set. | migrations ("Bút toán định khoản") |
| **Chứng từ** | Voucher / source document | The document that justifies a posting (invoice, receipt, payment order). | `vouchers`, `voucher_lines` |
| **Sổ** | Book / register | A statutory book (journal, ledger, cash book). | `sổ cái`, `nhật ký chung`, rail "Sổ sách" |
| **Sổ nhật ký chung** | General journal | Chronological list of all vouchers. The `books` screen. | `internal/ui/journal` |
| **Sổ cái** | General ledger | Per-account movements. Reached by drilling from the trial balance. | `internal/ui/ledger`; `cmd/fingo/main.go:339-351` |
| **Bảng cân đối số phát sinh** | Trial balance | Per-account opening/debit/credit/closing. The `reports` screen. | `internal/reports/trialbalance.go` |
| **BCTC** | Báo cáo tài chính (financial statements) | The statutory statement set. | `internal/reports/statutory` |
| **B01-DN / B02-DN / B03-DN** | Statement of financial position / income statement / cash flows | The three SME statements. B01 and B02 are modelled; **B03 is not** (no cash-flow classification exists). | `internal/reports/balancesheet.go`, `profitandloss.go`; `plan-native-shell.md:306` |
| **Kết chuyển** | Closing transfer / carry-forward | Moving revenue and expense balances through TK 911 into retained earnings at period end. | `internal/close/close.go` |
| **Khóa sổ** | Period lock / book closing | Irreversible lock of a date range. | `accounting_period_locks`; triggers 50/51 |
| **Mã số chỉ tiêu** | Indicator number | The numbered line on a statutory form. **The engine emits none** — this is the project's hardest external blocker. | `statutory.go:96-118`; `legal-basis.md` Q2 |
| **Giá vốn** | Cost of goods sold | TK 632. Posted per sale from the valuation engine. | `internal/inventory/cogs.go`; `posting.go:137-141` |
| **Hàng tồn kho** | Inventory | TK 152/153/155/156. | `internal/inventory` |
| **Dự phòng** | Provision / allowance | Bad-debt (TK 2293) and inventory write-down (TK 2294). Receivables implemented; inventory **not**. | `internal/close/provisions.go` |
| **CCDC** | Công cụ, dụng cụ | Tools & small equipment (TK 153/242), allocated over time. Not implemented. | rail `tools` |
| **TSCĐ** | Tài sản cố định | Fixed assets (TK 211/214). Stubbed only. | `internal/domain/asset` |
| **Khấu hao** | Depreciation | Straight-line / accelerated, capped at cost − residual. Engine only. | `close/provisions.go` |
| **GTGT** | Giá trị gia tăng (VAT) | Output TK 33311, input TK 1331. Rates in basis points. | `internal/vat` |
| **TNDN** | Thu nhập doanh nghiệp (corporate income tax) | Form 03/TNDN. CIT itself is **not modelled** (no TK 821/3334 posting). | `statutory`; `accounting-review.md:75` |
| **TNCN** | Thu nhập cá nhân (personal income tax) | PIT withholding, 5 brackets, deductions 15.5M/6.2M per the legal register. **Not implemented.** | `legal-basis.md:192-213` |
| **BHXH / BHYT / BHTN** | Social / health / unemployment insurance | Employee + employer contributions; rates and caps in `legal-basis.md:239-260`. Stubbed and currently wrong. | `internal/domain/payroll` |
| **BHTNLĐ-BNN** | Occupational accident & disease insurance | 0.5% employer contribution. | `legal-basis.md:279-286` |
| **KPCĐ** | Kinh phí công đoàn (trade-union fee) | 2% employer. **Not modelled** — one of the three payroll defects. | `legal-basis.md:279-286` |
| **UNC** | Ủy nhiệm chi | Bank payment order. Not implemented. | rail `bank`; `requirements-v1.md:89` |
| **Phiếu thu / Phiếu chi** | Receipt / payment voucher | Cash documents (forms 01-TT / 02-TT). Stubbed. | `internal/domain/cash` |
| **Hóa đơn điều chỉnh / thay thế** | Adjustment / replacement invoice | The only permitted corrections now. **Cancellation ("hủy") was abolished** — the model still has a `Canceled` status. | `accounting-review.md:17`; `internal/domain/tax` |
| **Đối chiếu sổ phụ** | Bank statement reconciliation | Not implemented. | rail `bank` |
| **Hàng mua đang đi đường** | Goods in transit | TK 151. Present in the chart; no flow uses it. | `coa_tt133.go:91` |
| **Chi phí trả trước** | Prepaid expenses | TK 242. Not allocated by any code. | `coa_tt133.go:120` |
| **Lương cơ sở / lương tối thiểu vùng** | Statutory pay base / regional minimum wage | Caps for insurance; figures in `legal-basis.md:253-260`. | — |

### The account codes you will actually meet

| Code | Name | Why it matters here |
| :--- | :--- | :--- |
| 111 / 112 | Cash / bank | Debit side of receipts; credit side of payments |
| 131 | Phải thu của khách hàng (AR) | Debit on a credit sale; settled by a receipt |
| 1331 | Input VAT deductible | Debit on a purchase |
| 156 | Hàng hóa (inventory) | Debited by a purchase, credited by COGS |
| 211 / 214 | Fixed asset / accumulated depreciation | Contra pair |
| 2293 / 2294 | Bad-debt / inventory provisions | Contra-asset; closed into expense |
| 242 | Prepaid expenses | Allocated monthly (unbuilt) |
| 331 | Phải trả cho người bán (AP) | Credit on a purchase |
| 33311 | Output VAT | Credit on a sale |
| 334 / 338x | Payroll payable / insurance payables | Payroll postings (unbuilt) |
| 4211 / 4212 | Retained earnings — loss / profit | Where the close lands |
| 5111 | Sales revenue — goods | Credit on a sale |
| 632 | Cost of goods sold | Debit per sale |
| 6421 / 6422 | Operating expense | Default expense accounts in TT133 |
| 911 | Xác định kết quả kinh doanh | Profit-determination clearing account; must end at zero |

### Legal instruments named in the repo

| Short form | Full form | Status in the project |
| :--- | :--- | :--- |
| **TT 133/2016** | Circular 133/2016/TT-BTC (SME accounting regime) | The **declared target regime**; chart transcribed from its Phụ lục 1 |
| **TT 46/2025** | Circular 46/2025/TT-BTC | Amends TT133 (Điều 10); "no account added, renamed or removed" |
| **TT 99/2025** | Circular 99/2025/TT-BTC (general regime) | In force FY2026; **replaces TT 200/2014**, not TT133 |
| **TT 58/2026** | Circular 58/2026/TT-BTC (micro enterprises) | In force 2026-07-01; no chart of accounts at all |
| **TT 152/2025 + NĐ 68/2026** | Household-business regime | Out of scope |
| **TT 118/2026** | IFRS track for international financial centres | Effective 2027; out of scope |
| **TT 89/2026** | Return forms (01/GTGT, 03/TNDN, …) | The source of the missing `mã số` |
| **Circular 48/2019** | Provisioning for CIT deductibility | In force, **expires ≤ 2026-12-31** |
| **NQ 204/2025** | Resolution reducing VAT to 8% | The 8% window the engine models |
| **Luật 108/2025, NĐ 252/2026, NĐ 254/2026, TT 91/2026** | New tax-administration layer (from 2026-07-01) | Recorded in the register; not implemented |
| **HTKK** | Tax-authority return/XML tool | README claims XML export; **no code** |

---

## 2. Project-internal names

| Name | Meaning | Evidence |
| :--- | :--- | :--- |
| **FinGo** | The product. ("Fin" + "Go", the language.) | `GEMINI.md:48` |
| **Operator** | The single human user: the accountant who runs the practice | `requirements-v1.md:61` |
| **Client** | The unit of books: a legal entity whose books the practice keeps. Also called "customer" in the UI (rail "Khách hàng"). | `internal/client`; `requirements-v1.md:62` |
| **Practice** | The accounting firm itself; the container of N clients. **Not modelled** as an entity. | `requirements-v1.md:63`; `internal/usecase/practice` |
| **Books** | One client's ledger plus the reports over it. `internal/books` is the assembly point. | `internal/books/books.go:1-12` |
| **Chart** | The chart of accounts (`gl.ChartOfAccounts`). | `internal/domain/gl/account.go` |
| **Postable / leaf** | An account that can receive a posting (not a heading). | `gl.IsPostable` |
| **Side** | `DEBIT` / `CREDIT`, one per line. | ADR 0001 |
| **Voucher** | A posting document: header + lines, immutable once posted. | ADR 0002 |
| **Reversal / counter-entry** | The only way to correct a posted voucher: a mirrored, posted voucher. | ADR 0002 `Reverse` |
| **Scope** | A value naming which client a query is for; the zero value means nobody. | ADR 0003 |
| **Period** | A date range (`period.Period`) — not a (year, month) pair. | `internal/period` |
| **Close** | The 911→4212 run plus the lock. | `internal/close` |
| **Provisions** | Depreciation + bad debt folded into the closing voucher. | `close/provisions.go` |
| **Landed cost** | Freight/duty added to inventory cost. **Named but not built.** | `requirements-v1.md:83` |
| **Three-way match** | PO vs goods receipt vs supplier invoice. | `trading/purchase.go` |
| **Allocation / settlement** | Applying a payment to a document; overpayment becomes a **credit**, never an error. | `internal/document.Settle` |
| **Engine vs skeleton** | The project's own terms: `internal/*` = the engine (built slice by slice); `internal/domain/*` = the original skeleton (partly dead). | `GEMINI.md:94-96` |
| **Wired / unwired** | "Wired" = imported by non-test production code. "Unwired" = exists and is tested but has no caller. This is the single most load-bearing distinction in the codebase. | `GEMINI.md:115,117,130` |
| **Slice / vertical / refactor** | The project's change-size taxonomy: add-only / one-signature-change / wide-no-new-behaviour. | `GEMINI.md:26-42` |
| **The gate** | The change-control rule: read-only work is always allowed; structural work needs a written, approved plan. | `GEMINI.md:8-24` |
| **chưa hỗ trợ** | "not supported yet" — the exact phrase every unavailable rail entry shows, always followed by the reason. Deliberately not a dead link or an empty table. | `internal/ui/nav/rail.go:50-60` |
| **Keeper** | A piece of a spike worth moving into the real codebase (e.g. `EntryGrid`). | `plan-native-shell.md:26-27` |
| **Harness** | `fingo/web` + `cmd/webui` — no longer shipped, kept for the e2e suite. | `plan-native-shell.md:83-86` |
| **Caveman mode** | The terse prose style mandated for agent replies in `GEMINI.md`. Affects only agent output, not code. | `GEMINI.md:1-6` |
| **VAS Accounting** | The git author name and the old project title. | `git log` |
| **`$cov`** | A stray coverage file whose name is a PowerShell variable that never interpolated. | `fingo/$cov` |
| **P0…P5** | The native-shell phases (P0 storage/shell · P0.5 move spike · P1 screens · P2 purchases/inventory · P3 close · P4 tax · P5 rest). | `plan-native-shell.md:378-388` |
| **Q1…Q8** | The legal register's open questions. `Q3` (TT133 chart) is **resolved**; `Q2` (mã số) and `Q8` (appendices) are not. | `legal-basis.md:131-163` |
| **`P` / `S` / `?`** | Legal confidence marks: primary text / named secondary source / unresolved — do not build on. | `legal-basis.md:25-33` |
| **`openable`** | The literal map in `main` that decides which rail entries can open. The de-facto definition of "done". | `cmd/fingo/main.go:426` |

---

## 3. Acronyms

| Acronym | Expansion |
| :--- | :--- |
| ADR | Architecture Decision Record (`docs/adr/`) |
| AP / AR | Accounts Payable / Accounts Receivable |
| CCDC | Công cụ, dụng cụ (tools & equipment) |
| CIT | Corporate income tax (TNDN) |
| COA | Chart of accounts |
| COGS | Cost of goods sold (giá vốn) |
| FK | Foreign key |
| GL | General ledger |
| HTKK | Phần mềm Hỗ trợ kê khai (tax declaration support software) |
| KCT / KKKNT | Không chịu thuế / Không kê khai nộp thuế (VAT categories the model cannot yet express) |
| PIT | Personal income tax (TNCN) |
| PO / GRN | Purchase order / Goods receipt note |
| RBAC | Role-based access control |
| SoD | Segregation of duties |
| TSCĐ | Tài sản cố định (fixed assets) |
| TT | Thông tư (Circular) |
| NĐ | Nghị định (Decree) |
| QĐ | Quyết định (Decision) |
| NQ | Nghị quyết (Resolution) |
| UNC | Ủy nhiệm chi (bank payment order) |
| VND | Vietnamese đồng — no subunit; all amounts are whole đồng |
