# FinGo v2 — test report

**Run:** 2026-10-18 on `v2`, against the system PostgreSQL 16.15 (`127.0.0.1:5432`, database `fingo_test`), as the application role `fingo_app` (`NOSUPERUSER NOBYPASSRLS`).

## Gate

| Command | Result |
| :--- | :--- |
| `go build ./...` | **rc=0** |
| `go vet ./...` | **rc=0** |
| `REQUIRE_DB=1 go test ./... -count=1` | **rc=0** |

| Metric | Value |
| :--- | ---: |
| Packages `ok` | **16** |
| Test results recorded (tests + subtests) | **788 pass** |
| Failures | **0** |
| Skips | **0** |
| Wall time | ~75 s |

**Zero skips is the point.** `REQUIRE_DB=1` turns a missing database into a failure (`internal/testdb`), so this run proves the database path executed rather than being silently skipped — the v1 defect the discovery report called its highest-value fix (`discovery/04-pending-work.md`).

## The 11 database tests that carry the gate

`internal/store/store_test.go`:

| Test | What it proves |
| :--- | :--- |
| `TestScopeIsRequired` | `InClient` refuses a zero scope; there is no "all clients" call |
| `TestUnscopedQuerySeesNoRows` | a query with no `app.client_id` returns **0** rows, not everybody's |
| `TestAppRoleCannotBypassRLS` | the suite's role has neither `rolsuper` nor `rolbypassrls` — the boundary is observable |
| `TestClientsCannotSeeEachOther` | client A sees only its own voucher; B's row by id is invisible (no enumeration oracle) |
| `TestWriteForAnotherClientIsRefused` | a write naming another client fails `WITH CHECK` |
| `TestUnbalancedVoucherIsRefusedAtCommit` | a deferred constraint trigger refuses debit 1000 / credit 900 **at COMMIT** |
| `TestPostedVoucherIsImmutable` | a posted voucher cannot be updated or deleted |
| `TestAllocationCannotExceedItsDocument` | 600 + 600 against a 1000 document is refused |
| `TestPeriodLockIsIrreversible` | a lock cannot be deleted (append-only trigger) |
| `TestEveryBusinessTableIsClientScoped` | `information_schema` sweep: any business table without `client_id` fails the suite |
| `TestChartIsSeededForEveryClient` | a provisioned client has exactly 123 TT133 accounts |

## Ported unchanged

`money`, `vat`, `posting`, `period`, `document`, `partner`, `inventory`, `trading`, `close`, `reports`, `domain/gl`, `domain/document`, `usecase/ledger`, `gltest`, `format`, `pkg/logger` — code and tests, with these two exceptions only:

| File | Change | Why it is not a behaviour change |
| :--- | :--- | :--- |
| `internal/reports/statutory/statutory_test.go` | `readLegalBasis` walks **up** to the repo root instead of `filepath.Join("..","..","..","..","docs",...)` | The v2 port moved the module from `fingo/` to the repository root. The old chain was a fact about directory depth; every assertion still reads the same file and checks the same content. |
| `internal/format/money_test.go` | import `fingo/internal/ui/format` → `fingo/internal/format` | Package moved out of the deleted `internal/ui` tree. No test body changed. |

No other test was edited. No test was deleted. No assertion was weakened.

## Not yet converted: money is still `decimal` in the engine

`internal/domain/gl` (and the packages that consume its amounts: `reports`, `close`, `posting`, `usecase/ledger`) still uses `shopspring/decimal`. ADR 0008 requires `int64` đồng everywhere.

- It does **not** affect this gate: the database is `bigint`, the store API takes `int64`, and no test asserts a fractional amount.
- It **is** a stack deviation and the loop is not closed until it is converted.
- Plan: one slice, `decimal.Decimal` → `int64` in `gl`, then mechanical propagation; tests change integer literals only and every changed test is listed here when it happens.

## Coverage

Not measured in this report. `go test -cover` was not run because a coverage number without assertion review is a gap finder, not a target; the gate is behavioural (tenant, immutability, balance) rather than a percentage.

## Money conversion (ADR 0008) — done

`shopspring/decimal` is gone from `go.mod`. `money.Dong` is `int64` whole đồng, with `Add`/`Sub`/`Mul`/`Div` (truncating), comparisons, identity `Round`/`Truncate`, and JSON as a **string**. `bigint` in PostgreSQL, `int64` in Go, string on the wire.

Behaviour preserved where it mattered: `monthlyRate` already did `.Div(...).Truncate(0)`, and the bad-debt charge already truncated — integer division truncates the same way, so no figure moved. All 788 assertions still pass.

Deliberate behaviour changes, all at the **input boundary**:

| Change | Why |
| :--- | :--- |
| `format.Amount` refuses a fractional amount (`"1.234,56"`, `"0,5"`) | đồng has no subunit; rounding what the operator typed would move a figure silently |
| `format.Money` no longer renders a fractional part | there is no fraction to render |
| `gl.VoucherLine.Amount` is `money.Dong` | a fractional line is unrepresentable |

Tests changed (no assertion weakened, four obsolete tests removed because the state they described no longer exists):

| File | Change |
| :--- | :--- |
| `internal/money/dong_test.go` | **new** — arithmetic, truncation direction, division-by-zero panic, JSON string round-trip above 2^53 |
| `internal/format/money_test.go` | integer literals; fractional-formatting test removed; new `TestAmount_RefusesAFraction` |
| `internal/domain/gl/gl_test.go` | `TestVoucher_FractionalDongIsRejected` removed (unrepresentable); "fractional đồng in storage" case removed |
| `internal/close/provisions_test.go` | three fractional-asset cases removed; `wholeDong` guard deleted as dead |
| `internal/reports/statutory/statutory_test.go` | helper `money`→`dong`; `readLegalBasis` walks to repo root |
| `internal/reports/statutory/review_test.go` | fractional JSON case and obsolete `roundDong` test removed; `money(...)`→`dong(...)` |

Gate re-run after the conversion: `go build` rc=0 · `go vet` rc=0 · `REQUIRE_DB=1 go test ./...` **16 packages ok, 0 failures, 0 skips**.
## Phase 2 slice — auth and audit (gate green, phase partial)

Added and tested:

| Piece | Library / mechanism | Tests |
| :--- | :--- | :--- |
| Password hashing | `golang.org/x/crypto/argon2` (argon2id, 64 MiB, t=3, p=2), PHC string, `crypto/subtle` comparison | right password, wrong password, per-call salt, seven malformed-hash refusals |
| Audit record | `audit_log` table (append-only trigger), one row inside the changing transaction | written with the change, rolled back with the change, UPDATE refused, missing actor refused |
| Session table | `scs` schema for its pgxstore, migration `00002` | migration applies |

Gate: `REQUIRE_DB=1 go test ./...` → **18 packages ok, 0 failures, 0 skips**.

**Still outstanding in Phase 2** (deliberately not faked): Gin login/logout handlers and session middleware, HTTPS with a generated certificate, `pg_dump` backup schedule with retention, the scripted restore drill, and the Windows service / systemd unit files. The gate items "unauthenticated request rejected" and "restore drill succeeds" therefore remain **open**.
## Phase 2 addendum — HTTP auth and the restore drill

| Gate item | Status | Evidence |
| :--- | :--- | :--- |
| Unauthenticated requests rejected | **PROVEN** | `internal/web`: GET /books with no session → 401; after login → 200; after logout → 401 |
| Audit rows for every post/lock | **PROVEN** | `internal/audit`: row written in the same transaction; rolled back with it; UPDATE refused; missing actor refused |
| Restore drill succeeds | **PROVEN** | `docs/v2/restore-drill.md`: dump → restore into a fresh database → `123|0|0` matched both sides; ledger balanced |
| Phase gate overall | **NOT MET** | HTTPS with a generated certificate and the Windows service / systemd unit are outstanding |

Auth surface, 8 tests: unauthenticated 401 · login then protected 200 · wrong password 401 and recorded in `failed_logins` · unknown email indistinguishable from wrong password (same status) · viewer allowed to read, refused 403 on an admin route · logout ends the session · missing credentials 400.

Libraries doing the work: `gin` (routing), `scs` + `scs/pgxstore` (sessions), `x/crypto/argon2` (hashing), `net/http.CrossOriginProtection` (CSRF), `pg_dump`/`pg_restore` (backup). Hand-written: the middleware that decides who is allowed, and the audit insert.