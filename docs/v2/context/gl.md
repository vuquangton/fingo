# gl — general ledger domain

**Package:** `internal/domain/gl` · **Status:** WIRED, DONE (ported from v1,
converted to `int64` per ADR 0008).

## Purpose

The journal model and the rules that make a voucher postable: one line = one
account, one side (`DEBIT`/`CREDIT`), one positive amount. Plus the TT133 chart,
book numbering, and the closed-period calendar. No storage, no SQL.

## Public API

| Symbol | What it does |
| :--- | :--- |
| `NewVoucher(id, description, type, date)` | build a draft |
| `(*Voucher).AddLine(VoucherLine)` | append one movement (`gl.go:133`) |
| `(*Voucher).ValidateBalance()` | debits == credits (`gl.go:160`) |
| `(*Voucher).ValidatePostable(PostingRules)` | balance + chart + period + book checks (`gl.go:203`) |
| `(*Voucher).Post(rules, Number, at, by)` | mark posted (`gl.go:243`) |
| `(*Voucher).Reverse(newID, at)` | mirror voucher, opposite sides (`gl.go:303`) |
| `ChartOfAccounts` + `Load`/`Get` | structural rules only; rejects a postable account with children (`account.go:128-138`) |
| `TT133Chart() []Account` | the 123-account seed source (`coa_tt133.go`) |
| `BookFor(VoucherType) (Book, error)` | number series: PT, PC, HD, HDM, PK (`numbering.go:38`) |
| `Number{Book,Year,Seq}.String()` | `PK2026-0001` (`numbering.go:68`) |
| `Periods` / `MonthOf` | closed periods, UTC month (`periods.go:19-50`) |

`VoucherType`: `SALES`, `PURCHASE`, `CASH_RECEIPT`, `CASH_PAYMENT`, `GENERAL`.

## Invariants

1. Amounts are `money.Dong` (`int64`), strictly `> 0`; a line is one side, never
   both.
2. A voucher cannot post with fewer than 2 lines or an imbalance.
3. `ValidatePostable` refuses an inactive/non-postable account, a closed period,
   and a type with no book — before a number is taken.
4. `Post` is one-shot: a posted voucher refuses further lines and a second post.
5. `Reverse` never mutates the original; it builds a new voucher.

## DB tables

`accounts`, `vouchers`, `voucher_lines`, `accounting_periods`,
`accounting_period_locks`, `chart_template`. See `db-schema.md`.

## Tests

`gl_test.go`, `account_test.go`, `coa_tt133_test.go`, `numbering_test.go`,
`periods_test.go`; shared helpers in `internal/domain/gl/gltest/rules.go`. Run:
`$env:REQUIRE_DB='1'; go test ./internal/domain/gl -count=1` (no DB needed —
this package is pure, the env var just keeps the gate uniform).

## Gotchas

- `ChartOfAccounts` is structural only: a client's chart is rows in `accounts`,
  loaded by `internal/ledger`/`books`, not by this package.
- Period granularity is a **month**, read in **UTC** (`MonthOf`). Documents are
  dated by local day in the UI; the boundary is UTC.
- `411` is a heading, not postable — tests that post to it are wrong.
- The chart seed lives twice: `TT133Chart()` in Go and the literal `INSERT` in
  migration `00001`. `tools/genchart` exists to regenerate the SQL; changing one
  without the other has already caused drift risk.
