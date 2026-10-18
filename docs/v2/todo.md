# FinGo v2 — engineering todo

Maintained by the solution architect. One list, ordered by what actually blocks the
next step, not by what is interesting.

Conventions:

- **P0** blocks the pilot. **P1** is needed in a real client's first month. **P2** is
  scale or polish.
- Estimates are engineer-days for one person who knows the code.
- Every item has a *done* test. "Written" is not done.
- WIP limit: one P0/P1 feature at a time, plus a standing 20% bug budget.

---

## Board

| Now (this fortnight) | Next (4–6 weeks) | Later |
| :--- | :--- | :--- |
| A1 pilot client · A2 parallel run · A4 CI green · A5 production host · B3 test isolation | B1 aging · B2 report tie-out · C1 stock transfer · C3 bill from receipt | C2 three-way match · D1 grid spike · D2 keyboard entry · E2 scale guard |

---

## A — Pilot readiness (P0)

### A1 · Pilot client and data
- **Owner:** practice lead, with the accountant who will run it.
- **Work:** choose one client (service firm preferred: no stock, no three-way match),
  import opening balances, fix the chart if their accounts differ.
- **Done:** `GET /opening` accepted a balanced file; trial balance equals their
  current software's opening trial balance to the đồng.
- **Risk if skipped:** everything else is a guess.

### A2 · Parallel month and difference log
- **Owner:** the accountant; the architect reviews the log.
- **Work:** run one full month in FinGo *and* their existing software. Log every
  difference in `docs/v2/pilot/differences.md` with: what differed, which side is
  right, classification.
- **Classifications:** `bug` · `scope` (we do not do it) · `expectation` (we do it
  differently on purpose) · `data` (their data was wrong).
- **Done:** every difference classified; no unexplained `bug` left open.
- **Note:** this is the Phase 6 gate. It cannot be closed from a keyboard alone.

### A3 · Bug budget
- **Owner:** engineer.
- **Work:** 20% of every week reserved for what A2 finds. No new feature displaces it.

### A4 · CI actually green on GitHub
- **Owner:** engineer.
- **Work:** push, watch `.github/workflows/test.yml` run; fix whatever the runner
  disagrees with (service container, role creation, `-race` timing).
- **Done:** a green run on `v2`, no skipped results, visible in the repo.
- **Why P0:** the two commits pushed with red tests are the proof that a human gate
  is not enough.

### A5 · Production host
- **Owner:** whoever runs the office machine; engineer pairs once.
- **Work:** real certificate, DSN from environment, service installed, backup task
  registered **and observed running once**, restore drill executed against the
  production database.
- **Done:** a dump from production restored into a scratch database; figures match;
  the run is recorded in `restore-drill.md`.

---

## B — Correctness hardening (P1)

### B1 · AR/AP aging
- **Work:** 30/60/90 buckets from `documents.due_date`, per partner, on the receipt
  and payment screens and as an export.
- **Done:** a test with three invoices at 10, 45 and 100 days produces the expected
  bucket totals; totals reconcile to the 131/331 control accounts.

### B2 · Report tie-out
- **Work:** property-style tests asserting the reports agree with each other:
  trial balance debits = credits; balance sheet assets = liabilities + equity;
  P&L net profit = change in 4212 for the period; ledger rows sum to the trial
  balance line.
- **Done:** a seeded book (random but balanced vouchers) satisfies all four.
- **Why:** three reports that each look right can still disagree. This is the test
  that catches it.

### B3 · Per-test database isolation
- **Work:** truncate client-scoped tables between tests, or give each test its own
  schema. Removes the accumulation that made the login test fail only in a suite.
- **Done:** the full suite passes twice in a row on the same database without cleanup
  by hand, and no test counts rows it did not create.

### B4 · Numeric edge cases
- **Work:** max int64 amount rejected gracefully; zero-amount voucher refused;
  quantity zero refused; a credit of exactly the outstanding amount settles.
- **Done:** each case has a test and a human-readable refusal.

---

## C — Trading completeness (P1–P2)

### C1 · Stock transfer between warehouses (P1 if a client has two locations)
- **Work:** paired movements out/in at the same average cost, one voucher, no P&L
  effect; warehouse dimension on the movement.
- **Done:** total quantity and total value unchanged across the transfer.

### C2 · Three-way match (P2)
- **Work:** order → goods receipt → supplier bill; bill blocked when it exceeds what
  was received; variance visible, not silent.
- **Done:** a bill for more than was received is refused with both quantities named.
- **Note:** a control, not a figure. Do not build before a client asks.

### C3 · Bill from a receipt (P2)
- **Work:** create the supplier bill from an existing stock receipt, carrying item,
  quantity and unit cost, so the same goods are not entered twice.
- **Done:** one action produces a bill whose 156 credit equals the receipt's 156 debit.

---

## D — Usability (P2, behind a go/no-go)

### D1 · Tabulator spike
- **Work:** a two-day box: keyboard-only entry of 50 lines, tab/enter/arrow navigation,
  paste from Excel. Then decide.
- **Done:** either a working grid behind a feature flag, or a written decision not to
  use it, with the reason. **No third outcome.**

### D2 · Keyboard-first entry
- **Work:** only if D1 says no: make the plain HTML entry form tab-order correct and
  add row duplication.

### D3 · Onboarding wizard
- **Work:** one flow — client, chart confirmation, opening balances, first partner and
  item — so a new client takes an hour, not an afternoon.

---

## E — Operations and scale (P2)

### E1 · Linux backup timer
- `fingo-backup.timer` beside the Windows task; same script, same drill.

### E2 · Scale guard (only if measured)
- **Trigger:** a real client exceeds 200k voucher lines.
- **Work:** index-only scan on `voucher_lines (client_id, account_code, side)` or a
  materialised period balance. Measure before and after.
- **Not now:** nothing measured asks for it.

### E3 · Restore drill in CI (nightly)
- Dump the seeded database, restore into a scratch DB, compare row counts and one
  trial balance figure.

### E4 · Monitoring
- Backup age alarm (a backup that stopped is worse than none, because it is believed),
  slow-query log review monthly, `/healthz` watched.

---

## Decisions owed (block items)

| # | Decision | Blocks | Default if undecided by the pilot |
| :--- | :--- | :--- | :--- |
| D-1 | VAT rate model: per line, per period, or date-keyed menu | multi-rate clients | stay at 10% and document |
| D-2 | Deployment: Windows service, scheduled task, or Linux systemd | A5, E1 | scheduled task (works today) |
| D-3 | Backup retention and offsite copy | A5 | 30 days local, no offsite — **accept the risk in writing** |
| D-4 | Lockout after N failed logins | security posture | none; failures recorded only |
| D-5 | Who may close a period (admin only, today) | role model | keep admin-only |
| D-6 | Whether the pilot client's chart must match their existing codes | A1 | yes — accountants navigate by code |

---

## Release gates

| Gate | Definition | Current |
| :--- | :--- | :--- |
| **G1 Pilot-ready** | A4, A5 done; B3 done; opening balance equals their current software | not met |
| **G2 First month closed** | A2 done, no open `bug`, close and lock executed on real data | not met |
| **G3 Second client** | onboarded in under one working day using D3 | not met |

---

## What I would reject, and why

| Request | Why not |
| :--- | :--- |
| More reports before A2 | Report shape is guesswork until an accountant reads one against their own |
| Microservices / message queue | one office, one database, one binary; the coupling is an asset |
| Caching balances before measurement | adds a second source of truth to fix a problem nobody has observed |
| A second UI framework | the entry grid is the only gap; it does not justify a rewrite |
| Feature work in the 20% bug budget | the budget exists because pilots find things no plan predicts |
