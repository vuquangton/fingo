# trading — sales and purchase flows

**Package:** `internal/trading` · **Status:** WIRED (sales, buy-direct bill) /
**ENGINE-ONLY** (`Purchasing` order→receipt→invoice).

## Purpose

Price document lines, decide VAT, and hand one built voucher to the ledger. The
package knows accounting events, not HTTP or SQL.

## Public API

| Symbol | What |
| :--- | :--- |
| `NewService(documents, poster, vatRegistry, rules)` | sales service |
| `(*Service).Invoice(ctx, InvoiceCommand) (Invoice, error)` | Dr 131 / Cr 5111 / Cr 33311 |
| `(*Service).Pay(ctx, PaymentCommand) (Payment, error)` | customer receipt + allocation |
| `(*Service).Credit(ctx, CreditCommand) (CreditNote, error)` | credit note |
| `Purchasing` (order → receipt → supplier invoice → payment) | three-way match engine (`purchase.go:306`) |
| `Posting{Voucher *gl.Voucher; Cost *gl.Voucher; By}` | the unit the ledger receives (`sales.go:80`) |

Wired through `internal/books`:
`Books.Sales` (`sales.go:20`), `Books.SaleWithCost` (`sale_cost.go:28`),
`Books.PurchaseBill` (`purchase.go:34`), `Books.Receipt` (`receipt.go:37`),
`Books.Payment` (`payment.go:31`), `Books.ReverseVoucher` (`reverse.go:35`).

## Invariants

1. An invoice needs a partner: TK 131 is kept per customer.
2. VAT is per line via `vat.Registry`; the rate in force on the document date is
   the only rate that can apply.
3. The voucher is built by trading but numbered/posted by `internal/ledger`; no
   arithmetic in `internal/web`.
4. A sale with stock posts **two** vouchers (revenue, then COGS). The stock check
   happens before revenue is posted.
5. `Purchasing` refuses an invoice exceeding what was received and returns
   `MatchWarning`s instead of silently accepting a variance.

## DB tables

`documents`, `document_lines`, `document_credits`, `allocations`, `items`,
`stock_movements`, `partners`, `vouchers*`.

## Tests

`sales_test.go`, `purchase_test.go`, `money_test.go` (pure); end-to-end through
`internal/books/{sales_test,cogs_test,tieout_test}.go` and `internal/web/*`.
`$env:REQUIRE_DB='1'; go test ./internal/trading -count=1`.

## Gotchas

- **`Purchasing` has no production caller.** `GET/POST /purchase` uses
  `Books.PurchaseBill`, the narrower cost bill (no goods receipt). Do not claim
  three-way match is live.
- `document.TypeInvoice` covers both a sales invoice and a supplier bill; the
  stored `documents.kind` distinguishes them (`PURCHASE_BILL` via
  `Documents.CreateAs`). `kindOf` alone would collapse both to `SALES_INVOICE`
  (`internal/books/documents.go:315-330`).
- VAT is hard-wired to 10% in `Books.salesVATRates` (`sales.go:47-49`); the 0/5/8
  menu is an open question (D-1).
- trading derives readable ids (`invoice-<uuid>`); the ledger replaces them with
  a UUID for the DB key.
