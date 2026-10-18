# web — HTTP surface, screens, assets

**Package:** `internal/web` + `cmd/fingo`. **Status:** WIRED, PARTIAL (19
screens, no login page).

## Purpose

Gin routing, session middleware, form parsing, server-rendered Vietnamese
screens. Every figure is produced by `internal/books`/`internal/reports`; the
handler formats, it does not compute.

## Public API / structure

| File | Owns |
| :--- | :--- |
| `auth.go` | `Auth`, sessions, roles, login/logout, `Handler`, CSRF, TLS policy |
| `entry.go` | embeds templates + static; entry form; `requireScope`; `accounts` picker |
| `screens.go` | generic table renderer; accounts, journal, trial balance, ledger |
| `reports_screens.go` | P&L, balance sheet, Excel export |
| `admin.go` | client registry, user accounts, password change |
| `{sales,purchase,receipt,payment,stock,partners,opening,close,aging,reverse,handover}.go` | one screen each |

Routes are registered **only** in `cmd/fingo/main.go:65-107`; the full
method/path/handler/role table is [`routes.md`](routes.md).

## Templates and assets

- `internal/web/templates/*.html` — 14 files, embedded (`go:embed templates/*.html`).
- `internal/web/static/vendor/` — `tabler.min.css` (Tabler 1.0.0-beta21) and
  `htmx.min.js`, embedded and served at `/static`.
- htmx is included in 3 templates but no `hx-*` attribute exists: it is inert.
- No Alpine, Tabulator, Tom Select, flatpickr, hotkeys-js — despite `GEMINI.md`
  and `decisions.md` saying otherwise.
- **No upstream LICENSE files** are committed beside the vendored assets.

## Invariants

1. No arithmetic in handlers: amounts arrive as strings, go through
   `format.Amount` → `money.Dong`, and the engine decides every figure.
2. Every route except `/healthz`, `/login`, `/logout` sits behind
   `RequireUser`; mutating routes add `RequireWriter`; admin routes add
   `RequireRole(RoleAdmin)`.
3. `requireScope` resolves the client from the session; with exactly one active
   client it selects it, otherwise it answers 409.
4. Errors are rendered as the same page with a Vietnamese message and the
   operator's typed rows preserved.
5. Rendered money is server-formatted (`1.100.000`), never JS.

## Tests

One `_test.go` per screen under `internal/web` (unauth 401, role 403, happy path,
refusal 422). `$env:REQUIRE_DB='1'; go test ./internal/web -count=1` (~65 s).

## Gotchas

- `GET /books` is a stub returning the literal text `books` (`main.go:70`).
- There is no `GET /` and no login template. `POST /login` only.
- Handlers open `store.Read`/`InClient` directly in some screens and go through
  `books` in others; do not assume one assembly point.
- `static` and `templates` are compiled into the binary — editing them requires a
  rebuild, not a restart.
