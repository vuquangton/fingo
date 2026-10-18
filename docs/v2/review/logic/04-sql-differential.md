# SQL differential (L-09 / U-09)

**Test:** `internal/books/sql_oracle_test.go` → `TestReportsEqualSQLRecomputation`.
**Result:** PASS — the application's three statements equal a recomputation done
entirely in SQL from `voucher_lines`.

## Why this test exists

`tieout_test.go` compares the Go reports with each other. Two reports that share a
mistake agree perfectly. This test asks the **database** what the books say, using only
`accounts.account_type` and the posted lines, and compares that to what the practice is
shown.

## Seeded book (hand-built, not the app's output)

| Voucher | Lines |
| :--- | :--- |
| Góp vốn | Dr 1111 10.000.000 · Cr 4111 10.000.000 |
| Bán hàng có VAT | Dr 131 2.200.000 · Cr 5111 2.000.000 · Cr 33311 200.000 |
| Giá vốn | Dr 632 1.200.000 · Cr 156 1.200.000 |
| Chi phí quản lý | Dr 6422 300.000 · Cr 331 300.000 |
| Thu tiền khách | Dr 1111 2.200.000 · Cr 131 2.200.000 |

## Result

| Figure | SQL oracle | Application |
| :--- | ---: | ---: |
| Assets | 11.000.000 | 11.000.000 |
| Liabilities | 500.000 | 500.000 |
| Equity | 10.000.000 + profit | 10.500.000 (includes unclosed result) |
| Revenue | 2.000.000 | 2.000.000 |
| Expenses | 1.500.000 | 1.500.000 |
| Net profit | 500.000 | 500.000 |
| Trial balance period Dr/Cr | 12.200.000 / 12.200.000 | 12.200.000 / 12.200.000 |

Both balance sheets balance: `11.000.000 = 500.000 + 10.500.000`.

## The defect it found first — in the oracle, not the product

The first run reported `assets: app 11.000.000, SQL 12.200.000`. The oracle had summed
only **positive** net balances per account type, so the 156 credit balance of
1.200.000 (inventory issued with no receipt) was dropped instead of reducing assets.
The application was right; my independent calculation was wrong.

That is the value of the exercise stated plainly: the oracle disagreed, and the
disagreement was investigated rather than the oracle being tuned until it matched. The
fix is in the test (`signed, not positive-only`) and the reason is in the comment.

## What remains unproven

| Item | Status |
| :--- | :--- |
| B01/B02 **line-by-line** mapping against the TT133 form | `[UNVERIFIED]` — the test compares totals, not statement lines |
| Close entries and the next year's opening balances in the differential | `[UNVERIFIED]` |
| A multi-year, multi-client differential | `[UNVERIFIED]` |
