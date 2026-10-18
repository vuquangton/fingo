# FinGo v2 — performance and concurrency

Measured on the development machine (Windows, PostgreSQL 16.15 local, one client),
with row-level security **on** for every query — the policies are part of the cost
being measured, not something switched off for the benchmark.

The point of these numbers is not speed. It is to know where the design breaks
before a client's books get there.

## Volume: reports at 50,000 lines

Seeded 25,000 posted vouchers (50,000 lines) into one client's 2026 period, then
read the reports through the normal code path.

| Operation | Time | Notes |
| :--- | ---: | :--- |
| Trial balance | 767 ms | 25,000 vouchers counted, 2 account rows |
| Profit and loss | 759 ms | same scan, different grouping |
| Journal read | 690 ms | all 50,000 lines through the adapter |

Scaling was close to linear: 10,000 lines took ~150 ms, 50,000 took ~770 ms. A
client with 500,000 lines would therefore see roughly **7–8 seconds** per report.

That is acceptable for a month-end read on an office machine, and unacceptable for
a screen that refreshes on every keystroke. The mitigation, if a client ever
reaches that size, is an index-only scan on `voucher_lines (client_id, account_code,
side)` or a materialised period balance. **Neither is added: nothing measured says
it is needed yet, and an index added on a guess is a guess.**

Reproduce: `REQUIRE_DB=1 go test ./internal/books -run TestVolumeIsSurvivable -v`
(`-short` skips it, so the normal suite stays fast).

## Concurrency: twenty posts at once

`TestParallelPostsGetDistinctNumbers` posts 20 vouchers concurrently into one book.

| Property | Result |
| :--- | :--- |
| posts that succeeded | 20 / 20 |
| distinct voucher numbers | 20 |
| distinct sequence numbers | 20 |
| debit vs credit | equal |

Voucher numbering is `MAX(seq)+1` under a transaction-scoped advisory lock. If that
lock were wrong, two vouchers would share a number and the fault would surface only
when an auditor asked why there are two entries numbered `GH2026-0007`. They do not
share one.

## Authentication cost

30 sign-in attempts with an unknown email: **2 ms each**, all recorded in
`failed_logins`, all answered 401.

An unknown email never reaches argon2 — there is no hash to verify — so a password
spray does not become a CPU denial of service on an office machine. A *correct*
email with a wrong password does pay the full argon2 cost (64 MiB, 3 iterations),
which is the intended trade: cheap to reject strangers, expensive to guess a
password.

**There is no lockout or rate limit.** `failed_logins` is written and never read.
Recorded as a risk, not a feature: a lockout that a practice cannot override is a
support call, and the choice belongs to whoever runs the system.

## What these measurements do not cover

| Untested | Why it matters |
| :--- | :--- |
| Reports on a client with 500,000+ lines | the ~8 s figure above is extrapolated, not measured |
| Multiple clients active at once | one client at a time is the session model; concurrent *posts* were tested, concurrent *clients* were not |
| Backup duration and restore time at volume | the dump was measured at 72 KB on a development database, which proves nothing |
| Memory under load | a report loads every line into Go before grouping |
| Test isolation | the tests share one database and accumulate rows; the login test counts only its own prefix because of it |
