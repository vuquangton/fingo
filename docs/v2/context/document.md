# document — AR/AP documents and allocations

**Packages:** `internal/document` (AR/AP model, wired) and
`internal/domain/document` (posting-path model, wired).
**Status:** WIRED, PARTIAL — **two models coexist by design note.**

## Purpose

A voucher says money moved; a document says what the money is for — invoice,
partner, date, due date, how much is still owed. Without it TK 131 can be
totalled but not aged.

## Public API

### `internal/document` (the one the screens use)

| Symbol | What |
| :--- | :--- |
| `Type`: `invoice`, `credit_note`, `receipt`, `payment`, `journal` | `document.go:73-92` |
| `Status`: `open`, `partially_paid`, `paid`, `void` | `document.go:106-121` |
| `Document{ID, PartnerID, Type, Date, DueDate, Total, Allocated, Status, PeriodID, Lines}` | header |
| `CheckNew`, `CheckLines` | validation before write |
| `Settle(d, Application) (Result, error)` | apply a payment; derives allocation + credit |
| `Repository` (`Apply` under `SELECT … FOR UPDATE`) | persistence port |
| `MemoryRepository` | test double |

### `internal/domain/document` (the posting path)

`Kind`: `SALES_INVOICE`, `PURCHASE_INVOICE`, `CREDIT_NOTE`; `Document` carries
`VoucherID`, `Reference`, `DueDate`, `Amount`; `Open`, `Plan` allocate a payment
across open documents and report `Unapplied` (overpayment) rather than absorbing
it.

## Invariants

1. `Allocated` and `Status` are one fact and move together inside one locked
   transaction (`document.go:1-21`; `internal/books/documents.go:167`).
2. `allocated <= total` is enforced in Go **and** by the DB trigger; an
   allocation row can never exceed its document.
3. An overpayment becomes a `document_credits` row, not a negative allocation.
4. `Apply` locks the document (`FOR UPDATE`), so two payments cannot both see the
   same outstanding amount.
5. Documents are never mutated by a payment; allocations are append-only rows.

## DB tables

`documents` (kind, partner, dates, total, allocated, status, period_id),
`document_lines` (debit/credit/vat/base per line), `document_credits`,
`allocations`.

## Tests

`document_test.go`, `allocation_test.go`;
`internal/books/{sales,receipt,payment,purchase}_test.go`;
`internal/web/{sales,receipt,payment,purchase}_test.go`.

## Gotchas

- **Duplicate models.** `internal/document` is the AR/AP model the screens use;
  `internal/domain/document` carries the voucher link and is what
  `trading`/`close`/`usecase/ledger` use. The package doc explicitly says one has
  to go and consolidation is a separate decision (`internal/document/document.go:17-21`).
  Do not add a third.
- `PURCHASE_INVOICE` (domain) vs `PURCHASE_BILL` (DB kind): the adapter maps via
  `CreateAs`, and `kindOf` alone would collapse a bill into `SALES_INVOICE`.
- `Settle` derives readable ids (`allocation-<uuid>`), which are not UUIDs; the
  books adapter issues the DB key.
- A receipt/payment is **two steps**: post the voucher, then apply it. A crash
  between them leaves an unallocated payment — visible, never wrong.
