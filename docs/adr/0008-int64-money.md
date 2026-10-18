# Money is `int64` đồng — in Go, in PostgreSQL, and on the wire

**Status:** accepted · 2026-10-18 · Reverses the "decimal đồng" half of [ADR 0001](0001-journal-line-model.md) and the two-regime compromise recorded in `GEMINI.md:64-66`.

## The decision

**One representation of money, everywhere: `int64`, whole đồng.**

| Layer | Representation |
| :--- | :--- |
| Go domain | `int64` (via `internal/money.Money`, which is an amount + a `Currency`) |
| Go everything else | `int64` |
| PostgreSQL | `bigint` |
| JSON | **string** — `"1100000"`, never a JSON number |
| HTML | formatted server-side with `.` group separators: `1.100.000` |
| JavaScript | never does arithmetic on an amount; it moves strings |

**No `float64`, no `float32`, no `decimal.Decimal`, no `numeric` column.** `github.com/shopspring/decimal` leaves `go.mod` in Phase 1.

**A conversion from a decimal is explicit and lossy-checked.** Where a v1 type returns `decimal.Decimal`, the port reads `IntPart()` and asserts the decimal equals that integer, returning an error otherwise:

```go
func dong(d decimal.Decimal) (int64, error) {
    v := d.IntPart()
    if !d.Equal(decimal.NewFromInt(v)) {
        return 0, fmt.Errorf("money: %s is not whole đồng", d)
    }
    return v, nil
}
```

Truncating silently is the one thing this function must not do — a fractional amount in a ledger is a bug upstream, and rounding it here would hide it.

**JSON carries amounts as strings.** `2^53` is the exact-integer limit of an IEEE-754 double, and every browser parses JSON numbers as doubles. An amount near `9.007e15` would be silently rounded by the client even though Go and PostgreSQL held it exactly. Strings close that hole permanently; the cost is that the client formats and never computes.

## Why

v1 ran **two money regimes at once**: `internal/domain/*` and `gl` used `shopspring/decimal`, while `internal/{money,vat,inventory,document}` used `int64` minor units with an explicit `Currency`. The project documented the split as a hazard in `GEMINI.md:64-66`, and the boundary between the two was exactly where amounts crossed from the engine into vouchers.

Two representations of the same concept is a class of bug, not a style question:

- **A silent unit mix** — 1,100,000 đồng read as 11,000.00 — passes every type check when the types are `decimal` and `int64`.
- **`decimal` invites scaling.** A `DECIMAL(18,4)` column (v1's original `amount`) stored fractions of a currency that has none, and every comparison had to choose a scale.
- **`int64` makes the invariant structural.** VND has no subunit. There is no `0.5` đồng, so there is nothing to represent.

PostgreSQL `bigint` is exact, sorts and sums exactly, and indexes tightly. `numeric` would work but re-opens the scale question and is slower to aggregate.

## Considered options

**1. Keep `decimal.Decimal` everywhere.** Rejected. It is exact, but it is also opinionated about scale, it makes every amount an object rather than a value, and it does not stop a fractional amount existing — it only stores one.

**2. Keep two regimes and convert at the boundary.** Rejected. That is v1, and the boundary is where the errors live.

**3. `int64` in minor units with a currency on every value** (v1's engine approach). **Chosen**, with `VND` as the only currency for v2. The `Currency` field stays because it is what makes the type honest about being an amount rather than a count; a single-currency product does not need it, but removing it would be a larger change than keeping it.

**4. `numeric` in PostgreSQL.** Rejected for the same reason as option 1, plus aggregate cost.

## Consequences

**`internal/domain/gl` changes shape.** Its `VoucherLine.Amount` and `Voucher` totals become `int64`. This is a **deliberate exception to "port unchanged"**, and it is recorded as such: the packages that already use `internal/money` (`vat`, `inventory`, `document`, `trading`, `close`, `reports`) port unchanged, while `gl` and the packages that consume `gl` amounts (`reports`, `close`, `usecase/ledger`, `posting`) are converted in one slice, with the test changes listed in `docs/v2/test-report.md`.

**Tests that asserted a decimal literal change.** They assert an integer literal and the same expected figure. No test's *behaviour* changes; a test asserting a fractional amount would now fail to compile, which is the intended outcome.

**Every SQL aggregate is `sum(bigint)` → `bigint`.** A report that sums a period cannot overflow: `int64` holds ±9.2 × 10¹⁸, and Vietnam's GDP is ~10¹⁶ đồng.

**The trade-off to watch is the wire format.** A JSON string is one more place a reader must know the convention. It is documented in the template helper and in the client code, and there is exactly one formatter (`internal/ui/format`) used by both.

### Not decided here

- **Multi-currency.** Out of v1 scope. If it returns, the shape is `(amount_vnd int64, currency, rate)` with the *functional* amount always in đồng.
- **Unit prices with fractions.** Quantities are `numeric(19,4)` and inventory unit costs are computed and truncated per the engine's existing rule; the stored *amounts* stay whole đồng. This ADR does not change inventory quantity storage.
