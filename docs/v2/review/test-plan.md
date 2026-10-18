# Test plan

Written before execution. Kept as the record of what was intended, so the gaps between
this and `01-findings.md` are visible.

## Environment

| Item | Value |
| :--- | :--- |
| Commit | `835bf8d` on `v2` |
| Database | throwaway `fingo_review`, created and migrated for this review; `fingo_test` read-only for web probes |
| Role under attack | `fingo_app` — `NOSUPERUSER`, `NOBYPASSRLS`, not the table owner |
| Server | `docs/v2/review/fingo-review.exe`, TLS with a throwaway certificate, bound to `127.0.0.1:8443` |
| Changes to source | **none**; only `docs/v2/review/` was written |

## Method

Default assumption: a claim is false until a command shows otherwise. Code reading is
used only where execution is impossible, and is marked `[UNVERIFIED]` when it is.

## Order and rationale

| Order | Domain | Why here |
| ---: | :--- | :--- |
| 1 | Direct SQL attacks on money and accounting | Highest blast radius; a wrong ledger is unrecoverable |
| 2 | Tenancy catalog + cross-client attempts | Second highest; one leak ends the product's premise |
| 3 | Build, tests, vulnerability scan | Cheap, and a vulnerable toolchain can invalidate everything else |
| 4 | AuthN/Z, CSRF, XSS, session | The internet-facing surface |
| 5 | Audit immutability | Depends on a scoped session, so after 2 |
| 6 | Backup and restore | Needs the database from 1 |
| 7 | Performance | Needs a populated database |
| 8 | Ops, legal, UI | Mostly read-only; untestable parts go to `06-unverified.md` |

## Attack list (executed as written)

Unbalanced voucher · update posted voucher · update/delete posted line · delete posted
voucher · post into locked period · over-allocate · post to heading account · post to
nonexistent account · cross-client insert · cross-client line attachment · cross-client
allocation · scope-less read · other-client read · session-scope leakage · DDL as app
role · audit update/delete · mint tenant · read tenant registry · deactivate tenants ·
stock issue without stock · unauthenticated route · CSRF · XSS · cookie flags ·
plaintext HTTP · HEAD request · security headers · 20 parallel posts.

## Stop conditions

If a BLOCKER were found in money or tenancy, later phases would still run but their
findings would be reported as secondary. Two blockers were found, both in domain 1/3
adjacent areas (period lock) and domain 1 (toolchain), so the plan ran to completion.
