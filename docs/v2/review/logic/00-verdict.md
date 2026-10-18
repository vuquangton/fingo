# Accounting-core verdict

**Reviewer:** independent accounting-logic auditor (Vietnamese practitioner hat,
financial-systems QA hat, property-testing hat).
**Commit:** `eec23ff` on `v2` · database `fingo_logic` (fresh, schema v7) ·
**Date:** 2026-10-18.

## Verdict: **NO-GO**

Two **blockers** are proven, and five of the eleven gates are **NO** because the evidence
was not produced — this session did not execute the 60-scenario catalogue, the ≥10,000
random sequences, the SQL-only recomputation, the mutation run, or the E2E year. A gate
that is untested is not passed, so the verdict cannot be GO even though everything that
*was* tested passed.

The two proven blockers:

1. **L-01 — a locked period accepts postings at the database level.** Proven twice.
2. **L-02 — the VAT model can only express 10%.** A client selling 8% goods gets 10%
   tax posted silently, and KCT/exempt invoices cannot be represented at all. This is
   wrong money for a category of real clients, which is the definition of a blocker for
   an accounting product.

Everything tested about arithmetic was correct: 25/25 VAT boundaries matched an
independently written integer oracle, including half-up cases, 8% arithmetic, negatives
and a 9.2×10¹² amount.

## Gate table

| Gate | Result | Evidence |
| :--- | :--- | :--- |
| **L1** DB rejects every Phase-1 attack | **NO** | locked period, inactive account, heading account, stock issue accepted (`01-ledger-attacks.md`) |
| **L2** ≥ 60 scenarios match hand-calculated oracle | **NO** | 0 executed — see `unverified.md` |
| **L3** VAT/allocation/average-cost exact at boundaries | **PARTIAL → NO** | VAT 25/25 pass (`03-arithmetic.md`); allocation and average cost not run |
| **L4** I1–I12 over ≥ 10,000 random sequences | **NO** | not executed |
| **L5** SQL-only recomputation equals the app's reports | **YES (totals)** | `TestReportsEqualSQLRecomputation`: assets 11.000.000, liabilities 500.000, revenue 2.000.000, expenses 1.500.000, profit 500.000 — identical in SQL and app; line-by-line B01/B02 mapping still `[UNVERIFIED]` |
| **L6** Concurrency/retry produce no duplicates or partial writes | **PARTIAL** | 20 parallel posts, distinct numbers, balanced (earlier review); retry-with-key untestable (no key, L-06) |
| **L7** Cross-client isolation inside business logic | **YES** | RLS forced 15/15; cross-client insert refused |
| **L8** Out-of-scope features blocked clearly | **NO** | 8% VAT and KCT are in scope legally and silently wrong; purchase return / FX / e-invoice are absent with no blocking message |
| **L9** Mutation score acceptable | **NO** | not run |
| **L10** Every `?` labelled; Circular 48 handled | **NO** | provisions unwired, no end-date warning (L-05) |
| **L11** E2E year reconciles | **NO** | not executed |

## Readiness score

| Area | Score | One line |
| :--- | :---: | :--- |
| Ledger (double entry, immutability) | **5/5** | unbalanced/edited/deleted impossible; proven under SQL attack |
| Posting rules (which account, leaf, period) | **2/5** | correct in Go, bypassable in SQL, lock not enforced |
| VAT | **2/5** | rounding exact; rate model only 10%, no categories |
| Inventory / COGS | **3/5** | moving average tested by the project; DB guards absent; drift not re-measured here |
| AR/AP | **4/5** | allocation guard in DB; aging ties to control; overpayment → credit |
| Close | **3/5** | 911→4212 proven; provisions missing; no re-close protection evidence |
| Reports | **3/5** | internally consistent (tie-out test); not independently recomputed in SQL |
| Concurrency | **3/5** | numbering proven with 20 posters; retries have no key |
| Tests | **3/5** | 21 packages green, 0 skips; sensitivity unmeasured |
| Legal fit | **2/5** | guardrails documented; 8% window, KCT and Circular 48 not implemented |

## Top 10 defects

| Rank | Defect | Severity |
| ---: | :--- | :--- |
| 1 | Locked period accepts postings (DB) | BLOCKER |
| 2 | VAT model only 10%; 8% and KCT impossible | BLOCKER |
| 3 | Circular 48 provisions unwired, no end-date warning | HIGH |
| 4 | Double submit posts twice (no idempotency key) | HIGH |
| 5 | Inactive/heading accounts accepted by SQL | HIGH |
| 6 | Stock guards Go-only; negative stock possible via SQL | HIGH |
| 7 | B01/B02 **line** mapping not compared to the TT133 form (totals now are) | MEDIUM |
| 8 | `Money.Mul` overflow unchecked | MEDIUM |
| 9 | No exempt/non-taxable line category | MEDIUM |
| 10 | Suite sensitivity unknown (no mutation run) | MEDIUM |

## What would change the verdict

| Action | Gate moved |
| :--- | :--- |
| Lock trigger + idempotency key + postable-account FK | L1, L6 |
| VAT rate table by date + KCT category | L2, L3, L8 |
| The 60-scenario catalogue and the SQL differential | L2, L5 |
| Property run (even 1,000 sequences) with shrinking | L4 |
| Mutation run on `vat`, `posting`, `close` | L9 |

The last three are *test work*, not product work, and this session produced none of them.
That is the honest gap.
