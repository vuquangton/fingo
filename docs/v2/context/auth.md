# auth — login, roles, sessions

**Packages:** `internal/auth` (passwords, user CRUD), `internal/web/auth.go`
(HTTP middleware, session, routes).
**Status:** WIRED, DONE.

## Purpose

Prove who is signed in and what they may do. Argon2id for passwords, `scs`
sessions stored in PostgreSQL, standard-library CSRF. Deny by default: only
`GET /healthz`, `POST /login`, `POST /logout` are registered outside
`RequireUser`.

## Public API

| Symbol | What |
| :--- | :--- |
| `auth.Hash(password)` / `auth.Verify(encoded, password)` | argon2id PHC string, 64 MiB / t=3 / p=2 (`password.go:42-64`) |
| `auth.ListUsers/CreateUser/ChangePassword` | admin user management (`users.go`) |
| `web.NewAuth(store)` | session manager + pool (`internal/web/auth.go:53`) |
| `(*Auth).Handler(engine)` | wraps engine: `CrossOriginProtection` → `sessions.LoadAndSave` |
| `(*Auth).RequireUser()` / `RequireRole(...)` / `RequireWriter()` | 401 / 403 middleware |
| `(*Auth).RequireClient()` | 409 if no client selected |
| `(*Auth).ClientFrom(c)` / `SelectClient` | session-pinned tenant |
| `web.TLSConfig()` | TLS 1.2 floor |

## Invariants

1. Unknown email and wrong password are indistinguishable (same 401, same empty
   body); the failure is recorded in `failed_logins`.
2. Session token is renewed on login (fixation defence).
3. Cookie: `Secure`, `HttpOnly`, `SameSite=Lax`, 8 h lifetime.
4. No lockout, no rate limit. `failed_logins` is written and never read.
5. Roles: `admin` (users, clients, close), `accountant` (documents, post),
   `viewer` (read-only). `RequireWriter` = admin + accountant.
6. CSRF is the whole `net/http.CrossOriginProtection` handler — no token table,
   no per-form field.
7. The server refuses to start without a TLS certificate, because a `Secure`
   cookie over HTTP is a login that silently fails.

## DB tables

`users`, `sessions`, `failed_logins` (all outside RLS — practice-level).

## Tests

`password_test.go`; `internal/web/auth_test.go` (401/200/403/400, viewer refused,
logout), `internal/web/login_abuse_test.go` (failures recorded, cost ~2 ms),
`internal/web/admin_test.go`. `$env:REQUIRE_DB='1'; go test ./internal/web`.

## Gotchas

- **There is no login page.** `GET /` is 404 and no `login.html` exists; login is
  `POST /login` (form: `email`, `password`). The operator guide still tells the
  operator to browse to the root URL — a real pilot blocker.
- `POST /logout` sits outside `RequireUser`; it destroys whatever session exists.
- `RequireUser` answers 401 with no body (API-shaped), not a redirect.
- There is no "creator ≠ poster" split: one person creates and posts in the same
  transaction.
