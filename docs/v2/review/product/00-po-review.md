# FinGo v2 — Principal Product Owner review (alpha → omega)

**Reviewed:** `v2` @ `56e0162` · 41 routes · 16 screens · 3 commands.
**Method:** end-to-end journey walk over the product surface, cross-checked against the
two prior reviews (`docs/v2/review/`, `docs/v2/review/logic/`) and the code paths behind
each screen. The server was stopped for this review; no live probing was done, so
"verified" below means *verified earlier in this engagement or in the code*, and
`[INFERRED]` marks reasoning from code alone.

**Lens:** a Vietnamese accounting practice with 5–50 SME clients, 2–5 staff, running one
month-end per client per month. Every pain below is judged by: *does this cost the
practice time, or does it put a wrong number in front of a client or a tax office?*

---

## 1. The journey as it actually is

| Stage | What a new user meets | Verdict |
| :--- | :--- | :--- |
| 1. Install | hand-run `pgsetup`, `migrate`, a self-signed cert, a binary from a terminal | **Ops-heavy**; no installer, no Windows service, no scheduled backup |
| 2. First sign-in | `/login` works; lands on `/clients` | Fine, but the landing page is a settings list, not a work queue |
| 3. Create a client | name + tax code; chart is provisioned automatically (123 TT133 accounts) | Good, **but** no choice of regime/chant, no fiscal-year setup, no address/legal rep |
| 4. Opening balances | Excel import, balanced voucher, one shot per file | Good idea; **cannot carry opening stock quantities**, only account values |
| 5. Partners | add customer/supplier, tax code, kind | No edit, no deactivate, no search, no statement |
| 6. Sales | per-line item code, qty, price, rate (10/5/0/KCT/exempt); posts 131/5111/33311 + COGS if stocked | Solid, **but** no credit note / sales return, no discount field, no due-date field |
| 7. Receipts | pick open invoice, amount, settle; overpayment → credit | Good; **no reversal path that un-settles the invoice** |
| 8. Purchases (cost) | expense account + rate; posts expense/1331/331 + payable | Fine for services |
| 9. Stock | item, receipt (156/331), stocktake, list with moving average | **Receipt creates a payable in the ledger with no AP document** — see C3 |
| 10. Payments | pick open bill, pay, settles | Same reversal gap as receipts |
| 11. Reports | trial balance, P&L, balance sheet, ledger, aging, VAT summary, Excel | Numbers are right; **no print/PDF, no period comparison, no drill from statement to ledger** |
| 12. Close | one click, 911→4212, locks the period | **No confirmation, no preview, no reopen**; a mis-click is final |
| 13. Handover | zip of per-client CSVs + manifest | Genuinely good; no import-back path |
| 14. Admin | clients list, users, own password | No user edit/deactivate, no reset flow, no audit view |

---

## 2. Pain points, sorted by criticality

**Critical** = puts a wrong number in the books or blocks a real client's month.
**High** = costs hours per month or forces a workaround outside the product.

### Critical

| ID | Pain | Who feels it | Evidence | Fix |
| :--- | :--- | :--- | :--- | :--- |
| **C1** | **Stock receipts create a payable in the ledger (Cr 331) but no AP document.** The 331 control account and the AP aging subledger drift apart by every stock receipt. | Accountant; auditor | `internal/books/stock.go` `ReceiveStock` posts `156/331` and writes no `documents` row; `/aging` reads `PURCHASE_BILL` only | create a `PURCHASE_BILL` document on receipt (or post 331 only via a bill) |
| **C2** | **Opening stock cannot be entered.** Opening balances are account values only; a trader's 156 balance arrives with no item, no quantity, no cost — so the first sale either refuses (no stock) or COGS is wrong. | Any client holding inventory | `/opening` posts a voucher only; `items`/`stock_movements` untouched | an opening-stock import (item, qty, unit cost) that posts 156 and seeds the ledger |
| **C3** | **8% VAT is absent.** During a reduced-rate window, the operator's only choices are 10/5/0/KCT — so an 8% invoice posts 10%, silently overstating output VAT by 2%. | Any client selling 8% goods in the window | `vat_rates` seeded with 0/5/10 only (`00008`), 8% deliberately omitted | one `INSERT` once the statutory dates are confirmed **and** a UI warning until then |
| **C4** | **Reversing a receipt/payment voucher leaves the document settled.** The cash is reversed, the allocation stays — so 131/331 and the aging disagree, and the same invoice still looks paid. | Accountant correcting a mistake | `books/reverse.go` posts a mirror voucher only; no allocation reversal | reversal must also un-apply the allocation, in the same transaction |
| **C5** | **Period close is a single irreversible click with no preview.** The screen shows figures, but there is no "here is what will be posted, confirm" step and no undo. | Whoever closes the month | `POST /close` → `ClosePeriod` → lock; no confirm token, no reopen | a confirmation step showing the closing journal and the locked period |
| **C6** | **No credit note / sales return.** The domain has `TypeCreditNote`, the UI has no screen. A real month almost always has one return or discount. | Every client, monthly | 16 screens, none for a credit note; `document.TypeCreditNote` unreachable | a credit-note screen that posts 521/33311 and reduces the receivable |

### High

| ID | Pain | Evidence | Fix |
| :--- | :--- | :--- | :--- |
| **H1** | **No draft state.** Entry posts on submit; a typo becomes a posted voucher that must be reversed. Accountants expect "lưu nháp". | `/entry` posts immediately; no draft column/path | draft voucher status + post action |
| **H2** | **Client switching is buried.** Every screen shows the client name, but changing it means going to `/clients` and back. A practice switches clients dozens of times a day. | nav links per template; no selector | a client switcher in the header |
| **H3** | **No search or filter in the journal.** It lists; it does not let you find one voucher among 5,000. | `/journal` query, no filter params | date range, account, partner, text |
| **H4** | **Partial settlement is invisible on the aging/statement.** Aging shows outstanding, but a partner cannot be given a statement of account. | no statement screen | partner statement (invoices, receipts, balance) |
| **H5** | **Print/PDF output missing.** Trial balance exports to Excel only; ledgers and statements cannot be printed for a file. | only `/reports/export.xlsx` | print CSS + PDF per report |
| **H6** | **No purchases three-way match, no purchase return.** A trader cannot reconcile order → receipt → bill inside the product. | no PO/GRN screens; scope notes admit it | later; state it clearly in the UI |
| **H7** | **Error messages leak English domain text into a Vietnamese UI**, e.g. `receipt PT2026-0001 posted, but settling … failed: document: invalid document …`. | `books/*.go` errors rendered directly by handlers | map domain errors to Vietnamese operator messages |
| **H8** | **Backup is a script and a task installer nobody has registered**; no offsite, no alert, no "last backup" indication in the UI. | `scripts/backup.ps1`, `install-backup-task.ps1`; earlier review F-04 | register, alert on staleness, show last-backup age |
| **H9** | **Rate is not persisted on sales documents** (purchase is). The VAT summary's rate breakdown therefore cannot be trusted for sales. | domain has `VATRateID`, `documents.vat_rate_id` written by purchase; sales path unverified | write and test the sales rate |
| **H10** | **No user management beyond create**: no deactivate, no role change, no forced reset, and no audit screen. | `/users` create + own password only | edit/deactivate + audit viewer |

### Medium

| ID | Pain | Evidence |
| :--- | :--- | :--- |
| **M1** | A locked period is not enforced by the database; only Go refuses. | earlier review, attack §K |
| **M2** | Inactive/heading accounts can be posted to via SQL; Go prevents it only. | logic review, A6 |
| **M3** | Stock guards (negative stock, source document) are Go-only. | logic review, A9 |
| **M4** | No idempotency on entry/sales/purchase/stock; a double submit posts twice. | logic review, L-06 |
| **M5** | No dashboard: after sign-in there is no "what needs doing today" — no unposted drafts, no unreconciled items, no month-end status per client. | `/clients` is the landing page |
| **M6** | VAT summary is internal only, with no 01/GTGT shape, no KCT turnover line, no carried-forward input VAT. | `/reports/vat` |
| **M7** | Amounts render without a currency suffix in several tables; number format is consistent but the unit is implicit. | templates |
| **M8** | No multi-currency, no FX — stated out of scope but no UI hint. | scope notes |
| **M9** | No client-level settings: fiscal year start, invoice template, rounding policy. | client create takes 2 fields |
| **M10** | `HEAD /login` returns 404; monitoring tools see a dead service. | earlier review |

### Low

| ID | Pain |
| :--- | :--- |
| **L1** | htmx is loaded but unused (dead weight). |
| **L2** | No keyboard-only entry grid; higher-volume entry is slow. |
| **L3** | No confirmation on item/partner creation duplicates beyond unique errors. |
| **L4** | Aging screen lists partners with no way to drill into the documents. |
| **L5** | Handover export has no import-back path. |
| **L6** | No dark mode / responsive audit for tablets. |

---

## 3. The three that would make me refuse a pilot

1. **C1** — every stock receipt silently widens the gap between 331 and the AP subledger.
   An aging report that prints "lệch" on day one destroys trust in every other number.
2. **C2** — a trading client cannot start: no opening stock means no first sale.
3. **C3** — 8% is a live statutory rate in the window this product is being sold into.

Everything else is either friction (fixable while running) or a documented scope line.

## 4. PO calls

| Decision | Call |
| :--- | :--- |
| Ship a pilot now? | **No** until C1, C2, C3 are closed. C4–C6 can follow inside the first month. |
| Freeze feature work? | **Yes** — the surface is wide enough. Fix the six criticals, then run a client month. |
| Kill anything? | The Tabulator question, the stock-transfer screen, and the three-way match should stay out until a client asks. |
| Buy or build print/PDF? | Build print CSS first; a PDF library is a cost with no buyer yet. |
| What to show a prospective client? | Sales → receipt → aging → VAT summary → close. That is a complete, defensible story; avoid stock until C1/C2 are closed. |

## 5. What is genuinely strong (do not break it)

- **Numbers cannot be silently wrong in shape**: unbalanced, edited, deleted vouchers are refused by PostgreSQL, not by good intentions.
- **Tenancy is architectural**, not a filter: 15/15 tenant tables forced RLS; a scope-less query sees nothing.
- **Every figure is traceable**: voucher → lines → report; reversal is the only correction, and it is one action.
- **VAT arithmetic is exact** at 25 boundaries, including the half-up edge and a 9.2×10¹² amount.
- **A client can leave**: per-client CSV export filtered by RLS.

## 6. Metrics I would hold the team to

| Metric | Target |
| :--- | :--- |
| Month-end per client, from first voucher to locked period | ≤ 2 working days for a 300-voucher client |
| Clicks from login to first entry of a chosen client | ≤ 3 |
| Aging screen differences (`lệch`) on a clean month | **0** |
| Reversals per 100 vouchers (proxy for entry friction) | < 2 |
| Time to onboard a new client (chart + opening + first entry) | ≤ 1 hour |
| Backup age, visible in the UI | < 26 h, always |
