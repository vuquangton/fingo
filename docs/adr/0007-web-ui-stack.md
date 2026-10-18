# The UI is a server-rendered Gin application with vendored Tabler, htmx and Alpine

**Status:** accepted · 2026-10-18 · **Supersedes** [ADR 0004](0004-admin-ui-is-server-rendered.md) and `docs/plan-native-shell.md` (the `lxn/walk` desktop plan). It keeps ADR 0004's central judgement — the browser is the right surface, and the server renders the page — and changes the framework, the styling and the deployment model.

## The decision

**FinGo v2 is a browser application served by one Go binary.** PostgreSQL is the only external process. There is no desktop toolkit, no Node build step, no CDN, and no client-side router.

| Concern | Choice | Vendored |
| :--- | :--- | :--- |
| HTTP | `gin-gonic/gin` | Go module |
| Rendering | `html/template`, embedded with `//go:embed` | repo |
| Reactivity | [htmx](https://htmx.org) 2.x — page fragments from server | `web/static/vendor/htmx.min.js` |
| Client state | [Alpine.js](https://alpinejs.dev) 3.x — menus, toggles, client-side disclosure only | `web/static/vendor/alpine.min.js` |
| CSS kit | [Tabler](https://tabler.io) (Bootstrap 5, MIT) | `web/static/vendor/tabler/` |
| Entry grid | [Tabulator](https://tabulator.info) — **journal entry screen only** | `web/static/vendor/tabulator/` |
| Pickers | [Tom Select](https://tom-select.js.org) for accounts/partners, [flatpickr](https://flatpickr.js.org) with `vi` locale for dates | vendored |
| Shortcuts | [hotkeys-js](https://github.com/jaywcjlove/hotkeys-js) | vendored |
| CSRF | Go 1.27's `net/http.CrossOriginProtection` (no third-party dependency) | stdlib |
| Sessions | `alexedwards/scs` with the `pgx` store | Go module |
| Auth | argon2id (`golang.org/x/crypto/argon2`) | Go module |
| Excel | `qax-os/excelize` | Go module |

**Every asset is committed.** The deployment target is an office machine with no internet access and no build toolchain; a stylesheet fetched from a CDN is a page that renders wrong on the day the network is filtered. `go:embed` puts the lot inside the binary.

**htmx is the interaction model; Tabulator is the exception.** A list, a form, a report, a close confirmation and a login are HTML that the server renders. The one screen where a spreadsheet grid is genuinely the right control is journal entry, where an operator types many rows with keyboard-only navigation; that screen uses Tabulator and is the only place a significant client-side library runs business UI logic.

**Alpine does not own state.** It may open a menu, toggle a row's detail, or run a picker. It may not hold a document being edited, and it may not decide a figure.

**CSRF comes from the standard library.** Go 1.25 added `http.CrossOriginProtection`, which rejects unsafe cross-origin browser requests using `Sec-Fetch-Site`/`Origin`. Go 1.27 is the toolchain, so `gorilla/csrf` is not needed and is not added. The protection is installed as the outermost `http.Handler` around the Gin engine, so it applies before any handler.

## Why

**The desktop assumption was the expensive one.** v1 shipped a Win32 app that had to bundle a 36 MB database server, fight antivirus, acquire a single-instance kernel object, and could still only be used by one person at one machine. The stated business — one practice, many clients, several staff — does not fit that shape. A browser application is reachable from the machines the practice already has, needs no per-workstation install, and has no version skew.

**Server-rendered HTML is the smallest possible front end.** ADR 0004 was right that a form which posts cannot lie about what it did, and that a no-JavaScript path is a requirement rather than a nicety. Both properties survive here: every screen works as a plain form POST, and htmx only upgrades the response into a fragment. The client holds no copy of the ledger.

**Tabler is the pragmatic choice for a data-dense admin tool.** Bootstrap 5 familiarity, MIT licence, a real component set (tables, tabs, modals, toasts), and no build step for the subset we use. It looks like an accounting tool rather than a marketing site, which is what an operator staring at it for eight hours wants.

**Vendoring is a deployment requirement, not a preference.** No CDN, no Node, no package manager on the target machine. A single `go build` must produce everything.

## Considered options

**1. Keep `lxn/walk` and add a web UI beside it.** Rejected. Two UIs means two wiring efforts forever; the desktop toolkit has been dormant since 2021 and has no accessibility bridge.

**2. A React/Vue/Svelte SPA.** Rejected. A Node build in the pipeline, a duplicated domain model in TypeScript, and the ledger's rules re-implemented in the browser — for screens that are, almost all, tables and forms.

**3. Templ/Go templates + htmx, Tabler, no framework.** Close second. Gin is chosen for routing/middleware ergonomics and because the team's existing Go experience is with it; the difference is small and reversible.

**4. Server-rendered Gin + vendored Tabler + htmx + Alpine, Tabulator only for entry.** **Chosen.**

## Consequences

**Two rendering modes exist and must both be tested.** A handler returns a full page for a normal request and a fragment for an `HX-Request`; the e2e suite drives the no-htmx path, and each screen's tests assert the fragment. This is inherited deliberately from ADR 0004, whose `web/e2e_test.go` is the model.

**No offline mode.** The application requires the server to be reachable on the LAN. That is the deployment we are building for; a genuinely offline requirement would be a different product.

**HTTPS is required for sessions.** The server runs with a locally generated certificate (Phase 2); `Secure` + `HttpOnly` + `SameSite=Lax` cookies. A plain-HTTP deployment is a misconfiguration the server refuses to start in.

**Accessibility is now tractable.** Native HTML controls plus Tabler's ARIA attributes give a keyboard and screen-reader path that the Win32 toolkit could not. It is not automatic: every screen still needs focus order, labels and an error summary that takes focus.

**Tabulator is a licence-sized dependency.** MIT, ~500 KB minified, and confined to one route. If the keyboard spike fails, the fallback is a plain server-rendered table with per-row forms — the screen still works, it is just slower to enter twenty lines.

### Not decided here

- **Charting.** No chart library is added. The dashboard uses Tabler's CSS-only figures; a chart is deferred until a screen needs one.
- **Print stylesheets.** Statutory print forms (Sổ cái, Sổ nhật ký chung) need `@media print` work and are not part of the Phase 3 screens.
- **Internationalisation.** The UI is Vietnamese; the code and comments are English. No i18n framework is added.
- **A service worker / PWA.** Not in scope.
