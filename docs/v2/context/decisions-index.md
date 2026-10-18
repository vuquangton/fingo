# decisions-index — ADRs and the plan of record

> Index only. ADR bodies live in `docs/adr/` and are not summarized beyond the
> decision they record. Status was read from each file's line 3 on 2026-10-18.

## ADRs

| ADR | Title | Status | Governs |
| :--- | :--- | :--- | :--- |
| [0001](../../adr/0001-journal-line-model.md) | Journal line model | **accepted, partly superseded** — tenant clause → 0006, money clause → 0008 | one line = account + side + amount (still stands) |
| [0002](../../adr/0002-posting-lifecycle.md) | Posting lifecycle | accepted | draft → validate → number → post; immutability |
| [0003](../../adr/0003-client-data-boundary.md) | Client data boundary | **superseded by 0006** | historical: why `client_id` must exist |
| [0004](../../adr/0004-admin-ui-is-server-rendered.md) | Admin UI is server-rendered | **superseded by 0007** | browser + server-rendered HTML, no CDN |
| [0005](../../adr/0005-connection-settings.md) | Connection settings | accepted | DSN/config handling (v1 desktop origin) |
| [0006](../../adr/0006-postgresql-rls-tenancy.md) | PostgreSQL + RLS tenancy | accepted | one DB, `client_id NOT NULL`, forced RLS, `app.client_id` per transaction |
| [0007](../../adr/0007-web-ui-stack.md) | Web UI stack | accepted | Gin + embedded templates + vendored assets, stdlib CSRF |
| [0008](../../adr/0008-int64-money.md) | `int64` money | accepted | `int64` đồng / `bigint` / JSON string |

## Plan of record

`docs/v2/decisions.md` — stack, tenancy, DB rules, scope, keep/replace/drop, port
order, provisioning, definition of done, phase gates, risks.

**Trust the code over this file where they disagree.** Known stale claims,
verified 2026-10-18:

| Claim in `decisions.md` | Reality |
| :--- | :--- |
| `pgx` + `sqlc` (`:18`) | pgx only; SQL is hand-written |
| Alpine, Tabulator, Tom Select, flatpickr, hotkeys vendored (`:46-52`) | only Tabler CSS + htmx exist |
| `robfig/cron` backup (`:29`) | Windows scheduled task script |
| `clients` is the only non-RLS table (`:62`) | five tables are outside RLS |
| creator ≠ poster (`:64`) | not implemented; create and post are one act |
| three-way match in scope (`:81`) | engine-only, no screen |
| `internal/client` (`:102`) | `store.Scope` |
| fallback cluster port 55432 (`:153`) | 65432 (`scripts/devdb.ps1:26`) |

## Other normative documents

| Document | Role | Freshness |
| :--- | :--- | :--- |
| `docs/v2/plan-status.md` | phase status + gate evidence | phase table current; §5 "Next" and §6 open item stale |
| `docs/v2/todo.md` | engineering backlog, owners, release gates | current |
| `docs/v2/open-questions.md` | human decisions + defaults taken | current; Q10/Q14 need rewording |
| `docs/legal-basis.md` | governs every rate/account/form code | living; verified 2026-10-08 |
| `docs/requirements-v1.md` | business requirements baseline | historical, still the "what" |
| `docs/adr/0005` | connection settings | from the v1 desktop app; partially obsolete |
