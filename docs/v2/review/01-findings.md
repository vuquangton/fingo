# Findings

Severity: `BLOCKER` / `HIGH` / `MEDIUM` / `LOW`. Confidence: `[HIGH]` observed,
`[MEDIUM]` observed indirectly, `[LOW]` inferred.

| ID | Domain | Severity | Finding | Evidence | Conf | Fix | Effort |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| F-01 | Accounting | **BLOCKER** | The period lock is not enforced by PostgreSQL: a posted voucher dated inside a locked period is accepted. | `docs/v2/review/attacks2.sql` §K → `COMMIT` succeeded; lock row existed (`attacks.sql` §4) | HIGH | Trigger on `vouchers` that refuses a posted voucher whose date falls in a locked period; or a FK from period to lock plus a constraint trigger | S |
| F-02 | Security | **BLOCKER** | 10 reachable stdlib vulnerabilities in Go 1.27.0, fixed in 1.27.2 (`net/http`, `net/http/internal/http2`, `crypto/tls`, `html/template`, `mime/multipart`). | `govulncheck.out.txt`: GO-2026-6617, -6613, -6612, -6611, -6609, -6608, -6607, -6603, -6600, -6599 | HIGH | Upgrade toolchain to 1.27.2, rebuild, re-run govulncheck | S |
| F-03 | Security | **HIGH** | `quic-go v0.59.0` reachable (GO-2026-5676, HTTP/3 QPACK memory exhaustion), pulled in by `gin`'s `http3` package. | `govulncheck.out.txt` #11; `go mod why` → `fingo/cmd/fingo → gin → quic-go/http3` | HIGH | `go get github.com/quic-go/quic-go@v0.59.1`; if HTTP/3 is never enabled, consider excluding it | S |
| F-04 | Recovery | **HIGH** | No backup schedule is registered, no off-machine copy, no failure alert. A script and a task installer exist; neither was verified running on this host. | `scripts/backup.ps1`, `install-backup-task.ps1`; no task present | HIGH | Register the task, add offsite copy, alert on dump age | M |
| F-05 | Testing | **HIGH** | `-race` has never been shown green in this environment (no C compiler); the CI workflow that would run it has never executed. | `test-race.out.txt`: `-race requires cgo`; no gcc on PATH | HIGH | Add gcc to the build host or run CI; record a race-clean run | S–M |
| F-06 | Auth | **HIGH** | Changing a password does not invalidate other sessions. `ChangePassword` updates only `password_hash`. | `internal/auth/users.go` (UPDATE password_hash); sessions live independently in `sessions` | HIGH | Delete the user's sessions on password change; test it | S |
| F-07 | Auth | **MEDIUM** | No rate limiting or lockout on login. `failed_logins` rows are written and never read. | `internal/web/auth.go` recordFailure; no reader | HIGH | Lockout or per-IP throttle; a policy decision first | M |
| F-08 | Accounting | **MEDIUM** | Chart postability is enforced only in Go: SQL can post to a heading account (`411`). | `attacks.sql` §6 committed; nonexistent accounts are refused by FK `voucher_lines_client_id_account_code_fkey` | HIGH | Add a `postable` column and a composite FK, or a trigger | M |
| F-09 | Accounting | **MEDIUM** | Stock guards are Go-only: SQL can insert an issue with no stock and an empty source document. | `attacks2.sql` §L committed | HIGH | Trigger on `stock_movements`, or document DB-level non-enforcement | M |
| F-10 | Tenancy | **MEDIUM** | The application role can read and modify `clients` without RLS: it deactivated every tenant and read all names/tax ids. | `attacks3.sql` §P `UPDATE 2`, §Q `clients_visible=4` | HIGH | Keep INSERT for onboarding, but restrict UPDATE to a controlled function; or accept as a documented trust boundary | M |
| F-11 | Web | **MEDIUM** | No `Content-Security-Policy`, `X-Frame-Options` or `X-Content-Type-Options`. | `curl -I /login` header dump; no header code in `main.go`/`web` | HIGH | Add three headers in one middleware | S |
| F-12 | Web | **LOW** | `HEAD /login` returns 404 (Gin registers GET only). External monitors that probe with HEAD see a dead service. | `curl -I https://…/login` → 404 | HIGH | Register HEAD or add a health probe to the docs | S |
| F-13 | Audit | **MEDIUM** | Successful logins and logouts are not audited; only failures are recorded. | `audit_log` schema vs `auth.go`; no `ActionLogin` exists | HIGH | Add `LOGIN`/`LOGOUT` audit rows | S |
| F-14 | Ops | **MEDIUM** | No Windows service wrapper. Deployment is a scheduled task; a task is not a service (no restart policy, no start-order dependency on PostgreSQL). | `cmd/fingo` has no `svc.Run`; `installer-notes.md` acknowledges | HIGH | `golang.org/x/sys/windows/svc` wrapper | M |
| F-15 | Ops | **MEDIUM** | No metrics, no request IDs, no structured access log of slow requests; `/healthz` is static and does not check the database. | `main.go` `/healthz` returns `ok` unconditionally | HIGH | `/healthz` should ping the DB; add request IDs | S |
| F-16 | Ops | **MEDIUM** | TLS certificate is generated for 825 days with no renewal or expiry monitoring; expiry is silent. | `cmd/devcert` output; no expiry check | HIGH | Calendar/automation + startup warning when < 30 days | S |
| F-17 | Accounting | **LOW** | Voucher numbering is gap-free per (client, book, year) via advisory lock — correct under 20 parallel posts — but a failed transaction can leave a consumed sequence number visible as a gap in the *sequence*, not the number. | `TestParallelPostsGetDistinctNumbers`; `ledger/post.go` | MEDIUM | Document; no action if gaps in `seq` are acceptable | — |
| F-18 | Data | **LOW** | `documents.due_date` is nullable and nothing sets it; aging silently assumes +30 days. | `internal/books/aging.go`; no writer of `due_date` | HIGH | Expose the field on the sales/purchase forms | S |
| F-19 | Ops | **LOW** | `goose` migrations are forward-only in practice; no rollback or forward-fix drill was executed. | `cmd/migrate`; no down-run in review | HIGH | Rehearse a failed migration on a copy | M |
| F-20 | Legal | **MEDIUM** | No UI marks unresolved statutory figures (`?` in `legal-basis.md`); a user cannot tell which numbers are provisional. | `legal-basis.md`; no `?` rendering in templates | HIGH | Render a banner on affected reports | S |

## What was checked and passed (not findings)

| Control | Result |
| :--- | :--- |
| Unbalanced voucher, direct SQL | Refused at commit: `voucher … is unbalanced: debit 1000000, credit 900000` |
| Update a posted voucher | `posted voucher GH2026-0001 is immutable` |
| Update/delete a posted line | `voucher … is posted; its lines are immutable` |
| Delete a posted voucher | `posted voucher GH2026-0001 cannot be deleted` |
| Over-allocate | `allocation 150000 exceeds document … (total 100000, already allocated 0)` |
| Cross-client insert | `new row violates row-level security policy for table "vouchers"` |
| Scope-less read | `vouchers_visible_without_scope = 0` |
| Another client visible while scoped | `b_vouchers_visible_from_a = 0` |
| `SET LOCAL` after commit | `app.client_id` is empty — no session-level leakage |
| App role DDL | `must be owner of table vouchers` |
| Audit update | `audit_log is append-only: UPDATE refused` |
| CSRF | cross-origin POST → 403 |
| XSS | `<script>alert("x")</script>` stored and rendered as `&lt;script&gt;` |
| Cookie | `HttpOnly`, `Secure`, `Path=/`, `SameSite=Lax` (code) |
| Restore drill | 4 vouchers, debit = credit = 1,601,000 on both sides |
