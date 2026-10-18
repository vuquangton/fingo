# A client column on every table, and a scope every repository must name

**Status:** accepted · 2026-10-09

**Supersedes:** the "Tenant scoping" clause of [ADR 0001](0001-journal-line-model.md) — specifically *"one database per client, so there is no `company_id` column"* — and the same claim in [`requirements-v1.md`](../requirements-v1.md) §1.1.

## The decision

**1. A `clients` table.** The practice's registry: an id, a name, a tax code, a registration instant, and whether the client is still served. It holds no financial data. Migration [`00015_client_scope.sql`](../../fingo/db/migrations/00015_client_scope.sql) creates it.

**2. A `client_id` column on every table that holds a client's data.** Eighteen of them, each a nullable `VARCHAR(36)` with a foreign key to `clients(id)`.

**3. A scope every repository must name.** Financial repository methods take a `client.Scope` as their first argument:

```go
type DocumentRepository interface {
    Put(ctx context.Context, scope Scope, d Document) error
    Get(ctx context.Context, scope Scope, id string) (Document, error)
    List(ctx context.Context, scope Scope) ([]Document, error)
    Delete(ctx context.Context, scope Scope, id string) error
}
```

There is no method without a scope, so *"list every document"* is not an operation the interface can express. The zero `Scope` is invalid, not a wildcard: the commonest multi-tenant bug — a query that forgot its `WHERE` clause — returns an error instead of everybody's data.

**4. The middleware does the choosing.** `client.Middleware` reads the client out of the authenticated request, checks it against the registry, and puts the scope on the request context. A handler is handed the scope as an argument; it never selects one.

## Why

**The connection boundary is real, but it stops at the process.** One database per client decides which books are *reachable*. It says nothing about which books are *asked for*. Nothing in Go can inspect it, assert it, or test it: a repository handed the wrong connection reads the wrong client's ledger, and no line of application code refuses. The failure is silent by construction, which is the worst property a tenant boundary can have.

**A boundary that is not a value cannot be enforced.** The claim in ADR 0001 was that the connection makes cross-client access *structurally impossible*. It makes it impossible to reach two clients through one connection. It does not make it impossible to reach the wrong one — and the difference is the whole of the risk. `Scope` makes the client a value that is passed explicitly and refused when absent, so the rule is expressed in the type system and provable in a test.

**Two clients in one schema must not leak.** Whether that ever happens is an operational question, not a design one. A schema that cannot leak across clients does not depend on the answer.

## Considered options

**1. Keep the connection as the only boundary** — the status quo. Rejected. Not inspectable in-process, not testable, no failure mode for a wrong connection. Isolation is asserted by an operational promise rather than checked.

**2. `client_id NOT NULL` on every table.** Rejected. Every row written before this migration belongs to the one client whose database it is, and nothing in the row says which. `NOT NULL DEFAULT ''` would file all of them under a client called nothing, invisible to every scope — worse than no column. A backfill is impossible: there is no row that knows the answer.

**3. Nullable `client_id`, plus a repository-layer scope guard.** **Chosen.** `NULL` is the honest encoding of "not yet stated". A scoped repository *adopts* a row with no owner and *refuses* a row owned by somebody else. New writes are refused without a scope, which is where the rule bites.

**4. Make the natural keys composite now** — `(client_id, code)`, `(client_id, book, fiscal_year, seq)` and so on. **Deferred.** In a database holding one client they are identical to the global keys, so deferring costs nothing today. They are named in the migration so the gap is a task rather than a surprise.

## Consequences

**ADR 0001's other conclusions stand.** One account and one side per line, integer đồng, identical schema in every client database, and a voucher that cannot span clients — all unchanged. Only the tenant clause is reversed.

**A lookup for another client's row is indistinguishable from a lookup for a row that does not exist.** `ErrUnknownDocument`, never `ErrForeignClient`. Reporting *"that exists but is not yours"* would let a caller enumerate another client's records one id at a time. `ErrForeignClient` is reserved for a *write* that names a different client, where the caller has already told us what they believe.

**A write cannot forge an owner.** `Put` stamps `ClientID` from the scope. A document that arrives naming another client is refused, not quietly refiled.

**Migration cost.** Eighteen `ALTER`s; 758 ms against the development MariaDB. The foreign keys create the indexes that read by client, so no separate index is needed. `Down` drops the constraints before the registry, which they depend on.

**The column is enforced in two different places, and that is deliberate.** The foreign key makes a `client_id` that names no registered client impossible to write. The scope guard makes a query that names no client impossible to run. They answer different questions — *"is this a real client?"* and *"did you say which?"* — and neither substitutes for the other.

### Not decided here

- **Composite natural keys.** Named in the migration; required the moment two clients share a schema.
- **Where the registry lives.** It is currently created by the same migration set that runs in each client database, because this repository has one migration set and one database. When the practice database is split out, `clients` moves there and the financial tables keep the column. The registry's own storage adapter does not exist yet.
- **Wiring.** Nothing outside `internal/client` imports it. No `Registry` adapter, no handler uses the middleware, and no existing repository yet takes a `Scope` — `internal/document`, `internal/voucher` and the MariaDB stores all still take none. This ADR settles the shape, not the migration of the call sites.

### Cost

One migration, one new package, one ADR. No existing signature changed: `internal/client` is purely additive, and no current repository was altered.
