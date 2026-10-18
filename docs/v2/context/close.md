# close — period close and lock

**Package:** `internal/close` · **Status:** WIRED, DONE (provisions ENGINE-ONLY).

## Purpose

Close one accounting period in one act: snapshot the result, post the closing
voucher (revenue/expense → 911 → 4212/4211), lock the period, store the report.
Any one part alone leaves books that disagree with themselves.

## Public API

| Symbol | What |
| :--- | :--- |
| `Command{PeriodID,VoucherID,Operator,At,Force,Reason}` | what to close |
| `NewService(periods, ledger, poster, documents, reports, closingAccounts)` | `close.go:309` |
| `(*Service).ClosePeriod(ctx, Command) (Report, error)` | the act |
| `(*Service).Report(ctx, periodID)` | read back the stored report |
| `(*Service).WithProvisions(Provisions)` | depreciation + bad-debt provisioning — **no caller** |
| `TT133ClosingAccounts()` | 911 → 4212/4211 mapping |

Ports: `Periods`, `Ledger`, `Poster`, `Documents`, `Reports`. `Books` implements
all five over one scoped store (`internal/books/close.go:25-31`).

## Invariants

1. Periods close **in order**; an earlier open period → `ErrEarlierPeriodOpen`.
2. Open documents refuse the close unless `Force` + a non-empty `Reason`.
3. 911 must be empty before the close; a balance means a previous close did not
   finish → `ErrResultAccountCarriesBalance`.
4. Order: write report snapshot → post closing voucher → lock period. The ledger
   refuses a posting into a locked period, so the lock is last; the close is
   retry-safe and a retry finishes the job.
5. The closing voucher travels `ledger.PostVoucher` — same rules, numbering,
   transaction and audit row as a hand-entered voucher.

## DB tables

`accounting_periods`, `accounting_period_locks` (append-only), `period_close_reports`,
`vouchers`, `voucher_lines`, `documents`.

## Tests

`close_test.go`, `documents_test.go`, `entries_test.go`, `provisions_test.go`
(engine); `internal/web/close_test.go` (screen, close, refusal after lock).
`$env:REQUIRE_DB='1'; go test ./internal/close -count=1`.

## Gotchas

- **Provisions never post in production**: `WithProvisions` has no call site
  (`internal/books/close.go:25-28` builds the service without it). The engine and its
  tests exist, but no route enables it.
- The lock row's `locked_by` is the literal `'period-close'`, and **no audit row
  is written for the lock** (`internal/books/close.go:60-69`) — despite docs claiming
  "audit rows for every post/lock".
- `period_close_reports` stores the serialized report; `Report()` is the only
  reader.
- Admin-only at the route level (`POST /close`, `main.go:99`).
