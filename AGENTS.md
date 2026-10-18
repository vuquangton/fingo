# FinGo v2 — agent entry point

FinGo v2: on-premise, multi-user Vietnamese accounting (Go 1.27 + Gin +
PostgreSQL 16, TT133/2016 as amended by TT 46/2025).

**Read [`CONTEXT.md`](CONTEXT.md) first.** It is the only file every session
needs; it links the module and reference layers under `docs/v2/context/`.

Three rules that cause real damage if broken:

1. **Money is `int64` đồng everywhere** — never `float64`, never a `numeric`
   money column, never silent truncation.
2. **Every business query goes through `store.InClient`** (which sets
   `app.client_id` for RLS) — never the raw pool, never an unscoped query.
3. **Posted vouchers are immutable and a `?` figure is never emitted** —
   corrections are mirror vouchers; an unresolved legal figure is a warning.
