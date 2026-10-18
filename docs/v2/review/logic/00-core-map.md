# Core map

## 1. Posting paths (who can write a posted voucher)

Every handler reaches one of two functions in `internal/ledger/post.go`:
`ledger.Post` (request → voucher) or `ledger.PostVoucher` (an already-built domain
voucher). Both funnel into `postInTx`, which is the only place `INSERT INTO vouchers`
and `INSERT INTO voucher_lines` exist (`internal/ledger/post.go:281,299`).

| HTTP handler | Service call | Gateway |
| :--- | :--- | :--- |
| `PostEntry` (`web/entry.go:117`) | — | `ledger.Post` |
| `PostOpening` (`web/opening.go:113`) | — | `ledger.Post` |
| `PostSales` (`web/sales.go`) → `books.SaleWithCost` | `sales.Invoice` (`books/sales.go:70`) | `ledger.PostVoucher` |
| ↳ cost of sale | `inventory.RecordIssue` (`books/sale_cost.go:109`) | `ledger.PostVoucher` |
| `PostReceipt` → `books.Receipt` (`books/receipt.go:62`) | — | `ledger.Post` |
| `PostPayment` → `books.Payment` (`books/payment.go:54`) | — | `ledger.Post` |
| `PostPurchase` → `books.PurchaseBill` (`books/purchase.go:65`) | — | `ledger.Post` |
| `PostStockReceipt` (`books/stock.go:163`) | — | `ledger.Post` |
| `PostStocktake` (`books/stocktake.go:90,124`) | `inventory.RecordIssue` / direct | both |
| `PostClose` (`books/close.go:164`) | `close.Service` | `ledger.PostVoucher` |
| `PostReverse` (`books/reverse.go:124`) | — | `ledger.Post` |

No handler writes `vouchers`/`voucher_lines` directly. **PASS** for "single gateway".

## 2. Invariants and where they are enforced

| Invariant | DB | Go only | UI only | None |
| :--- | :---: | :---: | :---: | :---: |
| Σ Dr = Σ Cr per voucher | **constraint trigger** (deferred) | — | — | — |
| Voucher has ≥ 1 line | **trigger** (`has no lines`) | — | — | — |
| Posted voucher/lines immutable (no UPDATE/DELETE) | **trigger** | — | — | — |
| Posted voucher cannot be deleted | **trigger** | — | — | — |
| Allocation ≤ document, append-only | **trigger + constraint** | — | — | — |
| Period locks append-only | **trigger** | — | — | — |
| Audit append-only | **trigger** | — | — | — |
| Stock movements append-only | **trigger** | — | — | — |
| Unique voucher number per client | **unique index** | — | — | — |
| Unique (client, book, year, seq) | **unique index** | — | — | — |
| Amount > 0, integer type | **CHECK** | — | — | — |
| Tenant isolation | **forced RLS** (15 tables) | — | — | — |
| Account must exist | **FK** (client_id, account_code) | — | — | — |
| Account is a postable leaf | — | **`posting` chart** | — | — |
| Account is active | — | **`posting` chart** | — | — |
| Period is open **and not locked** | — | **`posting` periods** | — | — |
| Stock issue ≤ on-hand | — | **`inventory`** | — | — |
| Movement needs a source document | — | **`inventory`** | — | — |
| VAT rate exists for the date | — | **`vat.Registry`** | — | — |
| Double submit prevented | — | **derived ids on receipt/payment/reverse** | — | entry, sales, purchase, stock |

The bold entries in "Go only" were each proven bypassable with SQL — see
`01-ledger-attacks.md`.

## 3. Money representation changes (rounding-risk sites)

| Site | Operation | Rounding |
| :--- | :--- | :--- |
| `internal/vat/vat.go:171 taxOn` | net × basis points ÷ 10000 | half away from zero, `big.Int` |
| `internal/vat/vat.go:215 Charge.Gross` | net + tax | none |
| `internal/money/allocate.go:21 Allocate` | total × weight ÷ Σweights | largest remainder, ties to earlier index |
| `internal/money/money.go:90 Mul` | amount × integer | none (caller checks?) |
| `internal/money/money.go:103 Div` | amount ÷ integer | quotient + remainder returned |
| `internal/money/dong.go:50,57` | `Dong` mul/div | integer division truncates toward zero |
| `internal/inventory/valuation.go:147 weightedAverage` | (qty×avg + in×cost) ÷ totalQty | truncates toward zero, `big.Int` |
| `internal/document/allocation.go:96 Settle` | allocated + payment vs total | integer compare |

No `float64` on any money path (grep: only comments mention `decimal`).

## 4. What the map suggests before any test

Three invariants are **strong** (DB): balance, immutability, allocation guard.
Six are **conventions** (Go): postable/active account, open+unlocked period, stock
guards, VAT rate presence. A bug in any Go path, or any future write path that forgets
one, produces wrong books with the database's blessing. That is the structural risk this
review is about — not whether the current code remembers, but whether the database would
notice if it did not.
