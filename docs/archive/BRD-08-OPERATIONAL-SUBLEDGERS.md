# FinGo Business Requirements Document (BRD)
## Module 08: Operational Subledgers (Transactions - Layer 4)
**Document ID**: `FINGO-BRD-08-OPERATIONAL-SUBLEDGERS`  
**Standard Authority**: `FINGO-QA-STRATEGY-2026-V1` / `VAS` / `Circular 99/2025/TT-BTC` / `Circular 133/2016/TT-BTC`  
**Classification**: Core Financial Engine / High-Assurance Accounting  
**Status**: DRAFT FOR PRODUCTION BENCHMARK  

---

## 1. Executive Summary & Production Readiness Verdict

### Production Readiness Assessment: **CANNOT OPERATE IN PRODUCTION ENV YET (NO)**

The current FinGo codebase successfully operates Layers 1, 2, and 3 (Spine, Catalogs, and Opening Balances). However, Layer 4 (Operational Subledgers) exists solely as rudimentary in-memory domain stubs (`internal/domain/{gl,cash,sales,purchase,inventory,asset,payroll}`). 

### Critical Blockers Precluding Production Operation:
1. **Zero Database Tables for Vouchers**: No MariaDB tables exist for vouchers, cash/bank records, invoices, warehouse tickets, asset schedules, or payroll runs.
2. **Missing Subledger-to-GL Double-Entry Auto-Journaling**: Operational vouchers must atomically generate balanced General Ledger entries ($\sum \text{Debit} \equiv \sum \text{Credit}$). A Sales Invoice must atomically record Revenue (511), Output VAT (33311), Customer AR (131), and COGS/Inventory (632/156) without balance drift.
3. **Statutory Non-Cash Payment Barrier (Decree 181/2025/NĐ-CP & Law 48/2024/QH15)**: Hard barrier missing for expenditures $\ge 5,000,000\text{ VND}$. Settling $\ge 5\text{M}$ bills via Cash Payment (Phiếu chi) without warning disqualifies VAT deductions (TK 133) and CIT deductibility.
4. **Resolution 204/2025/QH15 & Decree 174/2025/NĐ-CP VAT Engine**: Tax calculation engine missing for 8% VAT temporary rate (active 2025-07-01 to 2026-12-31), 10% standard rate, 5% reduced rate, 0% export, and KCT exempt lines.
5. **Circular 45/2013/TT-BTC Fixed Asset vs. CCDC Seam**: No allocation engine for Tools & Supplies (CCDC TK 153/242, $\le 36$ months) vs Fixed Assets (TSCĐ TK 211, $\ge 30\text{M}$).
6. **Statutory Payroll Engine (Social Insurance 2026)**: Statutory contribution rates (BHXH 17.5% + 8%, BHYT 3% + 1.5%, BHTN 1% + 1%, KPCĐ 2%) and progressive personal income tax (TNCN) withholding must be mathematically exact to the single Vietnamese Dong.
7. **Accounting Period Date Lock & Immutability**: Modifying or deleting posted vouchers before/on lock date is strictly illegal per Law on Accounting No. 88/2015/QH13. Physical deletions (`DELETE FROM`) are forbidden for posted vouchers.

---

## 2. Regulatory & Legal Framework (Vietnam Accounting Standards)

```
+--------------------------------------------------------------------------------------------------+
|                                    LEGAL & REGULATORY SPINE                                      |
+--------------------------------------------------------------------------------------------------+
| Law on Accounting No. 88/2015/QH13 : Principles of double-entry, immutability, audit trail       |
| Circular 99/2025/TT-BTC            : Mandatory enterprise accounting regime from 2026-01-01      |
| Circular 133/2016/TT-BTC           : Small and medium enterprise (SME) accounting regime         |
| Decree 123/2020/NĐ-CP & TT 32/2025 : Electronic invoices, mandatory XML schemas, invoice numbers |
| Decree 181/2025/NĐ-CP              : Non-cash payment enforcement >= 5,000,000 VND               |
| Resolution 204/2025/QH15           : 8% VAT rate active through 2026-12-31                       |
| Circular 45/2013/TT-BTC            : Fixed asset threshold (>= 30M VND) & straight-line depr     |
| Circular 111/2013/TT-BTC & Law PIT : Personal Income Tax withholding schedules                   |
| Law on Social Insurance 2024       : Statutory insurance contributions (10.5% EE / 21.5% ER)    |
+--------------------------------------------------------------------------------------------------+
```

---

## 3. Comparative Benchmark: FinGo vs. Market Leaders

| Feature Axis | MISA SME 2026 | Fast Accounting 11 | Bravo ERP 8.2 | FinGo Target (Layer 4) |
| :--- | :--- | :--- | :--- | :--- |
| **Architecture** | Client-Server / Hybrid Cloud | Desktop / Web Client | Three-Tier Windows/Web | Onion Modular Monolith (Go + Wails + MariaDB) |
| **Math Precision** | C# `decimal` (28-29 digits) | SQL Server `decimal(18,2)` | SQL Server `decimal(18,4)` | Pure `shopspring/decimal` (Zero-Float Policy) |
| **Voucher Numbering** | Strict monthly sequential | Prefixed sequence by branch | Multi-segment branch sequence | Atomic MariaDB sequence per voucher type/year |
| **Subledger to GL** | Automatic dual-posting | Auto or batch posting | Configurable workflow posting | Atomic transaction: Subledger + GL lines |
| **Non-Cash Barrier** | Warning dialog $\ge 20\text{M}$ (outdated) | Warning dialog $\ge 20\text{M}$ | Hard block or warning configurable | Strict $\ge 5\text{M}$ warning/gate (Decree 181/2025) |
| **VAT Rates** | 0%, 5%, 8%, 10%, KCT | 0%, 5%, 8%, 10%, KCT | 0%, 5%, 8%, 10%, KCT | Auto-detect 8% validity (expires 2026-12-31) |
| **Audit Retention** | Soft-delete with audit log | Soft-delete with audit log | Full row-level versioning | Immutable append-only audit log + soft delete |

---

## 4. System Architecture & Double-Entry Flow (ASCII Art)

```
                             +---------------------------------------+
                             |       BUSINESS EVENT / UI ACTION      |
                             +---------------------------------------+
                                                 |
         +-------------------+-------------------+-------------------+-------------------+
         |                   |                   |                   |                   |
         v                   v                   v                   v                   v
   [ 22. CASH ]        [ 23. BANK ]        [ 24. PURCHASE ]    [ 25. SALES ]      [ 26. INVENTORY ]
   Phiếu thu/chi       Báo có/báo nợ       Mua hàng / Trả lại  Bán hàng / Trả lại  Nhập kho / Xuất kho
   (TK 111)            (TK 112)            (TK 331, 156, 133)  (TK 131, 511, 3331) (TK 156, 632)
         |                   |                   |                   |                   |
         +-------------------+-------------------+-------------------+-------------------+
                                                 |
                                                 v
                             +---------------------------------------+
                             |   ATOMIC TRANSACTION MANAGER (REPO)   |
                             +---------------------------------------+
                                 |                               |
                                 v                               v
                     +-----------------------+       +-----------------------+
                     |  OPERATIONAL RECORD   |       |   GENERAL LEDGER (GL) |
                     | Subledger header/lines|       | Voucher + VoucherLines|
                     |  (Domain Specific)    |       | Sum(Dr) == Sum(Cr)    |
                     +-----------------------+       +-----------------------+
                                 |                               |
                                 +---------------+---------------+
                                                 |
                                                 v
                             +---------------------------------------+
                             |        MARIADB 12.3 PERSISTENCE       |
                             | RepeatableRead Tx + Strict Constraints|
                             +---------------------------------------+
```

---

## 5. Detailed Subledger Requirements & Double-Entry Matrix

### 21. General Voucher (`Voucher` & `VoucherLine` - Chứng từ nghiệp vụ khác)
- **Purpose**: General adjustments, accruals, period-end allocations, capital contributions, foreign exchange revaluations.
- **Accounting Template**: Circular 133 & Circular 99/2025 mẫu số 01-TT / Chứng từ chung.
- **Rules**:
  - Balanced lines: $\sum \text{DebitAmountVND} \equiv \sum \text{CreditAmountVND}$.
  - Must specify Cost Center (`CostCenterID`) if account is Class 6 (`642`, `641`, `627`, `154`).
  - Strict leaf-account posting check (`INV-SPINE-01`).

### 22. Cash Subledger (`CashReceipt` & `CashPayment` - Quỹ tiền mặt TK 111)
- **Voucher 01-TT (Phiếu thu)**:
  - Thu tiền bán hàng: Nợ 1111 / Có 131 (hoặc Có 5111, Có 33311).
  - Thu hoàn ứng nhân viên: Nợ 1111 / Có 141.
  - Rút tiền gửi ngân hàng về nhập quỹ: Nợ 1111 / Có 1121.
- **Voucher 02-TT (Phiếu chi)**:
  - Chi trả tiền nhà cung cấp: Nợ 331 / Có 1111.
  - Chi tạm ứng cho nhân viên: Nợ 141 / Có 1111.
  - Nộp tiền mặt vào tài khoản ngân hàng: Nợ 1121 / Có 1111.
  - **Decree 181/2025 Barrier**: If bill/invoice amount $\ge 5,000,000\text{ VND}$, require Chief Accountant override and flag "NON_CASH_WARNING: Payment $\ge 5\text{M}$ in cash forfeits input VAT deduction".

### 23. Bank Subledger (`BankTransaction` - Tiền gửi ngân hàng TK 112)
- **Báo Có (Bank Credit Advice / Giấy báo Có)**:
  - Khách hàng chuyển khoản thanh toán: Nợ 1121 (VND) hoặc 1122 (USD) / Có 131.
  - Lãi tiền gửi ngân hàng: Nợ 1121 / Có 515.
- **Báo Nợ / Ủy nhiệm chi (UNC / Bank Debit Advice)**:
  - Thanh toán tiền cho nhà cung cấp: Nợ 331 / Có 1121.
  - Nộp thuế vào ngân sách nhà nước: Nợ 33311, 3334, 3335 / Có 1121.
  - Phí dịch vụ ngân hàng: Nợ 642 / Có 1121.
- **Rule**: Currency of Bank Account must match transaction currency (VND $\to$ `1121`, USD $\to$ `1122`).

### 24. Purchase Subledger (`PurchaseInvoice` & `PurchaseReturn` - Mua hàng TK 331, 156, 133)
- **Chứng từ mua hàng nhập kho**:
  - Hàng hóa nhập kho: Nợ 1561 / Có 331 (hoặc Có 1111, 1121).
  - Thuế GTGT đầu vào được khấu trừ: Nợ 1331 / Có 331.
  - Chi phí mua hàng (vận chuyển, bốc xếp): Nợ 1562 / Có 331.
- **Chứng từ trả lại hàng cho nhà cung cấp**:
  - Giảm giá trị hàng mua xuất trả: Nợ 331 / Có 1561, Có 1331 (hoặc xuất hóa đơn trả hàng).
- **Rule**: Auto-generates `StockInward` (Phiếu nhập kho) linked to Purchase Invoice.

### 25. Sales Subledger (`SalesInvoice` & `SalesReturn` - Bán hàng TK 131, 511, 33311)
- **Chứng từ bán hàng kiêm hóa đơn điện tử**:
  - Ghi nhận doanh thu & công nợ:
    - Nợ 131 (hoặc 1111, 1121)
    - Có 5111 (doanh thu bán hàng hóa), 5112 (thành phẩm), 5113 (dịch vụ)
    - Có 33311 (thuế GTGT đầu ra)
  - Ghi nhận giá vốn xuất bán (auto-linked `StockOutward`):
    - Nợ 632 / Có 1561.
- **Chứng từ hàng bán trả lại (Sales Return)**:
  - Giảm doanh thu: Nợ 5212 (hoặc ghi giảm Nợ 5111 theo TT133), Nợ 33311 / Có 131.
  - Nhập lại kho hàng trả lại: Nợ 1561 / Có 632.
- **E-Invoice Validation**: Invoice Series (Ký hiệu `1C26T...`), Invoice Number (8 digits continuous), Tax Authority Code (Mã CQT).

### 26. Inventory Subledger (`StockInward` & `StockOutward` - Kho TK 152, 156, 632)
- **Phiếu nhập kho (01-VT)**: Nhập từ sản xuất (Nợ 155 / Có 154), nhập thừa kiểm kê (Nợ 156 / Có 711 hoặc 3381).
- **Phiếu xuất kho (02-VT)**: Xuất kho cho sản xuất (Nợ 621 / Có 152), xuất dùng nội bộ (Nợ 642 / Có 156), xuất hao hụt (Nợ 1381 / Có 156).
- **Costing Engine Integration**: `MovingWeightedAverage` computes unit cost at transaction time. Quantity cannot drive inventory balance below zero (`INV-INV-01`).

### 27. Asset & Tools Subledger (`FixedAsset` & `ToolAllocation` - TSCĐ & CCDC TK 211, 242)
- **Ghi tăng TSCĐ (Biên bản giao nhận 01-TSCĐ)**:
  - Mua sắm đưa vào sử dụng ngay: Nợ 211 / Có 331, 1121; Nợ 1332 / Có 331.
  - Ngưỡng ghi nhận TSCĐ: $\text{Nguyên giá} \ge 30,000,000\text{ VND}$ & thời gian sử dụng $> 1\text{ năm}$.
- **Xuất dùng CCDC phân bổ nhiều kỳ (CCDC TK 153/242)**:
  - Xuất kho CCDC: Nợ 242 / Có 1531.
  - Thời gian phân bổ tối đa: 36 tháng per Circular 45/2013/TT-BTC.
  - Trích khấu hao / phân bổ hàng tháng: Nợ 642, 627 / Có 2141 (TSCĐ) hoặc Có 242 (CCDC).

### 28. Payroll Subledger (`PayrollTable` & `Timesheet` - Tiền lương TK 334, 338, 642)
- **Bảng chấm công (01-LĐTL) & Bảng thanh toán lương (02-LĐTL)**.
- **Hạch toán chi phí lương**:
  - Nợ 6421 (Lương bộ phận bán hàng), Nợ 6422 (Lương bộ phận quản lý) / Có 334.
- **Trích các khoản bảo hiểm theo lương (Tỷ lệ chuẩn 2026)**:
  - Doanh nghiệp đóng (21.5%): Nợ 642 / Có 3383 (BHXH 17.5%), Có 3384 (BHYT 3.0%), Có 3386 (BHTN 1.0%).
  - Kinh phí công đoàn (2.0%): Nợ 642 / Có 3382 (2.0%).
  - Khấu trừ vào lương người lao động (10.5%): Nợ 334 / Có 3383 (8.0%), Có 3384 (1.5%), Có 3386 (1.0%).
  - Khấu trừ thuế TNCN: Nợ 334 / Có 3335.
- **Chi trả lương**: Nợ 334 / Có 1111 (tiền mặt) hoặc Có 1121 (chuyển khoản ngân hàng).

---

## 6. ASCII Wireframes & Desktop UI Layouts

### Cash Receipt Screen (Phiếu thu 01-TT)
```
+--------------------------------------------------------------------------------------------------+
| FinGo > Tiền mặt > Phiếu thu (01-TT)                                          [ESC: Đóng] [F9: Lưu]|
+--------------------------------------------------------------------------------------------------+
| Số chứng từ: [PT-2026-03-0001]  Ngày CT: [20/03/2026]  Ngày HT: [20/03/2026]  Loại: [Thu tiền KH v]|
| Đối tượng:   [KH001] CÔNG TY CỔ PHẦN CÔNG NGHỆ XANH               Người nộp: [Trần Văn A]         |
| Địa chỉ:     123 Lê Duẩn, Hoàn Kiếm, Hà Nội                       Lý do nộp: [Thu tiền HĐ 001]    |
| Nhân viên:   [NV005] Nguyễn Kế Toán                               Kèm theo:  [01] chứng từ gốc    |
+--------------------------------------------------------------------------------------------------+
| CHI TIẾT HẠCH TOÁN:                                                                              |
| #  | Diễn giải            | TK Nợ | TK Có | Số tiền (VND)  | ĐT Công nợ | Hóa đơn số | Hạn TT    |
|----+----------------------+-------+-------+----------------+------------+------------+-----------|
| 1  | Thu nợ HĐ GTGT 12345 | 1111  | 131   |  25,000,000    | KH001      | HD-12345   | 15/03/2026|
| 2  | Thu nợ HĐ GTGT 12346 | 1111  | 131   |  15,000,000    | KH001      | HD-12346   | 20/03/2026|
|    |                      |       |       |                |            |            |           |
+----+----------------------+-------+-------+----------------+------------+------------+-----------+
| TỔNG CỘNG:                                |  40,000,000    |                                     |
| Viết bằng chữ: Bốn mươi triệu đồng chẵn.                                                         |
+--------------------------------------------------------------------------------------------------+
| [F2: Thêm dòng] [F3: Xóa dòng] [F7: In phiếu thu] [F8: Hủy bỏ] [F9: Ghi sổ & Lưu]                |
+--------------------------------------------------------------------------------------------------+
```

### Sales Invoice & Auto-COGS Screen (Hóa đơn bán hàng kiêm phiếu xuất kho)
```
+--------------------------------------------------------------------------------------------------+
| FinGo > Bán hàng > Hóa đơn bán hàng kiêm xuất kho                             [F9: Lưu & Phát hành]|
+--------------------------------------------------------------------------------------------------+
| Số chứng từ: [BH-2026-03-0012]  Ngày CT: [20/03/2026]  Ký hiệu HĐ: [1C26TAA]  Số HĐ: [00000142]   |
| Khách hàng:  [KH002] TẬP ĐOÀN HOA SEN                  MST: [0301234567]                          |
| Địa chỉ:     9 Đại lộ Bình Dương, TP Dĩ An, Bình Dương Điều khoản TT: [Net 30] Hạn TT: [19/04/2026]|
| Kho xuất:    [KHO_TONG] Kho tổng FinGo                 HT thanh toán: [Chuyển khoản (UNC)]       |
+--------------------------------------------------------------------------------------------------+
| [TAB 1: Hàng hóa & Doanh thu]  [TAB 2: Giá vốn xuất kho]  [TAB 3: Thuế GTGT]                     |
|----+----------------------+-------+-----+---------+-----------+------+----------+----------------|
| #  | Mặt hàng             | ĐVT   | Kho | Số lượng| Đơn giá   | VAT% | Tiền VAT | Thành tiền VND |
|----+----------------------+-------+-----+---------+-----------+------+----------+----------------|
| 1  | Máy chủ Dell R750    | Bộ    | KHO1|    2.00 | 85,000,000| 10%  |17,000,000|    170,000,000 |
| 2  | Switch Cisco 24P     | Chiếc | KHO1|    5.00 | 12,000,000|  8%  | 4,800,000|     60,000,000 |
+----+----------------------+-------+-----+---------+-----------+------+----------+----------------+
| Tiền hàng: 230,000,000 VND | Thuế GTGT: 21,800,000 VND | Tổng thanh toán: 251,800,000 VND        |
| Hạch toán tự động:                                                                               |
| - Doanh thu: Nợ 131: 251,800,000 | Có 5111: 230,000,000 | Có 33311: 21,800,000                   |
| - Giá vốn:   Nợ 632: 185,000,000 | Có 1561: 185,000,000 (Tự động tính từ BQ di động)              |
+--------------------------------------------------------------------------------------------------+
```

---

## 7. User Journeys & Workflow Execution

### User Journey 1: Procurement to Payment (P2P)
1. **Purchase Invoice Entry**: Accountant creates `PurchaseInvoice` from vendor invoice. FinGo validates vendor MST, bank details (Decree 181), and leaf accounts.
2. **Auto Stock Inward**: FinGo automatically generates `StockInward` (Phiếu nhập kho 01-VT) with physical quantities.
3. **GL Posting**: Generates balanced GL voucher lines (Nợ 1561, Nợ 1331 / Có 331).
4. **Bank Payment (UNC)**: When bill is due, accountant creates Bank Payment (Báo Nợ). If $\ge 5\text{M}$, system ensures non-cash mode. Generates GL lines (Nợ 331 / Có 1121). Vendor subledger TK 331 clears automatically.

### User Journey 2: Order to Cash (O2C)
1. **Sales Invoice Entry**: Sales accountant enters lines with products, quantities, prices, and VAT rates (0%, 5%, 8%, 10%).
2. **Auto Stock Outward & Costing**: FinGo checks on-hand inventory balance. Calculates moving weighted average unit cost. Generates `StockOutward` (02-VT) (Nợ 632 / Có 1561).
3. **E-Invoice Issuance**: Generates XML according to Decree 123 schema, signs digitally, updates status to `E_INVOICE_ISSUED`.
4. **Customer Receipt**: Customer pays via bank transfer. Accountant registers Bank Receipt (Báo Có). Generates GL lines (Nợ 1121 / Có 131). Customer AR clears.

---

## 8. Defect Tolerances & Release Criteria

- **P1 Defects (Misstatement / Balance Drift)**: Zero tolerance ($0.00$). Any transaction where $\sum \text{Debit} \neq \sum \text{Credit}$ triggers immediate abort.
- **P2 Defects (Statutory Non-Compliance)**: Zero tolerance. Disallowing 8% VAT or missing Decree 181 $\ge 5\text{M}$ non-cash check blocks release.
- **Test Coverage Gates**:
  - `internal/domain`: Line coverage $\ge 95.0\%$.
  - `internal/usecase`: Line coverage $\ge 85.0\%$.
  - E2E Golden 7 Accounting Flows: $100.0\%$ pass rate.
