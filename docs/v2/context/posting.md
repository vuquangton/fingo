# posting — the only path to a posted voucher

**Packages:** `internal/ledger` (SQL path), `internal/posting` (account rules),
`internal/usecase/ledger` (domain service, no tests of its own).
**Status:** WIRED, DONE.

## Purpose

Take a voucher, apply the client's chart and period calendar, allocate its number
under a lock, insert voucher + lines + audit row in **one transaction**. A posting
that half-happens is the failure the design exists to prevent.

## Public API

| Symbol | Where | What |
| :--- | :--- | :--- |
| `ledger.Post(ctx, store, scope, Request)` | `post.go:61` | form-shaped entry: validates, builds, posts |
| `ledger.PostVoucher(ctx, store, scope, *gl.Voucher, actor)` | `post.go:115` | domain-built voucher (close, COGS, reversal) |
| `ledger.Request{ID,Description,Actor,Date,Type,Lines}` | `post.go:39-51` | input for `Post` |
| `ledger.Posted{ID,VoucherNo,Seq}` | `post.go:55-59` | result |
| `posting.Load(rules)` / `LoadFor(rules, keys)` | `posting.go` | validate a rule set; require keys |
| `posting.TT133Defaults()` | `posting.go` | debit/credit accounts per event |
| `usecase/ledger.Service.Post(Posting)` | `service.go:146` | domain-level poster over ports |

Both `ledger` entry points converge on `postInTx` (`post.go:129`), which does:
load chart → load period locks → `BookFor` → `nextSeq` under an advisory lock →
`v.Post(...)` → insert header → insert lines → `UPDATE … posted_at/posted_by` →
`audit.Record`.

## Invariants

1. **One path.** `usecase/ledger` does not write SQL; it calls a `Repository`
   that, in production, is `books`' adapter ending in `ledger.PostVoucher`
   (`internal/books/sales.go:60-74`, `internal/books/close.go:126-133`).
2. Numbering is `MAX(seq)+1` under a transaction-scoped advisory lock keyed on
   `(client, book, year)`; the unique constraint is the second line of defence
   (`post.go:262-275`; `internal/ledger/concurrency_test.go`).
3. The audit row is written in the same transaction (`audit.go:38-53`).
4. A refused voucher must not burn a sequence number: rules are checked before
   `nextSeq`.
5. Non-UUID domain ids (`invoice-<uuid>`, `allocation-<uuid>`) are replaced by a
   generated DB key; the domain id is kept in the description/entity id
   (`post.go:161-171`).
6. Header is inserted **without** `posted_at`, lines next, then updated — because
   the immutability trigger refuses lines on an already-posted header
   (`post.go:275-303`).

## DB tables

`vouchers`, `voucher_lines`, `accounts`, `accounting_period_locks`, `audit_log`.

## Tests

`post_test.go`, `concurrency_test.go` (20 parallel posts, distinct numbers),
`posting_test.go`. `$env:REQUIRE_DB='1'; go test ./internal/ledger -count=1`.

## Gotchas

- `internal/usecase/ledger` has **no `*_test.go`** — a change there is only
  covered through `internal/close` and `internal/reports`.
- The `Posting` struct carries an optional `Cost` voucher for sales; both are
  admitted before either is written (`service.go:160-185`).
- `ledger.Post` is the form path; if you add a new document flow, call
  `PostVoucher` with a built voucher, never raw SQL.
- There is no draft state in practice: a voucher is inserted and posted in the
  same transaction, so `created_by == posted_by`. Do not assume a draft table.
