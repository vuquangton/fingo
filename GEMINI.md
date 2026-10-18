# Caveman Mode Rules

- Use caveman style: ultra-terse, blunt, high signal, zero fluff.
- Drop corporate pleasantries, apologies, and chit-chat.
- Short sentences. Direct answers first.
- Keep technical precision 100%. Code and commands remain complete, typed, and battle-tested.

# Change Control — The Gate

Work reaches code only through one gate: an approved plan for a bounded scope.

| Tier | Work | Gate |
| :--- | :--- | :--- |
| Read-only | recon, analysis, codegraph, grep, `git log` / `git status` | Open. Always allowed. |
| Trivial | typo, stale doc, wrong constant, single-line fix | One line of intent, then explicit yes. |
| Structural | schema, migration, domain model, interface seam, generated code | Written plan, then explicit approval. |

**Proposal stops the turn.** Recommending a fix or proposing a plan ends my turn; I hold for your answer. A question I asked is not a mandate to build.

**Approval binds to presented scope.** "Approved" covers the plan on the table, exactly. Wider work re-enters through the gate.

**Plan breaks mid-flight → stop.** Re-plan, re-approve. I never improvise scope.

# Slice Discipline — One Kind of Change at a Time

| Size | What it is | Ends when |
| :--- | :--- | :--- |
| Slice | Adds a type, query or rule. **No existing signature changes.** | Red to green in one pass. |
| Vertical | Domain + migration + repository + caller + integration test. **At most one** signature change. | One acceptance test proves it through real PostgreSQL. |
| Refactor | A signature change across the codebase. **Zero new behaviour.** | Green before, green after, same tests. |

1. **Never mix a refactor with a feature.**
2. **Signature change gets its own slice.**
3. **Every slice ends green.** Nothing is left broken between slices.
4. **State the acceptance evidence up front:** the one test that proves the slice worked.

# FinGo v2 Project Context

- **App**: FinGo v2. Multi-user, browser-based, on-premise. Vietnamese accounting, **TT133/2016 as amended by TT 46/2025**.
- **Users**: one practice, many client companies, three roles (`admin`, `accountant`, `viewer`).
- **Strategy**: keep the v1 engine, replace the shell. Never rewrite engine logic. Port first, adapt imports only.
- **v1**: archived at tag `archive/pre-rewrite`. `main` is the archive; **work happens on `v2`**.
- **Decisions of record**: `docs/v2/decisions.md`, ADRs 0006 (PostgreSQL + RLS), 0007 (web UI), 0008 (int64 money).

# Tech Stack (v2 — fixed)

- **Backend**: Go 1.27, Gin.
- **Database**: PostgreSQL 16. `fingo` / `fingo_dev` / `fingo_test`.
- **Data access**: `pgx` v5 + `sqlc`. Migrations: `goose`, embedded. Triggers and RLS policies live **in migrations**.
- **UI**: `html/template` + Tabler, htmx, Alpine.js — all vendored, all embedded. Tabulator on the journal-entry grid only. Tom Select, flatpickr (`vi`), hotkeys-js.
- **Auth**: argon2id, `scs` session store in PostgreSQL.
- **CSRF**: `net/http.CrossOriginProtection` (Go 1.25+; installed toolchain 1.27). No `gorilla/csrf`.
- **Excel**: `excelize`. **Backup**: `robfig/cron` + `pg_dump`.
- **No** Docker, Node, CDN, Wails, walk, MariaDB, decimal.

# Money — one rule

**`int64` đồng. Always. Everywhere.**

- Go: `int64` (through `internal/money`). Never `float64`. Never `decimal.Decimal`. Never `numeric`.
- PostgreSQL: `bigint`.
- JSON: **string** (`"1100000"`), never a number — JavaScript would round above 2^53.
- HTML: formatted server-side, `1.100.000`.
- A decimal conversion is explicit and errors on a fraction. Silent truncation is a bug.

# Database & Tenancy — four rules

1. **One database.** Every business table has `client_id uuid NOT NULL` + FK. Composite unique keys `(client_id, …)`.
2. **RLS is forced.** `ENABLE` + `FORCE ROW LEVEL SECURITY`; policy `client_id = current_setting('app.client_id', true)::uuid` with `WITH CHECK`.
3. **Scope is per transaction.** `set_config('app.client_id', $1, true)`. No setting ⇒ **zero rows**, never all rows.
4. **The app role owns nothing**: `NOSUPERUSER NOBYPASSRLS`. Migrations run as the owner. `clients` is the only table without RLS — it holds no financial data.

# Database Rules That Must Be Constraints

| Rule | Mechanism |
| :--- | :--- |
| Posted voucher and lines immutable | `BEFORE UPDATE/DELETE` trigger, `RAISE EXCEPTION` |
| Voucher balances at commit | deferred constraint trigger |
| Period lock irreversible | trigger on `accounting_period_locks` |
| Allocation ≤ document | trigger |
| Stock movements, allocations append-only | triggers |
| TT133 chart (123 accounts) | seeded **by migration** |

# Essential Commands (run from `fingo/`)

- **Gate, after every slice**: `go build ./...` then `go vet ./...` then `$env:REQUIRE_DB='1'; go test ./...`
- **Local DB**: `..\scripts\devdb.ps1` → cluster in `.local\pgdata`, port **55432**, `trust` auth. Reversible.
- **Migrations**: `goose -dir db/migrations create <name> sql`; applied in code at startup.
- **SQLC**: `sqlc generate` after any change under `db/queries` or `db/migrations`.
- **Format**: `gofmt -w <paths>`. Never in the same command as an edit.
- **Never rewrite source with PowerShell**: `Set-Content`/`-replace` on a `.go` file collapses newlines. Use the edit tool.

`REQUIRE_DB=1` is not optional. Without it, database tests skip and `ok` means nothing. A green run with skips is a failed run.

# Layout

```
cmd/fingo/            server: config, router, startup
internal/domain/gl/   journal model · TT133 chart · posting rules
internal/{money,vat,posting,period,document,partner,inventory,trading,close,reports}
internal/client/      tenant scope
internal/store/       pgx pool · scoped transaction · RLS
internal/web/         Gin handlers · middleware · templates
db/migrations/        schema, triggers, policies, COA seed
web/                  templates · static · vendor
scripts/              devdb · backup · restore-drill · service install
docs/v2/              decisions · plan-status · open-questions · reports
```

**Ported from v1 (unchanged):** `money`, `vat`, `posting`, `period`, `document`, `partner`, `inventory`, `trading`, `close`, `reports` and their tests.
**Ported with one documented exception:** `domain/gl` — `decimal` → `int64` (ADR 0008). Behavioural assertions unchanged; changed tests listed in `docs/v2/test-report.md`.
**Replaced:** MariaDB adapters/migrations/triggers, `appconfig`, `boot`, `dbhost`, `singleton`, walk UI, `web/` templates.
**Dropped (archive only):** Wails/Vue, .NET remnants, Next.js, `spike/`, `dist/`, `$cov`, the 11 dead `internal/domain/*` packages, `internal/migrate`, `internal/idempotency`.

# Legal Guardrails

- `docs/legal-basis.md` governs every rate, form code and account number.
- A figure marked `?` is **never** emitted. It becomes a warning. **Never build on `?`.**
- Statutory filing (`mã số`) is **out of v2 scope**. Do not invent numbering.
- Circular 48/2019 provisioning is a parameter with an effective end date of **2026-12-31**, with a visible warning after it.

# Skill Selection Decision Table (5W1H Framework)

Installed skills are `.agents/skills/*`. This table covers the ones this project reaches for.

| Skill | Who (Trigger Role) | What (Task / Outcome) | When (Timing / Phase) | Where (Target Area) | Why (Goal) | How (Execution Method) |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `codegraph` | Recon / Impact Analyst | Symbol lookup, call paths, callers/callees, blast radius | Before touching a symbol | `internal/`, `.codegraph/codegraph.db` | Facts instead of assumptions | `codegraph sync` after adding files, then `query` / `callees` / `impact` |
| `tdd` | Backend Dev | Unit & integration tests before/with code | Adding business logic, fixing bugs | `internal/` | Zero balance drift | Red-Green-Refactor, table-driven tests |
| `codebase-design` | Lead Architect | Deep module interfaces | New module or refactor | package seams | Leak-free layers | Surface area vs depth |
| `domain-modeling` | BA / Architect | Ubiquitous language, ADRs | Clarifying VAS terms | `docs/`, `internal/` | No semantic drift | Define entities, write ADRs |
| `diagnosing-bugs` | Debugger | Root cause | Panic, wrong figure, lock | App-wide, SQL, logs | Fix the defect, not the symptom | Reproduce minimally, trace, patch |
| `code-review` | Reviewer | Diff review against spec + standards | Before finishing a slice | Modified files | Enforce layering, int64, RLS | Parallel spec & standard check |
| `research` | Tech Lead | Library/regulation evaluation | Unfamiliar dependency or rule | docs, POC | Grounded decisions | Primary sources only |
| `prototype` | UI Engineer | Throwaway UI spike | Testing a keyboard flow | `web/`, scratch | Fast feedback | Build, validate, discard |
| `grilling` | Lead Architect | Stress-test a plan | Before high-risk changes | plans, ADRs | Catch accounting flaws | Relentless Socratic questions |
| `karpathy-guidelines` | Core Dev | Disciplined step-by-step work | Non-trivial algorithms | engine, calculations | No premature complexity | Data first → baseline → scale |
| `writing-for-agents` | Agent Ops | Maintain this file and skills | Editing agent rules | `GEMINI.md`, `.agents/skills` | Instruction adherence | Terse, structured, concrete |
| `wizard` | DevOps | Guided human walkthrough | Certificate, service install | local OS | Steps an agent cannot automate | Interactive CLI |
