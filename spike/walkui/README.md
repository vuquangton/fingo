# SPIKE — is `lxn/walk` viable for FinGo's operator UI?

**THROWAWAY.** Not production. Delete this directory when the decision is made.
Run: `go run .` (add `-dialog` to open the validation dialog straight into its refused state).

## The question

FinGo's operator UI is currently HTML in WebView2 (Wails). The ask was a **WinForms-style
native desktop app**. Three things decide whether `lxn/walk` can carry that:

1. a `TableView` over **10,000 rows read from a real MariaDB**, with totals, staying responsive
2. **keyboard-only** operation — `F2` new, `F3` search, `F8` post, cell movement
3. a **modal dialog with validation and a focusable error summary**

## What was built

One file, no abstractions. It drops and rebuilds a scratch database `fingo_spike_walk`,
seeds 10,000 ledger-shaped rows through the real MySQL driver, then loads them into the grid
**off the UI thread** and marshals the result back with `mw.Synchronize`.

Two things are worth noting about the shape rather than the result:

- The load runs in a goroutine and every widget call happens inside `Synchronize`. walk
  requires all widget access on the thread that created them, so a slow query must never run
  there. This is the pattern the real shell would have to use everywhere.
- A **manifest embedding Common Controls v6 is mandatory.** Without it, window creation fails
  with `TTM_ADDTOOL failed`. [`app.manifest`](app.manifest) + `rsrc` fixes it, and
  `fingo/build/windows/wails.exe.manifest` already declares the same dependency.

## Numbers

Measured on this machine, MariaDB local, 10,000 rows:

| | |
| :--- | :--- |
| Window visible after process start | **202–238 ms** |
| Seed — schema | 31–50 ms |
| Seed — 10,000 rows inserted (one transaction) | **2.96–3.05 s** |
| **Fetch 10,000 rows from MariaDB** | **114–129 ms** |
| **Full scan of the model under a keystroke** | **6.1–10.5 ms** |
| Binary, debug + console | 14.2 MB |
| **Binary, `-H windowsgui -s -w`** | **9.6 MB** |
| For comparison: current Wails `FinGo.exe` | 12.9 MB **+ WebView2 runtime** |

The grid scrolls, renders alternately-shaded rows, right-aligns and dot-groups money, and
shows a totals line. All Vietnamese text renders correctly with no font work.

## Verdict

**Viable.** All three questions came back positive, and the numbers are better than I
expected — 129 ms to load and 10 ms to search 10,000 rows is comfortably inside a keystroke.

It is a genuinely native application: real menu bar, real `ListView`, real modal dialog, real
status bar, OS-drawn everything, 9.6 MB, no browser engine, no runtime prerequisite.

## What the spike did NOT prove — read this before deciding

1. **Cell editing was not tested.** The grid was populated, not edited. An accounting entry
   grid needs in-place cell edit, add/delete row, and commit-on-Tab. `walk.TableView` supports
   a `SetValue` path but I did not exercise it. **This is the largest untested risk** and it is
   the next thing to spike.
2. **No real keystrokes were driven.** `F2`/`F3`/`F8` are declared as menu accelerators and
   the search is measured fast, but nobody pressed a key. Keyboard-only operation is
   *plausible and declared*, not *demonstrated*.
3. **No accessibility bridge.** walk implements no UIA provider. The error summary is a
   read-only `TextEdit` and the thing that carries the announcement is **moving focus to it** —
   there is no `role="alert"` equivalent. Native controls are not automatically accessible.
4. **`TextEdit.SetText` collapsed the `\n`** in the error summary. Win32 edit controls want
   `\r\n`. Cosmetic, but it is the first thing to fix in a real shell.
5. **No native totals footer.** `TableView` has none; the totals line is a separate `Label`.
6. **The seed data does not balance** — debit and credit were assigned independently to
   generate volume. The totals line honestly reads `lệch 702.640.000`. Deliberate: this tested
   rendering, not accounting.

## The dependency risk, stated plainly

`lxn/walk`'s **last commit on master is `c389da54`, 2021-01-12** — verified against the GitHub
commits API, not the repo's `pushed_at` field. It is **five years dormant**, with 345 open
issues. 7,102 stars, BSD-3.

It works. Nobody is maintaining it. That is the price of the only Win32-native toolkit in Go,
and it should be a conscious part of the decision rather than a footnote.

## Files

| | |
| :--- | :--- |
| `main.go` | the whole spike |
| `app.manifest` | Common Controls v6 + per-monitor DPI. **Required.** |
| `rsrc.syso` | the manifest, compiled in by `rsrc -manifest app.manifest -arch amd64 -o rsrc.syso` |
| `spike-grid.png` | the grid: 10,000 rows, totals, status line |
| `spike-dialog.png` | the modal with its error summary populated |

**Wipe me:** the scratch database is `fingo_spike_walk`, dropped and rebuilt on every run.
`spike.exe` and `spike-release.exe` are build artefacts.
