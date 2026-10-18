# 00 — Executive Summary

**FinGo** is a single-user, on-premise Windows desktop accounting application for a Vietnamese accounting practice that keeps the statutory books of many SME clients. It is written in **Go 1.26**, renders its UI with the dormant Win32 toolkit **`lxn/walk`**, and ships with a **bundled MariaDB 12.3** server it starts and manages itself. Domain behaviour follows **Circular 133/2016/TT-BTC** (as amended by TT 46/2025).

The engineering is far better than the repository suggests. The repository is a **Wails skeleton committed on 2026-09-20**, and **96% of the actual system — 285 Go files, 64,116 lines, 15 migrations, 16 design documents — has never been committed**. That is the defining fact of this discovery.

## What exists (verified)

- A correct, tested **double-entry engine**: one account/one side/one amount per line (ADR 0001), a posting gate that actually fails on imbalance, immutable posted vouchers enforced in both domain and database triggers, transactional posting, idempotency keys, UTC discipline.
- A **TT133 chart of 123 accounts** transcribed from the gazette, period ranges with irreversible locks, a close that carries P&L through TK 911 into TK 4212 with depreciation and bad-debt provisions, and trial balance / P&L / balance sheet computed from posted lines with per-row voucher traceability.
- **848 test functions**, 30,097 test LOC (46.9% of all Go), stdlib-only, no mocking library, no `TODO`/`FIXME` markers anywhere.
- A working **native shell** with bundled-database provisioning, single-instance guard, and five operator screens (home, chart of accounts, general journal, trial balance, journal entry). `dist\FinGo.exe` (12 MB + 36 MB server) built at 2026-10-12 21:31, one minute after the last code change.

## What does not exist

- **No authentication, no authorization, no enforced client boundary.** `client.Scope` has zero call sites outside its own package. This is an accounting practice's system and there is no login.
- **0 of 18 business events are usable end-to-end by an operator.** 12 have engine code; 4 have none; 2 are stubs whose arithmetic the project's own review calls wrong.
- **No statutory `mã số chỉ tiêu`** — every filing line is emitted as a warning instead. Statutory filing is hard-blocked on reading TT 89/2026's appendices.
- **No CI, no scheduled backup, no installer, no login, no audit-log table.**

## Health scorecard

| Dimension | Score | One-line justification |
| :--- | :---: | :--- |
| Documentation | **4** / 5 | Exceptional volume and quality (`legal-basis.md`, `requirements-v1.md`, ADRs, per-module status) — but the two READMEs and `GEMINI.md`'s build commands are stale |
| Code quality | **4** / 5 | Clean layering, pure presentation models, comments that explain *why*; penalised for 30 unreachable packages and two generations of duplicated domain models |
| Test coverage | **3** / 5 | 848 tests and a genuine TDD habit, but no CI, DB tests skip silently without `REQUIRE_DB`, and the widget layer is untested |
| Security | **2** / 5 | Parameterised SQL and real immutability; zero authentication/authorization/tenancy enforcement, plaintext DB password |
| Buildability | **3** / 5 | One-command build proven by a fresh artifact; impossible from a fresh clone, `rsrc` missing, MariaDB discovery is hard-coded |
| Completeness | **2** / 5 | 5 of 17 rail destinations; no end-to-end flow; statutory output deliberately unwired |
| Maintainability | **3** / 5 | Good structure and naming; heavy dead code, colliding abandoned branch, dual migration runners |
| **Overall** | **3.0** / 5 | A high-quality engine with no delivery spine |

**Completion estimate: 35–40% of v1.** Two of 13 stated acceptance criteria are met, six partial, five not met (four absent, one broken); engine ~65–70% complete, operator-facing product ~25%.

## Why it stopped

Not a technical failure. The project hit a **legal reading wall** — it needs the indicator numbering from TT 89/2026's appendices and refused to invent it — while the work it *could* do kept moving without being committed. It also pivoted UI four times in five days (Next.js → WPF → Wails/Vue → walk), each pivot made before the previous one was committed. [INFERRED, medium confidence — see 04 §6]

A second, abandoned implementation lives on `backup-before-alpha-rollback` (25 commits, 130 files, its own `00002…00013` migrations, and the only copies of `docs/BRD-*`/`SPEC-*`).

## First two weeks for a new engineer

**Day 1 — before reading anything, make the work recoverable.**
1. `git add -A && git commit` in coherent slices (engine, migrations, UI, docs) and **push**.
2. `cd fingo; .\build.cmd` — confirms the toolchain and produces `dist\`.
3. `$env:REQUIRE_DB=1; go test ./...` against a local MariaDB — a green run **with** `REQUIRE_DB=1` is the only meaningful green.
4. Tag the abandoned branch: `git branch -m backup-before-alpha-rollback archive/2026-09-20-pre-rollback` and note it must not be merged.

**Days 2–3 — read in this order, not alphabetically.**
`GEMINI.md` (178 lines, the real state doc) → `docs/requirements-v1.md` → the five ADRs → `internal/ui/nav/rail.go:69-113` (the 12 honest "not built" reasons = the backlog) → `docs/legal-basis.md:131-163` (why P4 is blocked).

**Day 4 — run it, don't read it.** Launch `dist\FinGo.exe` on a machine with no MariaDB; enter a two-line balanced voucher; post it; read the trial balance; then drill through a row to the ledger. Compare against `docs/gui-*.png`. Then delete a `voucher_lines` row in MySQL and watch the trigger refuse — that is the quality level of the rest of the engine.

**Week 2 — pick one vertical and finish it.** Sales is closest: engine (`internal/trading/sales.go`), presentation model (`internal/ui/sales`), posting rules and COGS all exist and are tested; it lacks only a screen and a `true` in `cmd/fingo/main.go:426`. Wiring it end-to-end proves the architecture and gives the operator their first real workflow.

**People to ask, in this order:** the author (Q1, Q2, Q22–Q24), the accountant who will use it (Q5, Q7, Q8, Q15, Q16, Q20), a licensed tax practitioner (Q13, Q14).

**Quick wins:** commit + push (hours, removes the only irreversible risk); delete the 11 dead `internal/domain/*` packages or move them under `internal/experimental`; remove the `TRACE` print; rewrite the root `README.md`.

## Recommended path forward: **Stabilize then continue**

**Resume as-is is wrong** (nothing is committed; auth/tenancy absent). **Partial rewrite is wasteful** (the engine is the best part and is well tested). **Retire is unjustified** (this is a coherent, unusually well-reasoned design).

Recommendation: **stabilize the repository and the tenancy model, then continue the existing P1→P3 plan**, with statutory filing (P4) explicitly deferred until Q13 is answered by a human with the gazette appendices.

| Order | Action | Why |
| :--- | :--- | :--- |
| 1 | Commit + push everything; add CI with `REQUIRE_DB=1` | Removes unrecoverable risk; makes "green" mean something |
| 2 | Decide the tenant model (Q5) and enforce it in repository signatures | Blocks every future feature; the two current mechanisms contradict |
| 3 | Add authentication + append-only audit, or formally accept single-user attribution | Turns a filing tool into a defensible one |
| 4 | Wire sales, then purchase, then close (P1→P3) | Converts the best code into operator value |
| 5 | Defer statutory filing until `mã số` is closed; ship B01/B02 PDFs as internal statements | Avoids wasting effort on a blocked output |
| 6 | Retire the duplicate domain generation and the second migration runner | The largest maintainability tax |

## The five questions that matter most

1. **Q1** — Is the uncommitted working tree the real codebase, and where must it land? (only unrecoverable risk)
2. **Q2** — Is `backup-before-alpha-rollback` to be mined or deleted? (130 files of unique requirements; colliding migrations)
3. **Q5** — One database per client **or** a `client_id` scope? (both are half-built; nothing else can be designed until this is settled)
4. **Q7 / Q8** — Which accounting regime, and for which fiscal year must this go live? (sizes all of v1)
5. **Q13 / Q14** — Who supplies the `mã số` from TT 89/2026, and what replaces Circular 48/2019 after 2026-12-31? (statutory output and the close engine are legally blocked)

---

*Ten-line summary:*
1. **Stack:** Go 1.26 · `lxn/walk` Win32 · bundled MariaDB 12.3 · sqlc + goose · Windows desktop, on-premise.
2. **What it is:** VAS/TT133 bookkeeping for an accounting practice serving N SME clients.
3. **State:** engine ≈ 65–70% done; product ≈ 25%; **5 of 17 screens**; **0 of 18 flows end-to-end**.
4. **Completion:** 35–40% of v1 (2/13 acceptance criteria met, 6 partial, 5 not met).
5. **Risk #1:** 96% of the code is uncommitted and unpushed — one bad command from total loss.
6. **Risk #2:** no authentication, no authorization, no enforced client boundary.
7. **Risk #3:** a parallel 25-commit implementation with colliding migration numbers.
8. **Risk #4:** statutory output has no `mã số`; filing is legally blocked.
9. **Risk #5:** no CI, no scheduled backup, no installer; database tests skip silently.
10. **Verdict:** **Stabilize then continue** — commit, settle tenancy, add auth/audit, finish sales→purchase→close, defer filing.
