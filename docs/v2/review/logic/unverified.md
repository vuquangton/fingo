# Unverified — accounting core

Everything this session did **not** prove, why, and what would close it. Nothing here may
be read as passing.

| # | Item | Why not done | What closes it |
| ---: | :--- | :--- | :--- |
| U-01 | 60-scenario catalogue (Phase 2) | Session budget. Building 60 hand-calculated scenarios and driving each through the app is roughly a full working day. | Write the 60 rows in `02-scenario-catalogue.csv` with expected journals **first**, then a table-driven Go harness in `review_tmp/` that posts each and diffs |
| U-02 | Mixed VAT rates on one document | The engine refuses two active rates covering one date (`vat.Registry`), so the scenario cannot exist until L-02 is fixed | Fix L-02, then add the 0/5/8/10 scenario |
| U-03 | 8% window boundary (first/last day) | No 8% rate row exists in the app | Add the rate by date range and test both boundary days |
| U-04 | KCT / exempt / non-declared lines | No category field on the line | Add `VATCategory`; then the scenarios are expressible |
| U-05 | Allocation largest-remainder N=1…1000, amounts to 10¹² | Not run this session | A Go test calling `money.Allocate` with a property check: Σ parts = total, each within 1 of exact share |
| U-06 | Moving-average drift over 1,000 alternating ops; Σ 156 = stock value | Not run this session | Property test over `inventory.Ledger` asserting the reconciliation every step |
| U-07 | `Money.Mul` overflow at the int64 boundary | Code-read only | Checked multiply, plus a test at `math.MaxInt64/2` |
| U-08 | Property tests I1–I12, ≥ 10,000 sequences, shrinking | Not run this session | Generator over (sale, receipt, purchase, payment, reversal, close) with per-step invariant checks |
| U-09 | SQL-only recomputation of TB / P&L / BS | Not run this session | Views in SQL, diffed against the Go reports on the same client |
| U-10 | Concurrency beyond 20 posters (50+, many clients), deadlocks, lock waits | Only the project's 20-poster test exists | A load harness with 50 goroutines and a deadlock counter |
| U-11 | Retry after commit-before-response with the same key | No idempotency key exists (L-06) | Implement the key first, then kill the process between commit and response |
| U-12 | Partial failure across voucher + stock + document + allocation | Not run | Fault injection between the steps; assert full rollback including numbering |
| U-13 | Clock/period-boundary semantics (23:59:59, non-UTC server) | Not run | Set the process TZ, post at the boundary, compare `voucher_date` and period |
| U-14 | Mutation testing (≥40 mutations) | Not run | `go-mutesting` on `gl`, `posting`, `vat`, `money`, `inventory`, `trading`, `close`, `reports` |
| U-15 | Coverage per critical package | Not collected | `go test -cover ./...` with `REQUIRE_DB=1` |
| U-16 | Rule matrix vs `docs/legal-basis.md` | Not built | A row per rule: code · test · oracle result |
| U-17 | B01/B02 line mappings vs TT133 forms, sign conventions on 214/229/521 | Not examined | Read the statement mapping, compare to the statutory form line by line |
| U-18 | Closing out of order, second close, close then reversal | Not run | Scenario harness over `close.Service` |
| U-19 | Year-end to 4212/4211 and next-year opening balances | Not run | E2E year (Phase 8) |
| U-20 | E2E year: ≥300 vouchers through HTTP, 12 closes, 20 hand-checked vouchers | Not run | Phase 8 harness |
| U-21 | Purchase return / FX / e-invoice / landed cost: blocked or half-working? | Only "absent" is known; no message audit | Grep the UI for these features; confirm no partial path touches the ledger |
| U-22 | Inventory ↔ ledger reconciliation on the seeded dataset | Not run | SQL: `SUM(156)` vs stock valuation view |
| U-23 | Excel round-trip continuity for next-year opening balances | Not run | Export, re-import, compare trial balance |

## Why so much is unverified

The prompt asks for a full audit: 60 scenarios, 10,000 property sequences, 40 mutations,
an E2E year of 300+ vouchers through HTTP, and a SQL oracle. Each of those is a working
session in itself. This session produced the **static map**, **nine direct-SQL attacks**,
and **one clean arithmetic oracle result** (25/25). It did not produce the scenario,
property, mutation or E2E evidence, and the verdict reflects that rather than papering
over it.

The three items with the best value per hour, in order:

1. **SQL differential** (U-09): a view per statement, diffed against the Go reports. A
   morning, and it catches mapping errors the internal tie-out cannot.
2. **A 12-scenario slice** of the catalogue covering sale-with-VAT, COGS, receipt,
   purchase, close: hand-calculate 12 journals, run them, diff. Half a day.
3. **Allocation property test** (U-05): `money.Allocate` is self-contained and its
   guarantee is checkable in a few dozen lines.
