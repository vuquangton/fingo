# Remediation plan

Ordered by *risk removed per hour*. Nothing here is a rewrite; the engine's accounting
invariants already survived direct SQL attack.

## Phase 1 — before go-live (days)

| # | Action | Fixes | Owner | Effort | Done when |
| ---: | :--- | :--- | :--- | :--- | :--- |
| 1 | Toolchain **1.27.0 → 1.27.2**, rebuild, re-run `govulncheck` | F-02 | build | S | `govulncheck` reports 0 reachable in stdlib |
| 2 | `go get github.com/quic-go/quic-go@v0.59.1`, or drop gin's http3 import | F-03 | build | S | `govulncheck` clean; `go mod tidy` committed |
| 3 | Migration: refuse a posted voucher whose date falls in a locked period (constraint trigger on `vouchers`), with a test that posts directly in SQL | F-01 | db | S | `attacks2.sql` §K now errors; `REQUIRE_DB=1 go test` green |
| 4 | Invalidate all sessions for a user on password change | F-06 | backend | S | New test: old cookie 401 after change |
| 5 | Register the backup task, add offsite copy, alert when the newest dump is > 26 h old | F-04 | ops | M | `Get-ScheduledTaskInfo` shows a successful run; alert fires on a deleted dump |
| 6 | Run the CI workflow once on a real runner; record the URL | F-05 | build | S | Green run on `v2`, no skips, `-race` included |
| 7 | Add CSP (`default-src 'self'`), `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff` | F-11 | web | S | Header dump shows all three |
| 8 | `/healthz` checks the database (`SELECT 1`, 2 s timeout) | F-15 | web | S | Returns 503 when PostgreSQL is stopped |

## Phase 2 — first client month (1–2 weeks)

| # | Action | Fixes | Effort |
| ---: | :--- | :--- | :--- |
| 9 | Postability as data: `accounts.postable` + composite FK from `voucher_lines`, or a trigger | F-08 | M |
| 10 | Trigger on `stock_movements`: no issue beyond on-hand, no movement without a source document | F-09 | M |
| 11 | Login throttling / lockout policy — **decide first**, then implement | F-07 | M |
| 12 | Audit successful login and logout | F-13 | S |
| 13 | Expose `due_date` on sales and purchase screens | F-18 | S |
| 14 | Windows service wrapper (`x/sys/windows/svc`) or a written acceptance that a scheduled task is the deployment model | F-14 | M |
| 15 | Migration failure rehearsal on a copy, and a written rollback/forward-fix decision | F-19 | M |

## Phase 3 — accept or fix (within 30 days)

| # | Item | Recommendation |
| ---: | :--- | :--- |
| 16 | Tenant registry writable by the app role | **Accept**, documented: the app must onboard clients. Compensate with a database trigger that forbids deleting a client, and an audit row on every client change |
| 17 | Missing `voucher_lines (client_id, account_code, side)` index | Add only when a real client passes ~200k lines; measure first |
| 18 | `HEAD` returns 404 | Fix or document for the monitoring tool |
| 19 | Certificate renewal | Calendar + startup warning at < 30 days |
| 20 | `?` statutory figures not marked in the UI | Banner on the affected reports |

## What can be accepted as a documented risk

| Risk | Why acceptable now | Condition of acceptance |
| :--- | :--- | :--- |
| App role can read the client registry | It must, to onboard and switch clients | Written in `installer-notes.md`; add audit on changes |
| No lockout on login | Office LAN, one practice, fail log recorded | Revisit before any internet exposure |
| Restore only proven at 78 KB | Mechanism verified | Must be re-run at production size before sign-off |
| No `-race` locally | Windows host lacks gcc | CI must provide it; local claim withdrawn |

## Re-review criteria

A second review is **GO** when:

1. `attacks2.sql` §K errors and a new test proves it;
2. `govulncheck ./...` reports zero reachable vulnerabilities;
3. a scheduled backup has run **and** a restore from it matched checksums;
4. one green CI run on `v2` with `-race` and zero skips is linked;
5. session invalidation and security headers are proven by test;
6. a crash-restart and a failed-migration drill have been executed and recorded.

Item 6 is the only one that needs a production-like host; the rest fit in one working day.
