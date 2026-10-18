# FinGo v2

On-premise, multi-user Vietnamese accounting (Go 1.27 + Gin + PostgreSQL 16;
TT133/2016 as amended by TT 46/2025).

**Read [`CONTEXT.md`](CONTEXT.md) first** — it holds the stack, the verified
commands, the layout, the hard rules, the current phase and the trap list. It
links the module layer (`docs/v2/context/*.md`) and the reference layer
(`routes`, `db-schema`, `domain-glossary`, `decisions-index`, `legal-status`).

Three rules that cause real damage if broken:

1. **Money is `int64` đồng everywhere** — never `float64`, never a `numeric`
   money column, never silent truncation.
2. **Every business query goes through `store.InClient`** (which sets
   `app.client_id` for RLS) — never the raw pool, never an unscoped query.
3. **Posted vouchers are immutable and a `?` figure is never emitted** —
   corrections are mirror vouchers; an unresolved legal figure is a warning.
