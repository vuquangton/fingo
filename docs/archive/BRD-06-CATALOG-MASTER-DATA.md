# BRD-06: BUSINESS REQUIREMENTS DOCUMENT & OPERATIONAL READINESS ANALYSIS
## MODULE: CATALOGS & COUNTERPARTIES (MASTER DATA - LAYER 2)
### Standard Authority: `FINGO-BRD-CATALOG-2026-V1`
**Role Evaluation**: BA Lead (20+ Years Enterprise Architecture) & Chief Accountant (20+ Years VAS/Circular 99/2025/TT-BTC)  
**Target Platform**: FinGo SME Accounting Desktop Application (Go 1.25 + Wails v2 + MariaDB 12.3 + Vue 3)  
**Statutory Framework**: 
- Luật Kế toán số 88/2015/QH13 (Điều 10, 12, 13, 50, 52)
- Thông tư 99/2025/TT-BTC (Hiệu lực 01/01/2026 - Chuẩn mực Chế độ Kế toán Doanh nghiệp)
- Thông tư 133/2016/TT-BTC & Thông tư 200/2014/TT-BTC
- Nghị định 123/2020/NĐ-CP & Thông tư 32/2025/TT-BTC (Hóa đơn điện tử)
- Nghị định 181/2025/NĐ-CP & Luật số 48/2024/QH15 (Quy định chi trả không dùng tiền mặt $\ge 5.000.000$ VNĐ)
- Nghị quyết số 204/2025/QH15 & Nghị định 174/2025/NĐ-CP (Giảm thuế GTGT 8% đến hết 31/12/2026)
- Thông tư 105/2020/TT-BTC & Thông tư 86/2024/TT-BTC (Thuật toán Mã số thuế Modulo-11)
- Luật số 91/2025/QH15 về Bảo vệ Dữ liệu Cá nhân (PDPL - Hiệu lực 01/01/2026)
- Luật Bảo hiểm Xã hội số 41/2024/QH15

---

# 1. Executive Summary & Production Readiness Verdict

### Production Readiness Assessment: **FAILED (CANNOT OPERATE ALONE IN PROD YET)**

```
+----------------------------------------------------------------------------------------------------+
|                                    PRODUCTION READINESS GATEWAY                                    |
|                                                                                                    |
|   [Phase 1: Foundation]       [Phase 2: Accounting Spine]       [Phase 3: Catalogs / Master Data]  |
|   - CompanyProfile: PASS      - Currency / FX: PASS             - UOM & Multiplier: PASS           |
|   - BranchOrgUnit: PASS       - COA Tree & Leaves: PASS         - Warehouse (15x): PASS            |
|   - User/Role/SoD: PASS       - Fiscal Year / Period: PASS      - Bank Account (1121/22): PASS     |
|   - SystemOption: PASS        - Cost Center / Expense: PASS     - Customer / Vendor: PASS          |
|                                                                 - Item / Employee: PASS            |
|                                                                                                    |
|                                                |                                                   |
|                                                v                                                   |
|                               +---------------------------------+                                  |
|                               |      OPERATIONAL VERDICT        |                                  |
|                               |   CRITICAL ARCHITECTURAL GAPS   |                                  |
|                               |     BLOCKED FROM PRODUCTION     |                                  |
|                               +---------------------------------+                                  |
|                                                |                                                   |
|           +------------------------------------+-----------------------------------+               |
|           |                                    |                                   |               |
|           v                                    v                                   v               |
|   [GAP 1: NO VOUCHER ENGINE]         [GAP 2: NO OPENING BALANCES]        [GAP 3: NO WAILS DESKTOP UI] |
|   Cannot post journal entries        Cannot verify initial GL balance   Accountant cannot input or  |
|   or compute financial reports.      or subledger counterparty aging.   view records on Windows.    |
+----------------------------------------------------------------------------------------------------+
```

### 1.1 The 7 Operational Blockers Preventing Standalone Production Use

| # | Blocker Dimension | Technical & Accounting Reality | Statutory & Business Impact | Severity |
| :- | :--- | :--- | :--- | :--- |
| **1** | **Missing Layer 3 (Opening Balances - Số dư đầu kỳ)** | Master data exists (customers, vendors, items, bank accounts), but no opening debit/credit balances can be keyed in or verified. | Breaches Circular 99/2025/TT-BTC Art. 8. An accounting system cannot open a fiscal year without balancing trial balance numbers ($$\sum \text{Debit} \equiv \sum \text{Credit}$$). | **P1 (Blocker)** |
| **2** | **Missing Layer 4 (Voucher & Double-Entry Posting Engine)** | Backend domain entities cannot record transactions (no Cash Receipt 111, Bank Transfer 112, Sales Invoice 511/131, Purchase 156/331). | The software cannot perform day-to-day enterprise bookkeeping. Zero vouchers can be issued, approved, or locked. | **P1 (Blocker)** |
| **3** | **Missing Presentation Layer (Wails UI Desktop Handlers)** | Code exists as pure Go domain, MariaDB persistence, and usecases. No Wails frontend bridges, Vue 3 forms, or data tables exist. | Human accountants cannot interact with the system on desktop workstations without GUI forms, data tables, and shortcuts (F2/F3/Ctrl+S). | **P1 (Blocker)** |
| **4** | **Missing TCT Real-Time Tax Status Verification** | MST Modulo-11 validates syntax mathematically, but does not query General Department of Taxation (TCT) live status (*Đang hoạt động*, *Ngừng hoạt động*, *Tạm nghỉ*, *Bỏ trốn*). | Incurs severe tax penalty risk under Decree 125/2020/NĐ-CP for accepting invoices from inactive/shell companies (*Doanh nghiệp bỏ trốn*). | **P2 (High)** |
| **5** | **Missing Resolution 204/2025/QH15 Dynamic VAT Switch** | Standard VAT rate is configured as static 10% or 8%, but lacks automated statutory expiry on 2026-12-31. | From 2027-01-01, 8% VAT rate becomes illegal; system must enforce automatic transition back to 10% without code redeployment. | **P2 (High)** |
| **6** | **Missing Immutable Audit Log for Master Data Mutations** | Master data updates (changes to Vendor Bank Account, Customer Credit Limit, Employee Salary) are currently overwritten in place without historical change log. | Breaches Law on Accounting 88/2015/QH13 (10-year immutable audit retention) and creates internal fraud exposure. | **P2 (High)** |
| **7** | **Missing Statutory Formats & Excel/XML Data Import** | SMEs migrating from MISA, FAST, or Excel have no bulk import pipeline for master data catalogs. | Unacceptable manual onboarding cost for 1,000+ items, customers, and employees. | **P3 (Medium)** |

---

# 2. Statutory Legal Framework & Vietnamese Accounting Authority Matrix

```
+----------------------------------------------------------------------------------------------------+
|                                    STATUTORY REGULATORY UMBRELLA                                   |
|                                                                                                    |
|    [LUẬT KẾ TOÁN 88/2015]          [THÔNG TƯ 99/2025/TT-BTC]           [NGHỊ ĐỊNH 123/2020]        |
|    - 10-year retention rule        - New COA & leaf enforcement        - E-Invoice mandatory data  |
|    - No unposted records           - Hermaphroditic 131/331 balance    - Legal Name & address sync |
|                                                                                                    |
|    [NGHỊ ĐỊNH 181/2025]            [THÔNG TƯ 105/2020/TT-BTC]          [LUẬT 91/2025/QH15 (PDPL)]  |
|    - Non-cash rule >= 5M VND       - 10/13-digit MST Modulo-11         - Synthetic CCCD/BHXH mask  |
|    - Mandatory vendor bank acc     - Checksum: D10 = 10 - (Sum%11)     - Role-based unmask logging |
|                                                                                                    |
|    [NGHỊ QUYẾT 204/2025/QH15]      [LUẬT BHXH 41/2024/QH15]            [CIRCULAR 133 / 200]        |
|    - 8% VAT active thru 2026       - 10-digit social insurance no      - Subledger tracking by     |
|    - Automated cutoff 2026-12-31   - Statutory rate 10.5% / 21.5%      - Counterparty, Item, WH    |
+----------------------------------------------------------------------------------------------------+
```

### 2.1 Benchmark Comparison with Tier-1 Vietnamese Accounting Systems

| Capability / Invariant | FinGo Phase 3 Specification | MISA SME / AMIS 2026 | FAST Business Online | Bravo ERP 8R3 |
| :--- | :--- | :--- | :--- | :--- |
| **UOM Conversion Engine** | Multi-tier directed graph with Banker's Rounding (`RoundBank`) | 2-tier (Base + Auxiliary) with half-up rounding | Multi-tier with item-specific conversion rates | Dynamic multi-tier with packaging unit matrix |
| **Tax Code Checksum** | Real-time Circular 105 Modulo-11 check digit verification | Online lookup via MISA meInvoice sync | Real-time validation + TCT status sync | Custom API bridge to General Dept of Taxation |
| **Non-Cash Payment Safeguard** | Hard barrier on purchases $\ge 5,000,000$ VND lacking vendor bank details | Warning prompt on voucher save | Hard lock configurable in system parameters | Approval workflow trigger to Chief Accountant |
| **COA Seam Enforcement** | Hard reject on parent or inactive account assignment | Warning dialog with auto-split to leaf | Disallows parent account assignment | Configurable leaf-only posting validation |
| **PDPL PII Masking** | Dynamic mask (`001******345`) for non-authorized roles | Masked on UI; unmasked via audit authorization | View rights restricted by user profile | Column-level encryption and masking |
| **Credit Limit Barrier** | Hard check against ledger subledger balance (TK 131) | Warning or lock on invoice issuance | Hard lock on delivery order generation | Multi-level credit authorization hierarchy |

---

# 3. High-Level Architecture & Entity Topology

```
+----------------------------------------------------------------------------------------------------+
|                                    ACCOUNTING SPINE (LAYER 1)                                      |
|   +----------------------------+  +----------------------------+  +----------------------------+   |
|   |         Currencies         |  |      Chart of Accounts     |  |       Fiscal Periods       |   |
|   |   VND (Base) / USD / EUR   |  |   TK 111, 112, 131, 331... |  |   Lock Date / Status Gate  |   |
|   +--------------+-------------+  +--------------+-------------+  +--------------+-------------+   |
+------------------|-------------------------------|-------------------------------|-----------------+
                   |                               |                               |
                   |                               | Leaf Account Invariant Check  |
                   v                               v                               v
+----------------------------------------------------------------------------------------------------+
|                                    CATALOGS MASTER DATA (LAYER 2)                                  |
|                                                                                                    |
|   +--------------------------+                         +--------------------------+                |
|   |      UnitOfMeasure       |                         |        Warehouse         |                |
|   |  - Base UOM (Kg, Lon)    |                         |  - Default Account:      |                |
|   |  - Conversions (1T=24L)  |                         |    152, 153, 155, 156    |                |
|   +------------+-------------+                         +------------+-------------+                |
|                |                                                    |                              |
|                +-----------------------+  +-------------------------+                              |
|                                        v  v                                                        |
|                         +-------------------------------+                                          |
|                         |         Item / Product        |                                          |
|                         |  - Type: Material/Tool/FG/Good|                                          |
|                         |  - Default: 1561, 632, 5111   |                                          |
|                         |  - VAT: 0%, 5%, 8%, 10%       |                                          |
|                         +-------------------------------+                                          |
|                                                                                                    |
|   +--------------------------+                         +--------------------------+                |
|   |         Customer         |                         |    Vendor / Supplier     |                |
|   |  - Modulo-11 MST         |                         |  - Modulo-11 MST         |                |
|   |  - Default AR: TK 131    |                         |  - Default AP: TK 331    |                |
|   |  - Credit Limit Barrier  |                         |  - Bank Account (>= 5M)  |                |
|   +--------------------------+                         +--------------------------+                |
|                                                                                                    |
|   +--------------------------+                         +--------------------------+                |
|   |       BankAccount        |                         |         Employee         |                |
|   |  - Currency Alignment:   |                         |  - 12-digit CCCD (PDPL)  |                |
|   |    VND -> 1121           |                         |  - 10-digit BHXH         |                |
|   |    USD -> 1122           |                         |  - Advances: TK 141      |                |
|   |                          |                         |  - Payroll: TK 334       |                |
|   +--------------------------+                         +--------------------------+                |
+----------------------------------------------------------------------------------------------------+
                                                   |
                                                   v
+----------------------------------------------------------------------------------------------------+
|                                   TRANSACTIONAL ENGINE (LAYER 3 & 4)                               |
|       General Journal (GL) | Cash & Bank (111/112) | Sales & AR (131) | Purchases & AP (331)       |
+----------------------------------------------------------------------------------------------------+
```

---

# 4. Detailed Entity Specifications & Business Rules

### 4.1 Unit of Measure & Conversion Graph Engine (`INV-CAT-03`, `INV-CAT-04`)
- **Base Unit Requirement**: Every item has strictly one base unit ($Q_{\text{base}}$).
- **Directed Conversion Graph**:
```
    [ Pallet ]
        |
        | x 10 (MULTIPLY)
        v
    [ Thùng ]  <------------------------+
        |                               | / 24 (DIVIDE)
        | x 24 (MULTIPLY)               |
        v                               |
     [ Lon ] (Base UOM)  ---------------+
```
- **Banker's Rounding**:
  $$\text{ConvertedQty} = \begin{cases} (Q \times \text{Multiplier}).\text{RoundBank}(N) & \text{if } \text{Type} = \text{MULTIPLY} \\ (Q \div \text{Multiplier}).\text{RoundBank}(N) & \text{if } \text{Type} = \text{DIVIDE} \end{cases}$$

### 4.2 Counterparty Tax Code & Modulo-11 Checksum (`INV-CAT-02`)
- Standard 10-digit MST: $D_1 D_2 D_3 D_4 D_5 D_6 D_7 D_8 D_9 D_{10}$
- Circular 105 Weights: $W = [31, 29, 23, 19, 17, 13, 7, 5, 3]$
- Formula:
  $$\text{Sum} = \sum_{i=1}^{9} D_i \times W_i$$
  $$\text{Rem} = \text{Sum} \pmod{11}$$
  $$D_{10} = \begin{cases} 0 & \text{if } (10 - \text{Rem}) = 10 \\ 10 - \text{Rem} & \text{otherwise} \end{cases}$$
- 13-digit branch MST: Characters 1-10 must satisfy the above formula; characters 11-13 must range between `001` and `999`.

### 4.3 Decree 181/2025/NĐ-CP Non-Cash Rule Enforcement
```
   Transaction Voucher >= 5,000,000 VND
                 |
                 v
     Is Vendor Bank Registered?
        /              \
      Yes               No
      /                  \
    [ALLOW]         [HARD REJECT]
                    ErrNonCashBankDetailsRequired
                    (Statutory VAT Input & CIT Penalty)
```

### 4.4 Customer Credit Limit Barrier (`INV-CAT-05`)
- When `enforce_credit_limit = true`:
  $$\text{Current Outstanding AR (TK 131)} + \text{Voucher Amount} \le \text{Credit Limit}$$
- Exceeding limit blocks voucher posting unless overridden by Chief Accountant (`PERMISSION_OVERRIDE_CREDIT_LIMIT`).

---

# 5. Comprehensive Use Case Catalog

```
+----------------------------------------------------------------------------------------------------+
|                                      USE CASE TAXONOMY MATRIX                                      |
|                                                                                                    |
|   UC-CAT-01: Manage Customer Master Data & Credit Risk Barrier                                     |
|   UC-CAT-02: Manage Vendor Profile & Non-Cash Statutory Compliance (Decree 181)                    |
|   UC-CAT-03: Manage Item Master Data, Account Compatibility & Conversion Matrix                    |
|   UC-CAT-04: Manage Warehouse Location & Asset Account Compatibility (Group 15)                    |
|   UC-CAT-05: Manage Bank Account & Currency GL Alignment (TK 1121/1122)                            |
|   UC-CAT-06: Manage Employee Statutory Identity & PDPL PII Masking (CCCD/BHXH)                     |
+----------------------------------------------------------------------------------------------------+
```

### 5.1 UC-CAT-01: Register & Validate Customer Master Data

#### Actors
- Kế toán bán hàng (Sales Accountant)
- Kế toán trưởng (Chief Accountant - Approval & Override)

#### Preconditions
- Tenant company profile active.
- Chart of Accounts initialized; leaf sub-accounts for TK 131 exist.

#### Main Flow (Happy Path)
1. Accountant opens Customer Registration window (Shortcut: `F2`).
2. Enters Customer Code (e.g. `KH-ALPHA`), Legal Company Name, and 10-digit Tax Code (`0100109106`).
3. System runs Modulo-11 checksum in background $\to$ Tax code verified.
4. Accountant specifies Payment Term (30 days), Credit Limit (100,000,000 VND), and toggles `EnforceCreditLimit = true`.
5. System checks default AR account (`acc-1311`) $\to$ Verified as active leaf account.
6. Accountant clicks **Save** (`Ctrl+S`). System persists record and creates audit entry.

```
Accountant                   FinGo System                    MariaDB
    |                             |                             |
    |--- 1. Enter Form Data ----->|                             |
    |    (Code, MST, Limit, AR)   |                             |
    |                             |--- 2. Validate Modulo-11 -->|
    |                             |    (Tax Code Valid)         |
    |                             |--- 3. Check Leaf Account -->|
    |                             |    (1311 is active leaf)    |
    |                             |--- 4. Insert Customer ----->|
    |                             |<-- 5. Commit Transaction ---|
    |<-- 6. Display Success ------|                             |
```

#### Alternative Paths
- **A1 (Branch MST)**: User enters 13-digit code `0100109106-001`. System verifies 10-digit parent checksum and numeric suffix `001`. Record created.
- **A2 (Credit Limit Override)**: Transaction causes outstanding balance to reach 115,000,000 VND (> 100M limit). Chief Accountant enters override credential; voucher approved with tagged audit record.

#### Exception Paths
- **E1 (Invalid Tax Code)**: Modulo-11 check digit fails. System highlights field in red and rejects submission (`ErrInvalidTaxCode`).
- **E2 (Non-Leaf AR Account)**: User selects parent account `131`. System displays: `Lỗi: Tài khoản 131 là tài khoản tổng hợp. Vui lòng chọn tài khoản chi tiết (Lá)`.
- **E3 (Duplicate Customer Code)**: System rejects with `ErrDuplicateCode`.

---

### 5.2 UC-CAT-02: Register Vendor & Decree 181 Non-Cash Compliance

#### Flow Diagram
```
   [Start: Register Vendor]
              |
              v
     Input MST & Bank Info
              |
              v
    Is Tax Code Valid?  --------(No)--------> Show ErrInvalidTaxCode
              | (Yes)
              v
    Is AP Account Leaf? --------(No)--------> Show ErrPostingToParent
              | (Yes)
              v
     Is Bank Acc Present?
        /            \
     (Yes)           (No)
      /                \
   [Save Vendor]   [Flag Warning: Vendor restricted to cash vouchers < 5M VND]
```

#### Exception Paths
- **E1 (Missing Bank Account on $\ge 5M$ Payment)**: User attempts to create Purchase Voucher for 12,000,000 VND against vendor without registered bank details. System hard-blocks save: `ErrNonCashBankDetailsRequired: Vi phạm Nghị định 181/2025/NĐ-CP`.

---

### 5.3 UC-CAT-03: Register Item & Multi-Tier UOM Graph

#### Conversion Validation Flow
```
   User inputs Item: "BIA_SAIGON", Base UOM: "LON"
   Adds Conversion: 1 "THUNG" = 24 "LON" (MULTIPLY)
   Adds Conversion: 1 "PALLET" = 10 "THUNG" (MULTIPLY)
                       |
                       v
         System constructs UOM graph:
         - 1 Pallet  = 240 Lon
         - 1 Thùng   = 24 Lon
         - 1 Lon     = 1 Lon (Base)
                       |
                       v
     All quantities stored with exact decimal Banker's Rounding
```

---

# 6. User Journeys by Accounting Role

```
+----------------------------------------------------------------------------------------------------+
|                                      ROLE-BASED USER JOURNEYS                                      |
|                                                                                                    |
|   1. Kế toán bán hàng (AR):                                                                        |
|      Create Customer -> Verify MST -> Monitor Outstanding AR -> Enforce Credit Limit               |
|                                                                                                    |
|   2. Kế toán mua hàng (AP):                                                                        |
|      Create Vendor -> Verify MST & Bank Info -> Enforce Decree 181 Non-Cash Settlement              |
|                                                                                                    |
|   3. Thủ kho / Kế toán kho (Inventory):                                                            |
|      Setup Warehouse (15x) -> Define Item UOM conversion -> Execute Stock In/Out                    |
|                                                                                                    |
|   4. Kế toán tiền lương (Payroll):                                                                 |
|      Register Employee -> Mask CCCD/BHXH under PDPL -> Map Advance (141) & Salary (334)             |
|                                                                                                    |
|   5. Kế toán trưởng (Chief Accountant):                                                            |
|      Approve Credit Overrides -> Audit Master Data Logs -> Lock Catalog Changes at Period End      |
+----------------------------------------------------------------------------------------------------+
```

---

# 7. Desktop User Interface (Wails UI / Vue 3 Wireframes)

### 7.1 Customer Master Data Management Screen (`/catalogs/customers`)

```
+--------------------------------------------------------------------------------------------------+
|  FinGo Accounting - [Danh mục Khách hàng]                                              [-] [x]   |
+--------------------------------------------------------------------------------------------------+
|  [+ Thêm mới (F2)]  [Sửa (F3)]  [Xóa (F8)]  [Xuất Excel]  [Nhập Excel]      | Tìm kiếm: [________] |
+--------------------------------------------------------------------------------------------------+
| Mã KH    | Tên khách hàng                 | Mã số thuế   | Hạn nợ | Hạn mức nợ   | TK Công nợ | Trạng thái |
+----------+--------------------------------+--------------+--------+--------------+------------+------------+
| KH0001   | CÔNG TY CỔ PHẦN ALPHA VIỆT NAM | 0100109106   | 30     | 100,000,000  | 1311       | Đang dùng  |
| KH0002   | CÔNG TY TNHH PHÁT TRIỂN BETA   | 0100681592   | 45     | 250,000,000  | 1312       | Đang dùng  |
| KH0003   | DOANH NGHIỆP TƯ NHÂN GAMA      | 0109999888   | 15     | 50,000,000   | 1311       | Đang dùng  |
+--------------------------------------------------------------------------------------------------+
| [Chi tiết khách hàng: KH0001 - CÔNG TY CỔ PHẦN ALPHA VIỆT NAM]                                   |
| Địa chỉ: Tầng 5, Tòa nhà Keangnam, Mễ Trì, Nam Từ Liêm, Hà Nội                                    |
| Người liên hệ: Nguyễn Văn Tuấn - ĐT: 0912.345.678 - Email: tuan.nv@alpha.vn                      |
| Dư nợ hiện tại: 78,500,000 VND (Còn lại: 21,500,000 VND) [HẠN MỨC AN TOÀN]                       |
+--------------------------------------------------------------------------------------------------+
| F2: Thêm mới | F3: Chỉnh sửa | F8: Xóa | F12: Lưu | Esc: Đóng                     Tổng số: 3 bản ghi |
+--------------------------------------------------------------------------------------------------+
```

### 7.2 Customer Add/Edit Modal Dialog

```
+--------------------------------------------------------------------------------------------------+
|  Thêm mới Khách hàng                                                                   [?] [x]   |
+--------------------------------------------------------------------------------------------------+
|  Mã khách hàng (*):   [KH0004            ]     Loại: (*) Tổ chức  ( ) Cá nhân                     |
|  Tên khách hàng (*):  [CÔNG TY TNHH CÔNG NGHỆ THĂNG LONG                                       ] |
|  Mã số thuế (*):      [0100109106        ]  [Kiểm tra MST (TCT)]  (V) Hợp lệ (Cục Thuế Hà Nội)   |
|  Địa chỉ trụ sở:      [Số 12 phố Trần Phú, Quận Ba Đình, Hà Nội                                ] |
|                                                                                                  |
|  -- THÔNG TIN CÔNG NỢ & ĐIỀU KHOẢN THANH TOÁN -------------------------------------------------  |
|  Tài khoản công nợ (*): [1311 - Phải thu khách hàng VND        v]  [V] Tài khoản Lá              |
|  Thời hạn nợ (ngày):    [30   ]                                                                  |
|  Hạn mức nợ tối đa:     [150,000,000       ] VND                                                 |
|  [X] Chặn xuất hóa đơn / phiếu xuất khi vượt hạn mức nợ (INV-CAT-05)                             |
|                                                                                                  |
|  -- THÔNG TIN LIÊN HỆ -------------------------------------------------------------------------  |
|  Người liên hệ:       [Trần Thị Mai       ]    Chức danh: [Kế toán trưởng   ]                    |
|  Điện thoại:          [0988.112.233       ]    Email:     [mai.tt@thanglong.vn]                  |
+--------------------------------------------------------------------------------------------------+
|                                                      [  Lưu (Ctrl+S)  ]   [ Hủy bỏ (Esc) ]       |
+--------------------------------------------------------------------------------------------------+
```

### 7.3 Employee Master Data Screen with PDPL Masking

```
+--------------------------------------------------------------------------------------------------+
|  FinGo Accounting - [Danh mục Nhân viên & Tiền lương]                                  [-] [x]   |
+--------------------------------------------------------------------------------------------------+
|  Mã NV  | Họ và tên            | Phòng ban | Căn cước công dân (CCCD) | Mã số BHXH | Lương cơ bản|
+---------+----------------------+-----------+--------------------------+------------+-------------+
| NV001   | Nguyễn Văn Tuấn      | Kế toán   | 001******345             | 0123456789 | 15,000,000  |
| NV002   | Trần Thị Hương       | Kinh doanh| 036******890             | 0198765432 | 12,000,000  |
+--------------------------------------------------------------------------------------------------+
| [!] CẢNH BÁO BẢO MẬT (LUẬT 91/2025/QH15 - PDPL):                                                 |
| Dữ liệu CCCD được ẩn mặc định. Nhấp [Hiện CCCD] yêu cầu xác thực vân tay/mật khẩu & ghi log.     |
| [Hiện số CCCD đầy đủ (Cần quyền KTT)]                                                            |
+--------------------------------------------------------------------------------------------------+
```

---

# 8. Implementation Roadmap & Production Cutover Plan

```
+----------------------------------------------------------------------------------------------------+
|                                    PHASED EXECUTION ROADMAP                                        |
|                                                                                                    |
|  +---------------------------+    +---------------------------+    +---------------------------+   |
|  |   Phase 3 (COMPLETED)     |    |   Phase 4 (NEXT SLICE)    |    |   Phase 5 (TRANSACTIONS)  |   |
|  | - Domain Entities & Invars|    | - Layer 3 Opening Balance |    | - Layer 4 Core Vouchers   |   |
|  | - MariaDB Migration 00007 | -> |   * GL Account Balances   | -> |   * Cash / Bank (111/112) |   |
|  | - 22 SQLC Queries         |    |   * Counterparty AR/AP    |    |   * Purchases & AP (331)  |   |
|  | - Usecases & 100% Tests   |    |   * Inventory Balances    |    |   * Sales & AR (131)      |   |
|  +---------------------------+    +---------------------------+    +---------------------------+   |
|                                                 |                                                  |
|                                                 v                                                  |
|                                   +---------------------------+                                    |
|                                   |  Phase 6 (UI & DESKTOP)   |                                    |
|                                   | - Wails Desktop Bindings  |                                    |
|                                   | - Vue 3 + Naive UI Grids  |                                    |
|                                   | - Excel Batch Import Tool |                                    |
|                                   +---------------------------+                                    |
+----------------------------------------------------------------------------------------------------+
```

### Immediate Next Steps
1. **Sync Specification & Documentation**:
   - Save this BRD as `docs/BRD-06-CATALOG-MASTER-DATA.md`.
   - Update `docs/SPEC-06-CATALOG-MASTER-DATA.md` cross-references.
2. **Commitment & Verification**:
   - Run `codegraph sync`.
   - Commit and push to git repository.
3. **Execute Layer 3 (Phase 4)**:
   - Formulate specification for Opening Balances (*Số dư đầu kỳ* - Accounts, Customers, Vendors, Inventory).
