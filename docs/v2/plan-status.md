# FinGo v2 — plan status

Updated 2026-10-18 (Phase 1 foundation). One row per phase; a phase is only `DONE` when every line of `docs/v2/decisions.md` §9 passes and the gate below is evidenced.

## 1. Phase table

| Phase | Status | Gate result |
| :--- | :--- | :--- |
| **0 — Save and decide** | **DONE** | archive tag exists · `git status` clean · ADRs 0006–0008 written · pushed |
| 1 — Foundation | **DONE** | `go build` rc=0 · `go vet` rc=0 · `REQUIRE_DB=1 go test ./...` rc=0 (16 packages ok, 0 fail, 0 skip) · schema v1 · 11 database tests |
| 2 — Security and operations | **DONE (gate met)** | client registry (create/select, chart provisioned) · user accounts (create, roles, self-service password change) · write routes restricted to admin/accountant | login/session/roles · audit · backup · restore drill · TLS server binary · systemd unit. Windows SCM service not implemented (recorded). |
| 3 — Spike, then core screens | **PARTIAL — 9 screens live** | entry, chart of accounts, journal, trial balance, login. Trial balance reads through `internal/reports` over pgx. Tabulator keyboard spike outstanding. |
| 4 — Trading cycle | **PARTIAL — COGS live** | partners · sales invoice · receipt with allocation · supplier bill (expense → 1331 → 331) · supplier payment (331/1111) settling the bill. Item master data · goods receipt (156/331) with moving-average valuation · stock list. Cost of sales on a stocked line (632/156, issued from the stock ledger). Three-way match outstanding. |
| 5 — Period close | **DONE (gate met)** — plus one-action reversal (mirror entry, refused twice) | `POST /close` runs `internal/close`: result carried through 911 into 4212, period locked, report stored; a later voucher into the period is refused. Provisions not wired (fixed assets are out of v2 scope). |
| 6 — Pilot readiness | **PARTIAL — deliverables complete, gate unrun** | onboarding · handover export · operator guide (Vietnamese) · installer notes. The parallel run against a live client's current software needs a human and a client. |

## 2. Baseline: the v1 tree is green before the port

Run on `v2` at `67b03a1` (identical to the archive), before any v2 change:

| Command | Result |
| :--- | :--- |
| `go build ./...` | **rc=0** |
| `go vet ./...` | **rc=0** |
| `go test ./...` | **rc=0** — 44 packages `ok`, 3 packages with no test files, 0 failures |

This is the number to beat or keep: **every ported package must still report `ok` after the port**, and Phase 1's gate re-runs with `REQUIRE_DB=1` against PostgreSQL, which this baseline did not.

## 3. Phase 0 evidence

### Archive

| Item | Value |
| :--- | :--- |
| Branch | `main` |
| Archive commit | `67b03a1` |
| Tag | `archive/pre-rewrite` (annotated) → pushed |
| Second tag | `archive/wails-skeleton` → `f9193d3` (the Wails skeleton `main` stood on before the archive) |
| Remote | `git@github.com:vuquangton/fingo.git`, `main` pushed `f9193d3..67b03a1` |
| Abandoned branch | renamed `backup-before-alpha-rollback` → `archive/2026-09-20-pre-rollback`, pushed, old remote ref deleted |
| Working tree after commit | **clean** (`git status --porcelain` = 0 entries) |
| v2 branch | `v2` created at `67b03a1`, pushed to `origin/v2` |

### Commits composing the archive

| Commit | Contents |
| :--- | :--- |
| `48a54e4` | ignore v1 build output (`fingo/dist/`) |
| `b70845f` | 15 design documents (requirements, legal basis, review, testing strategy, plans, 5 ADRs) |
| `4b34733` | 18 GUI screenshots + the `discovery/` report (10 files) |
| `5f2829e` | accounting engine + tests (146 files) |
| `ab5dceb` | MariaDB schema, 15 migrations, 12 triggers, sqlc queries, adapters (77 files) |
| `9dfd09c` | native walk shell, provisioning, build scripts (103 files) |
| `17a4d8c`, `1a76fac` | remove Wails/Vue frontend and entry points (27 files) |
| `54056f3` | UI spikes and the stray `$cov` artifact (23 files) |
| `b3d529a` | v1 `GEMINI.md` state document |
| `67b03a1` | legal research text and extraction scripts (64 files, force-added past the `research/` ignore) |

### Decisions written

| Document | Contents |
| :--- | :--- |
| `docs/v2/decisions.md` | stack · tenancy · security · DB-level rules · scope · keep/replace/drop · port order · local cluster · gates · known risks · deviations |
| `docs/adr/0006-postgresql-rls-tenancy.md` | one database, `client_id NOT NULL`, forced RLS, per-transaction `app.client_id`; supersedes ADR 0003 and the tenant clause of ADR 0001 |
| `docs/adr/0007-web-ui-stack.md` | Gin + embedded templates + vendored Tabler/htmx/Alpine, Tabulator on entry only, stdlib CSRF; supersedes ADR 0004 and `plan-native-shell.md` |
| `docs/adr/0008-int64-money.md` | `int64` đồng in Go, `bigint` in PostgreSQL, string on the wire; retires `decimal` and the two-regime split |
| ADR 0001 / 0003 / 0004 | status lines amended with what supersedes them and what still stands |

### Phase 0 gate checklist

- [x] Working tree committed in coherent slices (10 commits)
- [x] Pushed to `origin/main`
- [x] `archive/pre-rewrite` tag exists locally and on the remote
- [x] `git status` clean
- [x] Old branch renamed and pushed; stale remote ref deleted
- [x] `v2` branch created
- [x] `docs/v2/decisions.md` written
- [x] ADRs 0006, 0007, 0008 written; 0001/0003/0004 marked superseded
- [x] `docs/v2/open-questions.md` opened with defaults recorded
- [x] Root `README.md` and `GEMINI.md` describe only the v2 stack
- [x] No `.env` and no production secret committed (test fixtures only; the v1 dev DSN remains in *history* and is scrubbed from the working `GEMINI.md`)
- [x] Build, vet and tests green on the archived tree

## 4. Deviations from the mission (Phase 0)

1. **`fingo/dist/` not committed** — 48 MB of reproducible output (`*.exe` was already ignored in v1). Added to `.gitignore`; the build script is committed.
2. **`research/` PDFs not committed** — 49 MB, ignored by the project's own policy. Extracted text and extraction scripts *are* committed (64 files, `git add -f`), because `docs/legal-basis.md` cites them as provenance.
3. **Second archive tag `archive/wails-skeleton`** at `f9193d3`, so the pre-Go-tree state is reachable by name.
4. **Deleting the stale remote branch name** after pushing the rename. This is the rename the mission asked for; the commit is unchanged and is also reachable as `archive/2026-09-20-pre-rollback`.

## 5. Next: first tasks of Phase 1

1. `scripts/devdb.ps1` — workspace-local PostgreSQL 16 cluster on port 55432 (`.local/pgdata`, `trust` auth). Unblocks `REQUIRE_DB=1`.
2. Remove the Replace/Drop items on `v2`; keep the archive tag as the only copy.
3. Port the unchanged packages; then `gl` with `int64` (ADR 0008).
4. One squashed init migration: schema, composite keys, RLS policies, immutability and balance triggers, period-lock trigger, allocation check, TT133 chart seed.
5. `internal/store`: pgx pool, `WithScope` transaction, and the tenant-isolation tests.

## 6. Phase 1 — foundation (DONE)

### Built

| Piece | What it is |
| :--- | :--- |
| Repository root | One module `fingo` at the repo root; `fingo/` removed. Keep packages ported unchanged: `money`, `vat`, `posting`, `period`, `document`, `partner`, `inventory`, `trading`, `close`, `reports`, `domain/gl`, `domain/document`, `usecase/ledger`, `gltest`, `format`, `pkg/logger`. |
| Dropped | MariaDB adapter and migrations, `appconfig`, `boot`, `dbhost`, `onboarding`, `singleton`, walk UI, `web/`, `idempotency`, `migrate`, `spike/`, `dist/`, `$cov`, 11 dead `internal/domain/*` packages. |
| Schema | `db/migrations/00001_init.sql` — 13 tables, composite `(client_id, …)` keys where natural, `bigint` money, `numeric(19,4)` quantities. |
| Tenancy | RLS `ENABLE` + **`FORCE`** on 10 tables, one `FOR ALL` policy each on `app_current_client()`; `app.client_id` set per transaction by `store.InClient`. |
| Rules in the database | deferred balance constraint triggers; posted-voucher and line immutability; allocation ≤ document; append-only allocations, stock movements, period locks, audit log. |
| Seed | TT133 chart generated from `gl.TT133Chart()` by `tools/genchart`, seeded through `chart_template`; `provision_client_chart(uuid)` gives each new client its own 123 accounts. |
| Store | `internal/store`: pgx pool, `Scope`, `InClient` (`set_config(..., true)`), `ErrNoScope`. |
| Test harness | `internal/testdb`: real database or a failure under `REQUIRE_DB=1`, app-role only, schema-version check with a runnable hint. |
| Tooling | `scripts/pgsetup.ps1` (databases + `fingo_app` role + gitignored `.env`), `cmd/migrate`, `scripts/devdb.ps1` (fallback cluster, stopped). |

### Gate evidence

```
go build ./...                         rc=0
go vet ./...                           rc=0
REQUIRE_DB=1 go test ./... -count=1    rc=0   16 packages ok, 0 failures, 0 skips
```

The 11 database tests in `internal/store` are the Phase 1 gate's real content: scope required, unscoped query sees zero rows, app role has neither `rolsuper` nor `rolbypassrls`, clients cannot see or address each other, a forged `client_id` is refused by `WITH CHECK`, an unbalanced voucher is refused at COMMIT, a posted voucher cannot be edited or deleted, an allocation cannot exceed its document, a period lock cannot be removed, every business table has `client_id`, and every client gets 123 accounts.

### Open item (the one deviation from the stack)

**`internal/domain/gl` still carries `shopspring/decimal`, and so do `reports`, `close`, `posting`, `usecase/ledger`.** ADR 0008 requires `int64` đồng everywhere. The port was done unchanged first (mission rule 3); the conversion is the next slice and is the only thing between this phase and `DONE`. It is recorded in `open-questions.md` as Q2 (was Q11) and in `test-report.md`.