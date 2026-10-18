# legal-status — what is in force, what the code implements

> Source of truth: [`docs/legal-basis.md`](../../legal-basis.md) (living, last
> verified 2026-10-08). This file is the agent-facing summary; if they disagree,
> `legal-basis.md` wins. Regenerate when that file changes.

## Regime

**Target: TT133/2016/TT-BTC as amended by TT 46/2025/TT-BTC** (SME regime; in
force; TT99 does not replace 133). Chosen and recorded in
`docs/v2/decisions.md:81`, `docs/v2/open-questions.md` Q4.

Other tracks exist and are **not implemented**: TT 99/2025 (general, from
FY2026), TT 58/2026 (micro, from 01/07/2026), TT 118/2026 (IFRS, from
01/01/2027).

## What the code implements

| Figure | Implemented as | Evidence |
| :--- | :--- | :--- |
| TT133 chart, 123 accounts / 49 cấp-1 | `gl.TT133Chart()` + migration seed, tested | `coa_tt133.go`; `coa_tt133_test.go` |
| Money | whole đồng `int64`; no subunit | `internal/money` |
| VAT standard 10% | only rate registered, from 2026-01-01 | `internal/books/sales.go:47-49` |
| VAT 0% / 5% / 8% | **not modelled** (no per-line rate dimension) | open question D-1 |
| Period close 911 → 4212/4211 | implemented | `internal/close` |
| Bad-debt / depreciation provisions | engine exists, **not wired** | `internal/close/provisions.go` |
| Statutory filing (`mã số`) | **out of scope**; no form set routed | `reports/statutory` unwired |
| PIT, BHXH, CIT | **not implemented** (out of scope) | no package |

## Time-critical

| Item | Date | Effect on this repo |
| :--- | :--- | :--- |
| **Circular 48/2019** provisioning (CIT deductibility) | **expires ≤ 2026-12-31** (QĐ 1760/QĐ-BTC) | provisioning policy must be a parameter with an end date and a visible warning; engine currently unused |
| **8% reduced-VAT window** (NQ 204/2025 + NĐ 174/2025) | **ends 2026-12-31**, reverts to 10% | no 8% support in code; after that date 10% is correct |
| Draft circular replacing TT133 | not issued (as of 2026-10-08) | do not build on it |

Today is 2026-10-18: **~2.5 months** to both deadlines.

## `?` items — never build on these

| Item | Why unresolved |
| :--- | :--- |
| `mã số chỉ tiêu` numbering (Q2/Q8) | Phụ lục IV / TT 89/2026 appendices not machine-readable |
| TT 152/2025 (household business) | reported in force, primary text not verified |
| 01/GTGT indicator numbers | Phụ lục I of TT 89/2026 not readable |
| Statutory book form codes under TT133+TT46 (Q4) | unconfirmed |
| Draft TT133 replacement | not an instrument |

Rule: a `?` figure is emitted as a warning, never as a number.

## Gotchas for this repository

- `docs/legal-basis.md` still cites `fingo/internal/domain/system/system.go` and
  `fingo/internal/domain/gl/coa_tt133.go`. The `fingo/` prefix is stale (module
  root moved) and `internal/domain/system` no longer exists.
- It also says the README claims 200/2014 — the README has since been corrected
  to TT133 + TT46.
- Account codes must come from the gazette text, not vendor copies (which swap
  cấp-2 codes); the seed test guards 49 cấp-1 accounts and the absence of
  521/621/622/627/641/1562.
- `AccountingStandard` as a free-form string read by nothing was a v1 artefact;
  there is no regime switch in v2 code.
