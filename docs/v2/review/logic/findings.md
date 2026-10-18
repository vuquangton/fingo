# Findings — accounting core

| ID | Area | Severity | Finding | Evidence | Expected vs actual | Fix | Effort |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| L-01 | Periods | **BLOCKER** | A locked period accepts a posted voucher at the database level. | `attacks.sql` §10 in `../attacks2.out.txt` §K: `INSERT … UPDATE … COMMIT` succeeded after a lock row existed | expected: refusal; actual: committed | constraint trigger on `vouchers` checking `accounting_period_locks` | S |
| L-02 | VAT | **BLOCKER** | The VAT model can express **one** rate (10%). The 8% window, 5%, 0% and exempt (KCT/KKKNT) cannot be represented; the sales screen hardcodes `vat10`. | `internal/books/sales.go` registers only `vat10`; UI label "Thuế suất đang áp dụng: 10%" in `sales.html` | a client selling 8% goods gets **10% tax posted**, silently | rate table keyed by date, plus a per-line category (KCT/KKKNT) that posts no 33311 | L |
| L-03 | Chart | HIGH | Inactive and heading accounts are refused only by Go; SQL accepts them. | `attacks.sql` A6: `UPDATE 1 / COMMIT` posting to inactive `5111`; earlier `attacks.sql` §6 for heading `411` | expected: DB refusal; actual: accepted | `accounts.postable` + composite FK, or a trigger | M |
| L-04 | Inventory | HIGH | Stock guards are Go-only: an issue of 3 with zero on hand and a draft source voucher was accepted. | `attacks.sql` A9: `INSERT 0 1 / COMMIT` | expected: refusal; actual: accepted | trigger on `stock_movements` (on-hand ≥ qty; source voucher posted) | M |
| L-05 | Close/legal | HIGH | Circular 48/2019 provisions are not wired, and no effective end date (2026-12-31) is enforced or warned about. `close.WithProvisions` has no caller. | `internal/close/close.go` provisions API; no caller in `books/close.go` | a practice cannot make the year-end provision entry the circular requires | wire provisions with a policy end date and a visible warning after it | L |
| L-06 | Idempotency | HIGH | `ledger.Post` has no idempotency key. A double-submitted entry, sale, purchase or stock receipt posts twice. Only receipt, payment and reversal derive their ids. | `internal/ledger/post.go` Request has no key; `books/sales.go` uses `uuid.NewString()` per request | expected: same key ⇒ one voucher; actual: two vouchers | client-supplied request key, unique index on `(client_id, request_key)` | M |
| L-07 | Arithmetic | MEDIUM | `Money.Mul` multiplies two `int64` without overflow protection, unlike VAT/allocation/average cost which use `big.Int`. | `internal/money/money.go:90` | a boundary quantity × price wraps silently | use `big.Int` or checked multiply | S |
| L-08 | VAT | MEDIUM | No exempt/non-taxable category on a line, so a KCT sale cannot be recorded without abusing the 0% rate (which is not the same thing on the return). | `vat.Line` has `VATRateID` only | expected: category; actual: absent | add `VATCategory` to the line | M |
| L-09 | Reports | MEDIUM | Contingent: trial balance, P&L and balance sheet are not recomputed independently in SQL anywhere in the suite; agreement is asserted only by the app's own reports. | `internal/books/tieout_test.go` compares reports to each other, not to SQL | expected: SQL oracle; actual: none | a SQL differential test in CI | M |
| L-10 | Tests | MEDIUM | The reporting of "implemented" for VAT/close rests on author-written tests; mutation testing was not run, so the suite's sensitivity is unknown. | no mutation run in this session | — | run `go-mutesting` on `vat`, `posting`, `close` | M |

## What is genuinely strong (evidence, not opinion)

| Control | Evidence |
| :--- | :--- |
| Unbalanced or line-less voucher cannot exist | `attacks.sql` A1, A2, A8 refused by constraint triggers |
| Zero and negative amounts cannot exist | CHECK constraint, A3/A4 refused |
| Posted vouchers and lines are immutable | `attacks2` A–D refused |
| Allocation cannot exceed a document | `attacks.sql` A7 refused |
| Cross-client writes refused | RLS, `attacks.sql` §11 |
| VAT rounding to the đồng is exact at 25 boundaries | 25/25 oracle match, including 0.5 cases, 8% arithmetic, negatives, and 9.2×10¹² amounts |
| Tenant scoping is DB-enforced on all 15 tenant tables | catalog: `relrowsecurity` and `relforcerowsecurity` both true |

## The pattern behind L-01, L-03, L-04

Three of the four serious findings share one shape: **a rule the accountant believes is
absolute lives in Go, not in the database.** The database protects the *double-entry
shape* of the books (balanced, immutable, isolated) and does not protect the *business
meaning* of an entry (is this account allowed, is this period closed, do these goods
exist). That is a defensible architecture only if every write path is Go — and it is —
but it means a single missing check in a future path is a silent wrong book, not a
constraint violation.

## L-11 (found while extending the VAT summary) — the chosen rate is not persisted

**Severity: HIGH. Confidence: HIGH (schema + code inspected).**

The sales and purchase screens now let the operator choose 10% / 5% / 0% / KCT / exempt per line, and the posting is correct at the moment of posting. But **no table stores which rate a document line was taxed under**:

| Evidence | Finding |
| :--- | :--- |
| `db/migrations/*.sql` | no `vat_rate_id` column on `documents`, `document_lines`, or `voucher_lines` |
| `internal/books/documents.go` | the insert lists `id, client_id, kind, partner_id, document_date, period_id, total, allocated, status` — no rate |
| `internal/document/document.go:154-157` | the domain carries `VATRateID` and its comment says it exists so a stored line stays reproducible — but the adapter drops it |

**Consequence:** the VAT summary cannot be broken down by rate or by KCT/exempt, because the information is not in the database. Re-opening last year's invoice cannot say whether it was 5%, 0%, KCT or exempt; only the amount 33311 moved is known. A change to the rate catalog therefore cannot corrupt the past — but it also cannot explain the past.

**Fix (effort S):** add `vat_rate_id text REFERENCES vat_rates (id)` to `document_lines`, write it in the document adapter from `document.Line.VATRateID`, backfill nothing (old rows stay null), and only then extend the VAT summary with the per-rate table.

I said in the previous message that the data was "already on the document lines". That was wrong: the field exists in the domain model and nowhere in the database. Corrected here rather than papered over.