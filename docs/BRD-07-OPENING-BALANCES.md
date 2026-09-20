# BRD-07: BUSINESS REQUIREMENTS DOCUMENT & OPERATIONAL CUTOVER MANUAL
## MODULE: INITIALIZATION & CUTOVER (OPENING BALANCES - LAYER 3)
### Standard Authority: `FINGO-BRD-OPENING-2026-V1`
**Authority**: Lead Business Analyst (20+ Years) & Chief Accountant (20+ Years VAS/Circular 99/2025/TT-BTC)  
**Target Application**: FinGo SME Accounting Desktop Application (Go 1.25 + Wails v2 + MariaDB 12.3 + Vue 3)  
**Statutory Regulatory Framework**:
- Luật Kế toán số 88/2015/QH13 (Điều 10, 12, 13, 50, 52)
- Thông tư 99/2025/TT-BTC (Hiệu lực 01/01/2026 - Chuẩn mực Chế độ Kế toán Doanh nghiệp)
- Thông tư 133/2016/TT-BTC & Thông tư 200/2014/TT-BTC
- Thông tư 45/2013/TT-BTC & Thông tư 147/2016/TT-BTC (Chế độ quản lý và trích khấu hao TSCĐ)
- Nghị định 123/2020/NĐ-CP & Thông tư 32/2025/TT-BTC (Hóa đơn điện tử - Quản lý công nợ mở theo hóa đơn)
- Nghị định 181/2025/NĐ-CP & Luật số 48/2024/QH15 (Thanh toán không dùng tiền mặt $\ge 5.000.000$ VNĐ)
- Chuẩn mực Kế toán Việt Nam VAS 01 (Chuẩn mực chung) & VAS 10 (Ảnh hưởng của việc thay đổi tỷ giá hối đoái)
- `FINGO-QA-STRATEGY-2026-V1` (Zero-Float Policy, Byte-Identical Reproducibility)

---

# 1. Executive Summary & Production Readiness Verdict

### Production Readiness Assessment: **FAILED (CANNOT OPERATE ALONE IN PROD YET)**

```
+----------------------------------------------------------------------------------------------------+
|                                    CUTOVER PRODUCTION READINESS                                    |
|                                                                                                    |
|   [Phase 1: Company Profile & System]  --> PASS                                                    |
|   [Phase 2: Accounting Spine Layer 1]  --> PASS (COA, Currencies, Fiscal Periods)                  |
|   [Phase 3: Catalogs Master Data L2]   --> PASS (UOM, Wh, Bank, Cust, Vend, Item, Emp)             |
|                                                                                                    |
|                                                |                                                   |
|                                                v                                                   |
|                               +---------------------------------+                                  |
|                               |     PHASE 4: OPENING BALANCES   |                                  |
|                               |    CUTOVER & INITIALIZATION     |                                  |
|                               |          CURRENT STATE:         |                                  |
|                               |       STUB IMPLEMENTATION       |                                  |
|                               |    CANNOT GO-LIVE IN PROD!      |                                  |
|                               +---------------------------------+                                  |
|                                                |                                                   |
|               +--------------------------------+--------------------------------+                  |
|               |                                |                                |                  |
|               v                                v                                v                  |
|    [MISSING GL CUTOVER]             [MISSING SUBLEDGER SYNC]         [MISSING INVOICE AGING]       |
|    Cannot establish initial         GL TK 131/331/15x/211 does not   Cannot clear open invoices    |
|    Trial Balance (Bảng CĐSPS).      reconcile to subledgers.         under FIFO matching.          |
+----------------------------------------------------------------------------------------------------+
```

### 1.1 The 6 Critical Gaps Blocking Production Operation
1. **Zero MariaDB Persistence for Opening Balances**: No database tables exist to store opening balances. If the server reboots, all cutover data is permanently lost.
2. **Absence of Cross-Layer Subledger Reconciliation Engine**: In professional accounting, an opening trial balance without subledger detail reconciliation (Sổ cái $\ne$ Sổ chi tiết) is illegal under Law on Accounting 88/2015/QH13 Art. 13.
3. **Absence of Invoice-Level AR/AP Cutover Tracking**: Sổ chi tiết công nợ 131/331 must record individual open unpaid invoices (*Số hóa đơn, Ngày hóa đơn, Số tiền gốc, Số tiền còn nợ*) to support Decree 123/2020 and Decree 181/2025 non-cash verification.
4. **Absence of Fixed Asset Depreciation Schedule Engine**: Accounts 211 and 214 must link to individual asset cards (*Thẻ tài sản cố định*) with historical cost, accumulated depreciation, remaining life, and monthly allocation accounts (627, 641, 642) per Circular 45/2013/TT-BTC.
5. **No Cutover State Machine with Chief Accountant Lock**: Opening records must transition through an immutable state gate (`DRAFT` $\to$ `VALIDATED` $\to$ `COMMITTED` $\to$ `LOCKED`). Once committed, opening balances must be strictly read-only.
6. **No Bulk Excel Migration Importer**: Accountants transitioning from MISA, FAST, or Bravo cannot type thousands of initial account balances, customers, items, and assets manually.

---

# 2. Statutory Legal & Accounting Framework

```
+----------------------------------------------------------------------------------------------------+
|                                      STATUTORY AUTHORITY MATRIX                                    |
|                                                                                                    |
|    1. LUẬT KẾ TOÁN 88/2015/QH13:                                                                   |
|       - Điều 13: Nghiêm cấm hạch toán sai lệch, lập chứng từ hoặc số dư khống.                    |
|       - Bảng cân đối số phát sinh mở đầu kỳ phải cân đối tuyệt đối: Tổng Nợ == Tổng Có.            |
|                                                                                                    |
|    2. THÔNG TƯ 99/2025/TT-BTC & THÔNG TƯ 133/2016/TT-BTC:                                          |
|       - Tài khoản loại 5, 6, 7, 8, 9 (Doanh thu & Chi phí) TUYỆT ĐỐI KHÔNG CÓ SỐ DƯ ĐẦU KỲ.       |
|       - Tài khoản lưỡng tính (131, 331, 1388, 3388, 421) phải theo dõi riêng biệt Dư Nợ và Dư Có. |
|       - Không được bù trừ số dư của khách hàng/nhà cung cấp khác nhau.                            |
|                                                                                                    |
|    3. THÔNG TƯ 45/2013/TT-BTC (QUẢN LÝ VÀ TRÍCH KHẤU HAO TSCĐ):                                   |
|       - Tiêu chuẩn ghi nhận TSCĐ: Nguyên giá >= 30.000.000 VNĐ, thời gian sử dụng >= 1 năm.        |
|       - Phải quản lý chi tiết từng tài sản: Nguyên giá (TK 211), Hao mòn lũy kế (TK 214),          |
|         Giá trị còn lại, Mức trích khấu hao hàng tháng.                                            |
|                                                                                                    |
|    4. THÔNG TƯ 105/2020/TT-BTC & NGHỊ ĐỊNH 181/2025/NĐ-CP:                                         |
|       - Công nợ mở đầu kỳ phải gắn với Mã số thuế hợp lệ và thông tin ngân hàng của NCC.           |
+----------------------------------------------------------------------------------------------------+
```

---

# 3. Core Architecture & Double-Entry Invariant Rules

```
+----------------------------------------------------------------------------------------------------+
|                                    FIVE PILLARS OF CUTOVER EQUILIBRIUM                             |
|                                                                                                    |
|   1. GENERAL LEDGER EQUILIBRIUM:                                                                   |
|      Sum(Debit 111..421) - Sum(Credit 111..421) == 0.00 VND                                        |
|                                                                                                    |
|   2. CUSTOMER AR RECONCILIATION:                                                                   |
|      Sum(Customer Opening Debit)  == Opening Debit(TK 131)                                         |
|      Sum(Customer Opening Credit) == Opening Credit(TK 131 - Khách trả trước)                      |
|                                                                                                    |
|   3. VENDOR AP RECONCILIATION:                                                                     |
|      Sum(Vendor Opening Credit)   == Opening Credit(TK 331)                                        |
|      Sum(Vendor Opening Debit)    == Opening Debit(TK 331 - Trả trước NCC)                         |
|                                                                                                    |
|   4. INVENTORY VALUATION RECONCILIATION:                                                           |
|      Sum(Warehouse Item Qty * UnitCost) == Opening Debit(TK 152 + 153 + 155 + 156)                 |
|                                                                                                    |
|   5. FIXED ASSET RECONCILIATION:                                                                   |
|      Sum(Asset Card Original Cost)       == Opening Debit(TK 211)                                  |
|      Sum(Asset Card Accumulated Depr)    == Opening Credit(TK 214)                                 |
+----------------------------------------------------------------------------------------------------+
```

---

# 4. End-to-End Business Process & Cutover Workflow

```
               [START: Fiscal Cutover / Hệ thống Chốt số dư đầu kỳ]
                                      |
                                      v
                 Step 1: Kế toán nhập Bảng CĐTK Đầu kỳ (GL)
                 - Các tài khoản loại 1 đến 4 (Tài sản & Nguồn vốn)
                 - Kiểm tra tự động: Tổng Nợ == Tổng Có (INV-OPEN-01)
                                      |
                                      v
                 Step 2: Kế toán nhập Sổ chi tiết Khách hàng (131)
                 - Chi tiết từng khách hàng & danh sách hóa đơn mở
                 - Kiểm tra tự động: Khớp 100% số dư TK 131 (INV-OPEN-02)
                                      |
                                      v
                 Step 3: Kế toán nhập Sổ chi tiết Nhà cung cấp (331)
                 - Chi tiết từng NCC & danh sách hóa đơn chưa thanh toán
                 - Kiểm tra tự động: Khớp 100% số dư TK 331 (INV-OPEN-03)
                                      |
                                      v
                 Step 4: Kế toán kho nhập Tồn kho đầu kỳ (15x)
                 - Chi tiết theo Kho, Mã vật tư/hàng hóa, Số lượng, Đơn giá
                 - Kiểm tra tự động: Tổng giá trị == Dư Nợ 152/153/155/156 (INV-OPEN-04)
                                      |
                                      v
                 Step 5: Kế toán nhập Danh mục Tài sản cố định (211/214)
                 - Chi tiết từng TSCĐ, Nguyên giá, Hao mòn, Khung khấu hao
                 - Kiểm tra tự động: Khớp 100% số dư TK 211 và 214 (INV-OPEN-05)
                                      |
                                      v
                +-------------------------------------------+
                |    HỆ THỐNG KIỂM TRA ĐỐI CHIẾU TỔNG THỂ   |
                |          (RECONCILIATION GATEWAY)         |
                +-------------------------------------------+
                        /                           \
               (Có sai lệch)                      (Khớp 100%)
                     /                                 \
           [BÁO CÁO LỖI LỆCH SỔ]              Step 6: Kế toán trưởng duyệt
           - Chỉ rõ dòng & số tiền             [KHÓA SỔ DƯ ĐẦU KỲ (COMMITTED)]
           - Chặn không cho mở sổ                          |
                                                           v
                                              [HỆ THỐNG SẴN SÀNG HẠCH TOÁN]
                                              - Mở khóa ghi sổ chứng từ năm mới
```

---

# 5. Detailed Use Case Specifications

```
+----------------------------------------------------------------------------------------------------+
|                                       USE CASE TAXONOMY                                            |
|                                                                                                    |
|   UC-OPEN-01: Initialize General Ledger Opening Trial Balance (Bảng cân đối tài khoản mở)           |
|   UC-OPEN-02: Import & Reconcile Customer AR Opening Subledger (Chi tiết công nợ phải thu 131)     |
|   UC-OPEN-03: Import & Reconcile Vendor AP Opening Subledger (Chi tiết công nợ phải trả 331)       |
|   UC-OPEN-04: Import & Reconcile Inventory Opening Stock (Chi tiết tồn kho 15x theo kho)          |
|   UC-OPEN-05: Register Fixed Asset Opening Cards & Depreciation Schedule (Sổ theo dõi TSCĐ)        |
|   UC-OPEN-06: Master Cutover Audit, Comprehensive Reconciliation & Ledger Hard Lock                |
+----------------------------------------------------------------------------------------------------+
```

### 5.1 UC-OPEN-01: Initialize General Ledger Opening Balances

#### Primary Actor
- Kế toán tổng hợp (General Ledger Accountant)
- Kế toán trưởng (Chief Accountant - Reviewer)

#### Main Flow (Happy Path)
1. Accountant selects menu **Số dư ban đầu $\to$ Số dư tài khoản** (Shortcut: `F2`).
2. System displays hierarchical COA grid for accounts from Class 1 to Class 4.
3. Accountant inputs Opening Debit and Opening Credit for leaf accounts.
4. System automatically computes live parent rollup totals:
   $$\text{ParentBalance} = \sum \text{ChildBalances}$$
5. System displays live footer comparison:
   - **Tổng dư Nợ (Total Debit)**
   - **Tổng dư Có (Total Credit)**
   - **Chênh lệch (Difference)**
6. When Difference is zero, accountant clicks **Lưu tạm (Save Draft)**.

#### Alternative Paths
- **A1 (Multi-Currency Account)**: Accountant keys in USD balance for account `11221`. System prompts for `AmountFC` ($10,000 USD) and `ExchangeRate` (25,450 VND/USD). System computes book value $254,500,000$ VND.

#### Exception Paths
- **E1 (Non-Leaf Posting)**: Accountant attempts to type balance directly into parent account `111`. System disallows editing and shows: `Chỉ được nhập số dư vào tài khoản chi tiết (Lá)`.
- **E2 (P&L Account Has Balance)**: Accountant inputs balance for account `5111` or `642`. System rejects with `ErrInvalidOpeningAccount: Tài khoản doanh thu/chi phí không được có số dư đầu kỳ`.

---

### 5.2 UC-OPEN-02: Customer AR Subledger & Open Invoice Matching (TK 131)

#### Main Flow
```
Accountant                     FinGo System                    MariaDB
    |                               |                             |
    |--- 1. Select Customer KH001 ->|                             |
    |    Add Invoice HD-001234      |                             |
    |    (Date: 2025-11-20, 50M)    |                             |
    |                               |--- 2. Validate Customer --->|
    |                               |    (Customer Active)        |
    |                               |--- 3. Calculate AR Total -->|
    |                               |    (Sum matches TK 131)     |
    |                               |--- 4. Insert Subledger ---->|
    |<-- 5. Show Reconciled (OK) ---|                             |
```

#### Exception Paths
- **E1 (Subledger Mismatch with GL)**: Sum of customer subledgers is 150,000,000 VND, but Account 131 Debit balance is 160,000,000 VND. System flags red warning: `Lệch 10,000,000 VND so với Sổ cái TK 131. Vui lòng rà soát lại!`.

---

# 6. User Journeys by Role

```
+----------------------------------------------------------------------------------------------------+
|                                     ROLE-BASED CUTOVER JOURNEYS                                    |
|                                                                                                    |
|   1. Kế toán công nợ (AR/AP Accountant):                                                           |
|      Export old invoice list -> Verify customer/vendor MST -> Enter open bills -> Check 131/331    |
|                                                                                                    |
|   2. Thủ kho / Kế toán kho (Inventory Accountant):                                                 |
|      Physical stocktaking count -> Input Warehouse-Item Qty/Price -> Reconcile to TK 15x           |
|                                                                                                    |
|   3. Kế toán tài sản (Fixed Asset Accountant):                                                     |
|      Review Asset Cards -> Input Historical Cost & Acc Depr -> Verify to TK 211 & 214               |
|                                                                                                    |
|   4. Kế toán trưởng (Chief Accountant):                                                            |
|      Open Reconciliation Dashboard -> Audit All Invariants -> Execute Final Cutover Lock          |
+----------------------------------------------------------------------------------------------------+
```

---

# 7. Desktop User Interface (Wails UI / Vue 3 Wireframes)

### 7.1 General Ledger Opening Balance Screen (`/opening/accounts`)

```
+--------------------------------------------------------------------------------------------------+
|  FinGo Accounting - [Bảng Cân đối Số dư Tài khoản Đầu kỳ]                              [-] [x]   |
+--------------------------------------------------------------------------------------------------+
|  Ngày chốt số dư: [31/12/2025  v]    Năm tài chính: [2026]     [Nhập từ Excel]  [Xuất Excel]     |
+--------------------------------------------------------------------------------------------------+
| Số hiệu TK | Tên tài khoản                  | Tính chất | Dư Nợ đầu kỳ (VND) | Dư Có đầu kỳ (VND)|
+------------+--------------------------------+-----------+--------------------+-------------------+
| 111        | Tiền mặt                       | Dư Nợ     |        150,000,000 |                 0 |
|   1111     |   Tiền Việt Nam                | Dư Nợ     |        150,000,000 |                 0 |
| 112        | Tiền gửi ngân hàng             | Dư Nợ     |        850,000,000 |                 0 |
|   1121     |   Tiền Việt Nam (VCB)          | Dư Nợ     |        850,000,000 |                 0 |
| 131        | Phải thu của khách hàng        | Lưỡng tính|        250,000,000 |        20,000,000 |
|   1311     |   Phải thu khách hàng VND      | Lưỡng tính|        250,000,000 |        20,000,000 |
| 156        | Hàng hóa                       | Dư Nợ     |        420,000,000 |                 0 |
|   1561     |   Giá mua hàng hóa             | Dư Nợ     |        420,000,000 |                 0 |
| 211        | Tài sản cố định hữu hình       | Dư Nợ     |      1,200,000,000 |                 0 |
| 214        | Hao mòn tài sản cố định        | Dư Có     |                  0 |       350,000,000 |
| 331        | Phải trả cho người bán         | Lưỡng tính|         15,000,000 |       180,000,000 |
| 411        | Vốn đầu tư của chủ sở hữu      | Dư Có     |                  0 |     2,375,000,000 |
+------------+--------------------------------+-----------+--------------------+-------------------+
| TỔNG CỘNG CÂN ĐỐI:                                      |      2,885,000,000 |     2,885,000,000 |
| TRẠNG THÁI CÂN ĐỐI: [V] ĐÃ CÂN ĐỐI TUYỆT ĐỐI (CHÊNH LỆCH: 0 VND)                                 |
+--------------------------------------------------------------------------------------------------+
| F2: Nhập nhanh | F3: Xem đối chiếu chi tiết | Ctrl+S: Lưu tạm | F12: Nộp duyệt KTT | Esc: Đóng   |
+--------------------------------------------------------------------------------------------------+
```

### 7.2 Cutover Reconciliation Dashboard (`/opening/reconciliation`)

```
+--------------------------------------------------------------------------------------------------+
|  FinGo Accounting - [Bảng Đối chiếu Khớp đúng Sổ Cái & Sổ Chi tiết]                    [-] [x]   |
+--------------------------------------------------------------------------------------------------+
| Phân hệ đối chiếu        | Số dư trên Sổ Cái   | Số dư Sổ Chi tiết   | Chênh lệch   | Trạng thái |
+--------------------------+---------------------+---------------------+--------------+------------+
| 1. Tổng Cân đối Thử (GL) | Nợ: 2,885,000,000   | Có: 2,885,000,000   | 0 VND        | [KHỚP 100%]|
| 2. Phải thu KH (TK 131)  | Dư Nợ: 250,000,000  | Chi tiết: 250,000,000| 0 VND        | [KHỚP 100%]|
|                          | Dư Có: 20,000,000   | Chi tiết: 20,000,000 | 0 VND        | [KHỚP 100%]|
| 3. Phải trả NCC (TK 331) | Dư Có: 180,000,000  | Chi tiết: 180,000,000| 0 VND        | [KHỚP 100%]|
|                          | Dư Nợ: 15,000,000   | Chi tiết: 15,000,000 | 0 VND        | [KHỚP 100%]|
| 4. Tồn kho (TK 152/156)  | Dư Nợ: 420,000,000  | Chi tiết: 420,000,000| 0 VND        | [KHỚP 100%]|
| 5. Tài sản CĐ (TK 211)   | Dư Nợ: 1,200,000,000| Chi tiết: 1,200,000k | 0 VND        | [KHỚP 100%]|
| 6. Hao mòn TSCĐ (TK 214) | Dư Có: 350,000,000  | Chi tiết: 350,000,000| 0 VND        | [KHỚP 100%]|
+--------------------------+---------------------+---------------------+--------------+------------+
| [!] KẾT LUẬN CỦA HỆ THỐNG: TẤT CẢ PHÂN HỆ ĐÃ ĐỐI CHIẾU KHỚP ĐÚNG 100.00%.                         |
|                                                                                                  |
|              [  XÁC NHẬN CHỐT VÀ KHÓA SỐ DƯ ĐẦU KỲ (CHIEF ACCOUNTANT ONLY)  ]                    |
+--------------------------------------------------------------------------------------------------+
```

---

# 8. Implementation Roadmap & Execution Plan

```
+----------------------------------------------------------------------------------------------------+
|                                  PHASE 4 IMPLEMENTATION ROADMAP                                    |
|                                                                                                    |
|  +---------------------------+    +---------------------------+    +---------------------------+   |
|  | Slice 4.1: Domain Layer   |    | Slice 4.2: MariaDB & SQLC |    | Slice 4.3: Usecase Layer  |   |
|  | - AccountOpeningBalance   |    | - Migration 00008 (6 tbls)|    | - OpeningUseCase          |   |
|  | - CustomerOpeningBalance  | -> | - DDL & Foreign Keys      | -> | - Import & Reconcile      |   |
|  | - VendorOpeningBalance    |    | - 18 SQL Queries          |    | - State Machine Gating    |   |
|  | - InventoryOpeningBalance |    | - SQLC querier generation |    | - In-Memory & Live Tests  |   |
|  | - AssetOpeningBalance     |    | - CatalogRepo adapter     |    | - >= 85% Usecase Coverage |   |
|  | - >= 95% Domain Coverage  |    |                           |    |                           |   |
|  +---------------------------+    +---------------------------+    +---------------------------+   |
|                                                 |                                                  |
|                                                 v                                                  |
|                                   +---------------------------+                                    |
|                                   | Slice 4.4: Dual Review    |                                    |
|                                   | - Standards Reviewer      |                                    |
|                                   | - Spec Reviewer           |                                    |
|                                   | - CodeGraph & Git Sync    |                                    |
|                                   +---------------------------+                                    |
+----------------------------------------------------------------------------------------------------+
```

### Execution Strategy:
- **TDD Workflow**: Red $\to$ Green $\to$ Refactor across domain and usecase seams.
- **Zero-Float Policy**: 100% enforcement of `shopspring/decimal`.
- **Live MariaDB Integration**: Real MariaDB 12.3 testing with foreign key and constraint integrity.
- **Dual-Axis Review**: Standards and Spec validation subagents before commit.
