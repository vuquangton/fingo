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
