# SPEC-05: THE ACCOUNTING SPINE (LAYER 1)
### Normative Technical & Statutory Architecture Specification
**Document Version**: `2.0.0-PROD-READY`  
**Authority**: Chief Accountant & Lead Business Analyst Review Board  
**Standard Compliance**: 
- Luật Kế toán số 88/2015/QH13 (Điều 10, 12, 13, 50, 52)
- Thông tư 99/2025/TT-BTC (Chế độ kế toán Doanh nghiệp áp dụng từ 01/01/2026)
- Thông tư 133/2016/TT-BTC & Thông tư 200/2014/TT-BTC
- Chuẩn mực Kế toán Việt Nam VAS 10 & VAS 01
- Chuẩn mực Báo cáo Tài chính Quốc tế IAS 21 (The Effects of Changes in Foreign Exchange Rates)
- `FINGO-QA-STRATEGY-2026-V1` (Normative Zero-Float, Strict Double-Entry Equilibrium)

---

# 1. Executive Summary & Production Readiness Verdict

### Production Readiness Assessment: **FAILED (CANNOT OPERATE IN PROD AS-IS)**

| Component | Current State | Production Readiness | Blocker Defects / Missing Capabilities |
| :--- | :--- | :--- | :--- |
| **5. Currency & Exchange Rate** | Skeleton / Stub only | **0% (Unusable)** | No table schema, no SBV/VCB rate feed ingestion, no historical rate lookup, no period-end revaluation engine (TK 413 $\to$ 515/635). |
| **6. Chart of Accounts (COA)** | Hardcoded string IDs | **15% (Defective)** | No tree hierarchy, no parent-child rollup, no enforcement of posting to leaf accounts only, no hermaphroditic (131/331) balance separation. |
| **7. Accounting Period & Fiscal Year**| Absent | **0% (Unusable)** | No fiscal year calendar, no period date boundary enforcement (`LockDate`), no audit lock state machine (`OPEN` $\to$ `SOFT_LOCKED` $\to$ `HARD_LOCKED`). |
| **8. Cost Center & Expense Item** | Absent | **0% (Unusable)** | No allocation tags on GL lines, no mandatory expense item validation for accounts 641/642/154, no multi-dimensional P&L reporting. |

**Verdict**: The Accounting Spine is the structural foundation of FinGo. Commencing transaction posting (AR, AP, Inventory, Cash) without this spine will lead to catastrophic data corruption, unbalanceable general ledgers, and illegal tax submissions. This specification establishes the comprehensive blueprint for Layer 1.

---

# 2. Statutory Legal Framework & Accounting Invariants

```
                               STATUTORY LEGAL UMBRELLA
 ┌──────────────────────────────────────────────────────────────────────────────────┐
 │                        LUẬT KẾ TOÁN SỐ 88/2015/QH13                             │
 │  - Điều 10: Đơn vị tiền tệ kế toán là Đồng Việt Nam (VND).                      │
 │  - Điều 12: Kỳ kế toán gồm kỳ năm, quý, tháng (01/01 - 31/12 hoặc tùy chọn).     │
 │  - Điều 13: Nghiêm cấm để ngoài sổ sách, sửa chữa sổ sách đã khóa sổ.           │
 └───────────────────────┬──────────────────────────────────┬───────────────────────┘
                         │                                  │
                         ▼                                  ▼
 ┌──────────────────────────────────────┐ ┌─────────────────────────────────────────┐
 │   THÔNG TƯ 99/2025/TT-BTC & TT 200   │ │       VAS 10 & IAS 21 (NGOẠI TỆ)        │
 │ - Danh mục Hệ thống Tài khoản Chuẩn  │ │ - Tỷ giá giao dịch thực tế khi phát sinh│
 │ - Chỉ hạch toán vào TK Chi tiết (Lá) │ │ - Tỷ giá mua/bán ngân hàng thương mại   │
 │ - Tài khoản Lưỡng tính (131, 331, 333│ │ - Đánh giá lại số dư ngoại tệ cuối kỳ   │
 │   không được bù trừ số dư tổng hợp). │ │ - Hạch toán chênh lệch tỷ giá vào TK 413│
 └──────────────────────────────────────┘ └─────────────────────────────────────────┘
```

### 2.1 The Seven Fundamental Invariants of Layer 1
1. **INV-SPINE-01 (Leaf-Only Posting)**: Journal entries (`VoucherLine`) MUST strictly post to leaf accounts (`is_leaf == true`). Attempting to post to a parent account MUST immediately throw `ErrPostingToParentAccount`.
2. **INV-SPINE-02 (Hermaphroditic Dual Balance Separation)**: Accounts marked `HERMAPHRODITE` (e.g. `131`, `331`, `1388`, `3388`, `333`, `421`) CANNOT net debit and credit balances across distinct counterparties. Balance sheets MUST report aggregate Debits under Assets and aggregate Credits under Liabilities.
3. **INV-SPINE-03 (Parent Account Total Equilibrium)**: For any parent account $A_p$ with immediate children $C(A_p)$:
   $$\text{Balance}(A_p) \equiv \sum_{c \in C(A_p)} \text{Balance}(c)$$
4. **INV-SPINE-04 (Period Lock Barrier)**: Any mutation (insert, update, delete, post, unpost) of a voucher with `VoucherDate` $\le$ `Period.LockDate` MUST fail immediately with `ErrPeriodLocked`.
5. **INV-SPINE-05 (Zero-Float Monetary Invariant)**: All amounts in base currency (`VND`) and foreign currencies (`USD`, `EUR`, etc.) MUST use 128-bit fixed-point decimal (`shopspring/decimal`). Floats (`float32`, `float64`) are strictly prohibited in domain and database layers.
6. **INV-SPINE-06 (Exchange Rate Precision)**: Exchange rates MUST support up to 6 decimal places (e.g. `25,450.123456 VND/USD`). Converted amount calculation MUST conform to:
   $$\text{AmountVND} = \text{Round}\left(\text{AmountFC} \times \text{ExchangeRate}, \text{CurrencyDecimals}\right)$$
7. **INV-SPINE-07 (Mandatory Expense Allocation)**: Every posting to nominal expense accounts (Accounts `154`, `621`, `622`, `627`, `641`, `642`, `635`, `811`) MUST require a valid `ExpenseItemID`. If `Account.requires_cost_center == true`, a valid `CostCenterID` is also mandatory.

---

# 3. Domain Model Architecture (ASCII Object Graphs)

### 3.1 Domain Relationship Diagram

```
 ┌──────────────────────────┐
 │      CompanyProfile      │
 └─────────────┬────────────┘
               │ 1
               │
               ├──────────────────────┬────────────────────────┬──────────────────────┐
               │ 1..*                 │ 1..*                   │ 1..*                 │ 1..*
               ▼                      ▼                        ▼                      ▼
     ┌──────────────────┐   ┌──────────────────┐    ┌──────────────────┐   ┌──────────────────┐
     │     Currency     │   │     Account      │    │    FiscalYear    │   │    CostCenter    │
     │ - code (USD,VND) │   │ - code (1111)    │    │ - year (2026)    │   │ - code (KD_MB)   │
     │ - is_base        │   │ - nature (DEBIT) │    │ - status (OPEN)  │   │ - parent_id      │
     │ - symbol ($)     │   │ - is_leaf        │    └─────────┬────────┘   └──────────────────┘
     └─────────┬────────┘   │ - parent_id      │              │ 1                     ▲
               │ 1          └──────────────────┘              │                       │
               │                      ▲                       │ 1..12                 │
               │ 1..*                 │                       ▼                       │
     ┌─────────┴────────┐             │             ┌──────────────────┐              │
     │   ExchangeRate   │             │             │ AccountingPeriod │              │
     │ - rate_date      │             │             │ - period_num (1) │              │
     │ - rate_type      │             │             │ - lock_date      │              │
     │ - rate (25450)   │             │             │ - is_closed      │              │
     └──────────────────┘             │             └──────────────────┘              │
                                      │                                               │
                                      │                                               │
                                      └───────────────────────┐                       │
                                                              │                       │
                                                      ┌───────┴──────────┐   ┌────────┴─────────┐
                                                      │   VoucherLine    ├───┤   ExpenseItem    │
                                                      │ - debit_acc_id   │   │ - code (CHI_PHI) │
                                                      │ - credit_acc_id  │   │ - category       │
                                                      │ - amount_fc/vnd  │   └──────────────────┘
                                                      │ - cost_center_id │
                                                      │ - expense_item_id│
                                                      └──────────────────┘
```

---

# 4. Detailed Module Specifications

---

## Module 5: Currency & Exchange Rate (`Tiền tệ & Tỷ giá`)

### 5.1 Business Rules & Statutory Requirements
- **BR-CURR-01 (Base Currency Invariant)**: Exactly ONE currency per company profile MUST have `is_base = true` (default `VND`). The base currency exchange rate is immutable and fixed to `1.000000`.
- **BR-CURR-02 (Rate Tri-Types)**: The system supports three standard operational rate types:
  1. `BUY_TRANSFER` (Tỷ giá mua chuyển khoản): Used when customers pay in foreign currency or bank converts FC $\to$ VND.
  2. `SELL_TRANSFER` (Tỷ giá bán chuyển khoản): Used when settling foreign vendor liabilities or bank converts VND $\to$ FC.
  3. `CENTRAL_SBV` (Tỷ giá trung tâm NHNN): Used for customs duty declarations and official foreign trade statistics.
- **BR-CURR-03 (Period-End Revaluation Workflow - VAS 10)**:
  At the end of each fiscal month/year, the system revalues foreign currency bank balances (`1122`), cash (`1112`), and outstanding foreign receivables/payables (`131`, `331`):
  - $\Delta > 0$ (Lãi tỷ giá chưa thực hiện): Nợ 1112/1122/131 / Có 4131.
  - $\Delta < 0$ (Lỗ tỷ giá chưa thực hiện): Nợ 4131 / Có 1112/1122/331.
  - At year-end closing: Kết chuyển số dư TK 4131 vào TK 515 (Doanh thu tài chính) hoặc TK 635 (Chi phí tài chính).

### 5.2 Exchange Rate Ingestion & Resolution Flow

```
                      VOUCHER CREATION INITIATED (e.g. Sales Invoice in USD)
                                                 │
                                                 ▼
                             ┌───────────────────────────────────────┐
                             │ Check User Specified Override Rate?   │
                             └───────────────────┬───────────────────┘
                                                 │
                                       Yes ──────┴────── No
                                        │                 │
                                        ▼                 ▼
                             ┌────────────────┐ ┌───────────────────────────────────────┐
                             │ Use Explicit   │ │ Fetch Stored Exchange Rate for Date D │
                             │ Voucher Rate   │ └───────────────────┬───────────────────┘
                             └────────────────┘                     │
                                                         Found? ────┴──── Not Found?
                                                           │                  │
                                                           ▼                  ▼
                                                 ┌────────────────┐ ┌───────────────────┐
                                                 │ Apply Stored   │ │ Query Nearest Past│
                                                 │ Daily Rate     │ │ Calendar Rate     │
                                                 └────────────────┘ └─────────┬─────────┘
                                                                              │ Found?
                                                                    Yes ──────┴────── No
                                                                     │                 │
                                                                     ▼                 ▼
                                                           ┌────────────────┐ ┌─────────┴───────┐
                                                           │ Emit Warning:  │ │ Hard Reject:    │
                                                           │ "Using past FX │ │ ErrMissingRate  │
                                                           │  rate from T-n"│ └─────────────────┘
                                                           └────────────────┘
```

---

## Module 6: Chart of Accounts (COA - `Hệ thống tài khoản`)

### 6.1 Statutory COA Structure in Vietnam (Circular 99/2025 vs Circular 133 vs Circular 200)

| Account Group | Class Name | Normal Nature | P&L / Balance Sheet Mapping |
| :--- | :--- | :--- | :--- |
| **Loại 1** | Tài sản ngắn hạn (Current Assets) | `DEBIT` | Tài sản ngắn hạn (Bảng Cân đối kế toán) |
| **Loại 2** | Tài sản dài hạn (Non-Current Assets) | `DEBIT` | Tài sản dài hạn (Bảng CĐKT) - Ngoại trừ TK 214, 229 (`CREDIT`) |
| **Loại 3** | Nợ phải trả (Liabilities) | `CREDIT` | Nợ phải trả (Bảng CĐKT) - Ngoại trừ TK 333, 331 (`HERMAPHRODITE`) |
| **Loại 4** | Vốn chủ sở hữu (Equity) | `CREDIT` | Vốn chủ sở hữu (Bảng CĐKT) - Ngoại trừ TK 419, 421 (`HERMAPHRODITE`) |
| **Loại 5** | Doanh thu (Revenue) | `NO_BALANCE` | Báo cáo Kết quả Hoạt động SXKD (P&L) - Kết chuyển về TK 911 |
| **Loại 6** | Chi phí sản xuất, kinh doanh (Expenses) | `NO_BALANCE` | Báo cáo KQKD (P&L) - Kết chuyển về TK 911 / TK 154 |
| **Loại 7** | Thu nhập khác (Other Income) | `NO_BALANCE` | Báo cáo KQKD (P&L) - Kết chuyển về TK 911 |
| **Loại 8** | Chi phí khác (Other Expenses) | `NO_BALANCE` | Báo cáo KQKD (P&L) - Kết chuyển về TK 911 |
| **Loại 9** | Xác định kết quả kinh doanh (Summary) | `NO_BALANCE` | Trung gian kết chuyển Lãi/Lỗ ròng cuối kỳ |

### 6.2 Dual / Hermaphroditic Balance Resolution (Accounts 131, 331, 338, 138)

```
                              RAW TRANSACTIONS POSTED TO ACCOUNT 131
                                                 │
                                                 ▼
             ┌───────────────────────────────────────────────────────────────────────┐
             │ Group Sub-Ledger Balances by Counterparty (Customer Profile ID)      │
             └───────────────────────────────────┬───────────────────────────────────┘
                                                 │
                                                 ▼
             ┌───────────────────────────────────┴───────────────────────────────────┐
             │                                                                       │
             ▼                                                                       ▼
 ┌───────────────────────────────────────┐               ┌───────────────────────────────────────┐
 │ Counterparty Balances > 0 (Debit Sum) │               │ Counterparty Balances < 0 (Credit Sum)│
 │ Customer A: +100,000,000 VND          │               │ Customer B: -30,000,000 VND (Advance) │
 └───────────────────┬───────────────────┘               └───────────────────┬───────────────────┘
                     │                                                       │
                     ▼                                                       ▼
 ┌───────────────────────────────────────┐               ┌───────────────────────────────────────┐
 │ PRESENT ON BALANCE SHEET ASSET:       │               │ PRESENT ON BALANCE SHEET LIABILITY:   │
 │ "Mã số 131 - Phải thu ngắn hạn KH"   │               │ "Mã số 312 - Người mua trả tiền trước"│
 │ Value: 100,000,000 VND                │               │ Value: 30,000,000 VND                 │
 └───────────────────────────────────────┘               └───────────────────────────────────────┘
                     │                                                       │
                     └───────────────────────────┬───────────────────────────┘
                                                 │
                                                 ▼
                               ┌───────────────────────────────────┐
                               │  STRICTLY FORBIDDEN AUDIT ACTION: │
                               │  Netting: 100M - 30M = 70M Asset  │
                               │  (Violation of Circular 99 & VSA) │
                               └───────────────────────────────────┘
```

---

## Module 7: Accounting Period & Fiscal Year (`Kỳ kế toán & Năm tài chính`)

### 7.1 Period Lifecycle & State Machine

```
              ┌────────────────────────────────────────────────────────┐
              │                     FISCAL YEAR                        │
              │   [01/01/2026 ----------------------------- 31/12/2026] │
              └───────────────────────────┬────────────────────────────┘
                                          │ contains 12 monthly periods
                                          ▼
     ┌──────────────┐          ┌──────────────┐          ┌──────────────┐
     │  Period M01  │ ───► ... │  Period M12  │ ───►     │ Period 13/ADJ│ (Year-End Audit Adj)
     └──────┬───────┘          └──────────────┘          └──────────────┘
            │
            ▼
 ┌───────────────────────────────────────────────────────────────────────────────────┐
 │                            PERIOD STATE TRANSITION                                │
 │                                                                                   │
 │     ┌────────────┐    Chief Accountant sets LockDate    ┌─────────────┐           │
 │     │    OPEN    │ ────────────────────────────────────►│ SOFT_LOCKED │           │
 │     └─────┬──────┘                                      └──────┬──────┘           │
 │           │                                                    │                  │
 │           │ Period Closed for Tax Filing                       │ Audit Finalized  │
 │           ▼                                                    ▼                  │
 │     ┌────────────┐                                      ┌─────────────┐           │
 │     │HARD_LOCKED │ ◄────────────────────────────────────│   CLOSED    │           │
 │     └─────┬──────┘                                      └─────────────┘           │
 │           │                                                                       │
 │           │ Director + Chief Accountant Dual Digital Signature Approval           │
 │           ▼                                                                       │
 │     ┌────────────┐                                                                │
 │     │ RE_OPENED  │ (Emits Statutory Audit Log Alert to VACPA Inspection Log)      │
 │     └────────────┘                                                                │
 └───────────────────────────────────────────────────────────────────────────────────┘
```

### 7.2 Lock Date Transaction Interception Rules
- **Rule L-01 (Strict Rejection)**: When `VoucherDate <= Period.LockDate`, any HTTP/Wails command attempting `CreateVoucher`, `UpdateVoucher`, `DeleteVoucher`, `PostVoucher`, or `UnpostVoucher` MUST be rejected with `ErrPeriodLocked`:
  ```json
  {
    "error_code": "ERR_PERIOD_LOCKED",
    "message": "Không thể hạch toán chứng từ vào kỳ kế toán đã khóa sổ (Ngày chứng từ: 2026-01-15 <= Ngày khóa sổ: 2026-01-31)",
    "lock_date": "2026-01-31",
    "voucher_date": "2026-01-15"
  }
  ```

---

## Module 8: Cost Center & Expense Item (`Trung tâm & Khoản mục chi phí`)

### 8.1 Multi-Dimensional Sub-Ledger Structure
Financial vouchers in FinGo record not only double-entry accounts, but full managerial cost dimensions:

```
  VOUCHER LINE (BÚT TOÁN CHI TIẾT)
  ├── Debit Account  : 6422 (Chi phí quản lý doanh nghiệp - Chi phí vật liệu)
  ├── Credit Account : 1111 (Tiền mặt Việt Nam)
  ├── Amount         : 12,500,000 VND
  ├── Expense Item   : KMCP_VPP (Khoản mục chi phí: Văn phòng phẩm, giấy in)
  ├── Cost Center    : TTP_KD_HCM (Trung tâm chi phí: Phòng Kinh doanh Miền Nam)
  └── Branch / Tenant: CN_HCM (Chi nhánh TP. Hồ Chí Minh)
```

### 8.2 Expense Item Catalog (Authoritative Default Categories)
1. `CHIPHI_NHANCONG` (Chi phí nhân công, tiền lương, BHXH 10.5%/21.5%).
2. `CHIPHI_KHAUHAO` (Chi phí khấu hao TSCĐ & phân bổ CCDC 242).
3. `CHIPHI_NGUYENVATLIEU` (Chi phí nguyên vật liệu trực tiếp, phụ tùng).
4. `CHIPHI_DICHVUMUANGOAI` (Điện, nước, viễn thông, thuê văn phòng, vận chuyển).
5. `CHIPHI_TIETKHACH` (Tiếp khách, hội nghị, công tác phí, quảng cáo).
6. `CHIPHI_TAICHINH` (Lãi vay ngân hàng, phí chuyển tiền, lỗ tỷ giá).
7. `CHIPHI_THUE_PHI` (Thuế môn bài, thuế nhà đất, lệ phí hải quan).

---

# 5. MariaDB 12.3 Relational Schema Blueprint

```sql
-- Migration: 00006_create_accounting_spine.sql
-- Modules: Currency, ExchangeRate, Account, AccountingPeriod, FiscalYear, CostCenter, ExpenseItem

USE fingo;

-- 1. Currencies Table
CREATE TABLE IF NOT EXISTS currencies (
    code VARCHAR(3) PRIMARY KEY, -- ISO 4217: VND, USD, EUR, JPY
    company_profile_id VARCHAR(36) NOT NULL,
    name VARCHAR(100) NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    decimal_places INT NOT NULL DEFAULT 2,
    is_base BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_curr_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    UNIQUE KEY uk_company_base_currency (company_profile_id, is_base, code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 2. Daily Exchange Rates Table
CREATE TABLE IF NOT EXISTS exchange_rates (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    currency_code VARCHAR(3) NOT NULL,
    rate_date DATE NOT NULL,
    rate_type ENUM('BUY_TRANSFER', 'SELL_TRANSFER', 'CENTRAL_SBV') NOT NULL DEFAULT 'BUY_TRANSFER',
    rate DECIMAL(18, 6) NOT NULL,
    source_bank VARCHAR(100) NOT NULL DEFAULT 'VIETCOMBANK',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR(36) NOT NULL,
    CONSTRAINT fk_fx_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    CONSTRAINT fk_fx_currency FOREIGN KEY (currency_code) REFERENCES currencies(code) ON DELETE RESTRICT,
    UNIQUE KEY uk_currency_date_type (company_profile_id, currency_code, rate_date, rate_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 3. Chart of Accounts (COA) Table
CREATE TABLE IF NOT EXISTS accounts (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    code VARCHAR(32) NOT NULL, -- e.g. 111, 1111, 1112
    name VARCHAR(255) NOT NULL,
    english_name VARCHAR(255) NULL,
    parent_id VARCHAR(36) NULL,
    account_level INT NOT NULL DEFAULT 1, -- 1: 3-digit, 2: 4-digit, 3: 5-digit
    nature ENUM('DEBIT', 'CREDIT', 'HERMAPHRODITE', 'NO_BALANCE') NOT NULL,
    category ENUM('ASSET', 'LIABILITY', 'EQUITY', 'REVENUE', 'EXPENSE', 'OTHER_INCOME', 'OTHER_EXPENSE', 'SUMMARY') NOT NULL,
    is_leaf BOOLEAN NOT NULL DEFAULT TRUE,
    is_foreign_currency BOOLEAN NOT NULL DEFAULT FALSE,
    requires_partner BOOLEAN NOT NULL DEFAULT FALSE, -- Bắt buộc theo dõi theo Khách hàng/NCC (131, 331)
    requires_bank_account BOOLEAN NOT NULL DEFAULT FALSE, -- Bắt buộc theo dõi theo Tài khoản NH (112)
    requires_cost_center BOOLEAN NOT NULL DEFAULT FALSE, -- Bắt buộc theo dõi Trung tâm chi phí
    requires_expense_item BOOLEAN NOT NULL DEFAULT FALSE, -- Bắt buộc theo dõi Khoản mục chi phí
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_acc_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    CONSTRAINT fk_acc_parent FOREIGN KEY (parent_id) REFERENCES accounts(id) ON DELETE RESTRICT,
    UNIQUE KEY uk_company_account_code (company_profile_id, code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 4. Fiscal Years Table
CREATE TABLE IF NOT EXISTS fiscal_years (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    year INT NOT NULL, -- e.g. 2026
    start_date DATE NOT NULL, -- typically 2026-01-01
    end_date DATE NOT NULL,   -- typically 2026-12-31
    status ENUM('OPEN', 'SOFT_LOCKED', 'HARD_LOCKED', 'CLOSED') NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_fy_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    UNIQUE KEY uk_company_fiscal_year (company_profile_id, year)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 5. Accounting Periods Table (12 Monthly Periods + Period 13)
CREATE TABLE IF NOT EXISTS accounting_periods (
    id VARCHAR(36) PRIMARY KEY,
    fiscal_year_id VARCHAR(36) NOT NULL,
    company_profile_id VARCHAR(36) NOT NULL,
    period_number INT NOT NULL, -- 1 to 12, 13 (Adjustment)
    name VARCHAR(50) NOT NULL,  -- "Tháng 01/2026"
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    lock_date DATE NOT NULL,    -- Transactions <= lock_date are rejected
    status ENUM('OPEN', 'SOFT_LOCKED', 'HARD_LOCKED', 'AUDITED') NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_ap_fy FOREIGN KEY (fiscal_year_id) REFERENCES fiscal_years(id) ON DELETE RESTRICT,
    CONSTRAINT fk_ap_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    UNIQUE KEY uk_company_fy_period (company_profile_id, fiscal_year_id, period_number)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 6. Cost Centers Table (Trung tâm chi phí / Phòng ban)
CREATE TABLE IF NOT EXISTS cost_centers (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    branch_id VARCHAR(36) NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    parent_id VARCHAR(36) NULL,
    is_leaf BOOLEAN NOT NULL DEFAULT TRUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_cc_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    CONSTRAINT fk_cc_branch FOREIGN KEY (branch_id) REFERENCES branch_org_units(id) ON DELETE SET NULL,
    CONSTRAINT fk_cc_parent FOREIGN KEY (parent_id) REFERENCES cost_centers(id) ON DELETE RESTRICT,
    UNIQUE KEY uk_company_cost_center_code (company_profile_id, code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 7. Expense Items Table (Khoản mục chi phí)
CREATE TABLE IF NOT EXISTS expense_items (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(64) NOT NULL, -- LABOR, MATERIAL, DEPRECIATION, OUTSOURCED, OTHER
    parent_id VARCHAR(36) NULL,
    is_leaf BOOLEAN NOT NULL DEFAULT TRUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_ei_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT,
    CONSTRAINT fk_ei_parent FOREIGN KEY (parent_id) REFERENCES expense_items(id) ON DELETE RESTRICT,
    UNIQUE KEY uk_company_expense_item_code (company_profile_id, code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

---

# 6. UI/XI ASCII Wireframes

### 6.1 Desktop UI: Chart of Accounts (COA) Hierarchy Manager

```
╔══════════════════════════════════════════════════════════════════════════════════════════════╗
║  FinGo SME - HỆ THỐNG TÀI KHOẢN KẾ TOÁN (CIRCULAR 99/2025/TT-BTC)           [−] [口] [X]      ║
╠══════════════════════════════════════════════════════════════════════════════════════════════╣
║ [Tìm kiếm tài khoản...  🔍] [Lọc loại TK: Tất cả ▼]  [+] Thêm tài khoản  [⚙] Nhập khẩu Excel   ║
╠══════════════════════════════════════════════════════════════════════════════════════════════╣
║ Mã TK   │ Tên Tài khoản                    │ Tính chất   │ Loại TK    │ Dư Nợ       │ Dư Có   ║
╠═════════╪══════════════════════════════════╪═════════════╪════════════╪═════════════╪═════════╣
║ ▼ 111   │ Tiền mặt                         │ Dư Nợ       │ Tài sản    │ 125,400,000 │       0 ║
║   1111  │   Tiền Việt Nam (VND)            │ Dư Nợ (Lá)  │ Tài sản    │  85,400,000 │       0 ║
║   1112  │   Ngoại tệ (USD, EUR)            │ Dư Nợ (Lá)  │ Tài sản    │  40,000,000 │       0 ║
║ ► 112   │ Tiền gửi ngân hàng               │ Dư Nợ       │ Tài sản    │ 840,000,000 │       0 ║
║   131   │ Phải thu của khách hàng          │ Lưỡng tính  │ Công nợ    │ 450,000,000 │35,000,00║
║ ► 156   │ Hàng hóa                         │ Dư Nợ       │ Tồn kho    │ 620,000,000 │       0 ║
║   211   │ Tài sản cố định hữu hình         │ Dư Nợ       │ TSCĐ       │1,500,000,000│       0 ║
║   214   │ Hao mòn tài sản cố định          │ Dư Có (Lá)  │ Điều chỉnh │           0 │320,000,0║
║   331   │ Phải trả cho người bán           │ Lưỡng tính  │ Công nợ    │  15,000,000 │210,000,0║
║   421   │ Lợi nhuận sau thuế chưa PP       │ Lưỡng tính  │ Vốn CSH    │           0 │480,000,0║
║   511   │ Doanh thu bán hàng và CCDV       │ Không số dư │ Doanh thu  │           0 │       0 ║
║   642   │ Chi phí quản lý doanh nghiệp     │ Không số dư │ Chi phí    │           0 │       0 ║
║   911   │ Xác định kết quả kinh doanh      │ Không số dư │ Trung gian │           0 │       0 ║
╠══════════════════════════════════════════════════════════════════════════════════════════════╣
║ Đang chọn: 131 - Phải thu của khách hàng | Bắt buộc theo dõi: [X] Đối tượng  [ ] Khoản mục CP ║
║ [Sửa tài khoản]   [Thêm TK con cấp 2]   [Xem sổ cái tài khoản]   [In Bảng CĐ Tài khoản]       ║
╚══════════════════════════════════════════════════════════════════════════════════════════════╝
```

### 6.2 Desktop UI: Accounting Period & Lock Date Controller

```
╔══════════════════════════════════════════════════════════════════════════════════════════════╗
║  FinGo SME - QUẢN LÝ KỲ KẾ TOÁN & KHÓA SỔ DỮ LIỆU                           [−] [口] [X]      ║
╠══════════════════════════════════════════════════════════════════════════════════════════════╣
║ Năm tài chính: [ 2026 ▼ ]   Ngày bắt đầu: 01/01/2026   Ngày kết thúc: 31/12/2026             ║
╠══════════════════════════════════════════════════════════════════════════════════════════════╣
║ Kỳ kế toán │ Từ ngày    │ Đến ngày   │ Ngày khóa sổ │ Trạng thái  │ Thao tác                 ║
╠════════════╪════════════╪════════════╪══════════════╪═════════════╪══════════════════════════╣
║ Tháng 01   │ 01/01/2026 │ 31/01/2026 │ 31/01/2026   │ [ĐÃ KHÓA SỔ]│ [Mở khóa có thẩm quyền]  ║
║ Tháng 02   │ 01/02/2026 │ 28/02/2026 │ 28/02/2026   │ [ĐÃ KHÓA SỔ]│ [Mở khóa có thẩm quyền]  ║
║ Tháng 03   │ 01/03/2026 │ 31/03/2026 │ 15/03/2026   │ [KHÓA MỀM ] │ [Thiết lập ngày khóa sổ] ║
║ Tháng 04   │ 01/04/2026 │ 30/04/2026 │ Chưa đặt     │ [ĐANG MỞ  ] │ [Khóa sổ kỳ này]         ║
║ ...        │ ...        │ ...        │ ...          │ ...         │ ...                      ║
║ Tháng 12   │ 01/12/2026 │ 31/12/2026 │ Chưa đặt     │ [ĐANG MỞ  ] │ [Khóa sổ kỳ này]         ║
╠══════════════════════════════════════════════════════════════════════════════════════════════╣
║ [!] CẢNH BÁO: Chứng từ có ngày <= Ngày khóa sổ sẽ bị CHẶN HOÀN TOÀN không cho thêm/sửa/xóa. ║
║ [ Cập nhật ngày khóa sổ toàn hệ thống: [ 15/03/2026 📅 ] ]             [ LƯU THIẾT LẬP ]    ║
╚══════════════════════════════════════════════════════════════════════════════════════════════╝
```

---

# 7. Complete Business Use Cases & Flows

### UC-SPINE-01: Leaf-Only Posting Enforcement
- **Actor**: Sales Accountant / General Accountant.
- **Trigger**: Attempting to post a cash receipt voucher.
- **Happy Path**:
  1. User selects Debit `1111` (Tiền Việt Nam - Leaf Account).
  2. User selects Credit `131` (Phải thu khách hàng - Leaf Account).
  3. System validates `1111.is_leaf == true` and `131.is_leaf == true`.
  4. Voucher is saved and posted successfully.
- **Exception Path E-01 (Posting to Parent Account)**:
  1. User attempts to select Debit `111` (Parent account of `1111` and `1112`).
  2. System rejects transaction immediately with `ErrPostingToParentAccount`.
  3. Error response cites: *"Tài khoản 111 là tài khoản mẹ. Theo quy định Thông tư 99/2025/TT-BTC, bắt buộc phải hạch toán vào tài khoản chi tiết (1111 hoặc 1112)"*.

### UC-SPINE-02: Period Lock Date Interception
- **Actor**: General Accountant.
- **Trigger**: Editing a purchase invoice voucher dated `2026-01-20`.
- **System State**: Period `Tháng 01/2026` has `lock_date = 2026-01-31`.
- **Flow**:
  1. System checks: `VoucherDate (2026-01-20) <= PeriodLockDate (2026-01-31)`.
  2. System intercepts mutation prior to database write.
  3. System returns `ErrPeriodLocked`.
  4. Workstation sounds notification and renders locked badge with red padlock icon.

### UC-SPINE-03: Foreign Exchange Revaluation at Fiscal Period Close
- **Actor**: Chief Accountant.
- **Trigger**: End-of-month foreign exchange revaluation workflow.
- **Flow**:
  1. System identifies all monetary accounts with foreign currency balances:
     - `1112` (USD cash: \$10,000, ledger balance: 250,000,000 VND, book rate: 25,000).
  2. Ingests bank transfer buying rate on 31/03/2026: `25,450 VND/USD`.
  3. Revalued balance: $\$10,000 \times 25,450 = 254,500,000\text{ VND}$.
  4. Exchange gain: $254,500,000 - 250,000,000 = +4,500,000\text{ VND}$.
  5. System generates automated period revaluation journal voucher:
     - **Nợ TK 1112**: 4,500,000 VND
     - **Có TK 4131**: 4,500,000 VND (Chênh lệch tỷ giá đánh giá lại)
  6. Ledger balances updated atomically.

---

# 8. Implementation Roadmap & Execution Slices

```
 ┌──────────────────────────────────────────────────────────────────────────────────┐
 │                         PHASE 2 IMPLEMENTATION SLICES                            │
 ├──────────────────────────────────────────────────────────────────────────────────┤
 │  SLICE 2.1: Domain Entities, Invariants & Unit Tests (TDD)                       │
 │  - internal/domain/spine: Currency, Account, Period, CostCenter, ExpenseItem     │
 │  - Strict type safety, Leaf checks, Hermaphroditic validators                    │
 │  - Target Coverage: >= 95% line coverage                                         │
 ├──────────────────────────────────────────────────────────────────────────────────┤
 │  SLICE 2.2: MariaDB 12.3 Relational Schema & SQLC Queries                        │
 │  - db/migrations/00006_create_accounting_spine.sql                               │
 │  - db/queries/spine_*.sql (CRUD, tree recursion, effective rates, locks)         │
 │  - Standardized repository adapters in internal/adapter/mariadb/repository       │
 ├──────────────────────────────────────────────────────────────────────────────────┤
 │  SLICE 2.3: Application Usecases & Workflow Services                             │
 │  - internal/usecase/spine: COA management, FX revaluation, Period Locking        │
 │  - In-memory read caching, audit logging, structured error wrapping              │
 ├──────────────────────────────────────────────────────────────────────────────────┤
 │  SLICE 2.4: Verification, Parallel Code Review & Sync                            │
 │  - Concurrency testing (parallel leaf balance rollups & lock barriers)           │
 │  - Parallel Standards & Spec Sub-Agent Reviews                                   │
 │  - CodeGraph sync & Git push to origin/main                                      │
 └──────────────────────────────────────────────────────────────────────────────────┘
```
