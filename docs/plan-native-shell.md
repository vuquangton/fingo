# FinGo — native shell plan (`lxn/walk`)

**Status:** proposed, awaiting approval on P0. Nothing built.
**Supersedes:** [plan-gui.md](plan-gui.md) — but only its *rendering* half. Its information
architecture, work-cycle idea, sequencing and definition of done all carry over unchanged;
they were never toolkit-specific. See §1.
**Supersedes** the "second UI" clause of [ADR 0004](adr/0004-admin-ui-is-server-rendered.md).
**Evidence:** [spike/walkui](../spike/walkui/README.md) (read side) and
[spike/editable](../spike/editable/README.md) (write side, TDD).

---

## 1. What carries over, and what is replaced

| From plan-gui.md | Fate |
| :--- | :--- |
| The module rail **plus** a live work-cycle home | **Carries over.** This is the IA, and it is toolkit-independent |
| "No screen before its engine" | **Carries over.** The governing rule |
| The safe-closing-order argument (the UI as a control) | **Carries over.** Unchanged and still the strongest reason to build the guide as software |
| The definition of done (§6 there) | **Carries over**, minus the JavaScript clause |
| `web.Books` over MariaDB | **Carries over, renamed.** Same adapter job, declared in the new UI package instead |
| HTMX, `html/template`, `app.css`, CDN-free stylesheet | **Replaced** by walk widgets and Win32 |
| Light-mode design system | **Replaced.** Native controls mean the OS decides the look. Token work shrinks to fonts, spacing and the money/number formatting rules |
| IBM Plex Sans, tabular numerals, dot-grouped đồng | **Carries over** — the formatting rules are ours, not the toolkit's |

**Two spikes produced keepers.** The `EntryGrid` model and its 26 subtests are toolkit-free and
move into the real codebase. The rest of both spikes is throwaway.

---

## 2. Three decisions this plan assumes — and where they bite

These were stated as defaults and never confirmed. Each blocks something specific.

| Decision | Assumed | Blocks |
| :--- | :--- | :--- |
| **Storage** | Bundle MariaDB in the installer, manage its lifecycle | P0. If SQLite is wanted instead, 21 `SELECT … FOR UPDATE` sequences and 12 triggers are a concurrency redesign — a separate project |
| **Regime** | TT133/2016 as amended by TT 46/2025 | P4 (statutory). The chart seed already implements it |
| **Statutory filing in Phase 1?** | No | P4. The engine emits no mã số by design |

---

## 3. The architectural rule

> **The UI is a thin widget layer over tested, toolkit-free presentation models. No accounting
> logic lives in a callback.**

Spike 2 is the proof this works: the grid's rules — whole đồng, account existence, side,
the posting gate, totals — are all in `EntryGrid`, covered by 26 subtests, and know nothing
about walk. The widget layer only moves strings in and errors out.

This is also the only way the UI is testable at all, given walk has no test harness.

**Consequence:** every screen gets a `*Model` with its own `_test.go`, and the screen file is
expected to be dull.

---

## 4. Package layout

```
cmd/fingo/main.go              walk message pump, LockOSThread, one binary
internal/ui/
  shell/                       MainWindow, MenuBar, StatusBar, rail, header
  workcycle/                   the cadence home — MISA's guide, with state
  voucher/                     the fixed-row entry form        ← spike 2's model, moved in
  ledger/                      read-only grids                 ← spike 1's shape
  report/                      trial balance, P&L, balance sheet, export
  dialog/                      modal helpers, error summary
internal/
  appconfig/  onboarding/      UNCHANGED
  usecase/practice/            UNCHANGED
  domain/ reports/ close/ ...  UNCHANGED
  adapter/mariadb/             UNCHANGED (sqlc, 14 migrations, 12 triggers)
  reports/statutory/           UNCHANGED (01/GTGT, 03/TNDN, B01-DN, B02-DN, PDF)
  migrate/                     UNCHANGED (backup before migrate)
web/                           KEPT, but demoted to a test harness + cmd/webui
```

**Deleted:** `frontend/`, `wails.json`, `//go:embed all:frontend/dist`, every Wails binding in
`app.go`, and `github.com/wailsapp/wails/v2` from `go.mod`.

**Kept deliberately:** `web/` is not just the old UI — [web/e2e_test.go](../fingo/web/e2e_test.go)
is the only end-to-end suite in the repository, driving the whole stack through a real HTTP
server. Deleting it would cost the integration tests and the `cmd/webui` support tool. It is
retired as a *shipped* UI, not as a harness.

---

## 5. The concurrency contract

walk requires every widget call to happen on the goroutine that created the widgets. This is
not advisory — see the [`golang-concurrency`] rules on goroutine ownership.

1. **One UI goroutine.** `walk.Main()` owns it and locks the OS thread. Nothing else touches a
   widget.
2. **Every background result marshals back** with `mw.Synchronize(func(){ … })`. Never
   `go func(){ someLabel.SetText(...) }()`.
3. **A `Dispatcher` helper in `internal/ui/shell`** wraps this, so screens do not call
   `Synchronize` ad hoc and a new screen cannot get it wrong by omission.
4. **Every query takes a context** tied to the screen's lifetime, so closing a window cancels
   work in flight rather than letting it land on a dead widget.
5. **The message pump is never blocked.** Anything slower than a few milliseconds goes off
   thread. Spike 1 measured a 10,000-row read at 129 ms — far too long to hold the UI.

---

## 6. Shell, keyboard and IA

Inherited from plan-gui.md §1. The rail is module-first; the home screen is the cadence.

```
┌──────────────────────────────────────────────────────────────────────────┐
│ FinGo  [ Công ty Bình Minh ▾ ]  FY2026 · T3 · 🔒 open     🔍 F3         │
├────────────────┬─────────────────────────────────────────────────────────┤
│ ◆ Hôm nay      │  Vị thế    [cash] [owed to us] [we owe] [result]        │
│ NGHIỆP VỤ      │  Việc kỳ này                                            │
│  Bán hàng      │   ● Khấu hao TSCĐ            ✓ run 05/03                │
│  Mua hàng      │   ● Tính giá xuất kho        ✓ run 05/03                │
│  Kho           │   ● Kết chuyển lãi lỗ        → blocked until 1–2 run    │
│  …             │   ● Khóa sổ kỳ kế toán       →                           │
│ SỔ & BÁO CÁO   │  Chứng từ gần đây                                       │
│ DANH MỤC       │   inv-0042 · Đông Á · 4.950.000 · open                  │
│ HỆ THỐNG       │                                                         │
└────────────────┴─────────────────────────────────────────────────────────┘
```

| Key | Action | walk mechanism |
| :--- | :--- | :--- |
| `F2` | New voucher | `Action{Shortcut{Key: walk.KeyF2}}` in a `MenuBar`, so it is also discoverable |
| `F3` | Search / pick | opens the picker dialog |
| `F5` | Refresh | re-runs the screen's query |
| `F8` | Post | runs the model's `Validate`, then the ledger |
| `F9` | Unpost | refused on locked periods, with the reason |
| `F12` / `Ctrl+S` | Save | the draft path |
| `Enter` / `Tab` | Commit cell, next field | `OnEditingFinished` — **see §7.4** |
| `Esc` | Cancel | closes the modal, discards the draft |

---

## 7. The five walk constraints the plan is built around

Learned from the spikes, not from documentation. Each one shapes a decision.

### 7.1 A Common Controls v6 manifest is mandatory — and it must be a real one

Without Common Controls v6, window creation fails with `TTM_ADDTOOL failed`.

**`build/windows/wails.exe.manifest` cannot be reused.** It is a Wails *template*:
its `assemblyIdentity` carries `{{.Info.ProductVersion}}`, which the Wails build substitutes
before linking. Copied verbatim, that placeholder is an invalid version and Windows refuses to
load the executable at all:

> *The application has failed to start because its side-by-side configuration is incorrect.*

`cmd/fingo/app.manifest` is therefore its own file with real values, plus `requestedExecutionLevel
level="asInvoker"` — FinGo writes only under the user's profile and installs no service, so it
has no reason to ask for elevation. `rsrc` compiles it into `cmd/fingo/rsrc.syso`.

**This is a build step, and it belongs in the repository, not in a README.**

### 7.2 `TableView` is read-only

`walk.TableModel` has no `SetValue`; the word "edit" does not appear in `tableview.go`. So:

- **Read screens use `TableView`** — ledgers, reports, the work-cycle list. It is excellent
  there: 10,000 rows at 129 ms, virtual, native.
- **Write screens use a fixed-row form** — one control per cell. This is what the current HTML
  admin UI does with its fixed 8-row journal form, so nothing is lost that existed.

### 7.3 Money never goes through `NumberEdit`

`walk.NumberEdit` is a `float64` control. The ledger is integer đồng and this codebase's first
rule is that a float never touches an amount. Amounts are `LineEdit` plus the tested
`parseAmount`, which refuses a comma outright.

That refusal fixes a live defect: `web/render.go`'s `formAmount` strips `,` as well as `.`, so
**`1.234,56` is silently read as `123456`** today.

### 7.4 Focus first, then write the error

`OnEditingFinished` fires when focus **leaves** a cell. Moving focus to the error summary fires
the cell's own commit handler, whose success path clears the summary — **the message is erased
by the act of showing it.** Spike 2 hit exactly this and lost an hour to it.

The rule for every screen: **move focus first, then set the summary text.** `internal/ui/dialog`
owns this so no screen re-learns it.

### 7.5 No accessibility bridge

walk implements no UIA provider. Native controls are not automatically accessible. The error
summary is a `TextEdit` and **focus is the only thing carrying the announcement**; there is no
`role="alert"` equivalent.

**This is a real risk for a filing instrument and it is not solved by this plan.** It is called
out in §11 rather than buried.

### 7.6 A kernel object name cannot contain a backslash

The single-instance guard is a named kernel object, and its name is path-like: a `\` is a
separator, not a character. Deriving the name from the installation root — the obvious thing,
since that is what identifies an installation — fails at `CreateMutex` with *"the system cannot
find the path specified"*, which says nothing about the real problem.

`singleton.NameFor(root)` builds a usable name, and `Acquire` refuses a separator outright. Both
were found by **running the application**, not by a test: the unit tests had been passing with a
sanitised name all along.

---

## 8. Phases

### P0 — one-click storage, and the shell that needs almost no setup

**The insight that reorders this phase:** if MariaDB is bundled and auto-configured, the
connection wizard mostly stops existing. Building a native setup form first would be building
something the next phase deletes. So storage comes first.

| | |
| :--- | :--- |
| P0.1 | **Bundled MariaDB.** Ship the portable Windows binaries; on first run initialise a data directory under `%LOCALAPPDATA%\FinGo`, start the server as a child process, shut it down on exit. Named pipes or a random loopback port to avoid collisions |
| P0.2 | **Single-instance lock.** Two copies must not start two servers against one data directory. Nothing in the repo does this today |
| P0.3 | **First-run bootstrap.** Company name and tax code. That is the whole setup: no server, no host, no port, no password |
| P0.4 | **The native shell.** `MainWindow`, `MenuBar`, `StatusBar`, the rail, the client + period + lock header. A rail entry with no engine behind it renders **"chưa hỗ trợ" with the reason** |
| P0.5 | **Delete Wails and Vue.** `frontend/`, `wails.json`, the embed, the bindings, the module dependency |
| P0.6 | **The build.** `rsrc` + `go build -ldflags "-H windowsgui -s -w"`, wired into the repo so it is one command |

*Evidence for P0:* a clean Windows account with no MariaDB installed runs the installer, launches
the app, types a company name and a tax code, and lands on the work-cycle home. No configuration
file is edited by hand.

**Status after implementing P0 (2026-10-11).** Verified by running, not by reading:

| | State | Evidence |
| :--- | :--- | :--- |
| P0.1 | **Done** | **36.3 MB** of server bundled beside the executable (73 files, from a 293.5 MB install — 94.5 MB of it was `.pdb`). Empty directory → provisioned in **12.6 s**: schema v15, 123 accounts; second `Ensure` creates nothing. Verified with **no MariaDB on the machine** and no `FINGO_MARIADB_BIN`: found `dist/server/bin`, initialised, migrated, first run completed |
| P0.2 | **Done** | 8 tests, 96.2%; a Windows job object kills the server when the app dies, verified by force-killing it |
| P0.3 | **Done** | First run collects name + tax code once, saves them, shows the name in the header; the second launch asks nothing |
| P0.4 | **Done** | Menu, status bar, five-section rail, client/period/lock header. `Sổ sách chưa hỗ trợ — nhật ký chung, sổ cái và sổ chi tiết` |
| P0.5 | **Done** | 63.5 MB removed; 40+ module dependencies down to 7; `build.cmd` → 10.4 MB `FinGo.exe` |
| P0.6 | **Done** | `go generate ./cmd/fingo` → `rsrc.syso`; `-H windowsgui -s -w`; one command, `build.cmd` |

**What P0 did not do:** there is no installer. `build.cmd` produces a `dist\` directory that runs
where it stands — the whole of the Evidence line above is demonstrated from it — but nothing wraps
it in an NSIS/MSI package or a portable ZIP. That is packaging, not P0: it does not change what the
application does, and it should not be allowed to hold up the screens.

**The severest finding of P0, worth more than everything above it:** a `mariadbd` that aborts at
startup was indistinguishable from one still starting, so a failed launch sat on "đang khởi động…"
for the **full five-minute timeout** before saying anything. `WaitReadyOrExit` watches the process
as well as the port: **300 s → 1.3 s**, with the server's own log tail attached.

**Findings that changed the plan:**

- `build/windows/wails.exe.manifest` is a Wails **template**. Copied verbatim, its
  `{{.Info.ProductVersion}}` is an invalid version and Windows refuses to load the binary. §7.1.
- A kernel object name cannot contain a backslash, and an installation root is a path. §7.6.
- `appconfig.Store.Save` validates the connection first, so a first run with no connection saved
  **nothing** — and a fresh install never has one. The connection is derived, and is now recorded.
- `walk.Dialog` with no default button ignores **Enter**. Type a name, press Enter, nothing happens.
- A `mariadbd` that aborts at startup is indistinguishable from one still starting, if readiness is
  judged only by dialling a port. `WaitReadyOrExit` watches the process too: **300 s → 1.3 s**.
- A batch file written with LF line endings makes `cmd.exe` mis-parse it.
- `mariadbd.exe` and `mysqld.exe` are both 13 KB launchers — the server is `bin\server.dll`. **Both**
  must ship: the application starts `mariadbd.exe`, but `mariadb-install-db.exe` runs the bootstrap
  by invoking `mysqld.exe` by name. Without it the first run dies with *"Can't write to mysqld's
  stdin"*, which names the wrong problem entirely.
- `walk.Synchronize` **queues** the function; it does not run it. A value written inside one
  `Synchronize` and read outside it is a race, and it hid a failed save behind complete silence.

### P0.5 — move the keepers in

Before writing screens, move `EntryGrid` + its tests from `spike/editable` into
`internal/ui/voucher`, and the money formatting rules into `internal/ui/format`.
*Evidence:* the spike's 26 subtests pass in their new home.

### P1 — the shell, the work cycle, and what the engine already backs

| Screen | Engine | Notes |
| :--- | :--- | :--- |
| Work-cycle home | `period.Current`, `internal/close` run state | read-only `TableView` of the cadence + four position figures |
| Chart of accounts | `gl.sql` queries ✓ | `TableView`, 123 rows |
| Documents | `internal/document` ✓ | list, view, new invoice / receipt / journal |
| Voucher entry | `usecase/sales` ✓ (COGS correct) | the fixed-row form |
| Trial balance | `internal/reports` ✓ | `TableView`, every row traced to its vouchers |
| **The books port** | new | `practice.Client` + `internal/reports`, declared in `internal/ui`, so screens read real books rather than a fixture |

*Evidence:* `web`'s e2e suite still green (the engine is untouched), plus a model test per screen.

### P2 — the two cycles that are built but unwired

Mua hàng — wire `internal/trading.Purchasing` (PO, goods receipt, three-way match, landed cost).
Kho — wire `internal/inventory` (receipts, issues, transfers); **no stocktake exists**, so the
stocktake screen waits.
Sổ nhật ký chung and sổ cái over the ledger reader from slice 1.

### P3 — the closing run, in order

Wire `internal/close`: depreciation, provisions, 911→4212. **Add the allocation run it lacks**
(CCDC and prepaid) — MISA's steps 1–2, and we have neither.
Then the work-cycle home stops being honest-but-inert and starts being a **control**: *kết chuyển*
is disabled until depreciation and costing have run.

### P4 — tax and statements

01/GTGT, 03/TNDN, B01-DN, B02-DN, PDF archive. **Gated on mã số.** B03-DN and B09-DN are not
modelled and are not filed empty — so the cash-flow screen does not exist until the model does.

### P5 — the rest

Master data · the practice registry and client switching (`internal/client` exists, unwired) ·
the ten opening-balance groups, each tying to its control account · company profile and the
regime field made real.

---

## 9. Packaging

`wails build` goes away with Wails, and so does its NSIS template. P0 needs:

- **`rsrc` for the manifest and icon**, run by a `go:generate` line or a small script.
- **An installer** that lays down the app, the bundled MariaDB, and a Start-menu entry. Inno
  Setup or NSIS, chosen in P0.6 — the existing `build/windows/installer/project.nsi` is Wails'
  and cannot be reused as-is.
- **A portable ZIP** as the alternative to installing, which is also the honest fallback when a
  site blocks installers.

---

## 10. Definition of done, per screen

Carried over, minus the JavaScript clause:

1. Every figure traces to a posted voucher, and the row links to them.
2. Every failure shows the books' own words, unchanged.
3. Fully operable from the keyboard.
4. The screen's accounting logic lives in a tested model, not in a callback.
5. Empty, loading and error states all exist.
6. A rail entry with no engine behind it says **"chưa hỗ trợ"** and why — never a dead link and
   never an empty table.

---

## 11. Risks, stated rather than buried

| Risk | Severity | Mitigation |
| :--- | :--- | :--- |
| **walk is dormant** — last commit 2021-01-12, 345 open issues | High | It is the only Win32-native Go toolkit. Mitigate by keeping all screens thin over tested models, so a future swap is mechanical rather than a rewrite |
| **No accessibility bridge** | High for a filing instrument | Not solved by this plan. Focus management is the only mechanism walk offers. If WCAG-level accessibility is a requirement, this decision must be revisited *before* P1 |
| **Runtime row add/remove in the widget** is unproven in walk's declarative API | Medium | Rust-spiked in P0.5 — one small screen that adds a row imperatively, before any real screen depends on it |
| **Bundled server**: antivirus, port conflicts, ~150–200 MB installer, upgrade path | Medium | Named pipes over TCP; a versioned data directory; explicit stop-on-exit |
| **Bundling is a phase-sized job on its own** | Medium | It is P0, not a footnote, precisely so it is not discovered late |
| Losing `web/`'s e2e coverage if it is deleted carelessly | Medium | It is **kept**. Only the shipped UI goes |
| This is UI direction **number four** (Next.js → WPF → Wails/HTML → walk) | High | The mitigation is scope, not tooling: the plan ships one usable company's books, not eighteen events. The deleted WPF app is the precedent — 296 files removed by `689b20b` |

---

## 12. What to verify before P1 starts

Cheap checks that would invalidate the plan if they fail:

1. ~~**A bundled `mariadbd` starts, migrates and serves on a machine with nothing installed.**~~
   **VERIFIED** — `internal/dbhost`, slice 1d. Data directory created from nothing in **3.5 s**,
   private server ready in **2.05 s**, reporting `12.3.2-MariaDB`, bound to `127.0.0.1` only, on
   the port chosen for it, and cleanly stopped. First run costs ~5.5 s; every run after that
   costs ~2 s, because the data directory already exists.
   *Note:* the raw MariaDB `bin` directory is **293 MB**, much of it `.pdb`. My "~150 MB"
   estimate was optimistic; stripping debug symbols and dropping unused tools is a P0.6 task.
2. **An imperative row-add works in a walk window.** If it does not, the entry form is
   fixed-row forever and long vouchers get a "more rows" button that rebuilds the window.
3. **`ShellExecute` opens a generated PDF** — that is the whole printing story, since the
   statutory PDFs are already produced in Go.
4. **A picker dialog over 123 accounts is comfortable from the keyboard.** If not, the account
   cell becomes a type-ahead `ComboBox` with a filter, which is a different component.

---

## 13. One line

```
P0    bundled storage · bootstrap · native shell · delete Wails/Vue · the build
P0.5  move the spike's tested grid model in
P1    work cycle · documents · sale · reports · the books port
P2    purchases · inventory · books
P3    the closing run, in order — the UI becomes a control
P4    tax and statements                          (gated on mã số)
P5    master data · practice registry · opening balances
```
