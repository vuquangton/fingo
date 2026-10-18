# inventory — stock ledger and valuation

**Package:** `internal/inventory` · **Status:** WIRED, DONE for receipt / COGS /
stocktake; stock transfer not built.

## Purpose

A pure, in-memory stock ledger: replay movements to get on-hand quantity and
moving-average cost, and build the cost voucher for an issue. No persistence —
`internal/books` persists movements and rebuilds the ledger on each read.

## Public API

| Symbol | What |
| :--- | :--- |
| `NewLedger(currency)` | empty ledger |
| `(*Ledger).AddItem`, `Item`, `Items` | item master in memory |
| `(*Ledger).Record(StockMove)` | append a movement, return value after it |
| `(*Ledger).Level(itemID)` / `Valuation(itemID)` / `Moves(itemID)` | replay results |
| `(*Ledger).RecordIssue(rules, move)` | moving-average cost voucher (`cogs.go:188`) |
| `(*Ledger).RecordReturn(original, move)` | reverse a cost at the original cost |
| `(*Ledger).BelowReorderPoint()` | reorder list |
| `Sale` / `PostSale` | cost-of-sale entry builder |

`StockMove{ItemID, Warehouse, Direction/kind, Quantity numeric(19,4), Cost …}`.

## Invariants

1. Valuation is **replayed** from the movement log on every read — no stored
   balance that can drift from its history (`stock.go:190-256`).
2. An issue is valued at the moving average *at that point in the log*.
3. Stock cannot go negative: `RecordIssue`/`Record` refuse an issue beyond
   on-hand, and `Books.SaleWithCost` checks before posting revenue.
4. Stock movements are append-only (DB trigger); corrections are new movements.
5. Quantity is `numeric(19,4)`; money is `bigint` đồng. Do not mix them.

## DB tables

`items` (SKU, name, unit, reorder_point, active), `stock_movements`
(`item_code`, `warehouse_code`, `kind RECEIPT|ISSUE`, `quantity numeric(19,4)`,
`cost bigint`, `unit_cost`, `voucher_id`, `source_doc_id`, `seq` identity).

## Tests

`ledger_test.go`, `valuation_test.go`, `cogs_test.go`, `precision_test.go`.
DB path: `internal/web/stock_test.go`,
`internal/web/cogs_test.go`. `$env:REQUIRE_DB='1'; go test ./internal/inventory`.

## Gotchas

- `books.stockLedger` rebuilds the whole ledger per request; fine at current
  volume, O(movements) by design.
- A goods receipt must name a source document (`source_doc_id`) — a movement
  nobody can trace is refused (422).
- Stocktake: shortage Dr 632 / Cr 156 at average; surplus Dr 156 / Cr 711.
  A count equal to the ledger is refused (nothing to post).
- Warehouse transfer (C1) does not exist; `warehouse_code` is a string with no
  paired in/out primitive.
