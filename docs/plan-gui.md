# FinGo — GUI plan v2 (HTMX, MISA-informed)

> **SUPERSEDED, rendering half only.** The toolkit decision moved to native Windows controls:
> see [plan-native-shell.md](plan-native-shell.md). **The information architecture, the
> work-cycle idea, the sequencing and the definition of done in this file all carry over
> unchanged** — they were never toolkit-specific. What is replaced is HTMX, `html/template`
> and the stylesheet.

**Status:** superseded by [plan-native-shell.md](plan-native-shell.md). Nothing built.
**Supersedes:** v1 of this file, and the "second UI" clause of [ADR 0004](adr/0004-admin-ui-is-server-rendered.md).
**Reference:** [reference-misa-sme-flow.md](reference-misa-sme-flow.md) — MISA SME 2026's documented flow.

---

## 0. Two corrections to v1

**1. I over-claimed about MISA's navigation.** I wrote that "our IA is module-first; MISA's
is cadence-first". That is wrong, and the difference matters.

MISA's *product* navigation is the twenty-item **module menu**. Its cadence organisation —
daily / monthly-quarterly / annual — is a **help article**, a workflow guide that sits on
top of the product. MISA ships both: a module menu for lookup, and a process guide for
"what do I do now".

So the correct reading is not "adopt cadence instead of modules". It is **build both, and
make the guide live.** That is the main design idea below.

**2. The capability map in v1 was optimistic.** Verified this round:

| Claim | Reality |
| :--- | :--- |
| `internal/close` does the closing | It does 911→4212 **and** depreciation **and** bad-debt provisioning — but there is **no allocation run** (CCDC, prepaid) |
| Statutory statements are built | B01-DN and B02-DN only. **B03-DN (cash flow) and B09-DN (notes) are not modelled** — and deliberately not filed empty |
| Inventory valuation is built | Moving average and a FIFO/specific enum; **no stocktake** |
| Everything else | `asset`, `cash`, `catalog`, `closing`, `costing`, `opening`, `payroll`, `purchase`, `report`, `sales` have **zero importers outside their own tests** |

---

## 1. The shape

```
┌──────────────────────────────────────────────────────────────────────────┐
│ FinGo   [ Công ty Bình Minh ▾ ]   FY2026 · T3 · 🔒 open    🔍 F3        │
├────────────────┬─────────────────────────────────────────────────────────┤
│ ◆ Hôm nay      │  Position          [cash] [owed to us] [we owe] [result]│
│                │  ─────────────────────────────────────────────────────  │
│ NGHIỆP VỤ      │  Việc kỳ này                          period 2026-03     │
│   Bán hàng     │  ● Phân bổ CCDC, chi phí trả trước     ✗ not modelled  │
│   Mua hàng     │  ● Khấu hao TSCĐ                       ✓ run 05/03      │
│   Quỹ          │  ● Tính giá xuất kho                   ✓ run 05/03      │
│   Ngân hàng    │  ● Khấu trừ thuế GTGT                  ✗ not modelled  │
│   Kho          │  ● Kết chuyển lãi lỗ                   →              │
│   CCDC         │  ● Lập & nộp báo cáo thuế              →              │
│   TSCĐ         │  ● Kiểm tra, đối chiếu sổ sách          →              │
│   Tiền lương   │  ● Khóa sổ kỳ kế toán                  →              │
│                │  ─────────────────────────────────────────────────────  │
│ SỔ & BÁO CÁO   │  Recent                                                 │
│   Sổ sách      │  inv-0042 · Đông Á · 4.950.000 · open                   │
│   Báo cáo      │  inv-0041 · Bình Minh · 1.100.000 · paid                │
│   Thuế         │                                                         │
│                │                                                         │
│ DANH MỤC       │                                                         │
│ HỆ THỐNG       │                                                         │
└────────────────┴─────────────────────────────────────────────────────────┘
```

**Two levels of navigation, deliberate:**

- **The rail is module-first**, like MISA's — because lookup work ("find HD001") needs a
  stable destination, not a calendar.
- **"Hôm nay" is the work-cycle**, MISA's *Quy trình nghiệp vụ* made live: the period's
  steps in their real order, each showing its own state, each linking into its module.

That second one is the piece MISA does not have in its product, and the piece this
application most needs — because the closing order is load-bearing (see §4).

**The header always answers three questions:** which client, which period, is it locked.
It replaces v1's `DATABASE / SERVER / ACCOUNTS` panel, which was diagnostics on a working screen.

---

## 2. What the engine can back — the honest map

Screens are built in this order. A screen with a `✗` behind it does not get built.

### Khởi tạo — initialisation

| Step | Engine | State |
| :--- | :--- | :--- |
| 1. Data — server, data-set, path | `appconfig`, `onboarding` | ✓ **backed** |
| 2. Company profile | `system.CompanyProfile` | ✗ scaffold |
| 3. **Regime** | `AccountingStandard` — a free string nothing reads | ✗ **inert** |
| 4. Modules in use | — | ✗ |
| 5. Fiscal year, periods | `period`, `practice.CreatePeriod` | ✓ **backed** |
| 6. **Inventory costing method** | `inventory.Policy`, `ChooseValuationMethod` | ✓ **backed** |
| 7. **VAT method** | `tax.Kind` treatments | ✓ **backed** |

### Nghiệp vụ — daily

| Module | Engine | State |
| :--- | :--- | :--- |
| **Bán hàng** | `usecase/sales` — posts COGS correctly | ✓ **backed** |
| **Mua hàng** | `internal/trading.Purchasing` | ✓ built, **unwired** |
| **Kho** | `internal/inventory` | ✓ built, **unwired**; no stocktake |
| Quỹ · Ngân hàng | `domain/cash` | ✗ scaffold |
| CCDC · TSCĐ | `domain/asset` | ✗ scaffold |
| Tiền lương | `domain/payroll` | ✗ scaffold |
| Hóa đơn điện tử | `domain/tax` EInvoice | ✗ scaffold |

### Định kỳ — the closing run

| MISA's step | Engine | State |
| :--- | :--- | :--- |
| 1. Phân bổ CCDC | — | ✗ **not modelled** |
| 2. Phân bổ chi phí trả trước | — | ✗ **not modelled** |
| 3. Khấu hao TSCĐ | `close.depreciationFor` | ✓ backed, unwired |
| 4. Tính giá xuất kho | `inventory` moving average | ✓ backed, unwired |
| 5. Tính tỷ giá xuất quỹ | — | ✗ no multi-currency |
| 6. Tính lương | — | ✗ |
| 7. Tính giá thành | `domain/costing` | ✗ scaffold, and **out of v1 scope** |
| 8. Khấu trừ thuế GTGT | `reports` VAT pots | ✓ backed |
| 9. Lập & nộp báo cáo thuế | `reports/statutory` 01/GTGT, 03/TNDN | ✓ backed, **no mã số** |
| 10. Kết chuyển lãi lỗ | `close.carrySide`, 911→4212 | ✓ backed, unwired |
| 11. Tạm tính TNDN | `statutory` 03/TNDN | ✓ partial |
| 12. Kiểm tra, đối chiếu | `reports` — three reports, voucher-traced | ✓ **backed** |
| 13. Khóa sổ | `period.Repository.Lock` | ✓ **backed** |

### Năm — annual

| Step | Engine | State |
| :--- | :--- | :--- |
| Kiểm kê | `inventory` | ✗ no stocktake |
| Đánh giá lại ngoại tệ | — | ✗ |
| Tính thuế TNDN | `statutory` 03/TNDN | ✓ partial |
| Lập BCTC | `statutory` B01-DN, B02-DN | ✓ **B03-DN and B09-DN missing**, no mã số |
| Nộp BCTC | — | ✗ |
| Tạo dữ liệu từ năm trước | — | ✗ no year rollover |
| Khóa sổ | `period.Lock` | ✓ backed |

### Opening balances — ten groups

`domain/opening` has a `ValidateBalance` that checks only that account totals balance. It is
wired to nothing. **All ten groups are ✗.**

---

## 3. The rule that governs the sequence

> **No screen is built before the engine behind it is wired.**

The repository has already run the other experiment. The deleted WPF application shipped a
"Master Data Management Center", a "Period-End Closing & Restore UI" and four UI sprints —
a large interface over an unverified engine — and commit `689b20b` deleted all 296 files of
it. **The failure mode of this project is UI breadth ahead of working depth.**

So this plan also names what will **not** be built:

> Quỹ · Ngân hàng · CCDC · TSCĐ · Tiền lương · Hóa đơn điện tử · Khế ước vay · Hợp đồng ·
> Ngân sách · Thủ kho · Thủ quỹ · Phân tích tài chính · Giá thành

Thirteen of MISA's twenty modules. They get rail entries that render **"chưa hỗ trợ" with the
reason**, not dead links and not empty tables — an accounting UI must not imply a feature
that is not there.

---

## 4. Why the closing order is load-bearing

MISA's monthly run is: allocate → depreciate → cost the issues → deduct VAT → **then**
transfer the result → prepare the tax return → reconcile → print → lock.

`internal/close` transfers 911→4212 and books provisions. It does **not** run depreciation or
inventory costing first. A result computed before those runs is wrong, and it is wrong
*silently* — which is the one failure this codebase otherwise refuses everywhere.

**The work-cycle home is therefore not decoration. It is the control.** If "Khấu hao TSCĐ"
has not run, "Kết chuyển lãi lỗ" must be disabled and say why. That is a UI feature that
prevents an accounting error, and it is the strongest argument for building the guide as
software rather than shipping MISA's version as a PDF.

---

## 5. Design system

**Style — Minimalism / Swiss.** Grid, no decoration, function first.

**Colour — light, one identity.** The app is dark, the admin UI is light; pick one and it is
light: the outputs are **printed and filed**, and the screen should match the paper.
*(Recorded: the design tool recommended dark-default. Overridden — its profile was
consumer/IoT; ours is a filing instrument.)*

**Type — IBM Plex Sans**, self-hosted `.woff2`, never the Google Fonts CDN the tool emitted.
Figures tabular, right-aligned, dot-grouped — `web/view.go`'s `group()` already does this and
must not be duplicated.

**Tokens** extend the existing `web/static/app.css`. Density 8/10. `--radius: 4px`.

**Accessibility floor:** contrast ≥ 4.5:1 · visible focus · `prefers-reduced-motion` ·
a **focusable error summary** that links to each invalid field, with inline errors retained ·
`aria-describedby` on every invalid field.

---

## 6. Build sequence

### P0 — remove Vue, safely

The connection wizard is the only way to configure the app and it is written in Vue. Delete
Vue first and a fresh install can never be configured.

| | |
| :--- | :--- |
| P0.1 | Setup surface in HTMX — detect, probe without writing, save only after books open (`appconfig` + `onboarding`, both pure Go) |
| P0.2 | Front door + mount — `Assets: nil`, `Handler` = mux: `/setup`, else `web.Server` when books are open, else redirect. `web` is a plain `http.Handler` with **zero Wails dependency** (verified) |
| P0.3 | Delete `frontend/`, the `//go:embed`, the generated bindings, the npm step |

Lands together or not at all. After P0: no node, no Vite, no IPC.

### P1 — the shell, and the screens the engine already backs

App shell + rail + header · the **work-cycle home** · Chart of accounts · Documents (list,
view, new invoice / receipt / journal) · Trial balance · **and `web.Books` over MariaDB**,
without which every screen above runs on an in-memory fixture.

### P2 — the two built-but-unwired cycles

Mua hàng (wire `internal/trading.Purchasing`) · Kho (wire `internal/inventory`: receipts,
issues, transfers) · sổ nhật ký chung / sổ cái over the slice-1 ledger reader.

### P3 — the closing run

Wire `internal/close`: depreciation, provisions, 911→4212. Add the **allocation run** it
lacks. Then the work-cycle home becomes real rather than honest-but-inert. Lock and unlock.

### P4 — tax and statements

01/GTGT · 03/TNDN · B01-DN · B02-DN · PDF archive. **Gated on the mã số decision** — the
engine deliberately emits no indicator numbers and a form that cannot be filed is not worth a
screen.

### P5 — initialisation proper

Company profile · the regime field made real · modules in use · the ten opening-balance
groups, each tying to its control account.

---

## 7. Definition of done, per screen

1. Renders a whole document with JavaScript disabled.
2. Every figure traces to a posted voucher, and the row links to them.
3. Every failure shows the books' own words, unchanged.
4. Fully operable from the keyboard.
5. Numbers carry both the text a reader sees and the number a test reads.
6. Empty, loading and error states exist.

---

## 8. One line

```
P0  setup in HTMX → mount → delete Vue       (never UI-less)
P1  shell + work-cycle home + MariaDB books  (not garbage starts here)
P2  purchases and inventory
P3  the closing run, in order                (the UI as a control)
P4  tax and statements                       (gated on mã số)
P5  initialisation and opening balances
```
