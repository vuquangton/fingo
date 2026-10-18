# FinGo v2 — plan status

Updated 2026-10-18. One row per phase; a phase is only `DONE` when every line of `docs/v2/decisions.md` §9 passes and the gate below is evidenced.

## 1. Phase table

| Phase | Status | Gate result |
| :--- | :--- | :--- |
| **0 — Save and decide** | **DONE** | archive tag exists · `git status` clean · ADRs 0006–0008 written · pushed |
| 1 — Foundation | NOT STARTED | — |
| 2 — Security and operations | NOT STARTED | — |
| 3 — Spike, then core screens | NOT STARTED | — |
| 4 — Trading cycle | NOT STARTED | — |
| 5 — Period close | NOT STARTED | — |
| 6 — Pilot readiness | NOT STARTED | — |

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
| v2 branch | `v2` created at `67b03a1` |

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
