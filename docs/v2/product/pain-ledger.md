# Pain ledger

One row per pain point from the PO review (`docs/v2/review/product/00-po-review.md`).
Dispositions: `FIXED-VERIFIED` · `FIXED-NEEDS-HUMAN` · `DEFERRED` · `DECISION` · `WONT` · `OPEN`.

Rules: `FIXED-VERIFIED` needs a regression test that failed before the fix; `DEFERRED`
needs the exact in-product hint and the trigger that reactivates it; nothing may remain
`OPEN` at the end of the goal.

**Baseline at `d715bcb`** (branch `v2-pain-points`): no `v2-logic-fixes` branch exists, and
`db/migrations` contains no period-lock enforcement, no postable-account constraint, no
stock guard and no idempotency key. **M1–M4 are therefore still open**, not already fixed.

## Critical

| ID | Pain | Status | Evidence / next |
| :--- | :--- | :--- | :--- |
| C1 | Stock receipt posts Cr 331 with no AP document; aging drifts | **FIXED-VERIFIED** | receipt now creates the `PURCHASE_BILL` (partner, period, total, 156/331 lines) in the same operation; `TestStockReceiptCreatesThePayable` failed before the fix (AP documents = 0) and passes after; aging shows `khớp` |
| C2 | Opening stock cannot be entered | **FIXED-VERIFIED** | `/opening/stock` creates items and opening receipt movements, and refuses unless the stock value equals the 156 balance to the đồng; re-import refused once the warehouse has movements. Tests: first sale costs 100 × 400.000 = 40.000.000; a 120.000.000 import against a 100.000.000 156 balance is refused 422 |
| C3 | 8% VAT absent | **FIXED-NEEDS-HUMAN** | the sales screen warns that 8% is not configured; a `vat_unresolved_windows` register holds no dates by default, and while a confirmed window covers the invoice date the sale needs an explicit acknowledgement that is written to `audit_log` as `VAT_RATE_ACK`. The 8% rate row is **not** added: `legal-basis.md` has no confirmed dates. Once a practitioner supplies them, one INSERT of the rate plus the window activates the path |
| C4 | Reversal leaves the document settled | **FIXED-VERIFIED** | the mirror voucher now un-applies every allocation and overpayment credit the reversed voucher created (append-only reversal tables, unique per allocation so a retry cannot decrement twice); the document returns to OPEN/PARTIAL with its outstanding restored. Tests: full receipt reversal reopens the invoice and aging ties; partial reversal restores the remainder; an overpayment credit is reversed; a second reversal is still refused |
| C5 | Close is a one-click irreversible action | **FIXED-VERIFIED** | two steps: `/close/preview` shows the exact closing journal and result from the same engine call the close uses and posts nothing; `/close/confirm` requires the typed period and a one-time session token bound to a digest of the previewed figures. Stale data → 409 and re-preview; replay → refused; audit row written by the close. Reopen remains out of scope with a DECISION memo (`decision-memos/C5-controlled-reopen.md`). Tests: preview writes no voucher, confirm locks, stale token 409, wrong typed period 422, replay refused |
| C6 | No credit note / sales return | OPEN | screen, 521/33311, stock back at original cost |

## High

| ID | Pain | Status |
| :--- | :--- | :--- |
| H1 | No draft state | OPEN |
| H2 | Client switching buried | OPEN |
| H3 | No journal search/filter | OPEN |
| H4 | No partner statement | OPEN |
| H5 | No print/PDF | OPEN |
| H6 | No three-way match | OPEN → target `DEFERRED` with hint |
| H7 | English domain errors in a Vietnamese UI | OPEN |
| H8 | Backup not scheduled, no visibility | OPEN |
| H9 | Sales documents do not persist the VAT rate | OPEN |
| H10 | Users cannot be edited/deactivated; no audit view | OPEN |

## Medium

| ID | Pain | Status |
| :--- | :--- | :--- |
| M1 | Locked period not enforced by the database | OPEN |
| M2 | Inactive/heading accounts accepted by SQL | OPEN |
| M3 | Stock guards Go-only | OPEN |
| M4 | No idempotency on posting | OPEN |
| M5 | No dashboard / work queue | OPEN |
| M6 | VAT summary missing KCT turnover and carried-forward input | OPEN |
| M7 | Amount unit not always shown | OPEN |
| M8 | No currency hint | OPEN |
| M9 | No client settings (fiscal year, address, regime) | OPEN |
| M10 | `HEAD /login` 404 | OPEN |

## Low

| ID | Pain | Status |
| :--- | :--- | :--- |
| L1 | htmx loaded but unused | OPEN |
| L2 | No keyboard-only entry grid | OPEN → spike or `DECISION` |
| L3 | No duplicate warning on partner/item | OPEN |
| L4 | Aging rows cannot drill into documents | OPEN |
| L5 | Handover has no import-back | OPEN → `DECISION` |
| L6 | Tablet responsiveness unverified | OPEN → blockers only; dark mode `WONT` |

## Round log

| Round | Work | Gate |
| ---: | :--- | :--- |
| 1 | Epic 0 (ledger, pilot scenario) + C1. **C1 also uncovered an aging bug**: the control side was compared as `'credit'` against a `CREDIT` column, so every AP aging printed a difference of twice the outstanding. Fixed and covered by the same test. | G1: 1 of 6 criticals closed |
