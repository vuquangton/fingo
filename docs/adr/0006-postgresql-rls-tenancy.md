# One PostgreSQL database, `client_id` on every business table, and row-level security

**Status:** accepted · 2026-10-18 · **Supersedes** [ADR 0003](0003-client-data-boundary.md) entirely, and the "Tenant scoping" clause of [ADR 0001](0001-journal-line-model.md) (`"one database per client, so there is no company_id column"`). Also supersedes `docs/requirements-v1.md` §1.1 and §6.2.

## The decision

**1. One database, one schema.** PostgreSQL 16, database `fingo` (`fingo_dev` for development, `fingo_test` for the suite). Every client's books live in the same schema. The connection no longer decides *which books are reachable* — there is only one set of books, and a row says whose it is.

**2. `client_id uuid NOT NULL` on every table that holds a client's data**, with a foreign key to `clients(id)`, `ON DELETE RESTRICT`. The migration asserts the column is `NOT NULL` on every such table, so a table added later without one fails the build rather than leaking.

**3. PostgreSQL row-level security on every one of those tables, forced:**

```sql
ALTER TABLE vouchers ENABLE ROW LEVEL SECURITY;
ALTER TABLE vouchers FORCE  ROW LEVEL SECURITY;

CREATE POLICY vouchers_client ON vouchers
    USING      (client_id = current_setting('app.client_id', true)::uuid)
    WITH CHECK (client_id = current_setting('app.client_id', true)::uuid);
```

- `FORCE ROW LEVEL SECURITY` matters: without it the **table owner bypasses every policy**, and a migration or a helper script would read all clients' rows.
- `current_setting('app.client_id', true)` is the two-argument form deliberately: the one-argument form **raises** when the setting is absent, which would turn a forgotten scope into a 500. The two-argument form returns `NULL`, `NULL = uuid` is `NULL`, and the row is filtered out. **No `app.client_id` set ⇒ zero rows.**
- The application role is a plain `LOGIN NOSUPERUSER NOBYPASSRLS` role. RLS is bypassed by superusers and by roles with `BYPASSRLS`, so neither is granted.

**4. `app.client_id` is set per transaction, and only through one function.** Every repository method takes a `client.Scope` and runs inside:

```go
tx, err := pool.Begin(ctx)
_, err = tx.Exec(ctx, "SELECT set_config('app.client_id', $1, true)", scope.ID()) // true = SET LOCAL
```

`set_config(..., true)` is transaction-local: the pool hands the connection to the next request with the setting cleared, so a pooled connection cannot carry one client's scope into another's request. A write that arrives naming a different client is refused by `WITH CHECK`, not silently refiled.

**5. The `clients` registry is the one table without RLS.** It holds the practice's list of clients (name, tax code, active flag) and **no financial data**. It is readable by any authenticated practice user and writable only by `admin`. Row-level security protects a client's books from another client's session; it cannot protect the registry from the operator who legitimately owns all of it.

**6. Schema identical for every client.** One migration set, no per-client schema drift, no migration fan-out across N databases. Upgrades are one `goose up` against one database.

## Why

ADR 0003 added a `client_id` column and a `Scope` type but left enforcement to a repository convention that **no repository implemented**: at the end of v1, `client.Scope` had zero call sites outside its own package. A boundary expressed only in a method signature is a boundary that one forgotten `WHERE` clause defeats, and the failure is silent.

Row-level security moves the guarantee from "every author remembers" to "the database refuses". Three properties make it the right mechanism here:

1. **It cannot be forgotten by a new query.** A hand-written report, a `psql` session, a future repository — all are filtered.
2. **It is testable as a property, not as a review.** The suite connects as the application role, unsets the scope, and asserts zero rows; it sets client A, attempts a read and a write of client B's row, and asserts both fail.
3. **The wrong connection stops being a failure mode.** With one database there is no second connection to reach, so the "pointed at the wrong books" incident is not merely unlikely — it is not expressible.

The cost is the one ADR 0003 named: a schema holding many clients needs **composite natural keys**. `accounts.code`, `vouchers.voucher_no` and `voucher_series(prefix, fiscal_year)` are per-client in reality, so every uniqueness constraint becomes `(client_id, …)`. That is done in the v2 init migration, not deferred.

## Considered options

**1. Database per client** (v1's stated model). Rejected. It makes upgrades an N-database fan-out, backup a per-client chore, and every cross-client report impossible; and it still does not stop the process from querying the wrong database. The multi-user web deployment makes the operational cost worse, not better.

**2. `client_id` column with repository-enforced scoping** (ADR 0003's status quo). Rejected. It is the failure mode that actually occurred: the shape was defined and never wired.

**3. One database + RLS + a scoped transaction.** **Chosen.**

**4. A separate schema per client in one database.** Rejected: same fan-out problem as database-per-client, and `search_path` becomes a security-sensitive setting that a query can change.

## Consequences

**Every query runs in a transaction.** A read that forgets `BEGIN`/`SET LOCAL` returns nothing rather than data. That is a loud, safe failure — and it means the adapter cannot use the pool directly for business queries. The cost is one extra round trip per request, which is irrelevant at one operator's scale.

**Migrations run as the owner, not the app role.** `goose` connects as a migration role that owns the tables; the app role is granted `SELECT/INSERT/UPDATE/DELETE` only. Because policies are `FORCE`d, the owner is also filtered when it queries business data outside a migration — which is why the seed of the TT133 chart is a migration (it runs as owner, and it must set a `client_id` for the rows it writes).

**`clients` is a trusted table.** Everything else is not. That asymmetry is deliberate and must be stated in any future review.

**Deleting a client is refused, not cascaded.** `ON DELETE RESTRICT`: a client with books cannot be removed by a `DELETE`, only deactivated. Handover/export is a `COPY` of their rows, not a file move.

**The `client_id` is stamped server-side.** A handler never reads a client id from a form field; it comes from the session's selected client, is checked against the registry, and becomes the transaction's `app.client_id`.

### Not decided here

- **Selection UI and switching.** The session carries the selected client; the switcher is a Phase 2 screen.
- **Per-client backup copies.** The backup is the whole database (Phase 2); a per-client export is a filtered `COPY` (Phase 6).
- **Cross-client reporting.** A practice-level rollup would need a role that bypasses RLS, or an aggregate table maintained by a privileged job. Not in v1 scope.
