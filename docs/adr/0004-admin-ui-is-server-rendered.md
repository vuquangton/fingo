# The admin UI is server-rendered Go, not the Wails frontend

**Status:** accepted · 2026-10-09

## The decision

Administration screens live in [`fingo/web`](../../fingo/web), a Go package that
answers with HTML built by `html/template`.

- **Every page is rendered by the server.** There is no client-side state, no
  store, no client router. A form submits, the server does the work, and the
  answer is the page as it now stands.
- **HTMX is an enhancement, never a dependency.** Loaded from the binary, it
  swaps the same responses into `#content`. Without it every form still posts
  and every page still renders a whole document — the two modes differ in one
  `if` in the renderer, not in a second set of routes.
- **The only thing JavaScript is asked for is the idempotency key**, and the
  server fills in a usable one first (`crypto.randomUUID()` where available).
- **The pages depend on one port**, `web.Books`, declared where it is used. The
  in-memory implementation in `web/books` composes the real chart
  (`gl.ChartOfAccounts`), the real document and settlement rules
  (`internal/document`), the real tax (`internal/vat`), the real posting
  decisions (`internal/posting`) and the real reports (`internal/reports`).

The Vue frontend in `fingo/frontend` stays as it is. This is a second UI, not a
replacement.

## Why

**A form that posts is a form that cannot lie about what it did.** The requirement
is that every form POSTs to the Go API and that the API's error text reaches the
operator unchanged. A server-rendered page makes that the only path: there is no
client state to fall out of step with the ledger and no optimistic update to
retract.

**No-JavaScript is a requirement, not a preference.** Progressive enhancement is
only real when the un-enhanced path exists. In a client-rendered application it
does not, and no amount of graceful degradation invents it.

**The deployment is a desktop binary.** Wails ships an offline application. A
CDN stylesheet, a node build step, or a second server process are all things that
can be absent on the machine it runs on. The templates, the stylesheet and HTMX
are embedded in the binary (`//go:embed`), the same way `db.Migrations` is.

**The existing frontend has a different job.** `fingo/frontend` is the Wails
shell's UI and is bound to `App`'s methods over IPC. Rewriting it to serve
administration screens would mean rebuilding the screens it already has.

## Considered options

**1. Extend the Vue frontend.** Rejected. It is client-rendered, so there is no
no-JavaScript path to preserve; it needs a node toolchain to build; and its
errors arrive as rejected promises rather than as a page.

**2. Next.js (or another node framework).** Rejected. It needs a node runtime
alongside the Go binary, which a desktop install does not have, and it puts the
accounting rules behind a second language for no gain — the reports are already
Go.

**3. Server-rendered Go with HTMX as an enhancement.** **Chosen.**

## Consequences

**Two UIs now exist.** The Wails frontend and the admin UI. They are different
surfaces with different constraints, and both will need wiring to the same books.
That consolidation is not made here.

**Every figure is rendered twice**: once as the text a reader sees, and once as
the number in a `data-` attribute. A test asserting that a trial balance balances
should not have to un-format "1.100.000" back into a number, and a reader should
not have to read `1100000`.

**A write answers with a page, not a redirect.** internal/idempotency records a
2xx answer and replays it; a 303 would not be recorded at all, so a refresh after
posting would post again. Answering with the page means a refresh is answered
from the record.

**The key is lifted out of the form body.** An HTML form cannot set a header and
internal/idempotency reads one, so a middleware reads the body, puts it back
unchanged, and promotes the field to the header before the guard runs. Putting it
back matters: the guard hashes the body to tell one request from another, and a
body already consumed would hash as empty and make every write look new.

### Not decided here

- **Wiring.** Nothing mounts `web.Server`. `cmd/webui` serves it over in-memory
  books so the pages can be looked at; mounting it in the Wails asset server over
  the MariaDB ledger is the next slice.
- **Authentication.** There is no login and no `client.Scope` on these routes.
  The pages are scoped to one set of books by construction, which is not the same
  as being authorised, and the difference will matter the moment a second client
  exists.
- **A partner directory.** The invoice form takes the customer's name as free
  text. `internal/partner` is where that becomes a lookup.

### Cost

One package, one template set, one command. No existing file changed, and no
existing signature moved: `web` is purely additive.
