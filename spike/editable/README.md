# SPIKE 2 — can walk carry an accounting ENTRY screen?

**THROWAWAY.** Delete when the decision is made.
Run: `go run .` · `go run . -demo` drives it into a refusal headlessly.
Tests: `go test ./...` — **7 tests, 26 subtests**, `go vet` clean.

Coverage, stated precisely because the package figure is misleading: the package reads
**38.9%** because the untested throwaway wiring sits in it. **The model is essentially fully
covered** — `Validate`, `Totals`, `parseAmount`, `NewEntryGrid`, `Cells`, `AddRow`, `RemoveRow`
all at **100%**; `SetCell` at 95.8% (the unreachable-column branch) and `group` at 94.1%
(the sign branch).

## The seam, agreed before any test

The **grid model** — a pure-Go type with no `walk` dependency:

| | |
| :--- | :--- |
| `SetCell(row, col, text) error` | the commit path a Tab triggers |
| `Validate() error` | the posting gate |
| `Totals() (debit, credit int64)` | what the form shows under the last row |
| `Chart` (one method: `Has(code) bool`) | so the rules need no chart and no database |

The widget wiring in `main.go` is **deliberately untested** — it is throwaway, and testing it would only pin the toolkit's shape.

## The TDD cycles

| Slice | Red | Green |
| :--- | :--- | :--- |
| 1 | `undefined: NewEntryGrid` | amount is whole đồng; dot-grouping accepted; comma refused |
| 2a/2b | all account and side cases failing | account must exist; side is debit or credit or nothing |
| 3 | `g.Validate undefined`, `g.Totals undefined` | the posting gate; half-filled rows; single line; running totals |
| 4 | `g.AddRow undefined` | add and remove a row; the last row is never removed |

## Verdict 1 — `walk.TableView` is **read-only**. There is no editable grid.

From the toolkit's own source, not from documentation:

- `walk.TableModel` (models.go:130) declares `RowCount`, `Value`, and five event publishers. **No `SetValue`.**
- The word `edit` appears **0 times** in `tableview.go` — not in code, not in comments.
- The only `SetValue` methods in the package are on `NumberEdit`, `NumberLabel`, `ProgressBar`, `RadioButton`, `Slider`. The table-model hits are `setValueFunc` — an unexported setter for the *getter*.

So the workaround this spike builds is the one a real shell would have to use: **a fixed-row form**, one `LineEdit` per cell, committing on `OnEditingFinished` (what both Tab and Enter raise), plus a `ComboBox` for the side.

**Verdict: walk can carry the read side well and the write side awkwardly.** It renders 10,000 rows at 129 ms and gives a native, fast, 9.6 MB application — but an accounting entry screen has to be built as a form of controls, not a grid. That is a real design constraint, not a detail.

## Verdict 2 — the best find: the error summary erases itself

`OnEditingFinished` fires when focus **leaves** a cell. `showError` moved focus to the summary. Moving focus out of the amount cell fired its `OnEditingFinished`, whose success path called `showError("")` — **wiping the message that had just been written.**

The diagnostic that found it, before and after:

```
setText: before="" want="Không ghi sổ được\r\n\r\n• the entry debits 1.000.000 and credits 900.000…"
         after="Không ghi sổ được\r\n\r\n• the entry debits 1.000.000 and credits 900.000…" err=<nil>
setText: before="Không ghi sổ được…" want="" after="" err=<nil>      ← erased by the focus move
```

The fix is **focus first, then write** — and it is a trap that only exists in a commit-on-focus-loss form, which is precisely the form a native accounting UI needs.

## Verdict 3 — TDD found a real defect in the shipped code

`web/render.go`'s `formAmount` strips `,` as well as `.` before parsing:

```go
strings.NewReplacer(".", "", ",", "", " ", "", "\u00a0", "", "_", "").Replace(trimmed)
```

In Vietnam `.` groups thousands and `,` marks the decimal. So **`1.234,56` is silently read as `123456`** — a fractional đồng turned into a valid-looking amount 100× larger. The grid's `parseAmount` refuses a comma outright, and slice 1 is the test that proves it.

That defect is live in the admin UI today.

## What this spike did not prove

1. **Runtime row add/remove in the widget.** The model supports it and the tests cover it; the *window* was built with a static row count, because walk's declarative API builds a fixed tree and adding a row means constructing widgets imperatively and inserting them into a layout. Untested, and it is the next unknown.
2. **Real keystrokes.** The demo sets text programmatically; nobody pressed Tab. `OnEditingFinished` is the documented commit event, but it was not exercised by hand.
3. **No accessibility bridge.** walk implements no UIA provider. Focus is the only thing carrying the announcement.
4. **No cell-level error display.** A refusal surfaces in the one summary box. There is no way to mark *the cell* that is wrong, which is what a grid gives you and a form does not.

## Files

| | |
| :--- | :--- |
| `grid.go` / `grid_test.go` | the model and its spec — the part worth keeping |
| `main.go` | the walk wiring — throwaway |
| `app.manifest`, `rsrc.syso` | Common Controls v6. **Required**, or window creation dies with `TTM_ADDTOOL failed` |
| `spike2-refusal.png` | the form in its refused state |
