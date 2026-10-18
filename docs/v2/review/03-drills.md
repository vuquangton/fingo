# Drills

## 1. Backup → destroy → restore

| Step | Command | Result |
| :--- | :--- | :--- |
| Dump | `pg_dump -Fc -f drill.dump fingo_review` | exit 0, **78 KB**, **0.9 s** |
| Destroy | `DROP DATABASE fingo_review_restore` | dropped/absent |
| Recreate | `CREATE DATABASE fingo_review_restore TEMPLATE template0` | ok |
| Restore | `pg_restore -d fingo_review_restore drill.dump` | exit 0, **0.8 s** |
| Verify rows | `SET app.client_id=…; SELECT count(*) FROM vouchers` | source **4**, restored **4** |
| Verify figures | `SUM(debit) / SUM(credit)` per tenant | source **1,601,000 / 1,601,000**, restored **1,601,000 / 1,601,000** |

**Verdict: PASS** for the mechanism, on a 78 KB database. This proves the dump is
restorable by a human with `pg_restore` and the documented credentials; it does **not**
prove restore time at real size, and it does not test the documented drill script
(`scripts/restore-drill.ps1`), which was not executed — see `06-unverified.md`.

## 2. Crash / restart

| Step | Result |
| :--- | :--- |
| Server started with a throwaway cert and DSN | started, served `GET /login` 200 |
| Server killed (`Stop-Process -Force`) | process gone |
| Restart | not exercised with an in-flight posting transaction |

**Verdict: NOT TESTED.** The graceful-shutdown path exists (`context.WithTimeout(10s)`
at `cmd/fingo/main.go:137`) but no test drove a SIGTERM/SIGKILL through an open posting
transaction. A crash mid-transaction relies on PostgreSQL rolling back, which is
sound in principle and unproven here.

## 3. Upgrade / migration

| Step | Result |
| :--- | :--- |
| Migrations applied to an empty database | `fingo_review: schema at version 7` in one run |
| Migration applied to a populated database | not executed |
| Rollback (`goose down`) | not executed |
| Failed-migration recovery | not executed |

**Verdict: NOT TESTED** beyond the empty-database path. `goose` runs each migration in a
transaction where the SQL allows it, but a migration that fails halfway on a populated
database has not been rehearsed, and the project's own guidance is "restore from backup
instead of rolling back" — which is a policy, not a drill.

## 4. TLS

| Probe | Result |
| :--- | :--- |
| HTTPS `GET /login` | 200, HTTP/1.1 |
| Plain HTTP to the TLS port | 400 (cleartext refused) |
| Certificate | self-signed development cert, 825 days, `localhost + 127.0.0.1` |

**Verdict: PASS (mechanism) / UNVERIFIED (production cert).** No production certificate,
renewal process or expiry alarm was exercised.

## What a real drill needs

1. A production-sized copy (≥ 1 GB or the real client database) with the clock measured.
2. The documented script, not raw `pg_dump`, run by someone who has not read the source.
3. A kill during an open posting transaction, followed by a ledger check.
4. A migration failure on a populated copy, followed by the documented recovery.
