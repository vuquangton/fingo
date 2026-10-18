# MISA SME 2026 — the main flow, as MISA documents it

**Source of record:** MISA's own help centre for the current release.
**Captured:** for reference against FinGo's IA. Not a specification — a description of
what the market leader's flow actually is.
**Confidence:** structure `P` (read from MISA's own site); the pre-2026 pages are dated and
noted where that matters.

## Which product

| | |
| :--- | :--- |
| **MISA SME 2026** | the on-premise Windows desktop product. Help tree: <https://helpsme.misa.vn/2026/> |
| **MISA AMIS Kế toán** | the online product — a *different* product, not a version of SME. <https://amis.misa.vn/amis-ke-toan/> |
| **MISA ASP** | the service-accountant programme — one accountant, N client companies. <https://asp.misa.vn> |

The last one matters to us: it is the same actor model as [requirements-v1.md](requirements-v1.md) §1.1.

---

## 1. The flow is organised by **cadence**, not by module

MISA's own *Quy trình nghiệp vụ* page is not a feature list. It is a calendar:

```
Khởi tạo dữ liệu          one-off
   ↓
CÔNG VIỆC HÀNG NGÀY       daily entries
   ↓
CÔNG VIỆC HÀNG THÁNG/QUÝ  allocation → costing → tax → close
   ↓
CÔNG VIỆC HÀNG NĂM        year-end: stocktake → FX → CIT → FS → file → lock
```

That is the primary information architecture. The twenty business modules are
*secondary* — they are where a daily entry is made, not how the work is organised.

---

## 2. Initialisation — the 7-step wizard

*Tạo dữ liệu kế toán mới*, per <https://helpsme.misa.vn/2019/html_10010000.htm> (page dated
2019; the 2026 tree keeps the same three getting-started steps).

| Step | Collects |
| ---: | :--- |
| 1 | **Thông tin dữ liệu** — SQL Server instance, data-set name, storage path |
| 2 | **Thông tin người dùng** — the MISA ID that administers this data set. *Offline installs skip it and auto-create `Admin` with no password.* |
| 3 | **Thông tin doanh nghiệp** — company name, trading name, address, tax code, phone, email; multi-branch flag |
| 4 | **Lĩnh vực hoạt động** — trading/service and/or production/construction; **which modules are in use** (Quỹ, Ngân hàng, Mua hàng, Bán hàng…); production costing method; shared-vs-separate catalogues per branch |
| 5 | **Dữ liệu kế toán** — fiscal year, **accounting regime** (TT133 / TT200), e-invoice management, currency |
| 6 | **PP tính giá xuất kho** — one of four: bình quân cuối kỳ · bình quân tức thời · nhập trước xuất trước · đích danh |
| 7 | **PP tính thuế GTGT** — khấu trừ (deduction) or trực tiếp trên doanh thu (direct) |

Steps 4–7 are **policy decisions, captured once and then binding.** Not preferences.

> The step-5 page carries a worked comparison of TT133 vs TT200 that matches
> `Q3` in [legal-basis.md](legal-basis.md) independently: TT133 has **no** 113, 157, 158,
> 161, 212, 213, **521**, **641**; TT133 does not subdivide 1113, 1123, 153, 155, 156, 334,
> 413, 611, 821; and TT133 uses **only TK 154** for production cost where TT200 uses
> 154/621/622/623/627.

### The 2026 addition

The 2026 getting-started tree adds a step the older versions did not have:

> **3. Cập nhật dữ liệu kế toán theo Thông tư 99/2025/TT-BTC**

A regime *migration* is a first-class product flow in the current version — not a data
import, a guided conversion. See <https://helpsme.misa.vn/2026/kb/5-cap-nhat-du-lieu-ke-toan-theo-thong-tu-99-2025-tt-btc/>.

---

## 3. Opening balances — ten groups, entered separately

*Nhập số dư ban đầu*, per <https://helpsme2021.misa.vn/2023/kb/html_27000000/> (updated 27/10/2025).

| # | Group |
| ---: | :--- |
| 1 | Bank account balances |
| 2 | Customer receivables (AR) |
| 3 | Supplier payables (AP) |
| 4 | Employee balances |
| 5 | Inventory — materials and goods |
| 6 | Work in progress |
| 7 | Tools and equipment (CCDC) |
| 8 | Fixed assets |
| 9 | Prepaid expenses |
| 10 | **Other accounts** — the catch-all |

Two things worth noting:

- It is **not one trial-balance grid.** It is a set of purpose-built screens, one per
  sub-ledger, each of which has to tie to its control account.
- *Số dư ban đầu được lưu trữ độc lập giữa hai hệ thống **Sổ quản trị** và **Sổ tài chính*** —
  opening balances are held **independently for the management books and the financial
  books**. Two parallel ledgers, from day one.

---

## 4. Daily work — the ten entry modules

1. Quỹ (cash) · 2. Ngân hàng (bank) · 3. Mua hàng (purchases) · 4. Bán hàng (sales) ·
5. Hóa đơn đặt in/tự in (pre-printed invoices) · 6. Hóa đơn điện tử (e-invoices) ·
7. Kho (inventory) · 8. Công cụ dụng cụ (tools) · 9. Tài sản cố định (fixed assets) ·
10. Tiền lương (payroll)

## 5. Monthly / quarterly work — the closing run

This is the part most worth copying, **in this order**:

| # | Step |
| ---: | :--- |
| 1 | Phân bổ công cụ dụng cụ — allocate tools |
| 2 | Phân bổ chi phí trả trước — allocate prepaid expenses |
| 3 | Tính khấu hao TSCĐ — depreciation |
| 4 | Tính giá xuất kho — inventory issue costing |
| 5 | Tính tỷ giá xuất quỹ — FX rate on foreign-currency payments |
| 6 | Tính lương → hạch toán chi phí lương → trả lương |
| 7 | Tính giá thành — production costing |
| 8 | **Khấu trừ thuế** — VAT deduction |

**Then, and only then:**

1. Lập và nộp báo cáo thuế — prepare and submit the tax returns
2. Kết chuyển lãi lỗ — the P&L transfer (closing entries)
3. Tạm tính thuế TNDN — provisional CIT
4. Kiểm tra, đối chiếu chứng từ, sổ sách — check and reconcile
5. In sổ sách, chứng từ — print the books
6. Quản lý công nợ phải thu / phải trả — AR / AP
7. Đối chiếu công nợ với khách hàng/NCC — reconcile balances with counterparties
8. Lập báo cáo phân tích tài chính — analysis

**The order is the point.** Allocation and costing run *before* the result is transferred and
*before* the tax return is prepared. A closing entry computed before depreciation and
inventory costing have run is wrong, and it is wrong silently.

## 6. Annual work

**Đầu năm (start of year)**
1. Tạo mới dữ liệu từ năm trước — create the new year's data from the previous year
2. Kết chuyển lợi nhuận năm nay sang năm trước — carry the result forward

**Cuối năm (year end)**
1. Kiểm kê, đối chiếu thực tế — stocktake and physical reconciliation
2. Đánh giá lại tài khoản ngoại tệ — FX revaluation
3. Kiểm tra, đối chiếu chứng từ, sổ sách — reconcile documents and books
4. Tính thuế TNDN phải nộp — CIT
5. Lập và kiểm tra BCTC — prepare and check the financial statements
6. Nộp báo cáo tài chính cho cơ quan thuế — **file with the tax authority**
7. Khóa sổ kỳ kế toán — lock the period

Note the sequence at year end: **file, then lock.** The lock is the last act, and it is
irreversible. FinGo's `period.Lock` is irreversible too — that part is right.

---

## 7. The twenty business modules

MISA SME's *Menu Nghiệp vụ*:

Quỹ · Ngân hàng · Mua hàng · Bán hàng · Hóa đơn điện tử · Kho · CCDC · TSCĐ ·
Quản lý hóa đơn · Thuế · Tổng hợp · Giá thành · Tiền lương · Khế ước vay · Hợp đồng ·
Ngân sách · Thủ kho · Thủ quỹ · Phân tích tài chính · Báo cáo

Plus industry overlays: dược phẩm, thiết bị văn phòng, ô tô/xe máy, vật liệu xây dựng,
vận tải, du lịch, sản xuất, xây lắp, logistics.

---

## 8. What this changes for FinGo

Five things, in order of consequence.

1. **Our IA is module-first; MISA's is cadence-first.** [plan-gui.md](plan-gui.md) proposes
   Documents / Books / Reports / Master / Period / Statutory. MISA organises the operator's
   screen around *today, this month, this year*. The rail should carry a **work-cycle**
   view — and "Period" should be promoted from one rail item to the organising idea.

2. **Our close is missing the eight steps that precede it.** `internal/close` transfers
   911→4212 and books provisions. It has **no allocation run, no depreciation run, no
   inventory-costing run, no FX run, no payroll run**. Computing the result before those
   have run is exactly the silent error described above.

3. **Step 4–7 of the wizard are missing entirely.** We store `AccountingStandard` as a free
   string that nothing reads. MISA asks four binding policy questions at setup, including
   the **two** whose absence this repo already records as debt: inventory costing method and
   VAT method.

4. **Opening balances are ten screens, not one grid.** Our prototype offers a
   trial-balance grid as variant A. MISA's answer — and the accounting answer — is one
   screen per sub-ledger, each tying to its control account. Note this against
   `accounting-review.md`'s finding that `opening.ValidateBalance` checks only that account
   totals balance.

5. **Two parallel ledgers exist in the market.** Sổ quản trị and Sổ tài chính, with
   independent opening balances. FinGo has a single ledger and no concept of the second.
   That is not a v1 item, but it is an architectural fork worth knowing about **before** the
   chart and the close are frozen.

### And the confirmation worth having

MISA SME **2026** ships a guided **TT99/2025 conversion**, and its own TT133-vs-TT200
comparison independently reproduces the chart findings in our
[legal-basis.md](legal-basis.md) `Q3`. The regime question we left open is a live product
flow for the market leader — which is evidence that it is a real question and not
over-caution.
