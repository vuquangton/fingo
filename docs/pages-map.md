# Page map — FinGo

**What this is.** The *shape* of the application: what frames what, which page leads
to which, where a voucher goes after it is written, and what has to finish before
what. It is the companion to [pages.md](pages.md), which lists the pages
themselves. Read this one to understand the machine; read that one to find a screen.

**Prepared as:** lead BA + chief accountant. **Reference:**
[reference-misa-sme-flow.md](reference-misa-sme-flow.md) (MISA SME 2026).
**Legal confidence:** as recorded in [legal-basis.md](legal-basis.md); unverified
items are marked inline and again in §10.

**Counts in this document are derived from [pages.md](pages.md)** — 141 pages,
verified by counting its table rows rather than by arithmetic.

---

## 1. The frame — what never moves

Every page in the application renders inside this. It is the shell that P0 already
ships (see `gui-bundled-ready.png`).

```
┌──────────────────────────────────────────────────────────────────────────────┐
│  FinGo — SME Accounting (TT133)                                     _ □ ×    │
├──────────────────────────────────────────────────────────────────────────────┤
│  Nghiệp vụ   Sổ & Báo cáo   Danh mục   Hệ thống   Trợ giúp      ← menu bar   │
├──────────────────────────────────────────────────────────────────────────────┤
│  Công ty TNHH Bình Minh   ·   FY2026   ·   đang mở        ← client/period/lock│
├───────────────────────┬──────────────────────────────────────────────────────┤
│  HÔM NAY              │                                                      │
│  ▸ Hôm nay            │                                                      │
│                       │                                                      │
│  NGHIỆP VỤ            │              the page                                │
│  ▸ Bán hàng           │              (one of the 141)                         │
│  ▸ Mua hàng           │                                                      │
│  ▸ Quỹ                │                                                      │
│  ▸ Ngân hàng          │                                                      │
│  ▸ Kho                │                                                      │
│  ▸ CCDC               │                                                      │
│  ▸ TSCĐ               │                                                      │
│  ▸ Tiền lương         │                                                      │
│  ▸ Hóa đơn điện tử    │                                                      │
│                       │                                                      │
│  SỔ & BÁO CÁO         │                                                      │
│  ▸ Sổ sách            │                                                      │
│  ▸ Báo cáo            │                                                      │
│  ▸ Thuế               │                                                      │
│                       │                                                      │
│  DANH MỤC             │                                                      │
│  HỆ THỐNG             │                                                      │
├───────────────────────┴──────────────────────────────────────────────────────┤
│  sẵn sàng                                              ← status bar          │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Three things the frame guarantees on every page**, which is why they are in the
frame and not in the pages:

- **whose books** — the client name, always visible
- **which period** — including whether it is open or locked
- **whether anything is wrong** — the status bar

A figure without those three is a figure nobody can act on. This is the whole
argument for the header existing, and it is why `HeaderText` is a tested function
rather than a label assigned inline.

**An entry the shell cannot open says so.** The rail renders
`Sổ sách chưa hỗ trợ — nhật ký chung, sổ cái và sổ chi tiết` rather than opening an
empty screen. That rule is enforced in code: `nav.Entry` cannot be constructed
without a reason.

---

## 2. The navigation tree

Sixteen rail entries, 141 pages. The rail is the *modules*; the pages inside a
module are reached from the module's own list page, not from the rail.

```
FinGo
├── HÔM NAY
│   └── Hôm nay ──────────────► today.home
│       ├── Việc còn lại của kỳ ──► today.tasks
│       └── Chứng từ vừa nhập ────► today.recent
│
├── NGHIỆP VỤ
│   ├── Quỹ ───────────────► cash.*         (7 pages)
│   ├── Ngân hàng ─────────► bank.*         (5)
│   ├── Mua hàng ──────────► purchase.*     (8)
│   ├── Bán hàng ──────────► sales.*       (10)
│   ├── Hóa đơn điện tử ───► einvoice.*     (5)
│   ├── Kho ───────────────► stock.*        (9)
│   ├── Công cụ dụng cụ ───► tools.*        (5)
│   ├── Tài sản cố định ───► asset.*        (6)
│   ├── Tiền lương ────────► payroll.*      (7)
│   └── Thuế ──────────────► tax.*         (11)
│
├── SỔ & BÁO CÁO
│   ├── Sổ sách ───────────► book.*         (9)
│   ├── Báo cáo ───────────► fs.* + mgmt.* (11)
│   └── Thuế ──────────────► tax.*          ─┐  same pages as NGHIỆP VỤ ▸ Thuế,
│                                             │  reached from where the work is
├── DANH MỤC ─────────────► md.*           (16)
│
└── HỆ THỐNG
    ├── Khách hàng ────────► sys.client      the books FinGo is allowed to open
    └── Hệ thống ──────────► sys.* + init.*  (7 + 5)
```

**One deliberate duplication.** `tax.*` appears under `NGHIỆP VỤ` *and* under
`SỔ & BÁO CÁO`. That is not an oversight: an operator preparing a return enters
through the reports section, while an operator recording a payment enters through
the business section. Same pages, two doors — which is the same reason a shop has
a front and a back entrance.

**Not on the rail.** `init.*` (khởi tạo) is reached from `HỆ THỐNG` and only while
no vouchers exist. `close.*` (the closing run) is reached from `today.tasks` when a
closing step is outstanding, and from `HỆ THỐNG ▸ Kỳ kế toán` always.

---

## 3. The four page archetypes

141 pages, but only four shapes. Every page in [pages.md](pages.md) is one of them.
This is what stops the catalogue being 141 separate designs.

```
 ARCHETYPE 1 — DANH SÁCH (DS)            ARCHETYPE 2 — CHỨNG TỪ (CT)
 a filterable list of things             one document, editable, postable
 ┌───────────────────────────┐           ┌───────────────────────────┐
 │ [kỳ] [đối tượng] [trạng thái]          │ Số: .......... Ngày: .....│
 │───────────────────────────│           │ Đối tượng: ...............│
 │ ◦ PN00012  12/03  NCC A   │           │───────────────────────────│
 │ ◦ PN00013  14/03  NCC B   │           │ Mã  SL  ĐG  Thành tiền    │
 │ ◦ PN00014  15/03  NCC A   │           │───────────────────────────│
 │───────────────────────────│           │ Thuế GTGT ..........      │
 │ [+ Mới] [Sửa] [Xóa] [In]  │           │───────────────────────────│
 └───────────────────────────┘           │ Nợ 152  Có 331  ......... │
        │ double-click                   │───────────────────────────│
        ▼                                │ [Lưu] [Ghi sổ] [In]       │
   ARCHETYPE 2                           └───────────────────────────┘
        │ after posting                          │ after posting
        ▼                                        ▼
 ARCHETYPE 3 — SỔ (SO)                    ARCHETYPE 4 — BÁO CÁO (BC)
 read-only register, running balance      read-only, figures, printable
 ┌───────────────────────────┐           ┌───────────────────────────┐
 │ Sổ chi tiết TK 331        │           │ Bảng cân đối số phát sinh │
 │───────────────────────────│           │───────────────────────────│
 │ Ngày  Diễn giải  Nợ   Có  │           │ TK  Số dư đầu  PS  Số dư  │
 │ 01/03 Mua NCC A  100      │           │     Nợ   Có   Nợ   Có  ...│
 │ 05/03 Trả tiền        100 │           │───────────────────────────│
 │───────────────────────────│           │ 111  ...                  │
 │       Cộng      100  100  │           │ 131  ...                  │
 │       Dư cuối     0       │           │───────────────────────────│
 └───────────────────────────┘           │ [Xem] [In] [Xuất Excel]   │
        ▲                                        ▲
        └──────────── both are READ-ONLY ────────┘
                  over what was posted
```

**The rule that follows from the shapes: nothing writes except `CT` and `RUN`.**

`DS` lists and filters. `SO` and `BC` read. Only a chứng từ (`CT`) or a batch run
(`RUN`) changes the books. That is the entire state machine of the application, and
it is why an `SO` or `BC` page has no save button — not as a UI convention, but
because there is nothing for it to save.

---

## 4. The document lifecycle

Where a voucher goes, and the three irreversible moments.

```
   ┌──────────┐   save    ┌──────────┐   ghi sổ   ┌──────────┐   khóa sổ  ┌──────────┐
   │  NHÁP    │ ────────► │  CHƯA    │ ─────────► │  ĐÃ GHI  │ ─────────► │  ĐÃ KHÓA │
   │ (editing)│           │  GHI SỔ  │            │   SỔ     │            │          │
   └──────────┘           └──────────┘            └──────────┘            └──────────┘
        │                      │                       │                       │
     editable               editable               immutable               immutable
     not in any book        not in any book        in every book           in every book
        │                      │                       │                       │
        └── may be deleted     └── may be deleted      └── correction only     └── nothing
                                                          via a new voucher      at all

   IRREVERSIBLE MOMENTS
   1. Ghi sổ      — the voucher enters the ledger. Số dư thay đổi.
   2. Khóa sổ kỳ  — the period closes. No posting into it, ever again.
   3. Nộp BCTC    — the statements leave the building. The tax authority has them.

   MISA's year-end order puts NỘP (file) BEFORE KHÓA (lock).
   The lock is the last act. FinGo's period.Lock is irreversible — that part is right.
```

**Correction, not deletion.** A posted voucher is never edited or deleted. It is
corrected by writing another voucher that reverses it. The database enforces this:
this repository has **12 immutability triggers**. Any page offering "edit" on a
posted document is a page that will fail at the database, and it should not offer it.

---

## 5. The cadence overlay

The rail is module-first. The *work* is cadence-first. This is the mapping — and
it is the reason `HÔM NAY` sits at the top of the rail.

```
                    HÀNG NGÀY   HÀNG THÁNG/QUÝ        HÀNG NĂM
                    ─────────   ──────────────        ────────
  Bán hàng          ████████    ░░ phân tích          ░░
  Mua hàng          ████████    ░░ phân tích          ░░
  Quỹ               ████████    ░░                     ░░
  Ngân hàng         ████████    ░░ đối chiếu sổ phụ    ░░
  Kho               ████████    ██ tính giá xuất kho   ██ kiểm kê
  CCDC              ░░          ██ phân bổ             ██
  TSCĐ              ░░          ██ khấu hao            ██
  Tiền lương        ░░          ██ tính lương          ██ quyết toán
  Hóa đơn điện tử   ████████    ░░                     ░░
  Thuế              ░░          ██ khấu trừ → kê khai   ██ quyết toán
  Sổ sách           ░░          ██ in sổ               ██ in sổ
  Báo cáo           ░░          ██ BCTC quý            ██ BCTC năm
                    ██ = heavy  ░░ = light
```

**`today.home` is the page that renders this.** Without it the cadence lives only
in the operator's head, and a monthly run that nobody remembers to start is the
normal way a set of books goes wrong.

---

## 6. The closing pipeline

**This is the most important diagram in the document.** MISA documents eight runs
that must complete before the P&L transfer and before any tax return. The order is
not a preference — a closing entry computed before depreciation and inventory
costing have run is wrong, and it is **wrong silently**.

```
  ┌─── 1 ──────────────┐
  │ Phân bổ CCDC       │ close.allocate.tools
  └─────────┬──────────┘
            ▼
  ┌─── 2 ──────────────┐
  │ Phân bổ trả trước  │ close.allocate.prepaid
  └─────────┬──────────┘
            ▼
  ┌─── 3 ──────────────┐
  │ Khấu hao TSCĐ      │ close.depreciate
  └─────────┬──────────┘
            ▼
  ┌─── 4 ──────────────┐      needs init.policy ▸ phương pháp tính giá
  │ Tính giá xuất kho  │ close.cost.stock
  └─────────┬──────────┘
            ▼
  ┌─── 5 ──────────────┐
  │ Tỷ giá xuất quỹ    │ close.fx
  └─────────┬──────────┘
            ▼
  ┌─── 6 ──────────────┐
  │ Lương + chi phí    │ close.payroll
  └─────────┬──────────┘
            ▼
  ┌─── 7 ──────────────┐
  │ Giá thành sản xuất │ close.cost.production        ← P5, sản xuất/xây lắp only
  └─────────┬──────────┘
            ▼
  ┌─── 8 ──────────────┐      needs init.policy ▸ phương pháp tính thuế
  │ Khấu trừ GTGT      │ close.vat
  └─────────┬──────────┘
            ▼
 ╔══════════════════════╗
 ║  B A R R I E R       ║   ← nothing below may run until 1–8 are done
 ╚══════════┬───────────╝
            ▼
  ┌────────────────────┐
  │ Kết chuyển lãi lỗ  │ close.transfer       911 → 4212
  └─────────┬──────────┘
            ▼
  ┌────────────────────┐
  │ Tạm tính TNDN      │ close.cit.provisional
  └─────────┬──────────┘
            ▼
  ┌────────────────────┐
  │ Kê khai thuế       │ tax.vat.return, tax.cit.provisional
  └─────────┬──────────┘
            ▼
  ┌────────────────────┐
  │ Kiểm tra, đối chiếu│ close.reconcile
  └─────────┬──────────┘
            ▼
  ┌────────────────────┐
  │ In sổ sách         │ close.print
  └─────────┬──────────┘
            ▼
  ┌────────────────────┐
  │ Khóa sổ kỳ         │ close.period          ← IRREVERSIBLE, and last
  └────────────────────┘
```

**What this means for the build order.** `today.tasks` must know which of steps 1–8
have run, and `close.transfer` must **refuse to run** while any is outstanding. It
does not refuse today: `internal/close` transfers 911→4212 and has no caller outside
tests, and none of steps 1–8 exist. **That is the largest correctness gap in the
product** — larger than any missing screen, because it produces a plausible wrong
number rather than an obvious missing one.

```
  YEAR END — the same shape, one level up
  kiểm kê ─► đánh giá lại ngoại tệ ─► đối chiếu ─► tính TNDN ─► lập BCTC
                                                            ─► NỘP  ─► KHÓA
                                                                    ▲
                                                        file, THEN lock
```

---

## 7. The traceability spine

The rule that makes this a filing instrument rather than a reporting tool.

```
   BÁO CÁO          SỔ              CHỨNG TỪ        DANH MỤC
   ───────          ──              ─────────       ────────

   ┌────────┐   ┌──────────┐   ┌────────────┐   ┌──────────┐
   │ B01-DN │──►│ Sổ cái   │──►│ Hóa đơn    │──►│ KH A     │
   │ dòng   │   │ TK 131   │   │ bán 00042  │   │ MST ...  │
   │ 131    │   │          │   │            │   │          │
   └────────┘   └──────────┘   └────────────┘   └──────────┘
        │             │              │
        │             │              └──► dòng hàng ──► kho, giá vốn
        │             │
        │             └──► bút toán Nợ/Có ──► kỳ, người nhập, thời điểm
        │
        └──► EVERY ARROW IS CLICKABLE, IN BOTH DIRECTIONS

   The test of the spine, from plan-native-shell.md §P1:
     "every row traceable to its vouchers."
   A trial balance whose figures cannot be opened is a printout, not a book.
```

**Two consequences the schema already supports and the UI does not yet use:**

- Vouchers carry **audit columns** (migration `00003`) — who and when. The spine
  should show them, because "which figure did I change and when" is the first
  question in a dispute.
- `SELECT … FOR UPDATE` is in use in 21 places. The spine reads, so it must read
  without locking — a report that blocks the entry screen is a report nobody runs
  during the day.

---

## 8. Master-data dependencies

Which master records a page needs before it can be used at all. This is the
practical reason `DANH MỤC` is a top-level section and why `init.chart` is step 3
of setup.

```
  MODULE              NEEDS, BEFORE ANY VOUCHER CAN BE WRITTEN
  ──────              ─────────────────────────────────────────
  Bán hàng            md.customer, md.item, md.unit, md.taxrate,
                      md.warehouse, md.term
  Mua hàng            md.supplier, md.item, md.unit, md.taxrate, md.warehouse
  Quỹ                 md.account (111)
  Ngân hàng           md.bank, md.account (112), md.currency if FX
  Kho                 md.item, md.unit, md.warehouse
  CCDC                md.tools, md.department
  TSCĐ                md.asset, md.department
  Tiền lương          md.employee, md.department, tax.pit parameters
  Hóa đơn điện tử     md.customer, md.taxrate, + provider credentials
  Thuế                everything above, plus the mã số chỉ tiêu (§10)
  ─────────────────────────────────────────────────────────────
  ALL OF THEM         md.account, sys.period

  THE HARD EDGE: md.account is seeded by init.chart, and init.policy decides
  WHICH chart. Get init.policy wrong and every module is wrong, inherited.
```

That is why `init.policy` and `init.opening` come second in the build order
([pages.md](pages.md) §13) despite having no vouchers of their own: they are the
cheapest things in the product to get wrong forever.

---

## 9. Entry and exit points per module

Where a module is entered from, and where its output surfaces. A module that
produces something nobody can reach is a module nobody uses.

```
  MODULE          ENTERED FROM                    OUTPUT REACHES
  ──────          ────────────                    ──────────────
  Quỹ             NGHIỆP VỤ ▸ Quỹ                 book.journal.cash, mgmt.cashflow
  Ngân hàng       NGHIỆP VỤ ▸ Ngân hàng           book.journal.cash, bank.reconcile
  Mua hàng        NGHIỆP VỤ ▸ Mua hàng           book.journal.purchase, stock.in.*,
                                                  purchase.payable, tax.vat.input
  Bán hàng        NGHIỆP VỤ ▸ Bán hàng           book.journal.sales, stock.out.*,
                                                  sales.receivable, tax.vat.output,
                                                  einvoice.issue
  Kho             NGHIỆP VỤ ▸ Kho                stock.summary, close.cost.stock
  CCDC            NGHIỆP VỤ ▸ CCDC               tools.summary, close.allocate.tools
  TSCĐ            NGHIỆP VỤ ▸ TSCĐ               asset.summary, close.depreciate
  Tiền lương      NGHIỆP VỤ ▸ Tiền lương         payroll.sheet, tax.pit.return,
                                                  close.payroll
  Thuế            SỔ & BÁO CÁO ▸ Thuế      ─┐    fs.b01…b09
                  NGHIỆP VỤ ▸ Thuế         ─┘    tax.payment
  Sổ sách         SỔ & BÁO CÁO ▸ Sổ sách         close.print
  Báo cáo         SỔ & BÁO CÁO ▸ Báo cáo         close.print, nộp BCTC
  ────────────────────────────────────────────────────────────────────────
  DEAD END today (engine exists, nothing reaches it):
    internal/trading             no caller outside tests
    internal/reports/statutory   no caller outside tests
    internal/close service       no caller outside tests
  ────────────────────────────────────────────────────────────────────────
  These three are the P2/P3 work. Each already has an engine and no door.
```

---

## 10. Coverage map, and what gates what

```
                    P1      P2      P3      P4      P5
                    ██      ░░      ▒▒      ▓▓      ··
  ✓ engine backs     8       —       —       —       —      ← gl, sales w/ COGS,
  ~ partial/wired    —     ~20     ~22      ~6      ~2        document, reports,
  · nothing          —      ~6     ~24     ~28     ~25        partners, chart
```

Approximate, from the `Máy` column of [pages.md](pages.md): **8 backed**, roughly
**50 partial**, the rest absent.

```
  GATE                                       BLOCKS                          STATUS
  ────                                       ──────                          ──────
  01/GTGT mã số chỉ tiêu (TT 89/2026 Phụ lục I)  the whole tax.vat.* family  UNVERIFIED
  B01-DN naming, TT 46/2025 vs TT 99/2025        fs.b01…fs.b09 printing      UNVERIFIED
  PIT bracket article number                     payroll.pit, tax.pit.return  open
  Inventory costing method @ init.policy         stock.costing, close.*      DECISION
  VAT method (khấu trừ vs trực tiếp)             tax.vat.* entirely          DECISION
  Sổ quản trị vs Sổ tài chính                    the ledger model itself     ARCHITECTURAL
```

**The last one is the one to decide now.** MISA holds opening balances
**independently for the management books and the financial books** — two parallel
ledgers from day one. FinGo has one ledger and no concept of a second. Adding it
before the chart, the close and five report families exist is a column; adding it
after is a rewrite. It does not have to be *built* in v1 — but the decision has to
be made before the schema freezes.

---

## 11. Reading order

| If you want to | read |
| :--- | :--- |
| understand the machine | this document, §1–§4 |
| find a specific screen | [pages.md](pages.md) |
| know why the cadence matters | this document §5–§6 |
| know what legal facts are solid | [legal-basis.md](legal-basis.md) |
| know what to build next | [pages.md](pages.md) §13 |
| know what MISA actually does | [reference-misa-sme-flow.md](reference-misa-sme-flow.md) |
| know what P0 shipped | [plan-native-shell.md](plan-native-shell.md) §P0 |
