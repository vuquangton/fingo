# Production readiness review — verdict

**Reviewer:** independent review agent (SRE + application security + accounting systems).
**Commit:** `835bf8d` on `v2` · **Go** 1.27.0 · **PostgreSQL** 16.15 · **Date:** 2026-10-18.
**Environment:** throwaway databases `fingo_review` (created for this review) and
`fingo_test`; no client data; no source file modified.

## Verdict: **NO-GO**

Two independent blockers, both narrow and both fixable within a week:

1. **The database does not enforce the period lock.** A voucher dated inside a locked
   period was inserted and posted directly with SQL as the application role, and the
   transaction committed (`attacks2.sql` §K, output `attacks2.out.txt`). The Go layer
   refuses it; the database does not. This is exactly what the review's gate item 1
   requires, and it is the one control an accountant would assume is absolute.
2. **The shipped toolchain is vulnerable.** `govulncheck ./...` reports **11 reachable
   vulnerabilities**: 10 in the Go 1.27.0 standard library (fixed in 1.27.2) and one in
   `quic-go v0.59.0` (fixed in v0.59.1), reached through `gin`'s HTTP/3 support.

Everything else in the gate checklist is either met or has a documented gap that a
practice can accept knowingly. The core of the system — tenancy, immutability,
balance, allocation, audit — **held under direct SQL attack**, which is the hardest
part of this kind of system to get right and the part most likely to have been wrong.

## Gate checklist

| # | Gate | Result | Evidence |
| ---: | :--- | :--- | :--- |
| 1 | DB refuses unbalanced, edited, deleted, locked-period, cross-client writes | **NO** | 4 of 5 refused; locked period accepted — `02-attack-log.md` |
| 2 | RLS forced on every tenant table; app role not owner/superuser/BYPASSRLS | **YES** | 15/15 `relrowsecurity=t relforcerowsecurity=t`, owner `postgres`, `fingo_app` `rolsuper=f rolbypassrls=f` |
| 3 | No unauthenticated route except login and health | **YES** | `/clients` anon → 401; `/` → 303 `/login`; `/login` → 200 |
| 4 | CSRF and XSS attacks fail | **YES** | cross-origin POST → 403; `<script>` in a partner name rendered escaped |
| 5 | Audit log append-only for the app role | **YES** | `UPDATE audit_log` → `audit_log is append-only: UPDATE refused` |
| 6 | Restore drill succeeded with matching checksums | **YES** (small dataset) | 4 vouchers / 1,601,000 both sides after restore |
| 7 | Backups run on schedule and a copy exists off the machine | **NO** | script + task installer exist; no schedule registered, no offsite copy |
| 8 | `REQUIRE_DB=1 go test -race` green, zero skips, CI enforces | **NO** | plain suite 21 ok / 0 fail / 0 skip; `-race` cannot build here (no gcc); CI never executed |
| 9 | `govulncheck` shows no reachable high-severity vulnerabilities | **NO** | 11 reachable (10 stdlib, 1 quic-go) |
| 10 | Survives crash, DB restart, reboot without data loss | **NO** | not tested here; no service wrapper on Windows |
| 11 | Realistic-load performance meets stated targets | **CONDITIONAL** | 465k lines aggregated in 0.80 s; upper bound (50 clients × 5 y) not tested |
| 12 | Operator completes a month by keyboard and books agree | **NO** | plain form, no grid, no pilot parallel run |
| 13 | Runbooks followed successfully by someone other than the author | **NO** | not attempted |
| 14 | Legal limitations stated; unresolved items not presented as final | **YES (docs)** / **NO (UI)** | `legal-basis.md`, operator guide §12; no `?` markers are rendered anywhere |

Gate items 1 and 7 are **NO**, and the instruction is explicit: any NO on 1–7 is
automatically NO-GO.

## Readiness score

| Domain | Score | One line |
| :--- | :---: | :--- |
| 1 Build & reproducibility | **4/5** | clean build, pinned deps, embedded assets; vulnerable toolchain, no `-race` locally |
| 2 Money & accounting correctness | **4/5** | balance, immutability, allocation all refused by PostgreSQL; postability and period lock are app-only |
| 3 Multi-tenancy | **5/5** | 15/15 forced RLS; cross-client insert refused; no scope ⇒ 0 rows |
| 4 AuthN/Z, sessions | **3/5** | solid primitives, no rate limit, password change does not revoke other sessions, no security headers |
| 5 Audit & traceability | **4/5** | append-only at DB; login successes are not audited |
| 6 Data protection & recovery | **3/5** | restore verified; no schedule registered, no offsite, no alerting |
| 7 Operations & observability | **2/5** | no Windows service, no metrics, HEAD 404, no CSP |
| 8 Performance & capacity | **4/5** | 465k lines ≈ 0.8 s; no index on `(client_id, account_code)` |
| 9 UI & operator readiness | **2/5** | no grid, no pilot, no print layout |
| 10 Legal & compliance | **3/5** | guardrails documented; no UI marking of unresolved figures |
| 11 Test & process | **3/5** | 21 packages green, 0 skips; race unproven; CI unrun |
| 12 Deployment & upgrade | **2/5** | single binary + PG fine; no service, no tested rollback, no hardware spec |

## Top 10 blockers, ranked

| Rank | Blocker | Fix effort |
| ---: | :--- | :--- |
| 1 | Period lock not enforced by the database | S |
| 2 | Go 1.27.0 → 1.27.2 (10 reachable stdlib vulns) | S |
| 3 | `quic-go` 0.59.0 → 0.59.1 (reachable via gin) | S |
| 4 | No registered backup schedule, no offsite copy, no failure alert | M |
| 5 | `-race` never proven green; CI workflow has never run | S–M |
| 6 | Password change does not invalidate other sessions | S |
| 7 | No login rate limiting or lockout (`failed_logins` unread) | M |
| 8 | Chart postability and stock guards are Go-only | M |
| 9 | No security headers (CSP, frame, no-sniff) | S |
| 10 | No Windows service; crash/DB-restart behaviour unproven | M |

## What this verdict is not saying

It is not saying the accounting engine is wrong. Under direct SQL attack the engine's
core invariants held: an unbalanced voucher was **refused at commit**, a posted
voucher and its lines could not be **updated or deleted**, an over-allocation was
**refused**, a cross-client write was **refused by row-level security**, and a scope-less
session saw **zero rows**. Those are the failures that destroy an accounting system, and
they did not happen.

It is saying two specific, provable things are not ready: the period lock is a
convention rather than a constraint, and the binary is built from a vulnerable
toolchain. Both are days, not months.
