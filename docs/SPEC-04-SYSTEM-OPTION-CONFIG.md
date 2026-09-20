# Business Requirements Document (BRD) & Statutory Specification
## Module: `SystemOption` & `SystemConfig` (Tùy chọn Hệ thống & Tham số Hạch toán)

```
Document Reference : BRD-FIN-SYS-004
Version            : 1.0.0
Classification     : Statutory Specification / Production-Ready BRD
Authors            : Lead Business Analyst (20+ yrs ERP) & Lead Chief Accountant (20+ yrs VAS, Circular 99/2025, Law 88/2015 Lead)
Reviewers          : Principal Software Engineer, QA/QC Leader, Enterprise Architect
Approvers          : Chief Technology Officer (CTO), Board of Management
Effective Date     : 2026-09-20
Status             : APPROVED / NORMATIVE SPECIFICATION
Target Platform    : FinGo (SME Accounting Desktop Application)
```

---

# 1. Production Readiness Audit: Current App vs. Statutory Reality

### Executive Verdict: **CANNOT OPERATE IN PRODUCTION (CRITICAL FAIL)**

Currently, FinGo has **0% implementation** of system options or configuration tables. All accounting policies, inventory costing algorithms, cash overdraft controls, voucher numbering patterns, and tax compliance thresholds are hardcoded or completely missing.

```
                    PRODUCTION GAP ASSESSMENT
  Current Codebase                  Statutory & Enterprise Requirement
 ┌──────────────────────────────┐  ┌───────────────────────────────────────────┐
 │ Zero configuration entities. │  │ Configurable accounting regimes (TT99/133)│
 │ No negative stock guard.     │  │ Physical inventory cannot be negative.    │
 │ Hardcoded voucher numbers.   │  │ Gapless statutory numbering sequences.    │
 │ No cash overdraft check.     │  │ Cash on hand (TK 111) cannot be negative. │
 │ Hardcoded tax thresholds.    │  │ Law on VAT 2024 (5M non-cash threshold).  │
 └──────────────────────────────┘  └───────────────────────────────────────────┘
```

### Critical Business & Statutory Failure Points:

1. **Breach of Law on Value-Added Tax 2024 (Luật Thuế GTGT số 48/2024/QH15) & Decree 181/2025/NĐ-CP**:
   - Effective **01/07/2025**, the mandatory non-cash payment threshold for input VAT deduction and CIT deductible expense was statutory reduced from **20,000,000 VND** down to **5,000,000 VND** (inclusive of VAT). 
   - The current application has zero threshold detection. An enterprise paying 6,000,000 VND in cash will fail tax audits, face disallowance of input VAT deduction, and incur back-taxes with late payment penalties (0.03%/day).
2. **Breach of Vietnamese Accounting Standards (VAS 02) & Circular 99/2025/TT-BTC (Inventory Costing & Negative Stock)**:
   - Selling or dispatching goods without available physical stock results in negative inventory balances.
   - If `ALLOW_NEGATIVE_INVENTORY` is enabled without automated recalculation, the Weighted Average Cost and FIFO costing algorithms calculate erroneous Costs of Goods Sold (COGS - TK 632), corrupting Gross Profit and Balance Sheets.
3. **Cash Overdraft Exposure (Kiểm soát Quỹ tiền mặt TK 111 - Luật Kế toán 88/2015/QH13)**:
   - Physical cash in a company vault cannot be less than zero ($\text{Cash Balance} \ge 0$). Hardcoded posting allows negative cash vouchers, which is an immediate red flag for tax inspectors (often presumed to be unrecorded revenue or fictitious expense accrual).
4. **Voucher Sequence Gaps & Numbering Collisions (Decree 123/2020/NĐ-CP & Circular 32/2025/TT-BTC)**:
   - E-invoices, internal stock transfers, and accounting vouchers require continuous, verifiable sequence numbering per fiscal year and branch. Hardcoded strings produce duplicate key errors or illegal sequence gaps under multi-user concurrency.
5. **Rigid Inability to Accommodate Multi-Regime Enterprises (TT 133/2016 vs. TT 99/2025)**:
   - An SME adopting Circular 133 uses accounts 152, 153, 154, 155, 156 without separate production accounts 621, 622, 627. An enterprise adopting Circular 99/2025 requires the full account catalog. Without a dynamic system option matrix, the app cannot toggle behavior between regimes.

---

# 2. Vietnamese Legal & Regulatory Framework

| Legislation / Circular | Regulatory Authority | Mandatory Requirement for System Configuration |
| :--- | :--- | :--- |
| **Luật Thuế GTGT 2024 & Nghị định 181/2025/NĐ-CP** | Quốc hội / Chính phủ | Mandates non-cash payment (Bank transfer / UNC / e-payment) for invoices $\ge$ **5,000,000 VND** (formerly 20,000,000 VND prior to 01/07/2025) to qualify for VAT deduction and CIT expense deduction. |
| **Luật Kế toán số 88/2015/QH13** (Điều 12, 14, 16) | Quốc hội | Governs fiscal years, accounting records, and base currency (`VND` or approved foreign currency for FDI enterprises). |
| **Thông tư 99/2025/TT-BTC & Thông tư 133/2016/TT-BTC** | Bộ Tài chính | Governs allowable inventory valuation methods: FIFO, Moving Weighted Average, Periodic Weighted Average. Strictly prohibits LIFO. |
| **Nghị định 123/2020/NĐ-CP & Thông tư 32/2025/TT-BTC** | Chính phủ / BTC | Governs electronic invoice numbering: 1-digit type code, 2-digit year code, 1-digit invoice character, 2-digit series code, 8-digit sequence (e.g. `1C26TAA-00000001`). Zero gaps allowed. |
| **Nghị quyết 204/2025/QH15 & Nghị định 174/2025/NĐ-CP** | Quốc hội / Chính phủ | Enforces time-limited 8% VAT rate policy valid until **2026-12-31**. System option must provide dynamic activation window and automatic reversion to 10% on 2027-01-01. |
| **Quyết định 48/2006/QĐ-BTC & Chuẩn mực Kế toán VAS 02** | Bộ Tài chính | Physical inventory consistency and year-end inventory counting adjustment rules. |

---

# 3. Domain Model & Configuration Hierarchy

### 3.1 Hierarchical Resolution Architecture

Configuration values are resolved through a 4-tier cascading hierarchy:

```
  ┌────────────────────────────────────────────────────────┐
  │ 1. SYSTEM DEFAULT (Global Hardcoded Factory Baseline)  │
  └───────────────────────────┬────────────────────────────┘
                              │ overrides
                              ▼
  ┌────────────────────────────────────────────────────────┐
  │ 2. TENANT / COMPANY (Enterprise Accounting Regime)     │
  └───────────────────────────┬────────────────────────────┘
                              │ overrides
                              ▼
  ┌────────────────────────────────────────────────────────┐
  │ 3. BRANCH / ORG UNIT (Branch Specific Series / Vault)  │
  └───────────────────────────┬────────────────────────────┘
                              │ overrides
                              ▼
  ┌────────────────────────────────────────────────────────┐
  │ 4. USER PREFERENCE (UI Grid Sizes, Printing Defaults)  │
  └────────────────────────────────────────────────────────┘
```

### 3.2 Domain Entities & Value Objects

```go
package system

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// OptionCategory classifies configuration options by business sub-system
type OptionCategory string

const (
	OptionCategoryGeneral    OptionCategory = "GENERAL"     // Định dạng số, ngày tháng, tiền tệ
	OptionCategoryInventory  OptionCategory = "INVENTORY"   // Phương pháp tính giá, xuất âm kho
	OptionCategoryCashBank   OptionCategory = "CASH_BANK"   // Chi âm tiền mặt, ngưỡng thanh toán không dùng tiền mặt
	OptionCategoryVoucher    OptionCategory = "VOUCHER"     // Quy tắc sinh số chứng từ, ghi sổ tự động
	OptionCategorySales      OptionCategory = "SALES"       // Giá bán, hạn mức nợ, chiết khấu
	OptionCategoryPurchase   OptionCategory = "PURCHASE"    // Giá mua, công nợ phải trả
	OptionCategoryTax        OptionCategory = "TAX"         // Thuế suất GTGT, chính sách giảm thuế 8%
	OptionCategoryClosing    OptionCategory = "CLOSING"     // Ngày khóa sổ, xử lý chênh lệch tỷ giá
	OptionCategorySecurity   OptionCategory = "SECURITY"    // Khóa mật khẩu, phiên làm việc
)

// OptionDataType defines the strictly-typed value representation
type OptionDataType string

const (
	DataTypeString  OptionDataType = "STRING"
	DataTypeInt     OptionDataType = "INT"
	DataTypeDecimal OptionDataType = "DECIMAL"
	DataTypeBoolean OptionDataType = "BOOLEAN"
	DataTypeJSON    OptionDataType = "JSON"
)

// ScopeLevel defines where an option is configured and applied
type ScopeLevel string

const (
	ScopeGlobal  ScopeLevel = "GLOBAL"  // Toàn hệ thống
	ScopeCompany ScopeLevel = "COMPANY" // Áp dụng theo Công ty / Tenant
	ScopeBranch  ScopeLevel = "BRANCH"  // Áp dụng theo Chi nhánh
	ScopeUser    ScopeLevel = "USER"    // Áp dụng theo Người dùng
)

// SystemOption represents a strongly-typed, auditable system configuration parameter
type SystemOption struct {
	ID               string         `json:"id"`
	CompanyProfileID string         `json:"company_profile_id"`
	BranchID         *string        `json:"branch_id,omitempty"`
	Category         OptionCategory `json:"category"`
	OptionKey        string         `json:"option_key"`
	OptionValue      string         `json:"option_value"`
	DataType         OptionDataType `json:"data_type"`
	DefaultValue     string         `json:"default_value"`
	ScopeLevel       ScopeLevel     `json:"scope_level"`
	Description      string         `json:"description"`
	IsReadonly       bool           `json:"is_readonly"`
	IsEncrypted      bool           `json:"is_encrypted"`
	UpdatedAt        time.Time      `json:"updated_at"`
	UpdatedBy        string         `json:"updated_by"`
}

// VoucherNumberingConfig defines automated continuous sequence numbering rules
type VoucherNumberingConfig struct {
	ID               string    `json:"id"`
	CompanyProfileID string    `json:"company_profile_id"`
	BranchID         *string   `json:"branch_id,omitempty"`
	VoucherType      string    `json:"voucher_type"` // e.g. "CASH_RECEIPT", "SALES_INVOICE"
	Prefix           string    `json:"prefix"`       // e.g. "PT", "PC", "HDBR", "XK"
	Pattern          string    `json:"pattern"`      // e.g. "{PREFIX}-{YYYY}{MM}-{SEQUENCE:05}"
	ResetFrequency   string    `json:"reset_freq"`   // "MONTHLY", "YEARLY", "CONTINUOUS"
	CurrentSequence  int64     `json:"current_seq"`
	LastResetDate    time.Time `json:"last_reset_date"`
	UpdatedAt        time.Time `json:"updated_at"`
}
```

---

# 4. Standard System Options Catalog (Authoritative Key Registry)

The platform maintains the following pre-defined, normative configuration registry:

| Category | Option Key | Data Type | Default Value | Statutory / Architectural Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **GENERAL** | `DECIMAL_SEPARATOR` | `STRING` | `,` (Comma) | TCVN standards in Vietnam use comma for decimal fractions. |
| **GENERAL** | `THOUSANDS_SEPARATOR` | `STRING` | `.` (Dot) | TCVN standards in Vietnam use dot for thousands separator. |
| **GENERAL** | `CURRENCY_DECIMALS` | `INT` | `0` | VND standard has zero decimal coins/subunits in legal tender. |
| **GENERAL** | `FOREIGN_CURRENCY_DECIMALS`| `INT` | `2` | USD, EUR, JPY transactions require 2 to 4 decimal tracking. |
| **INVENTORY**| `ALLOW_NEGATIVE_INVENTORY` | `STRING` | `DISALLOW` | VAS 02 & Circular 99: Physical stock cannot be negative. Values: `DISALLOW`, `WARN`, `ALLOW`. |
| **INVENTORY**| `COSTING_METHOD` | `STRING` | `MOVING_WEIGHTED_AVG`| Statutory inventory costing method (`FIFO`, `MOVING_WEIGHTED_AVG`, `PERIODIC_AVG`). |
| **INVENTORY**| `COSTING_SCOPE` | `STRING` | `WAREHOUSE` | Costing evaluated per warehouse (`WAREHOUSE`) or company-wide (`COMPANY`). |
| **CASH_BANK**| `ALLOW_NEGATIVE_CASH` | `STRING` | `DISALLOW` | Law on Accounting: Physical vault cash cannot be negative (`DISALLOW`, `WARN`). |
| **CASH_BANK**| `NON_CASH_PAYMENT_THRESHOLD`| `DECIMAL`| `5000000` | **Luật Thuế GTGT 2024 (effective 01/07/2025)**: Mandatory non-cash payment $\ge 5$ million VND. |
| **VOUCHER** | `AUTO_POST_ON_SAVE` | `BOOLEAN`| `FALSE` | Circular 99 internal control: Save as Draft by default; posting requires review. |
| **VOUCHER** | `REQUIRE_MAKER_CHECKER` | `BOOLEAN`| `TRUE` | Circular 99/2025 Four-Eyes Principle: Voucher creator cannot self-approve. |
| **TAX** | `VAT_8_PERCENT_ACTIVE` | `BOOLEAN`| `TRUE` | Resolution 204/2025/QH15: 8% VAT relief valid through `2026-12-31`. |
| **TAX** | `VAT_8_PERCENT_EXPIRY_DATE`| `STRING` | `2026-12-31` | Automated rejection of 8% VAT invoices on and after 2027-01-01. |
| **CLOSING** | `ALLOW_BACKDATED_VOUCHERS` | `BOOLEAN`| `FALSE` | If transaction date $\le$ system lock date, posting is blocked (`ErrPeriodLocked`). |
| **SECURITY**| `SESSION_TIMEOUT_MINUTES` | `INT` | `15` | Financial desktop security standard: Auto-lock after 15 min idle. |
| **SECURITY**| `MAX_FAILED_LOGIN_ATTEMPTS`| `INT` | `5` | 5 failed login attempts triggers 30-minute account lockout. |

---

# 5. ASCII Art Diagrams & Visual Architectural Flows

### 5.1 System Option Resolution Workflow

```
       USER OR USECASE REQUESTS OPTION (e.g. "ALLOW_NEGATIVE_INVENTORY")
                                   │
                                   ▼
                   ┌───────────────────────────────┐
                   │ Look in In-Memory Local Cache │
                   └───────────────┬───────────────┘
                                   │
                       Hit? ───────┴─────── Miss?
                        │                     │
                        ▼                     ▼
               ┌────────────────┐   ┌────────────────────────────────┐
               │ Return Cached  │   │ Query MariaDB `system_options` │
               │     Value      │   └───────────────┬────────────────┘
               └────────────────┘                   │
                                                    ▼
                                    ┌────────────────────────────────┐
                                    │ Check Branch Specific Option?  │
                                    └───────┬────────────────────────┘
                                            │ Found?
                                  Yes ──────┴────── No
                                   │                 │
                                   ▼                 ▼
                          ┌────────────────┐┌────────────────────────────────┐
                          │ Use Branch Val ││ Check Company Specific Option? │
                          └────────────────┘└───────┬────────────────────────┘
                                                    │ Found?
                                          Yes ──────┴────── No
                                           │                 │
                                           ▼                 ▼
                                  ┌────────────────┐┌────────────────────────┐
                                  │ Use Comp Val   ││ Use Hardcoded System   │
                                  └────────────────┘│ Default Factory Value  │
                                                    └────────────────────────┘
```

### 5.2 Negative Stock Interception Flowchart (VAS 02 & Circular 99)

```
        STOCK DISPATCH / SALES DELIVERY NOTE INITIATED
                             │
                             ▼
            ┌─────────────────────────────────┐
            │ Query Current Physical Stock in │
            │ Warehouse W for Item I          │
            └────────────────┬────────────────┘
                             │
                             ▼
         Is (Stock On Hand - Dispatch Quantity) < 0 ?
                             │
                   Yes ──────┴────── No
                    │                 │
                    ▼                 ▼
    ┌───────────────────────────────┐ ┌───────────────────────────────────┐
    │ Fetch ALLOW_NEGATIVE_INVENTORY│ │ Stock Sufficient: Permit Dispatch │
    └───────────────┬───────────────┘ └───────────────────────────────────┘
                    │
         Option Setting Value?
                    ├──────────► "DISALLOW" ──────────► [ABORT: Throw ErrNegativeStockBlocked]
                    │
                    ├──────────► "WARN" ──────────────► [Show Warning Prompt: Require KTT Override]
                    │
                    └──────────► "ALLOW" ─────────────► [Record Negative Balance + Flag for Recalc]
```

### 5.3 Non-Cash Payment Threshold Interception (Luật Thuế GTGT 2024 - 5 Triệu Đồng)

```
             PURCHASE INVOICE OR EXPENSE PAYMENT DRAFTED
                                  │
                                  ▼
      ┌───────────────────────────────────────────────────────┐
      │ Total Invoice / Voucher Payment Amount (Gross with VAT)│
      └───────────────────────────┬───────────────────────────┘
                                  │
                                  ▼
             Is Payment Method == CASH (Tiền mặt / TK 111) ?
                                  │
                        Yes ──────┴────── No
                         │                 │
                         ▼                 ▼
       Is Amount >= 5,000,000 VND ?    ┌──────────────────────────────┐
                         │             │ Non-Cash (Bank Transfer/UNC):│
               Yes ──────┴────── No    │ Fully Compliant for VAT & CIT│
                │                 │    └──────────────────────────────┘
                ▼                 ▼
   ┌────────────────────────┐ ┌──────────────────────────────┐
   │ THRESHOLD BREACHED:    │ │ Amount < 5,000,000 VND:      │
   │ Display Statutory Alert│ │ Cash Payment Permitted       │
   │ per Luật Thuế GTGT 2024│ └──────────────────────────────┘
   └────────────┬───────────┘
                │
                ▼
   ┌──────────────────────────────────────────────────────────┐
   │ Check CASH_PAYMENT_NON_CASH_WARNING Policy:              │
   │ • Mode BLOCK: Reject Cash Voucher (Must use Bank/UNC)    │
   │ • Mode WARN : Warn Accountant: "Non-deductible input VAT │
   │   and non-deductible CIT expense under Law 48/2024/QH15" │
   └──────────────────────────────────────────────────────────┘
```

### 5.4 Gapless Voucher Number Sequence Generator Flow

```
              NEW VOUCHER CREATION (e.g. Sales Invoice)
                                  │
                                  ▼
        ┌────────────────────────────────────────────────────┐
        │ Begin Database Transaction (Serializable/Update)   │
        └─────────────────────────┬──────────────────────────┘
                                  │
                                  ▼
        ┌────────────────────────────────────────────────────┐
        │ SELECT current_seq FROM voucher_numbering_configs  │
        │ WHERE company_id = C AND voucher_type = T          │
        │ FOR UPDATE;  -- Lock Sequence Row                  │
        └─────────────────────────┬──────────────────────────┘
                                  │
                                  ▼
        ┌────────────────────────────────────────────────────┐
        │ Check Reset Frequency (e.g. YEARLY / MONTHLY):     │
        │ If new month/year, reset current_seq = 1           │
        │ Else: current_seq = current_seq + 1                │
        └─────────────────────────┬──────────────────────────┘
                                  │
                                  ▼
        ┌────────────────────────────────────────────────────┐
        │ Format Number per Pattern:                         │
        │ "HDBR-" + "202603" + "-" + FormatZero(seq, 5)     │
        │ Result: "HDBR-202603-00042"                        │
        └─────────────────────────┬──────────────────────────┘
                                  │
                                  ▼
        ┌────────────────────────────────────────────────────┐
        │ UPDATE voucher_numbering_configs                   │
        │ SET current_seq = 42, updated_at = NOW();          │
        └─────────────────────────┬──────────────────────────┘
                                  │
                                  ▼
        ┌────────────────────────────────────────────────────┐
        │ Commit Transaction & Return Formatted Voucher No   │
        └────────────────────────────────────────────────────┘
```

---

# 6. User Journey & UI/XI ASCII Wireframes

### 6.1 Desktop UI: System Configuration Center Wireframe

```
╔══════════════════════════════════════════════════════════════════════════════════════════════╗
║  FinGo SME - THIẾT LẬP TÙY CHỌN HỆ THỐNG (SYSTEM CONFIGURATION)             [−] [口] [X]      ║
╠══════════════════════════════════════════════════════════════════════════════════════════════╣
║ [Tùy chọn chung] [Hàng tồn kho] [Tiền mặt & NH] [Quy tắc đánh số] [Thuế & HĐĐT] [Khóa kỳ]    ║
╠══════════════════════════════════════════════════════════════════════════════════════════════╣
║                                                                                              ║
║  PHƯƠNG PHÁP TÍNH GIÁ VÀ XUẤT KHO HÀNG TỒN KHO                                               ║
║  ------------------------------------------------------------------------------------------  ║
║  Phương pháp tính giá xuất kho : ( ) Bình quân gia quyền tức thời (Moving Weighted Avg)     ║
║                                  (o) Nhập trước - Xuất trước (FIFO - Prescribed VAS 02)      ║
║                                  ( ) Bình quân gia quyền cuối kỳ (Periodic Weighted Avg)     ║
║                                                                                              ║
║  Phạm vi tính giá              : (o) Tính theo từng Kho hàng      ( ) Toàn doanh nghiệp      ║
║                                                                                              ║
║  Xử lý khi xuất kho âm         : ( ) Không cho phép xuất âm (Chặn chứng từ - Khuyến nghị)    ║
║                                  (o) Cảnh báo khi xuất âm nhưng cho phép ghi sổ              ║
║                                  ( ) Cho phép xuất âm tự do                                  ║
║                                                                                              ║
║  KIỂM SOÁT QUỸ TIỀN MẶT & THANH TOÁN (LUẬT THUẾ GTGT 2024 & NGHỊ ĐỊNH 181/2025)             ║
║  ------------------------------------------------------------------------------------------  ║
║  Kiểm soát chi âm tiền mặt     : [X] Không cho phép chi âm quỹ tiền mặt (TK 111 <= 0)        ║
║                                                                                              ║
║  Ngưỡng thanh toán không TM    : [ 5,000,000 ] VND  (Bắt buộc chuyển khoản để khấu trừ VAT)  ║
║                                                                                              ║
║  Hành động khi chi TM >= ngưỡng: (o) Cảnh báo nguy cơ không được khấu trừ thuế GTGT/TNDN     ║
║                                  ( ) Chặn hoàn toàn không cho lập phiếu chi tiền mặt         ║
║                                                                                              ║
║  QUY TẮC PHÊ DUYỆT & GHI SỔ (CIRCULAR 99/2025/TT-BTC)                                        ║
║  ------------------------------------------------------------------------------------------  ║
║  Nguyên tắc Bốn mắt (SoD)      : [X] Bắt buộc người phê duyệt phải khác người lập chứng từ   ║
║  Chế độ ghi sổ                 : (o) Lưu chứng từ ở trạng thái Nháp (Draft) -> Duyệt ghi sổ  ║
║                                  ( ) Tự động ghi sổ Cái ngay khi Lưu (Chỉ dùng cho Admin)    ║
║                                                                                              ║
╠══════════════════════════════════════════════════════════════════════════════════════════════╣
║ [ Khôi phục mặc định ]                              [ Hủy bỏ ]   [ Áp dụng & Lưu cấu hình ]  ║
╚══════════════════════════════════════════════════════════════════════════════════════════════╝
```

### 6.2 Desktop UI: Voucher Auto-Numbering Setup Wireframe

```
╔══════════════════════════════════════════════════════════════════════════════════════════════╗
║  FinGo SME - CẤU HÌNH QUY TẮC ĐÁNH SỐ CHỨNG TỪ TỰ ĐỘNG                                       ║
╠══════════════════════════════════════════════════════════════════════════════════════════════╣
║                                                                                              ║
║  Bảng quy tắc định dạng số chứng từ:                                                         ║
║  ┌────────────────┬────────┬──────────────────────────────┬────────────┬─────────┬────────┐  ║
║  │ Phân hệ        │ Tiền tố│ Định dạng mẫu                │ Định kỳ    │ Số hiện │ Xem    │  ║
║  │                │        │                              │ làm mới    │ tại     │ trước  │  ║
║  ├────────────────┼────────┼──────────────────────────────┼────────────┼─────────┼────────┤  ║
║  │ Phiếu thu tiền │ PT     │ {PREFIX}-{YYYY}{MM}-{SEQ:05} │ Hàng tháng │ 00018   │PT-2603-│  ║
║  │ Phiếu chi tiền │ PC     │ {PREFIX}-{YYYY}{MM}-{SEQ:05} │ Hàng tháng │ 00024   │PC-2603-│  ║
║  │ Hóa đơn bán ra │ HDBR   │ {PREFIX}-{YYYY}-{SEQ:06}     │ Hàng năm   │ 000142  │HDBR-26-│  ║
║  │ Phiếu nhập kho │ PNK    │ {PREFIX}-{YYYY}{MM}-{SEQ:04} │ Hàng tháng │ 0089    │PNK-2603│  ║
║  │ Phiếu xuất kho │ PXK    │ {PREFIX}-{YYYY}{MM}-{SEQ:04} │ Hàng tháng │ 0105    │PXK-2603│  ║
║  │ Bút toán tổng  │ PKT    │ {PREFIX}-{YYYY}{MM}-{SEQ:05} │ Hàng tháng │ 00007   │PKT-2603│  ║
║  └────────────────┴────────┴──────────────────────────────┴────────────┴─────────┴────────┘  ║
║                                                                                              ║
║  [+] Thêm phân hệ mới     [-] Đặt lại số thứ tự     [!] Kiểm tra tính liên tục không ngắt quãng ║
╚══════════════════════════════════════════════════════════════════════════════════════════════╝
```

---

# 7. Business Rules & Statutory Invariants

| Rule ID | Rule Name | Normative Requirement | Legal & Control Basis | Failure Consequence |
| :--- | :--- | :--- | :--- | :--- |
| **BR-OPT-01** | **Inventory Costing Invariant** | Costing method MUST be one of `FIFO`, `MOVING_WEIGHTED_AVG`, `PERIODIC_AVG`. `LIFO` is strictly forbidden. | VAS 02; Thông tư 99/2025/TT-BTC; Thông tư 133/2016/TT-BTC. | Illegal financial statement filing. |
| **BR-OPT-02** | **Negative Inventory Blocking** | When `ALLOW_NEGATIVE_INVENTORY == "DISALLOW"`, attempting to post a stock outward movement exceeding available physical stock MUST fail with `ErrNegativeStockBlocked`. | VAS 02; Chế độ kế toán DN. | Distorted COGS (TK 632) and gross margin. |
| **BR-OPT-03** | **Vault Cash Invariant** | Cash on hand (Account 111) MUST NEVER be negative under any circumstance. When `ALLOW_NEGATIVE_CASH == "DISALLOW"`, payment vouchers driving cash balance below zero MUST be blocked. | Luật Kế toán 88/2015/QH13; VSA 240. | Presumption of accounting fraud / tax evasion. |
| **BR-OPT-04** | **Non-Cash Payment Warning (5M Rule)**| When a purchase or payment voucher is drafted in cash for an invoice with gross amount $\ge 5,000,000\text{ VND}$, the system MUST emit a statutory alert citing Law 48/2024/QH15. | **Luật Thuế GTGT 2024 (effective 01/07/2025)** & Nghị định 181/2025/NĐ-CP. | Loss of input VAT credit and CIT deduction. |
| **BR-OPT-05** | **Voucher Sequence Continuity** | Generated voucher numbers MUST be gapless and strictly monotonically increasing within each reset frequency period. | Nghị định 123/2020/NĐ-CP; Luật Kế toán 88/2015. | Tax penalties for missing or out-of-order vouchers. |
| **BR-OPT-06** | **Period Lock Date Boundary** | Any transaction mutation with `VoucherDate <= SystemLockDate` MUST be immediately rejected with `ErrPeriodLocked`. | Điều 13, Khoản 8 Luật Kế toán 88/2015. | Tampering with finalized audit books. |
| **BR-OPT-07** | **8% VAT Expiration Enforcement** | If system option `VAT_8_PERCENT_EXPIRY_DATE` is reached (e.g. `2027-01-01`), any attempt to issue an 8% invoice MUST throw `ErrVatRateExpired`. | Nghị quyết 204/2025/QH15 & Nghị định 174/2025/NĐ-CP. | Severe tax under-declaration penalties. |
| **BR-OPT-08** | **Maker-Checker Enforcement** | When `REQUIRE_MAKER_CHECKER == true`, the creator of any voucher CANNOT approve or post that same voucher (`CreatorID != ApproverID`). | Thông tư 99/2025/TT-BTC; COSO Internal Controls. | Unauthorized journal injection. |
| **BR-OPT-09** | **Configuration Mutation Auditing** | Every modification to a system option MUST capture `OldValue`, `NewValue`, `UpdatedBy`, `Timestamp`, and `IP` in `system_config_history`. | Điều 18, 41 Luật Kế toán 88/2015. | Inadmissible audit logs in court. |
| **BR-OPT-10** | **Session Idle Invariant** | Inactivity exceeding `SESSION_TIMEOUT_MINUTES` MUST invalidate active desktop tokens and require re-authentication. | TCVN 11945:2017; OWASP Desktop Security. | Unauthorized physical workstation tampering. |

---

# 8. Complete Use Cases & Scenarios

### UC-01: Update Enterprise Inventory Costing & Stock Outward Policy
- **Primary Actor**: Chief Accountant (`ROLE_CHIEF_ACCOUNTANT`).
- **Main Flow (Happy Path)**:
  1. Chief Accountant opens **System Configuration** $\to$ **Hàng tồn kho (Inventory)**.
  2. Selects `COSTING_METHOD = "FIFO"`.
  3. Selects `ALLOW_NEGATIVE_INVENTORY = "DISALLOW"`.
  4. Clicks **Áp dụng & Lưu (Apply & Save)**.
  5. System validates that user holds `ROLE_CHIEF_ACCOUNTANT` or `ROLE_DIRECTOR`.
  6. System checks whether open fiscal periods already have moving weighted average postings in current month. (If so, prompts for mandatory inventory cost revaluation confirmation).
  7. System persists options in MariaDB, invalidates memory cache, and logs change to `system_config_history`.
- **Exception Flow E-01 (Unauthorized Operator)**:
  - Inventory clerk attempts to modify costing policy.
  - System intercepts: `403 Forbidden: ErrConfigPermissionDenied`.

---

### UC-02: Non-Cash Payment 5M Threshold Interception on Cash Voucher
- **Primary Actor**: Cashier / Payment Accountant (`ROLE_CASHIER` / `ROLE_PURCHASE_ACCOUNTANT`).
- **Trigger**: Creating cash payment voucher for supplier invoice.
- **Main Flow (Happy Path)**:
  1. Operator creates Cash Payment Voucher (`PC-202603-00055`) for Supplier X.
  2. Enters amount: `6,500,000 VND` (Account Credit 1111).
  3. System queries option `NON_CASH_PAYMENT_THRESHOLD` (returns `5,000,000`).
  4. System detects `Amount (6,500,000) >= Threshold (5,000,000)`.
  5. If `CASH_PAYMENT_NON_CASH_WARNING == "WARN"`:
     - Modal warning appears: *"CẢNH BÁO PHÁP LÝ (Luật Thuế GTGT 2024 & NĐ 181/2025): Hóa đơn có giá trị từ 5,000,000 đ thanh toán bằng tiền mặt sẽ KHÔNG ĐƯỢC KHẤU TRỪ THUẾ GTGT ĐẦU VÀO và KHÔNG ĐƯỢC TÍNH CHI PHÍ HỢP LÝ TNDN. Khuyến nghị chuyển sang hình thức Ủy nhiệm chi / Chuyển khoản ngân hàng."*
     - Operator acknowledges warning and proceeds, or switches payment method to Bank Transfer (TK 112).
  6. If `CASH_PAYMENT_NON_CASH_WARNING == "BLOCK"`:
     - System rejects saving as cash voucher, forcing transition to Bank Payment.

---

### UC-03: Gapless Voucher Number Generation
- **Primary Actor**: System Domain Engine.
- **Trigger**: New voucher being saved.
- **Main Flow (Happy Path)**:
  1. Sales module requests next voucher number for `VoucherType = "SALES_INVOICE"`.
  2. Generator initiates database transaction on MariaDB with pessimistic lock (`FOR UPDATE`) on `voucher_numbering_configs`.
  3. Checks `ResetFrequency = "MONTHLY"` and compares `LastResetDate`. Current month is March 2026.
  4. Increments sequence: `current_seq = 142`.
  5. Formats result: `"HDBR-202603-000142"`.
  6. Updates `voucher_numbering_configs` row and commits transaction.
  7. Returns unique, gapless voucher number in $< 2\text{ms}$.

---

### UC-04: Automated 8% VAT Rate Expiration Rejection
- **Primary Actor**: Sales Accountant.
- **Trigger**: Creating invoice with date `2027-01-02`.
- **Main Flow**:
  1. Operator attempts to select `VAT Rate = 8%` on an invoice dated `2027-01-02`.
  2. System queries `VAT_8_PERCENT_EXPIRY_DATE` (`2026-12-31`).
  3. System detects `InvoiceDate > ExpiryDate`.
  4. System immediately blocks selection with `ErrVatRateExpired: "Nghị quyết 204/2025/QH15 về giảm thuế GTGT 8% đã hết hiệu lực từ ngày 31/12/2026. Hóa đơn năm 2027 bắt buộc áp dụng thuế suất chuẩn 10% hoặc thuế suất theo luật định."`

---

# 9. Database Schema Specification (MariaDB 12.3)

Migration: `00005_create_system_option_config.sql`

```sql
-- +goose Up
-- Migration: 00005_create_system_option_config.sql
-- Module: SystemOption & SystemConfig (BRD-FIN-SYS-004)

-- 1. System Options Table
CREATE TABLE IF NOT EXISTS system_options (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    branch_id VARCHAR(36) NULL,
    category ENUM('GENERAL', 'INVENTORY', 'CASH_BANK', 'VOUCHER', 'SALES', 'PURCHASE', 'TAX', 'CLOSING', 'SECURITY') NOT NULL,
    option_key VARCHAR(64) NOT NULL,
    option_value TEXT NOT NULL,
    data_type ENUM('STRING', 'INT', 'DECIMAL', 'BOOLEAN', 'JSON') NOT NULL DEFAULT 'STRING',
    default_value TEXT NOT NULL,
    scope_level ENUM('GLOBAL', 'COMPANY', 'BRANCH', 'USER') NOT NULL DEFAULT 'COMPANY',
    description VARCHAR(255) NULL,
    is_readonly BOOLEAN NOT NULL DEFAULT FALSE,
    is_encrypted BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    updated_by VARCHAR(36) NULL,
    CONSTRAINT fk_sys_opt_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE CASCADE,
    CONSTRAINT fk_sys_opt_branch FOREIGN KEY (branch_id) REFERENCES branch_org_units(id) ON DELETE CASCADE,
    UNIQUE KEY uk_company_branch_opt_key (company_profile_id, branch_id, option_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 2. Voucher Numbering Sequences Table
CREATE TABLE IF NOT EXISTS voucher_numbering_configs (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    branch_id VARCHAR(36) NULL,
    voucher_type VARCHAR(64) NOT NULL,
    prefix VARCHAR(16) NOT NULL,
    pattern VARCHAR(64) NOT NULL DEFAULT '{PREFIX}-{YYYY}{MM}-{SEQUENCE:05}',
    reset_frequency ENUM('MONTHLY', 'YEARLY', 'CONTINUOUS') NOT NULL DEFAULT 'MONTHLY',
    current_sequence BIGINT NOT NULL DEFAULT 0,
    last_reset_date DATE NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_vnum_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE CASCADE,
    CONSTRAINT fk_vnum_branch FOREIGN KEY (branch_id) REFERENCES branch_org_units(id) ON DELETE CASCADE,
    UNIQUE KEY uk_vnum_scope (company_profile_id, branch_id, voucher_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 3. System Configuration Audit History Table (10-Year Immutability)
CREATE TABLE IF NOT EXISTS system_config_history (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    option_key VARCHAR(64) NOT NULL,
    old_value TEXT NULL,
    new_value TEXT NOT NULL,
    updated_by VARCHAR(36) NULL,
    client_ip VARCHAR(64) NULL,
    reason VARCHAR(255) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_cfg_hist_company_key (company_profile_id, option_key),
    INDEX idx_cfg_hist_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS system_config_history;
DROP TABLE IF EXISTS voucher_numbering_configs;
DROP TABLE IF EXISTS system_options;
```

---

# 10. Phased Implementation Roadmap & TDD Execution Plan

```
                  PHASED TDD IMPLEMENTATION SCHEDULE
┌────────────────────────────────────────────────────────────────────────┐
│ Phase 1: Pure Domain Layer (internal/domain/system)                    │
│ • Domain Entities: SystemOption, VoucherNumberingConfig.               │
│ • Domain Services: SystemOptionResolver, VoucherNumberFormatter.       │
│ • Invariants: Negative stock guard, 5M cash threshold rule, 8% VAT.   │
│ • Table-Driven Unit Tests: 100% line coverage in system_option_test.go │
├────────────────────────────────────────────────────────────────────────┤
│ Phase 2: Schema Migration & Persistence (db/migrations & sqlc)         │
│ • Goose migration: 00005_create_system_option_config.sql.              │
│ • SQLC queries: db/queries/system_options.sql.                         │
│ • Seed default statutory option catalog.                               │
├────────────────────────────────────────────────────────────────────────┤
│ Phase 3: Adapter Layer & Concurrency Integration Tests                 │
│ • Repository: SystemOptionRepo, VoucherNumberingRepo.                  │
│ • MariaDB Concurrency Tests: 50 concurrent goroutines requesting       │
│   sequential voucher numbers without collisions or gaps.               │
├────────────────────────────────────────────────────────────────────────┤
│ Phase 4: Application Service / Usecase Layer                           │
│ • UpdateSystemOptionUseCase: Auditing, validation, cache invalidation. │
│ • NextVoucherNumberUseCase: Fast pessimistic sequence generation.      │
│ • CheckTransactionComplianceUseCase: Pre-flight validator (5M non-cash,│
│   negative stock, negative cash, period lock).                         │
├────────────────────────────────────────────────────────────────────────┤
│ Phase 5: Wails Handler & Vue 3 Desktop UI                              │
│ • System Configuration Center (Tabs: General, Inventory, Cash, Number).│
│ • Live voucher number preview generator.                               │
│ • 5M non-cash payment statutory warning modal alert.                   │
└────────────────────────────────────────────────────────────────────────┘
```
