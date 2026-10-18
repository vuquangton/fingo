# Voucher lines carry one account and one side, not a debit–credit pair

**Status:** accepted · 2026-10-08 · **partly superseded 2026-10-18:** the tenant clause ("one database per client, so there is no `company_id` column") is replaced by [ADR 0006](0006-postgresql-rls-tenancy.md); the money clause ("`decimal.Decimal`, integer đồng") is replaced by [ADR 0008](0008-int64-money.md). The line model itself — one account, one side, one amount — stands and is what v2 implements.

## The decision

A voucher line is **one account, one side, one amount**:

```go
type EntrySide string

const (
    SideDebit  EntrySide = "DEBIT"
    SideCredit EntrySide = "CREDIT"
)

type VoucherLine struct {
    ID        string
    VoucherID string
    LineOrder int
    AccountID string          // one account
    Side      EntrySide       // one side
    Amount    decimal.Decimal // one amount, integer đồng
    Note      string
}
```

The pair shape currently at [`gl.go:21`](../../fingo/internal/domain/gl/gl.go#L21) — one debit account, one credit account, one shared amount — is replaced.

## Why

It is not a preference. Sixteen of the eighteen business events in the trading cycle post **three or more accounts**, so they cannot be represented by a pair no matter how it is encoded.

The simplest counterexample is the most common transaction in the business — a VAT sales invoice:

```
Nợ 131    110
   Có 511    100
   Có 33311   10
```

One debit, two credits. Under the pair shape the closest encoding is `(131,511,110)` + `(131,33311,10)`, which yields debit 220 against credit 110. The alternatives are a falsified TK 511 or an unbalanced entry. Both are wrong.

The same wall blocks every import purchase with landed cost, every payroll run, every period close, and every provision. See the full inventory in [`requirements-v1.md`](../requirements-v1.md), §3.

## Considered options

**1. Keep the pair shape.** Rejected — cannot express N:M. Not a limitation that can be worked around; the data model has no room for it.

**2. Keep the pair shape in the UI, auto-expand to lines in the ledger.** Rejected as a *model*, accepted as a *presentation*. The expansion is not derivable: given "debit 152, 150" and a credit total of 150, the split across TK 331 and TK 111 cannot be inferred. Asking the operator for it is the only way — which means the underlying model must already hold separate lines. So this option collapses into option 3 plus a UI affordance.

**3. One account, one side, one amount.** **Chosen.**

## Consequences

**A future reader will ask why this deviates from the familiar shape.** Vietnamese accounting software conventionally presents one row as `TK Nợ | TK Có | Số tiền`, and an experienced accountant expects that grid. That convention is **presentation, not storage**. The UI may keep rendering a two-account row; it maps to two ledger lines that must net to zero. The ledger stays in normal form while the data entry stays familiar. This is deliberate and should not be "fixed" by reverting the model.

**Balance checking becomes real.** Summing debit-side and credit-side amounts across lines now produces a check that can actually fail. The current `ValidateBalance` cannot, because both sides of every line are the same number by construction.

**COGS, output VAT, input VAT, landed cost and payroll become expressible.** These were previously unrepresentable, which is why thirteen findings in [`accounting-review.md`](../accounting-review.md) trace back to this one shape.

**Amounts become integer đồng.** VND has no subunit. Rounding happens per line at entry; the voucher total is the sum of rounded lines, so the invoice total always equals the sum of its parts.

**Identical schema in every client database.** One migration set applies to all clients; no per-client schema drift.

### Not decided here

- **Tenant scoping.** Settled separately: one database per client, so there is no `company_id` column. See [`requirements-v1.md`](../requirements-v1.md), §1.1.
- **Immutability on posting, `posted_at` / `posted_by`, reversal instead of edit.**
- **Idempotency keys on payments.**
- **Whether a voucher may span multiple clients.** It may not, and the model makes that structurally impossible — a voucher lives in one database.

### Cost

A schema migration, regenerated sqlc code, and a rewrite of `Voucher`, `AddLine` and `ValidateBalance`. Everything currently built on the pair shape is scaffolding, so nothing of value is lost. Available account numbers remain provisional pending `Q3` in [`legal-basis.md`](../legal-basis.md).
