# Context changelog

One entry per Context Curator run. Counts, not prose. Entries are keyed by
commit: re-running at the same commit with no repo change amends the entry
rather than adding a second one.

## 2026-10-18 — first full curation at `986e572` (branch `v2`)

**Added**

- `CONTEXT.md` (L0, 150-line budget) — did not exist.
- `AGENTS.md` — did not exist; `GEMINI.md` reduced to a thin pointer.
- 10 module files: `gl`, `posting`, `trading`, `inventory`, `reports`, `close`,
  `document`, `auth`, `tenancy`, `web`.
- 5 reference files: `routes` (41 routes), `db-schema` (20 tables, 15 RLS),
  `domain-glossary`, `decisions-index` (8 ADRs), `legal-status`.
- `drift-report.md`, `_collected.md` (working; delete before commit),
  `open-questions.md` (audit deltas).

**Corrected**

- Removed sqlc, Alpine.js, Tabulator, Tom Select, flatpickr, hotkeys-js and
  `robfig/cron` from the stated stack — none exist in the code.
- `internal/client/` → `store.Scope`; top-level `web/` → `internal/web/`.
- Fallback cluster port 55432 → 65432; migrations are `cmd/migrate`, not startup.
- "only `clients` has no RLS" → five tables.
- "creator ≠ poster", "every lock audited" → neither is implemented.
- Phase status: Phase 0 → phases 0–2/5 done, 3/4/6 partial.
- `internal/partner` marked UNWIRED; `reports/statutory` and `pkg/logger` marked dead.

**Removed**

- 126 stale lines from `GEMINI.md` (kept as a 15-line pointer).
- `db/queries/`, `client/`, `web/` from the README layout.
- "purchase order → receipt → invoice" from the README's delivered scope.

**Counts**

| | Before | After |
| :--- | ---: | ---: |
| Files in context set | 2 (README, GEMINI) | 19 |
| Documented stack claims that are false | 11 | 0 |
| Module docs | 0 | 10 |
| Reference docs | 0 | 5 |
| Drift rows STALE + WRONG (before) | 30 | 0 open in the new files |
