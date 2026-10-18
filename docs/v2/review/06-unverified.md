# Unverified

Everything I could not test, why, and what would settle it. None of these is a claim of
success.

| # | Item | Why not tested | What would settle it |
| ---: | :--- | :--- | :--- |
| U-01 | `-race` clean | No C compiler on this host: `go test -race` fails with `-race requires cgo`. `gcc` is absent from `PATH`. | Install gcc, or read the first green GitHub Actions run |
| U-02 | CI workflow executes | This environment has no Actions runner; `.github/workflows/test.yml` has never run. | First push to a repository with Actions enabled |
| U-03 | Windows service behaviour | No `svc.Run` wrapper exists. Only a scheduled-task installer was reviewed, not executed. | Install as a service, reboot, kill the DB, observe restart |
| U-04 | systemd unit | Linux host unavailable; the unit file was read only. | `systemctl start fingo` on a real Linux box, then `systemctl kill -s SIGTERM` mid-post |
| U-05 | Crash mid-transaction | Killing the process during an open posting transaction was not driven. | Send SIGKILL 100 ms into a post, restart, assert ledger state |
| U-06 | Migration on a populated database / failed migration | Only the empty-database path was run (`schema at version 7`). | Apply to a copy of a real database; force a failing migration and follow the documented recovery |
| U-07 | Production-size restore | The dump was 78 KB. | Dump/restore the largest real database; record time and checksums |
| U-08 | Offsite backup, retention, encryption, alerting | No schedule registered, no offsite target exists. | Register the task, copy off-machine, delete a dump and confirm the alert |
| U-09 | Business-continuity RPO/RTO | Depends on a schedule that is not running. | State the schedule, then measure actual loss window |
| U-10 | Capacity at 50 clients × 5 years | Largest measured: 465k lines in one database. | Seed the stated upper bound and measure; decide on the missing index first |
| U-11 | Windows file permissions, disk-full, permission-denied behaviour | Not exercised. | Fill a scratch volume, run a post, observe the error |
| U-12 | Vietnamese collation and sorting of names in reports | Sorting is by code/date; name ordering was not tested with real Vietnamese data. | Insert names with diacritics, compare `ORDER BY` to the accountant's expectation |
| U-13 | Print/export layout of ledger and trial balance | Excel export exists; printed output was not visually reviewed. | Print one month of each and compare to the practice's current output |
| U-14 | Keyboard-only month entry | No grid exists; the plain form was not timed with a keyboard-only operator. | Time an accountant entering 50 lines without a mouse |
| U-15 | Legal: `?` figures, Circular 48 expiry, retention, mã số | Requires the accountant's sign-off, not a test. | `legal-basis.md` reviewed by a Vietnamese practitioner; confirm the 2026-12-31 provision end date is parameterised |
| U-16 | IDOR across every route | Client scope comes from the session and no route takes a client id, so the surface is structurally small; not every route was individually probed. | Enumerate routes, attempt cross-client ids on each |
| U-17 | Session fixation under a real browser | `RenewToken` on login is in code; not exercised with two browsers and a pre-set cookie. | Browser test: copy a pre-login cookie, log in, confirm it changed |
| U-18 | Excel import hostile files | Formula injection, zip bombs, oversized sheets were not tested. Upload is capped at 8 MiB by `io.LimitReader`. | Upload crafted workbooks; verify export neutralises leading `=`, `+`, `-`, `@` |
| U-19 | `restore-drill.ps1` script as documented | The drill was performed with raw `pg_dump`/`pg_restore`, not the project script. | Run the script literally, as a non-author |
| U-20 | Email/password reset | No reset flow exists to test. | Decide whether one is needed at all for an on-premise practice |

## Deliberately not attempted

- **Destructive tests against a real database.** The instruction was a throwaway instance;
  that was honoured.
- **Scanning the repository for secrets beyond name-level checks.** `.env` exists and is
  gitignored; no secret value is reproduced in this review.
- **Modifying the product to "make a test pass".** The review is read-only for source.
