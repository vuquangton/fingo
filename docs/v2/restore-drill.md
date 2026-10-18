# Restore drill

**Run:** 2026-10-18, on the same machine as the database, against `fingo_dev`.

## Why a drill, and not just a backup

A dump that has never been restored is a file, not a backup. The only evidence that a
backup is restorable is a restore that produces the same figures, into a database that
did not exist, followed by the proof that they match. `scripts/restore-drill.ps1` does
exactly that and exits non-zero when they do not.

Both scripts use **PostgreSQL's own tools** (`pg_dump`, `pg_restore`, `psql`) rather
than a hand-written dumper. v1 wrote its own and its runner was never reachable from
the application — a backup that could not run was indistinguishable from one that did.

## The drill

```
$env:PGADMIN_PASSWORD = '...'
.\scripts\backup.ps1              # pg_dump -Fc, verified with pg_restore --list, 30-day retention
.\scripts\restore-drill.ps1       # createdb scratch → pg_restore → compare → drop
```

Observed output:

```
backup: D:\accounting\backups\fingo-fingo_dev-2026-10-18_12-03-40.dump (56,075 bytes)

drill: D:\accounting\backups\fingo-fingo_dev-2026-10-18_12-03-40.dump → fingo_restore_drill
source : 123|0|0
restore: 123|0|0
drill: PASS — counts match and the restored ledger balances
```

The three figures are `accounts|vouchers|Σ(debit − credit)`. The third being 0 is the
ledger invariant: a restore that produced unbalanced lines would fail the drill even if
the row counts matched.

## Deliberate choices worth knowing

| Choice | Reason |
| :--- | :--- |
| Dump as the **owner**, not `fingo_app` | row-level security would filter an app-role dump to zero rows, producing a backup that restores an empty database. The script refuses without `PGADMIN_PASSWORD`. |
| `pg_restore` exit code is a warning, not a verdict | `pg_restore` exits 1 on non-fatal grant notices. The drill's own assertions decide success; the warning is printed. |
| Scratch database always dropped, in `finally` | a failed drill must not leave a half-restored database behind that someone later mistakes for a real one. |
| 30-day retention, counted in files | simple, inspectable, and independent of any scheduler. |

## Limits of this record

The development database held **123 seeded accounts and no vouchers**, so this drill
proves the schema, the data copy and the balancing assertion — not a year of trading
history. The drill must be re-run against a pilot client's database, with vouchers,
before the Phase 6 parallel run; that is recorded as an open item.

Scheduling is **not** wired yet: `scripts/backup.ps1` is a script a human or a scheduler
runs, and the Windows service / systemd unit that would run it is outstanding.