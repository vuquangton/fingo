# Testing strategy

**Status:** proposed · 2026-10-09
**Scope:** everything under `fingo/`.

This is written against the suite that exists, not against a blank page. Where
the repo already does the right thing it says so and leaves it alone; where it
does not, §6 and §7 say exactly what is wrong and what to change.

---

## 0. The shape of the suite today

| | |
| :--- | :--- |
| Test files | 92 |
| Database-backed test files | **14** |
| Test functions that need a database | **81** |
| Of those, gated by `testsupport.Config` / `testsupport.Admin` | 58, in 8 files |
| Of those, carrying their own `testing.Short()` + `.env` walk | 23, in 6 files |
| CI | **none** — no `.github`, no GitLab/Azure config, no Makefile |
| Linter config | **none** — no `.golangci.yml` |
| Stray artifact | `fingo/$cov`, 21,961 bytes — a coverage profile written to a path that never interpolated |

**What the repo already gets right, and should keep:**

- **Ports declared where they are used.** `reports.Journal`, `ledger.ChartSource`,
  `web.Books`, `close.Periods`. This is the seam rule in §1, applied consistently.
- **Real implementations beside in-memory ones.** `document.MemoryRepository`,
  `partner.MemoryRepository`, `period.StaticRepository`, `vat.Registry`,
  `web/books`. Most business rules are testable with no container at all.
- **No mocking library, and no need for one.** Nothing in the tree imports
  `testify` or `go.uber.org/mock`. Every "mock" is a hand-written type with a
  field on it, which is what §1 recommends as the default.
- **Scratch databases go through `db.Up`,** which runs goose **and** re-applies
  `db/triggers/*.sql`. A test that migrated with the goose CLI alone would be
  testing a schema the application never runs against.
- **`fingo/web` tests over `httptest.NewServer`** with no `HX-Request` header —
  a real HTTP surface, driven the way a browser drives it.

**The one structural problem is §6.**

---

## 1. Unit tests

### Philosophy

A unit test is fast (< 1ms), deterministic, and does no I/O. If it opens a
socket, a file or a database, it is an integration test — and it belongs in §2,
where it will be run under different flags and reported differently.

The useful question is not "how many layers am I mocking" but **"does this touch
the outside world"**. The repo is already structured so the answer is usually
"no": pricing, settlement, chart rules, posting decisions and report arithmetic
are all pure functions or in-memory types. Keep it that way. A rule that can only
be tested with a container is a rule that needs a seam, not a fixture.

### Substituting collaborators

Use, in this order:

1. **A hand-written fake** — a struct with fields, defined in the test file.
   Right answer for any port with ≤ 4 methods. Readable, no codegen, no
   `EXPECT()` DSL, and it beats a mock because it can hold state.
2. **An in-memory repository** — `document.MemoryRepository` and friends. Tests
   the caller against the real interface, not against a recording.
3. **`httptest.Server`** — for anything HTTP. This is a fake *server*, not a
   mock: it exercises the real client, including JSON decoding, status handling
   and timeouts. Mocking an `*http.Client` skips all of that, which is where the
   bugs live.

**Never mock `*sql.DB`, `http.Client` or a third-party struct.** A test that
asserts "the code called the methods I told the mock to expect" is a
change-detector test: it fails on every refactor and passes on every bug.

If a port ever does need a generated mock (large interface, call-order
matters), use `go.uber.org/mock`. `github.com/golang/mock` is archived.

### Naming

`TestSubject_Behaviour`, with the condition as the subtest:

```go
func TestChartOfAccounts_RejectsDuplicateCode(t *testing.T)
func TestSettle_CarriesAnOverpaymentIntoACredit(t *testing.T)

t.Run("a period with no vouchers", func(t *testing.T) { ... })
t.Run("an account the chart does not hold", func(t *testing.T) { ... })
```

A failing test name should read as a sentence a reader can act on. `TestFoo`,
`TestFooWorks` and `TestFoo_ShouldBar` are all rejected: the first says nothing,
the second says nothing when it fails at 03:00, and the third is Jest grammar.

### Table-driven tests

Use a **slice of structs**, never a map. Map iteration order is randomised, which
makes `-failfast` output irreproducible and hides tests that depend on a sibling
having run first.

```go
tests := []struct {
    name    string
    in      decimal.Decimal
    want    decimal.Decimal
    wantErr error
}{
    {"zero stays zero", decimal.Zero, decimal.Zero, nil},
    {"half rounds away from zero", decimal.NewFromFloat(0.5), decimal.NewFromInt(1), nil},
}
```

Two rules:

- **No logic in the table.** A computed `want` reimplements the function under
  test, and then both agree while both are wrong.
- **`t.Fatalf` for errors, `t.Errorf` for values**, so one bad case does not mask
  the other nineteen.

### What not to unit test

Generated code (`internal/adapter/mariadb/sqlc`), struct tags, accessors, the
standard library, and anything whose test would be a copy of its implementation.

---

## 2. Integration tests

Everything that touches MariaDB. The repo's canonical seam is
`internal/adapter/mariadb/testsupport`, and **new database-backed tests should go
through it rather than repeating the pattern** — six files currently do not, and
that duplication is half of §6.

### Database state

The existing strategy is **a migrated scratch database per test**:
`testsupport.MigratedDatabase(t, cfg, admin)` creates one and drops it in
`t.Cleanup`. That is the strongest practical isolation — no state leaks between
tests, no ordering dependency, safe under `t.Parallel()`.

Its cost is real: each scratch database re-runs the full migration set, which is
why `documentstore`, `periodstore` and `practice` take 50–110s each. That is
acceptable now. If the suite grows past a few minutes, the options are:

| If you need | Do |
| :--- | :--- |
| Faster, same isolation | **Template database**: migrate once, `CREATE DATABASE x TEMPLATE migrated`. Roughly 10× faster than re-migrating. |
| Fastest, weaker isolation | **One database + `TRUNCATE ... RESTART IDENTITY CASCADE`** between tests, from a canonical table list. Breaks under `t.Parallel()` unless each test owns distinct rows. |
| Perfect isolation, fastest, at a cost | **Transaction per test** — but every repository must accept a `Querier` satisfied by both `*sql.DB` and `*sql.Tx`. That is an architectural concession; the repo has not made it and should not without deciding to. |

Keep the per-test scratch database for now. Revisit only when the runtime is
actually hurting.

### Testcontainers

The repo currently assumes a MariaDB on `127.0.0.1` described by a `.env`. The
container alternative buys "works identically on a fresh clone", at the price of
a Docker dependency:

```go
ctr, err := mariadb.Run(ctx, "mariadb:11.4",          // pin the tag, never :latest
    mariadb.WithDatabase("fingo_test"),
    mariadb.WithUsername("dev"),
    mariadb.WithPassword("dev"),
    testcontainers.WithWaitStrategy(
        wait.ForLog("ready for connections").WithOccurrence(2).
            WithStartupTimeout(60*time.Second)),
)
dsn, err := ctr.ConnectionString(ctx)
```

Three things that decide whether this is reliable or flaky:

- **Wait strategies, never `time.Sleep`.** MariaDB logs "ready for connections"
  more than once — once while initialising, once for real. `WithOccurrence(2)` is
  the difference between a trustworthy suite and a 5% flake rate.
- **One container per package**, started in `TestMain`, never per test.
- **`Terminate` after `m.Run()`**, unconditionally. A leaked container will fill
  a CI runner's disk over a handful of runs.

**Recommendation:** adopt Testcontainers only if and when the suite moves to CI
*and* the `.env`-based path becomes the thing that breaks. Today the `.env` path
works and costs nothing. §6 is the more urgent problem by a wide margin.

### External third-party APIs

The repo calls none yet. When the first one arrives, the pattern is:

1. The client takes its **base URL as a field**, never a package-level constant.
2. Tests point it at an `httptest.Server` returning **fixtures captured from the
   real API**, sanitised, in `testdata/`. Hand-written JSON is a fiction that
   passes forever and fails in production.
3. Assert on **your mapping** of their response, not on the transport. "Given
   this documented payload, `ParseInvoice` returns X" is the test that catches an
   upstream change.
4. A **nightly live smoke test** against the vendor sandbox. It must not gate
   pull requests — it fails when the internet is down — but it is the only thing
   that tells you the vendor moved.

---

## 3. End-to-end

Two levels, and the distinction is load-bearing.

### In-process E2E — the workhorse

`httptest.NewServer(realRouter)` plus a real database. This is what `fingo/web`
does today, and it is the right level for almost everything: real routing, real
middleware, real serialisation, real SQL.

Assert on **observable outcomes** only — HTTP status, response body, and database
state afterwards. Never an internal field, never a log line.

### Through-the-binding E2E

`app_connect_test.go` drives `App.Connect` with a real connection and asserts the
status, the saved settings and the resolved source. This is the level at which a
Wails-bound method can be tested at all, and it is why `App` must stay a thin
marshalling layer: anything that lives in `app.go` is untestable by construction.

Note the `t.Chdir(t.TempDir())` in `TestStartup_ReportsAFreshInstallationWithoutCrashing`.
Without it, `findEnvFile` found the module's `.env` and the test **ran migrations
against the development database**. Any test that reaches `startup` needs it.

### Process-level E2E — currently absent, and that is fine

There is no "build the binary, exec it, poll `/healthz`" test. When it is wanted,
it should cover exactly four things and no more:

- migrations run correctly against an empty database at boot
- configuration is parsed, and a missing required value fails loudly
- graceful shutdown drains in-flight work
- the binary starts with no database reachable and says so rather than hanging

Three to six such tests, total. Everything else belongs in-process.

### What belongs at E2E and nowhere else

- **Idempotency keys** — the same request twice writes one row.
  `web/e2e_test.go` already has this.
- **Partial failure** — a downstream failure mid-transaction rolls back.
- **Auth and session boundaries.**

---

## 4. Go tooling

### The gate is currently unsafe

`GEMINI.md` specifies the gate as:

```
go build ./... && go vet ./... && go test ./...
```

Three problems. `go test ./...` with no flags:

- **may report cached results** — a `PASS` that was computed against different
  code. Add `-count=1`.
- **never runs the race detector.** Add `-race`; this codebase has goroutines,
  connection pools and concurrent map access.
- **runs in a fixed order**, so tests that depend on each other stay hidden. Add
  `-shuffle=on`.

Corrected gate:

```bash
gofmt -l .
go build ./...
go vet ./...
go test -race -count=1 -shuffle=on -timeout=10m ./...
```

### Flags that matter here

| Flag | Why |
| :--- | :--- |
| `-race` | Non-negotiable. Needs `CGO_ENABLED=1` and a C toolchain. |
| `-count=1` | Defeats the test cache. A cached `PASS` is not evidence. |
| `-shuffle=on` | Finds order dependence. Cheap, and it found nothing today because it has never run. |
| `-short` | The fast local loop only. See §6 for why it must never gate CI. |
| `-covermode=atomic` | Required with `-race`. |
| `-coverpkg=./...` | Cross-package coverage. Without it a package's tests do not credit the package they exercise. |
| `-timeout=10m` | Explicit beats the default panic dump, which hides the real failure. |
| `-run` / `-bench` | Narrow the loop while developing. |

### `t.Parallel()`

- Not free. It converts a deterministic failure into an intermittent one, and it
  makes output harder to read. Use it where tests are genuinely independent —
  the scratch-database pattern in §2 makes that true for most of the adapter
  suite.
- **`t.Setenv` and `t.Chdir` panic under `t.Parallel()`.** The runtime is
  protecting you. `app_connect_test.go` needs `t.Chdir`, so that test must not
  be parallel — and it is not.
- **Shared resources need coordinating.** A fixed connection pool, a shared
  container, or a `TRUNCATE` between tests will all corrupt under parallelism.
  The current per-test scratch database is immune; the moment someone switches to
  "one database and truncate", this becomes the bug.
- Set `-parallel N` explicitly in CI so a noisy neighbour is not blamed on the
  change under review.

### Other tools worth adopting

- **`go.uber.org/goleak`** — `goleak.VerifyTestMain(m)` in `TestMain`. Catches
  leaked goroutines before they are a production memory leak. Highest
  value-per-line on this list.
- **Golden files** with an `-update` flag for anything structured
  (`internal/reports/statutory` emits JSON and PDF — golden files would make
  those diffs reviewable rather than invisible).
- **`t.Context()`** (Go 1.24+) instead of `context.Background()` in tests, so a
  context is cancelled when the test ends.
- **`testing/synctest`** for time-dependent rules — period locking, VAT windows,
  depreciation. It replaces `time.Sleep` entirely.
- **Fuzzing** on parsers: the SQL splitter in `internal/migrate`, amount parsing
  in `web/formAmount`, `conn.ParseParams`.
- **`gotestsum`** or `-json` parsing in CI for per-test timing and newly-slow
  tests.

**Never `time.Sleep` in a test.** Poll with a deadline and a message that says
what was expected:

```go
deadline := time.Now().Add(5 * time.Second)
for time.Now().Before(deadline) {
    if got, _ := store.Get(ctx, id); got.Status == document.StatusPaid { return }
    time.Sleep(20 * time.Millisecond)
}
t.Fatalf("document %s never reached paid", id)
```

---

## 5. CI

There is none today. This is the recommended shape.

### Job graph

```
lint ──┐
       ├─→ unit (-short, -race) ──┐
       │                          ├─→ coverage gate
       └─→ database (testsupport)─┤
                                  │
              web/e2e (httptest) ─┘
```

Lint and unit finish in under a minute — that is the feedback loop contributors
actually feel. The database job gates the merge.

### Workflow

```yaml
name: test

on:
  pull_request:
  push:
    branches: [main]

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

env:
  GOFLAGS: -mod=readonly
  CGO_ENABLED: 1

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod, cache: true }
      - uses: golangci-lint-action@v6
        with: { version: latest }

  unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod, cache: true }
      - run: go vet ./...
      - run: |
          go test -short -race -count=1 -shuffle=on -timeout=5m \
            -covermode=atomic -coverprofile=unit.out ./...

  database:
    runs-on: ubuntu-latest
    services:
      mariadb:
        image: mariadb:11.4
        env:
          MARIADB_ROOT_PASSWORD: root
          MARIADB_DATABASE: fingo_ci
          MARIADB_USER: dev
          MARIADB_PASSWORD: dev
        ports: ['3306:3306']
        options: >-
          --health-cmd "healthcheck.sh --connect --innodb_initialized"
          --health-interval 10s --health-timeout 5s --health-retries 10
    env:
      # Written by the job, not committed: .env is gitignored, which is the
      # whole reason the suite can vanish (see §6).
      DB_HOST: 127.0.0.1
      DB_PORT: '3306'
      DB_USER: dev
      DB_PASSWORD: dev
      DB_NAME: fingo_ci
      DB_PARAMS: charset=utf8mb4&parseTime=True&loc=UTC
      REQUIRE_DB: '1'
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod, cache: true }
      - name: Write .env
        run: |
          cat > .env <<EOF
          DB_HOST=$DB_HOST
          DB_PORT=$DB_PORT
          DB_USER=$DB_USER
          DB_PASSWORD=$DB_PASSWORD
          DB_NAME=$DB_NAME
          DB_PARAMS=$DB_PARAMS
          EOF
      - name: Database-backed tests
        run: |
          go test -race -count=1 -timeout=20m ./... \
            -json > test.json || { cat test.json; exit 1; }
      - name: Assert the database tests actually ran
        run: |
          n=$(jq -r 'select(.Action=="pass" or .Action=="skip") | .Test' test.json \
              | grep -c . || true)
          req=$(jq -r 'select(.Action=="skip" and (.Output//"" | test("REQUIRE_DB"))) | .Test' test.json | grep -c . || true)
          if [ "$req" -gt 0 ]; then
            echo "::error::$req database test(s) skipped even though REQUIRE_DB=1"
            exit 1
          fi
          echo "$n test results recorded"

  coverage:
    runs-on: ubuntu-latest
    needs: [unit]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod, cache: true }
      - uses: actions/download-artifact@v4
        with: { name: unit-coverage }
      - name: Floor and ratchet
        run: |
          go tool cover -func=unit.out | tail -1
          total=$(go tool cover -func=unit.out | tail -1 | awk '{print $3}' | tr -d '%')
          awk -v t="$total" 'BEGIN { if (t+0 < 70) { print "::error::coverage "t"% < 70% floor"; exit 1 } }'
```

### Coverage policy

**Coverage is a smell detector, not a target.** A global gate set high enough to
feel meaningful produces assertion-free tests written to clear it — and those are
worse than no tests, because they cost maintenance and buy nothing.

What to enforce, in order of value:

1. **Patch coverage on the PR** — 80% of *changed* lines. This is the metric that
   correlates with defect reduction, and it does not punish untouched legacy
   code. `diff-cover coverage.out --compare-branch=origin/main --fail-under=80`.
2. **A ratchet, not a cliff** — fail if the total drops more than 1% from `main`.
   Monotonic progress, and no merge ever blocks on an arbitrary number.
3. **A modest whole-repo floor** — 60–70%, enough to catch "this package has no
   tests at all".
4. **Never chase coverage with `set` mode**; it under-reports branches. Always
   `atomic`, alongside `-race`.
5. **Exclude generated code** — everything under
   `internal/adapter/mariadb/sqlc` — or the number is meaningless.

### Flake policy

- **A flaky test is a broken test.** Quarantine behind `//go:build flaky` for at
  most two weeks with a linked issue, and exclude it from the required check.
- **No retries at the test level.** A retry that turns red green hides a real
  race. `-count=1` plus a fix is the answer.
- **Track newly-slow tests** from `-json` output. 20ms becoming 4s is the leading
  indicator of the next flake.
- **Nightly**: `-race -count=10` on the database suite. Repetition is the
  cheapest flake detector there is.

---

## 6. The gap: 81 tests that can vanish

`.env` is gitignored (`.gitignore:689`). Every database-backed test skips when it
is absent, and `go test ./...` then reports **`ok` for every package**.

| Gate | Where | Tests behind it |
| :--- | :--- | :--- |
| `testsupport.Config` → `t.Skip("no .env found…")` | `internal/adapter/mariadb/testsupport:116` | 58, across 8 files |
| `testsupport.Admin` → `t.Skipf("MariaDB not reachable…")` | `testsupport:132` | the same 58, second gate |
| Local `.env` walk + `testing.Short()` | `db/*_integration_test.go`, `internal/migrate`, `internal/onboarding`, `app_connect_test.go` | 23, across 6 files |

The failure mode is not "the tests are broken". It is that **a green suite proves
nothing about the database path**, and the person reading `ok fingo/internal/adapter/mariadb/postingrepo`
has no way to tell whether nine tests ran or none did. On a fresh clone, in CI,
or on a colleague's machine without the `.env`, 81 tests — including every
trigger, every foreign key, and every migration — silently do not execute.

That is the single highest-value thing to fix in this repository.

### Why not build tags

Build tags (`//go:build integration`) are the usual answer, and I would normally
recommend them. Here they are the wrong tool: the repo's gate is
`go test ./...` and its database tests are meant to run by default on a developer
machine that has a database. Tags would exclude them from that run and force
`-tags=integration` onto everyone, changing the workflow to fix a reporting
problem. **The tests are fine; the reporting is not.**

---

## 7. Migration plan

Four phases. Each ends green, and each is independently useful.

### Phase 0 — make skipping a decision, not a default *(the important one)*

> **Status: steps 1–2 done, step 3 outstanding.**
>
> `testsupport.RequireEnv`, `testsupport.unavailable` and `testsupport.Skipped`
> exist and both gates go through them. Verified by compiling the partnerstore
> test binary and running it from a directory with no `.env` in reach:
> without `REQUIRE_DB` it **skips** (exit 0) with a message that says how to make
> it a failure; with `REQUIRE_DB=1` it **fails** (exit 1) naming the cause.
>
> The six local `.env` walkers below still skip silently. Until they are
> consolidated, 23 of the 81 tests are still behind the old behaviour.

1. Add `REQUIRE_DB` handling in one place — **done**:

   ```go
   func unavailable(t *testing.T, format string, args ...any) {
       t.Helper()
       if os.Getenv(RequireEnv) == "1" {
           t.Fatalf("%s=1, so a missing database is a failure: %s",
               RequireEnv, fmt.Sprintf(format, args...))
       }
       atomic.AddInt64(&Skipped, 1)
       t.Skipf(format, args...)
   }
   ```

2. Call it from `testsupport.Config` and `testsupport.Admin` instead of
   `t.Skip`/`t.Skipf` — **done**.
3. Delete the six local `.env` walkers and route those files through
   `testsupport.Config` — **outstanding**:
   `db/client_scope_integration_test.go`, `db/immutability_integration_test.go`,
   `db/migrate_integration_test.go`, `internal/migrate/runner_test.go`,
   `internal/onboarding/onboarding_test.go`, `app_connect_test.go`. The duplicate
   walkers are how the second gate drifted apart from the first. They need
   `testsupport.Config` plus, where relevant, `MigratedDatabase` and
   `EmptyDatabase`; the two that create a database that must *not* exist yet
   (`onboarding`, `app_connect`) need one new helper for "a scratch name and its
   cleanup".
4. Set `REQUIRE_DB=1` in the CI database job.

**Acceptance:** with no `.env` and `REQUIRE_DB=1`, the suite **fails loudly**
naming the missing database. With no `.env` and no `REQUIRE_DB`, it skips with a
message that says how many tests were skipped.

### Phase 1 — make the skips visible even when they are allowed

- A `TestMain` per database-backed package that counts started-vs-skipped tests
  and prints `tests: 12 run, 0 skipped` — or better, a `-json` summary step in CI
  asserting the executed count is non-zero.
- This is the belt to Phase 0's braces: even if someone forgets `REQUIRE_DB`, CI
  fails on "zero database tests ran".

**Acceptance:** deleting the CI `.env` turns the pipeline red.

### Phase 2 — the gate and the linter

- Change the `GEMINI.md` gate to
  `go test -race -count=1 -shuffle=on ./...`.
- Add `.golangci.yml` and the lint job. The repo has none, so start with the
  `govet`, `errcheck`, `staticcheck`, `ineffassign`, `unused`, `gocritic`
  defaults and *do not* add style linters in the same commit — a lint job that
  arrives with 400 findings gets disabled.
- Delete `fingo/$cov` and add `*cov*` (or the correct `coverage.out`) to
  `.gitignore`.

**Acceptance:** `go test -race ./...` is green, and `golangci-lint run` is green.

### Phase 3 — CI

- The workflow in §5, starting with lint + unit + database. Add coverage last,
  and start with a floor of 60% rather than 70%.
- Adopt `goleak` in the database packages first, where goroutine leaks would
  actually appear.

**Acceptance:** a pull request runs four jobs and a deliberate
`t.Fatal`-inducing change turns the right one red.

### Deliberately not in this plan

- **Testcontainers.** Worth doing; not worth doing before Phase 0. The `.env`
  path works today and costs nothing.
- **`t.Parallel()` rollout.** The scratch-database pattern is already safe, so
  this is a speed change, not a correctness one. Do it after CI exists and
  timings are visible.
- **Golden files for `internal/reports/statutory`.** High value, but a different
  piece of work from "make the suite tell the truth".

---

## Appendix — house conventions

| | |
| :--- | :--- |
| Assertions | stdlib `testing` only. No `testify`. |
| Failure style | `t.Fatalf` for errors, `t.Errorf` for values. |
| Tables | slice of structs, named cases, no computed expectations. |
| Fakes | hand-written structs in the test file; in-memory repositories next to their interfaces. |
| HTTP | `httptest.NewServer` for E2E, `httptest.NewRecorder` for a single handler. |
| Database | `testsupport.Config` → `Admin` → `MigratedDatabase`; never a second walker. |
| Time | `testing/synctest` or a polling deadline. Never `time.Sleep`. |
| Cleanup | `t.Cleanup`, never a bare `defer` in a helper — cleanup has to run on `t.Fatal` too. |
| Escaping the CWD | `t.Chdir(t.TempDir())` before anything that calls `findEnvFile`. |
