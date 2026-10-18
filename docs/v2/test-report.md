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
## Phase 2 close-out — the server binary

`cmd/fingo` now exists and was smoke-tested on 2026-10-18:

```
$ go run ./cmd/fingo -addr 127.0.0.1:18443
{"level":"INFO","msg":"listening","addr":"127.0.0.1:18443"}

$ curl -k https://127.0.0.1:18443/healthz     → 200 "ok"
$ curl -k https://127.0.0.1:18443/books       → 401   (no session)
$ curl    http://127.0.0.1:18443/books        → connection refused (TLS only)
```

TLS certificate comes from `cmd/devcert` (standard library `crypto/x509`), because a
Secure session cookie over plain HTTP is a cookie the browser never sends back.
`internal/web.TLSConfig` sets TLS 1.2 as the floor.

`deploy/fingo.service` is the systemd unit; it reads the DSN from
`/etc/fingo/fingo.env`, never from the unit itself, because a unit file is
world-readable and appears in `systemctl cat`.

**Not implemented:** a Windows SCM service. `sc.exe create` with a plain executable
does not work — the binary must call `svc.Run` — and that wrapper is not written. Windows
deployments therefore run the binary under a scheduled task or manually until it is.
## Phase 3 read screens

| Screen | Route | Source of the figure |
| :--- | :--- | :--- |
| Chart of accounts | `GET /accounts` | `accounts` rows for the scoped client |
| General journal | `GET /journal` | posted vouchers, debit total per voucher |
| Trial balance | `GET /reports` | `internal/reports.TrialBalance` over `internal/books` ports — **not** a second SQL sum |

`internal/books` is the assembly: it implements `reports.Periods`, `reports.Journal` and `ledger.ChartSource` over the scoped store, so the report engine that already had 88 tests now reads PostgreSQL with no change to its own code.

Three end-to-end tests: chart lists 5111/33311/911; journal shows the posted voucher and its 1.100.000 total; trial balance balances and names the period and voucher count.

Defect found by the test: `accounting_periods.id` is a `uuid`, so a readable `FY2026` id failed at INSERT. The database now generates it (`RETURNING id`).
## Phase 3 — reports, drill-down, Excel

| Route | Screen | Proven by |
| :--- | :--- | :--- |
| `GET /reports/pnl` | Kết quả kinh doanh | revenue 1.000.000, net profit, closing vouchers excluded count |
| `GET /reports/balance-sheet` | Bảng cân đối kế toán | three sections, totals, balances |
| `GET /ledger?account=131` | Sổ chi tiết TK | voucher line and running balance |
| `GET /reports/export.xlsx` | Excel export | file opened with excelize; account 131 closing debit read back as 1100000 |

No figure is computed in a handler. `internal/books` exposes them from `internal/reports`, whose 88 tests were already green before PostgreSQL existed.
## Phase 5 — period close (gate met)

`GET /close` lists periods and their lock state; `POST /close` runs `internal/close`.

Three end-to-end tests:

| Test | Assertion |
| :--- | :--- |
| screen | period listed as open, with the close button |
| close | after the close, **revenue nets to zero**, the period row is locked, and exactly one report is stored |
| after close | a voucher dated inside the locked period is refused with 422 |

Two defects the tests forced out:

1. **Closing lines carry no id.** `internal/close` builds entries as account/side/amount; the ledger inserted `''` into a `uuid` column. The ledger now issues a line id when the domain left it empty — the same rule for any domain-built voucher.
2. **The close's `Poster` needs the ordinary posting path.** `ledger.Post` took a form `Request`; `PostVoucher` was extracted so a domain-built voucher travels the same rules, numbering, transaction and audit row. No second path to the ledger exists.
## Phase 4 prerequisite — partners

A sale cannot post without a partner: `trading` refuses an invoice with no `PartnerID`, because TK 131 is kept by customer. So the first Phase 4 slice is master data, not the invoice.

| Piece | What it does |
| :--- | :--- |
| migration `00004` | `partners` table, RLS forced, unique `(client_id, name)`, partial unique tax id, walk-in allowed with no tax code |
| `internal/books/partners.go` | List · Create (validated kind, duplicate refused as a readable error) · WalkIn (idempotent standing retail customer) |
| `internal/web/partners.go` + template | list + add form on one page |
| tests | empty state · create appears in list · duplicate 422 with a readable message · blank name 422 |

Defect found by the test: the redirect carried a Vietnamese name unescaped in the query string, so the followed redirect was not a valid URL. Escaped now with `url.QueryEscape`.
## Phase 4 — sales invoice posts

`internal/books` now assembles the sales flow: a pgx `document.Repository` (7 methods, `Apply` under `SELECT … FOR UPDATE`), the VAT rate in force, TT133 posting rules, and a poster that sends trading's voucher through the ordinary ledger path.

`TestSalesInvoicePostsRevenueVATAndReceivable` posts 2 × 500.000 at 10% and asserts:

| Assertion | Value |
| :--- | --- |
| invoice total / VAT | 1.100.000 / 100.000 |
| TK 131 debit | 1.100.000 |
| TK 5111 credit | 1.000.000 |
| TK 33311 credit | 100.000 |
| receivable | open, total 1.100.000, 3 document lines |

Three defects surfaced, each a real design fact:

1. **VAT registry holds the rate in force, not a menu.** Two active rates may not cover one date, so the 0/5/8/10 menu cannot be registered. Standard 10% is registered; a per-line rate dimension is recorded as an open question.
2. **trading derives `invoice-<uuid>` as the voucher id**, which is not a uuid. The ledger now generates the database key when the domain id is not one — the domain keeps its own identifier.
3. **A document's lines mirror its postings** (AR debit, revenue credit, VAT credit), not the invoice's item lines. The test was corrected to the engine's actual contract.
## Phase 4 — sales screen

`GET /sales` shows the partner picker and line form; `GET /sales` with no partners sends the operator to add one first. `POST /sales` prices, taxes and posts through `internal/trading`.

Three tests: no partner → guidance instead of a dead form · invoice posts (131 1.100.000 / 5111 1.000.000 / 33311 100.000, one open receivable) · empty invoice refused 422.

The engine still owns every figure: the handler parses a form and converts `money.Dong` to `money.Money`. No VAT arithmetic in the web layer.
## Phase 4 — receipt and allocation (AR cycle closed)

`GET /receipt` lists open invoices with the outstanding amount; `POST /receipt` posts Dr 1111 / Cr 131 and settles the invoice.

| Test | Assertion |
| :--- | :--- |
| settle | document SETTLED, allocated 1.100.000, 1 allocation row, cash debit 1.100.000 |
| overpayment | 2.000.000 against 1.100.000 is accepted; one credit row of 900.000 |

Three defects, each a schema or layering fact:

1. **`document.Validate` requires a period**, and our `documents` table had no `period_id`. Migration `00006` adds it and backfills from the date's period.
2. **The domain derives readable ids** (`allocation-<uuid>`, `credit-<uuid>`) that are not uuids. The adapter generates the database key, as the ledger already does for vouchers.
3. **A receipt is two steps**: post the voucher, then apply the payment. A crash between them leaves an unallocated receipt — visible in the ledger, never a document marked paid by money that was not recorded. Retry-safe: the voucher id is derived from (client, document, amount, date), so a retry collides with itself.
## Phase 6 — opening balances from Excel

`GET /opening` uploads a workbook; `POST /opening` posts one voucher.

| Rule | Where it is enforced |
| :--- | :--- |
| totals must agree | `gl.Voucher.Post` — the same check every voucher gets |
| accounts must be postable leaves | `gl.PostingRules.Chart` |
| period must be open | `gl.PostingRules.Periods` |
| same file twice is refused | voucher id derived from a sha256 of the workbook |

Three tests: import (4 lines, 800.000.000 each side) · unbalanced refused with **nothing written** · same file twice leaves exactly one voucher.

One defect worth recording: the first test used account `411`, and the engine refused it — `411` is a heading in the TT133 chart with postable children. The test was wrong; the engine was right. A user would have seen the same clear refusal.
## Phase 6 — per-client handover export

`GET /handover` streams a zip: one CSV per client-scoped table plus `MANIFEST.json`.

The export is `COPY (SELECT * FROM t) TO STDOUT` **inside the scoped transaction**, so row-level security is the filter. A Go-written SELECT per table would be a second, weaker guarantee; this one cannot include another client because those rows are not visible to the query.

Test: two clients exist; the first exports; the archive holds exactly 1 voucher row, 123 account rows, the manifest names only the exporting client, and no row of the second client appears.

`clients` is deliberately absent from the export — it is the practice's registry, not the client's books.
## Phase 4 — supplier bill (cost path)

`GET /purchase` posts a supplier invoice for a cost: Dr expense / Dr 1331 / Cr 331, and records an open payable.

The three-way match is deliberately **not** used here: `trading.Purchasing.Invoice` requires an order and its receipts, which is the warehouse path. A service or utility bill has no goods receipt, so this path is narrower, not a second implementation.

`TestPurchaseBillPostsCostInputVATAndPayable`: 1.000.000 net at 10% → 6422 debit 1.000.000, 1331 debit 100.000, 331 credit 1.100.000, payable document `PURCHASE_BILL` open at 1.100.000.
## Phase 4 — supplier payment (AP cycle closed)

`GET /payment` lists open supplier bills; `POST /payment` posts Dr 331 / Cr 1111 and settles the bill through the same `Documents.Apply`.

| Test | Assertion |
| :--- | :--- |
| settle | bill SETTLED, allocated 1.100.000, 331 debit = 1111 credit = 1.100.000 |
| settled bill | paying it again is refused 422 |

Bills and sales invoices share `document.TypeInvoice`, so the payable list reads the stored `kind` (`PURCHASE_BILL`) rather than guessing from the domain type — the same distinction the export adapter already makes.
## Phase 6 — documentation closed

`operator-guide.md` is the Vietnamese task guide (13 sections, ending with the explicit list of what the product does **not** do yet). `installer-notes.md` is the operations note: prerequisites, provisioning, TLS, service install, backup, upgrade, and the five things that must never be done.

No code changed, so the gate is unchanged: 21 packages ok, 0 failures, 0 skips. The Phase 6 **gate** — a parallel run against one client's current software — is not something this repository can execute; it needs a client, a period of real data, and a human to compare output.
## Phase 4 — stock ledger and goods receipt

`GET /stock` lists items with on-hand, average cost and carrying value; `POST /stock/item` stocks an item; `POST /stock/receive` records a receipt and posts Dr 156 / Cr 331.

The valuation is **replayed** from the movement log through `internal/inventory` on every read. There is no stored balance that could drift from the history that produced it, and a receipt is refused by the engine before any voucher is written if it would make stock negative.

| Test | Assertion |
| :--- | :--- |
| receipt | 156 debit = 331 credit = 1.000.000; one movement; screen shows 1.000.000 and 100.000 |
| no source document | 422 — a movement nobody can trace to a document is refused |

Migration `00007` adds `items`, plus `unit_cost` and `source_doc_id` on `stock_movements`.
## Phase 4 — cost of sales

A sale now posts two vouchers: revenue (131/5111/33311) and the cost of what left the warehouse (632/156). The cost comes from `inventory.RecordIssue`, which values the issue at the moving average and hands back an unposted voucher; the ledger numbers and posts it like any other.

The stock check happens **before** the sale is posted: a sale the warehouse cannot cover is refused 422 and no revenue line is written.

| Test | Assertion |
| :--- | :--- |
| sale of 2 from 10 @ 100.000 | revenue 1.000.000, receivable 1.100.000, **COGS 200.000**, inventory credit 200.000, on hand 8 |
| sale of 5 from 3 | 422, **zero** revenue lines posted |

Ordering is deliberate: the sale posts first, then its cost. A crash between them leaves a sale whose cost can be posted by hand — visible — rather than revenue silently carrying no cost.
## Phase 4 — stocktake adjustment

`POST /stock/count` brings an item to what a physical count found.

| Direction | Posting | Why |
| :--- | :--- | :--- |
| shortage | Dr 632 / Cr 156 at the moving average | goods are gone; the cost is a cost |
| surplus | Dr 156 / Cr 711 at the current average | the goods exist and nobody paid for them; calling it revenue would be a fiction |

Test: 10 on hand, count 7 → 632 debit 300.000, 156 credit 300.000, on hand 7. A count that matches the ledger is refused 422 — there is nothing to adjust, and posting a zero voucher would be noise.
## Operable milestone — clients and users

| Screen | Route | Who |
| :--- | :--- | :--- |
| Client registry | `GET /clients`, `POST /clients`, `POST /clients/:id/select` | any user lists/selects; **admin** creates |
| Users | `GET /users`, `POST /users` | **admin only** |
| Own password | `POST /password` | any signed-in user, current password required |

Creating a client also provisions its 123-account TT133 chart, inside a transaction scoped to the new client — row-level security permits exactly those rows.

Four tests: create client → chart provisioned → switch → empty books · accountant cannot create a client (403) · admin creates a user who can sign in, and a viewer cannot post (403) · password change refuses a wrong current password (403) and works with the right one.

**Authorization bug found and fixed:** write routes had no role check, so a `viewer` reached handlers and failed with 409 (no client selected) instead of 403. A `RequireWriter` middleware (admin + accountant) now guards every mutating route, and period close is admin-only.
## Reversal — the correction model, reachable

`GET /reverse` + `POST /reverse` post the mirror of a posted voucher: same accounts, opposite sides, same amounts, dated when the mistake was noticed. The original is never touched.

| Test | Assertion |
| :--- | :--- |
| reverse | two vouchers exist, the books net to **0**, the mirror is labelled "Đảo bút toán …" |
| reverse twice | 422 — a second mirror would undo the correction silently |

The refusal is by description lookup, which is the ledger's own record of what the entry is; no extra table was added for it.
## Operations — CI and a backup that happens

`.github/workflows/test.yml` runs the gate the repository has been running by hand: build, vet, schema as owner, then the whole suite as `fingo_app` with `REQUIRE_DB=1` and `-race`, plus a step that fails if any result is a skip.

`scripts/install-backup-task.ps1` registers the nightly dump as a Windows scheduled task. No password is on the task's command line — `backup.ps1` falls back to the libpq password file, which is the standard mechanism and keeps the secret out of process listings.

Both scripts are syntax-checked by the PowerShell parser; the task registration itself was not executed on this machine (it would add a scheduled task), so it is reviewed, not proven.
## Validation — volume, numbering under load, unauthenticated cost

Three tests added for the failure modes that would otherwise appear in production:

| Test | Result |
| :--- | :--- |
| `TestVolumeIsSurvivable` (25,000 vouchers / 50,000 lines) | trial balance 767 ms, P&L 759 ms, journal 690 ms |
| `TestParallelPostsGetDistinctNumbers` (20 concurrent posts) | 20/20 succeeded, 20 distinct numbers and sequences, debits = credits |
| `TestFailedLoginsAreRecordedAndCostNothing` (30 unknown emails) | 2 ms each, all recorded, no argon2 work |

Numbers and extrapolations are in `docs/v2/performance.md`. The volume test is skipped under `-short`; CI runs it.
## Tie-out — the reports agree with each other

`TestReportsAgreeWithEachOther` posts capital, a sale with output VAT, its collection, a cost with input VAT and its payment, then asserts five things:

1. trial balance period debits/credits equal the ledger's own sum;
2. the period balances, and closing balances;
3. every account with movements has exactly one row, and no row invents an account;
4. net profit = revenue − expenses, and equals the balance sheet's unclosed `Result`;
5. assets = liabilities + equity, with the unclosed result inside equity.

Expected figures are computed by hand in the test (profit 700,000; assets 5,800,000) so the assertions are not merely self-consistent.

The first version expected assets of 5,770,000 and failed: **the test forgot the 30,000 input VAT still sitting on 1331.** The balance sheet was right, the test was wrong — the same kind of error the test exists to catch in the code.
## Aging — AR and AP by age, tied to the control account

`GET /aging?as_of=YYYY-MM-DD` buckets open documents by days overdue: current, 1–30, 31–60, 61–90, over 90 — per partner, with totals.

Two decisions worth recording:

1. **A document with no due date is aged from document date + 30 days.** The invoice date is a fact; the payment term is a policy. Treating a missing term as "due immediately" would overstate every overdue column for a client who never fills it in.
2. **The control figure comes from the voucher lines, not from the documents.** Subledger and control account are two independent records of the same debt; the screen prints both and marks them `khớp` or `lệch <amount>`. An aging report that cannot disagree with the ledger is not a check.

Test: three invoices at 149, 76 and 10 days before 30 June 2026 land in over-90, 31–60 and current; the total is 3.300.000 and equals the 131 balance. A malformed `as_of` is refused 422.
## Sign-in page — the defect that made everything else unreachable

The drift report found that only `POST /login` existed: `GET /` answered 404 and there was no form, so a human could not create the session every screen requires. The tests passed because the harness posts directly — the suite had a door the product lacked.

| Route | Behaviour |
| :--- | :--- |
| `GET /` | redirect to `/login` when anonymous, `/clients` when signed in |
| `GET /login` | the form; a signed-in visitor goes to `/clients` |
| `POST /login` (browser, `Accept: text/html`) | 303 to `/clients`, or the form again with a message on failure |
| `POST /login` (program) | 204 / 401, unchanged, so existing tests keep proving the API contract |

A wrong password and an unknown email render **byte-identical** pages: the address is deliberately not echoed back, or the page itself would say which addresses exist.

Also fixed while here: the period lock now writes a `PERIOD_LOCK` audit row (the Phase 2 claim was false), the `GET /books` stub is gone, and the missing-migration hint names a command that exists.

Tests: root redirects to the form · form sign-in reaches `/clients` · identical bodies for both failure kinds · one audit row per lock.