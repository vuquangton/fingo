# DECISION — a controlled reversal of a closed period

**Asked by:** C5 (close is irreversible). **Status:** decision required before any reopen feature.
**Default today:** there is no reopen. A locked period is locked permanently, and the only
correction is an adjustment in the next period.

## Why this is a decision, not a task

A closed month is a statement to a client and, once filed, to a tax office. Reopening it
changes numbers somebody has already read. The question is not "can we build it" — the
closing voucher is an ordinary voucher, so a mirror plus an unlock row would work — but
"what must be true before an accountant is allowed to do it".

## Options

### A. Adjustment in the next period (status quo, recommended default)
Post the correction in the open period, with a description naming the closed period, and
leave the closed report as filed.

- **Cost:** none — already possible.
- **Effect:** the closed month stays as filed; the current month carries the correction.
- **Weakness:** the two months must be read together to understand the result.

### B. Admin-only reopen with a reversal voucher and an audit record
Unlock the period, post the mirror of the closing voucher, and record who, when and why.

- **Cost:** M — an `accounting_period_reopens` table, a mirror of the close, audit rows,
  and a rule that the reopen must happen before the period's report is handed over.
- **Effect:** the closed month changes; anything already sent to the client is now wrong.
- **Weakness:** it removes the property that makes the lock trustworthy.

### C. Reopen with dual control
As B, but requires two administrators: one requests, one approves, and the request names
the reason and the report version being superseded.

- **Cost:** L — a request/approve workflow, plus the B mechanics.
- **Effect:** the strongest audit story, at the price of a process a small practice will
  find heavy.

## Recommendation

**Keep A as the only path until a client asks otherwise.** Add B only with these
preconditions, and record them in the ADR:

1. the period's report has not been handed to the client (`handover` timestamp is null);
2. the actor is an admin, and the reason is required, free text, stored;
3. the mirror closing voucher and the unlock happen in one transaction;
4. the reopened period is visible on the dashboard for 30 days with who and why.

C is not recommended: two administrators is a control a 3-person practice cannot always
staff, and an unenforceable control is worse than a documented single one.

## What this means for C5 today

The two-step close (preview → typed confirmation → one-time token) is built. The absence
of reopen is deliberate and stated on the close screen. This memo is the design to pick up
**only** if a client asks, with the preconditions above.
