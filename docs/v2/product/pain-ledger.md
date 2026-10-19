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
| C2 | Opening stock cannot be entered | OPEN | import of item/qty/cost, reconciled to 156 |
| C3 | 8% VAT absent | OPEN → target `FIXED-NEEDS-HUMAN` | mechanism + visible warning; legal dates unset |
| C4 | Reversal leaves the document settled | OPEN | un-apply allocations with the mirror voucher |
| C5 | Close is a one-click irreversible action | OPEN | preview + typed confirmation + token |
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
