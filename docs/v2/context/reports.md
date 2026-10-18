# reports — trial balance, P&L, balance sheet, aging

**Packages:** `internal/reports` (pure), `internal/books` (ports over pgx).
**Status:** WIRED, DONE.

## Purpose

Sum posted voucher lines into the three statutory statements and age AR/AP.
Nothing is read from a stored balance: the summing *is* the report, so a report
cannot drift from the ledger it describes.

## Public API

| Symbol | What |
| :--- | :--- |
| `NewService(periods, journal, chart, closing)` | `reports.go:121` |
| `(*Service).TrialBalance(ctx, periodID)` | opening / period / closing, per account |
| `(*Service).ProfitAndLoss(ctx, periodID)` | period result, closing vouchers excluded |
| `(*Service).BalanceSheet(ctx, periodID)` | position at period end |
| `reports.TrialBalance.SourceVoucherCount` | how many distinct vouchers every row came from |
| `Books.Service/TrialBalance/ProfitAndLoss/BalanceSheet` | assembly over a scoped store (`books.go:37-66`) |
| `Books.AccountLines(account, from, to)` | drill-down ledger |
| `Books.Aging(kind, asOf)` | AR/AP buckets + control-account check (`aging.go:65`) |
| `Books.Lines` / `Chart` / `List` / `EnsureCurrentYear` | the ports |

## Invariants

1. Only **posted** lines count (`v.posted_at IS NOT NULL`).
2. A row exists only if at least one voucher line touched the account; a zero
   with nothing behind it is not printed (`reports.go:1-17`).
3. P&L excludes closing vouchers (identified through 911/4212), so a closed
   period still reports its trading result.
4. Balance sheet: assets = liabilities + equity, with an unclosed result shown
   inside equity.
5. Aging prints the subledger total **and** the 131/331 control balance and
   computes `Difference()`; a report that cannot disagree is not a check.
6. `ErrUnknownAccount` / `ErrUnusableLine` — an account or line the report cannot
   place is refused, never skipped.

## DB tables

Reads `vouchers`, `voucher_lines`, `accounts`, `accounting_periods`,
`accounting_period_locks`, `documents`, `document_lines`.

## Tests

`reports_test.go`, `profitandloss_test.go`, `balancesheet_test.go`,
`invariants_test.go`, `edge_test.go`, `traceability_test.go`;
`internal/books/tieout_test.go` (five cross-report identities, hand-computed
figures). `$env:REQUIRE_DB='1'; go test ./internal/reports ./internal/books`.

## Gotchas

- `internal/reports` imports `internal/close` only for `ClosingAccounts` — the
  type is aliased, not restated (`reports.go:61-67`).
- `Books` does not cache; every screen re-reads and re-sums. Measured ~770 ms at
  50k lines, ~7–8 s projected at 500k (`docs/v2/performance.md`).
- `reports/statutory` (B01/B02 form set) is **not** wired to any route and is out
  of scope; do not route it without a decision.
- Aging's default due date is invoice date + 30 days when `due_date` is NULL.
