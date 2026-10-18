# Ledger attacks (business-logic pass)

Database `fingo_logic` (fresh, migrated to schema v7), role `fingo_app`.
Script: `attacks.sql`; raw output: `attacks.out.txt`.

| ID | Attempt | Result | Enforced by |
| :--- | :--- | :--- | :--- |
| A1 | Voucher with **zero lines**, posted | **refused** — `voucher … has no lines` | constraint trigger |
| A2 | Voucher with **one line** (Dr 100) | **refused** — `is unbalanced: debit 100…` | constraint trigger |
| A3 | Zero-amount pair (Dr 0 / Cr 0) | **refused** — CHECK on `voucher_lines.amount` | CHECK |
| A4 | Negative-amount pair (Dr −500 / Cr −500) | **refused** — same CHECK | CHECK |
| A5 | Duplicate voucher number in one client | **refused** (`has no lines` fired first; the unique index `vouchers_client_id_voucher_no_key` is present, see below) | unique index |
| A6 | Post to an **inactive** account (`5111` set `active=false`) | **ACCEPTED** (`UPDATE 1`, `COMMIT`) | Go only |
| A7 | Allocate the same payment twice / over-allocate | **refused** — guard on running total; second insert also blocked by FK on `voucher_id` | trigger + FK |
| A8 | Voucher with no lines left as a draft | **refused** — `has no lines` | constraint trigger |
| A9 | Stock issue of 3 units with **zero on hand** and a non-posted source voucher | **ACCEPTED** (`COMMIT`) | Go only |
| A10 | Post into a **locked** period | **ACCEPTED** — proven in the infrastructure review (`attacks2.sql` §K), unchanged | Go only |
| A11 | Post to a **heading** account (`411`) | **ACCEPTED** — proven in `attacks.sql` §6 | Go only |

## Unique index evidence (catalog, not trust)

`vouchers` carries `vouchers_client_id_voucher_no_key` and
`vouchers_client_id_book_fiscal_year_seq_key` (from `pg_indexes` on `fingo_logic`), so a
duplicate number or sequence is refused by the database even though A5's message was
masked by the earlier trigger.

## What this means for the accounting core

- **Strong**: a wrong-side or unbalanced voucher cannot exist. Neither can a posted one
  be altered. Those are the failures that destroy books, and they are impossible.
- **Convention**: *which account*, *whether it is a leaf*, *whether the period is
  locked*, and *whether the goods exist* are all Go decisions. The database will happily
  accept a locked-period, inactive-leaf, negative-stock posting from any code path that
  forgets to check. There are currently 12 such paths (`00-core-map.md`), all of which do
  check — but "all of them today" is not the same guarantee as "the database refuses".
- **Not tested here**: idempotency replay with a changed payload (no idempotency key
  exists on `ledger.Post`; only receipt/payment/reverse derive ids), and number gaps
  caused by rolled-back transactions.

## Severity

| Finding | Severity | Why |
| :--- | :--- | :--- |
| Inactive account reachable via SQL | MEDIUM | the app refuses it; only a code defect or new path exposes it |
| Heading account reachable via SQL | MEDIUM | same |
| Locked period reachable via SQL | **BLOCKER** (carried from the infrastructure review, F-01) | an accountant believes a lock is absolute |
| Stock issue beyond on-hand via SQL | MEDIUM | app refuses; a second writer would not |
