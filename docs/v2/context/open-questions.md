# open questions raised by the 2026-10-18 context audit

> **This is not the canonical list.** That is
> [`docs/v2/open-questions.md`](../open-questions.md) (Q1–Q20). This file holds
> only the questions this audit could not answer from code, newest first. When
> one is answered, add it to the canonical file and strike it here.

| # | Question | Blocks | Default taken meanwhile | Who can answer |
| ---: | :--- | :--- | :--- | :--- |
| C1 | **Is a login page in scope for the pilot?** There is no `GET /` or login template; only `POST /login`. The operator guide tells the operator to open the root URL. | A1 (pilot), every UI test by a human | Keep the API-shaped login; do not add a page without a decision | Product owner |
| C2 | **Which exact vendored versions and licences, and where are the LICENSE files?** Only Tabler 1.0.0-beta21 and an unversioned htmx are committed; no upstream LICENSE exists despite `decisions.md:34`. | Release/legal sign-off; Q10 | Treat as non-compliant; do not add more vendored assets | Author / legal |
| C3 | **Delete or wire `internal/partner` and `internal/reports/statutory`?** Both have zero importers; `pkg/logger` is dead transitively. | Clean build, agent confusion | Leave in place, marked UNWIRED in context | Architect |
| C4 | **Creator ≠ poster: implement or strike?** `decisions.md:64` requires it; `created_by` and `posted_by` are the same actor today. | Security posture, Phase 2 claim | Document as not implemented | Practice owner + architect |
| C5 | **Should a period lock write an audit row?** Phase 2 gate claims "every post/lock"; the lock inserts `locked_by = 'period-close'` and no audit row. | Phase 2 evidence | Treat the gate as unmet on this point | Architect |
| C6 | **Wire provisions into the close, or delete the engine?** `WithProvisions` has no caller and Circular 48 expires ≤ 2026-12-31. | Phase 5/6 scope for bad-debt provisioning | Leave unwired; fixed assets are out of scope, bad debt is not decided | Practice owner |
| C7 | **Three-way match before the pilot?** The `Purchasing` engine exists but no route uses it; `/purchase` is the narrower cost bill. | A1 client choice, C2 scope | Not built; prefer a service client for the pilot | Architect |
| C8 | **Should `GET /books` (stub → text "books") stay?** It is a dead route. | Route inventory hygiene | Leave | Engineer |
| C9 | **Should `db.Up` run at startup as `db/db.go` claims?** Today a server on an unmigrated DB fails at first query. | Deployment robustness | Keep the explicit `cmd/migrate` step | Architect |
| C10 | **When is B3 (per-test DB isolation) scheduled?** The suite shares one database and accumulates rows. | A4 (CI green), trustworthy local runs | Known issue; tests scope themselves where they count rows | Engineer |
