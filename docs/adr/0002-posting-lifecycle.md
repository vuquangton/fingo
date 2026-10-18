# Posting is a one-way transition; corrections are counter-entries

**Status:** accepted · 2026-10-08

## The decision

A voucher's state is unexported. It can only reach a valid state through its own methods:

- `AddLine` validates the line on entry and **refuses once posted**
- `Post(at, by)` refuses an unbalanced voucher and **refuses a second post**
- `Reverse(newID, newVoucherNo, at, by)` produces a **posted mirror** — same accounts, opposite sides, same amounts
- There is **no** unpost, no edit, and no delete

## Why

The review found `IsPosted` was a plain settable bool with no guard, no `posted_at`, and no reversal — so any posted entry could be altered in place with no trace. A practice is professionally liable for the books it keeps, and an editable ledger cannot be defended to an auditor.

Two invariants follow from that, and both are enforced at the seam rather than by convention:

1. **The double-entry invariant is checked at the last possible moment.** `Post` is where the books accept an entry, so an unbalanced voucher is refused there — not merely flagged earlier.
2. **History is append-only.** Correcting a posted entry adds a counter-entry. Both the mistake and its correction stay visible, which is the point.

## Considered options

**1. Editable posted vouchers with an audit log.** Rejected — the ledger would be a mutable current-state table with history bolted alongside. Reconstruction depends on the log being complete and trustworthy, and a filed figure could still change underneath.

**2. Unpost → edit → repost.** Rejected — it destroys the evidence that a figure was ever reported, which is exactly what a tax audit asks about. Also invites the sequence to be repeated until the numbers suit.

**3. Immutable once posted; correct by reversal.** **Chosen.**

## Consequences

**Unexported state.** Callers cannot flip a posted voucher back to draft, and `Lines()` returns a copy so the slice cannot be mutated from outside. Persistence must rehydrate through a constructor that trusts stored rows — that adapter is not yet written.

**`AddLine` and `Post` return errors**, so every call site must handle refusal. This is a deliberate cost: silent acceptance is what the previous model did.

**A reversal needs its own voucher number and identity.** `Reverse` takes both, because a voucher number is a real-world artifact with statutory sequence rules, not something the domain can invent. Number issuance is deliberately *not* decided here.

**Reversing a draft is refused** — nothing has been admitted to the ledger, so there is nothing to counter.

### Not decided here

- **Voucher number series and gap detection.** The `voucher_no` column is still globally unique, which cannot express per-book, per-year statutory numbering. See [`accounting-review.md`](../accounting-review.md).
- **Idempotency keys on payments.**
- **Rehydrating a posted voucher from the database**, and how the persistence adapter earns the right to set state that the domain otherwise protects.
- **Period locks** — a reversal must be refused into a closed period. Currently it is not.

### Cost

`IsPosted` and the other fields are no longer directly readable as struct fields; callers use accessors. The `gl` package is imported by nothing else in the repository yet, so the blast radius today is the package and its tests.
