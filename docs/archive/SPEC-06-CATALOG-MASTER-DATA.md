# SPEC-06: Catalogs & Counterparties (Master Data - Layer 2)
## Standard Authority: FINGO-SPEC-CATALOG-2026-V1
### Domain Seam: `internal/domain/catalog` | Usecase Seam: `internal/usecase/catalog` | Persistence: `internal/adapter/mariadb`

```
====================================================================================================
           F I N G O   A C C O U N T I N G   S O F T W A R E   P L A T F O R M
                    MODULE SPECIFICATION: MASTER DATA CATALOGS (LAYER 2)
  Statutory Compliance: Circular 99/2025/TT-BTC | Circular 105/2020/TT-BTC | Decree 123/2020/NĐ-CP
    Decree 181/2025/NĐ-CP | Law 48/2024/QH15 | Law 41/2024/QH15 | Law 91/2025/QH15 (PDPL)
====================================================================================================
```

---

## 1. Production Readiness Audit & Gap Analysis

### Verdict: **NOT PRODUCTION READY (BLOCKING)**

| Dimension | Current State | Required Production State | Severity |
| :--- | :--- | :--- | :--- |
| **Persistence (MariaDB)** | Zero tables for Layer 2 catalogs. Stubs only. | 8 normalized tables with foreign keys, tenant isolation, and audit columns. | **CRITICAL (P1)** |
| **UOM Multipliers** | Simple flat string `BaseUOM string`. | Multi-tier conversion graph (e.g. 1 Pallet = 10 Thùng = 240 Lon) with exact decimal multipliers and reverse ratios. | **CRITICAL (P1)** |
| **Tax Code (MST) Engine** | Unvalidated string field. | Algorithmic Modulo-11 checksum validation (Circular 105/2020/TT-BTC), 10-digit primary & 13-digit branch formats. | **CRITICAL (P1)** |
| **Non-Cash Thresholds** | Not tracked on counterparties. | Bank account linkage mandatory for purchases $\ge 5,000,000\text{ VND}$ (Decree 181/2025/NĐ-CP & Law 48/2024/QH15). | **HIGH (P2)** |
| **COA Seam Enforcement** | Loose string fields for default accounts. | Leaf-only account verification against `accounts` (Circular 99/2025/TT-BTC, `INV-SPINE-01`). | **CRITICAL (P1)** |
| **Employee & PII (PDPL)** | Bare entity missing department, insurance, and payroll accounts. | Law 91/2025/QH15 (PDPL) compliance, 12-digit CCCD, 10-digit BHXH, TK 141/334 linkages, encrypted sensitive PII. | **HIGH (P2)** |
| **Warehouse Default Accounts** | Missing inventory asset account mapping. | Default stock accounts (`152`, `153`, `155`, `1561`) validated at warehouse and item intersection. | **MEDIUM (P3)** |

---

## 2. Statutory Legal & Accounting Framework

1. **Circular 99/2025/TT-BTC & Circular 133/2016/TT-BTC**:
   - Master data entities act as mandatory subledger dimensions (*Sổ chi tiết*):
     - `Customer` $\rightarrow$ Subledger for TK `131`, `1388`.
     - `Vendor` $\rightarrow$ Subledger for TK `331`, `3388`.
     - `Item` $\rightarrow$ Subledger for TK `152`, `153`, `155`, `156`, `511`, `632`.
     - `Warehouse` $\rightarrow$ Subledger location for physical inventory controls.
     - `BankAccount` $\rightarrow$ Subledger for TK `1121`, `1122`.
     - `Employee` $\rightarrow$ Subledger for TK `141` (Tạm ứng) and TK `334` (Phải trả người lao động).
2. **Circular 105/2020/TT-BTC & Circular 86/2024/TT-BTC (Taxpayer Identification Number - MST)**:
   - Standard 10-digit MST format: $D_1 D_2 D_3 D_4 D_5 D_6 D_7 D_8 D_9 D_{10}$.
   - Checksum formula for $D_{10}$:
     $$D_{10} \equiv 10 - \left( \sum_{i=1}^{9} D_i \times W_i \pmod{11} \right)$$
     Weights $W = [31, 29, 23, 19, 17, 13, 7, 5, 3]$. If $10 - \text{Remainder} == 10$, $D_{10} = 0$.
   - Branch 13-digit format: $D_1 \dots D_{10}\text{-}D_{11}D_{12}D_{13}$ where $D_{11}D_{12}D_{13}$ ranges `001` to `999`.
3. **Decree 123/2020/NĐ-CP & Circular 32/2025/TT-BTC (E-Invoicing)**:
   - Mandatory catalog attributes for buyer/seller invoice generation: Legal Name (*Tên pháp lý theo ĐKKD*), Valid MST, Registered Address (*Địa chỉ trụ sở*), Delivery Contact, Unit Price, and Standard UOM matching General Department of Taxation code list.
4. **Law 48/2024/QH15 & Decree 181/2025/NĐ-CP (Non-Cash Payment Enforcement)**:
   - From 2025-07-01, tax deductibility and VAT input credit claims for transactions $\ge 5,000,000\text{ VND}$ mandate verified non-cash banking settlement. Vendor profiles MUST maintain valid domestic bank account details (*Số tài khoản, Tên ngân hàng, Chi nhánh*).
5. **Law 41/2024/QH15 (Social Insurance) & Law 91/2025/QH15 (PDPL)**:
   - Employee records require verified Social Security numbers (10 digits), Personal Income Tax (PIT) codes (10 digits), and Citizen ID (CCCD - 12 digits).
   - Under PDPL, synthetic data masking is enforced in non-production environments; PII fields are restricted to authorized HR/Chief Accountant roles with immutable audit trails.

---

## 3. Core Architectural & Accounting Invariants

- **INV-CAT-01 (COA Seam Validation)**: All default accounts assigned to catalogs (Customer `131`, Vendor `331`, Item `1561/632/5111`, BankAccount `1121`, Employee `141/334`) MUST exist in the COA and be **active leaf accounts** (`is_leaf = true`, `is_active = true`).
- **INV-CAT-02 (MST Algorithmic Checksum)**: Every non-empty corporate Tax Code (`tax_code`) must strictly satisfy the Circular 105 Modulo-11 checksum. Invalid tax codes are rejected with `ErrInvalidTaxCode`.
- **INV-CAT-03 (UOM Conversion Exactitude)**:
  - Each item has exactly one **Base UOM** ($R = 1.0$).
  - Conversion units must define exact decimal conversion rates:
    $$Q_{\text{base}} = Q_{\text{trans}} \times \text{Multiplier}$$
  - Multipliers must be positive decimals ($> 0$). Division zero or negative rates raise `ErrInvalidConversionMultiplier`.
- **INV-CAT-04 (Zero-Float Precision)**: Quantities, multipliers, credit limits, base salaries, and standard prices strictly utilize `shopspring/decimal`. Floating-point operations are completely prohibited.
- **INV-CAT-05 (Credit Limit Safeguard)**:
  - If a Customer has `credit_limit > 0` and `enforce_credit_limit = true`, voucher approval evaluates:
    $$\text{Outstanding AR} + \text{Voucher Amount} \le \text{Credit Limit}$$
  - Exceeding credit limits raises `ErrCreditLimitExceeded` requiring Chief Accountant override.
- **INV-CAT-06 (Bank Account Currency Alignment)**:
  - Foreign currency bank accounts (e.g. `USD`) must link to sub-accounts of TK `1122` (`Tiền gửi ngoại tệ`).
  - Domestic VND accounts must link to sub-accounts of TK `1121` (`Tiền gửi Việt Nam`).
- **INV-CAT-07 (Warehouse Inventory Account Compatibility)**:
  - Warehouse default asset accounts must belong to Group 15 (`151`, `152`, `153`, `155`, `156`, `157`, `158`).
  - Incompatible accounts raise `ErrIncompatibleWarehouseAccount`.
- **INV-CAT-08 (Employee Statutory Uniqueness)**:
  - Citizen ID (`citizen_id` - CCCD), Tax Code (`tax_code`), and Social Security Number (`social_insurance_no`) must be unique per company profile.
- **INV-CAT-09 (Tenant Isolation & Dual Scope)**:
  - Catalogs are scoped to `company_profile_id`.
  - Warehouses and Bank Accounts optionally carry `branch_id` for location-based permission filtering.

---

## 4. Entity Topology & ASCII Architecture

```
+--------------------------------------------------------------------------------------------------+
|                                    ACCOUNTING SPINE (LAYER 1)                                    |
|   +--------------------------+   +--------------------------+   +----------------------------+   |
|   |   Currencies (VND/USD)   |   |   Chart of Accounts(COA) |   | Fiscal Periods / Lock Date |   |
|   +------------+-------------+   +------------+-------------+   +--------------+-------------+   |
+----------------|------------------------------|--------------------------------|-----------------+
                 |                              |                                |
                 |                              | Account Invariant Validation   |
                 v                              v                                v
+--------------------------------------------------------------------------------------------------+
|                                  CATALOG MASTER DATA (LAYER 2)                                   |
|                                                                                                  |
|   +-------------------------+              +-------------------------+                           |
|   |      UnitOfMeasure      |              |        Warehouse        |                           |
|   |  - Base UOM (Kg, Cái)   |              |  - Default Account      |                           |
|   |  - Conversion Table     |              |    (152, 153, 155, 156) |                           |
|   +------------+------------+              +------------+------------+                           |
|                |                                        |                                        |
|                +--------------------+  +----------------+                                        |
|                                     v  v                                                         |
|                       +-------------------------------+                                          |
|                       |         Item / Product        |                                          |
|                       |  - Code, Name, Barcode        |                                          |
|                       |  - ItemType: Material/Good... |                                          |
|                       |  - Defaults: 1561, 632, 5111  |                                          |
|                       |  - Default VAT: 0/5/8/10%     |                                          |
|                       +-------------------------------+                                          |
|                                                                                                  |
|   +-------------------------+              +-------------------------+                           |
|   |        Customer         |              |    Vendor / Supplier    |                           |
|   |  - Legal Name, MST      |              |  - Legal Name, MST      |                           |
|   |  - Default AR (131)     |              |  - Default AP (331)     |                           |
|   |  - Credit Limit & Terms |              |  - Bank Account Link    |                           |
|   +-------------------------+              +-------------------------+                           |
|                                                                                                  |
|   +-------------------------+              +-------------------------+                           |
|   |       BankAccount       |              |        Employee         |                           |
|   |  - Bank, Account No.    |              |  - CCCD, MST, BHXH      |                           |
|   |  - Currency (VND/USD)   |              |  - Dept, Base Salary    |                           |
|   |  - Linked GL (1121/22)  |              |  - Advance (141) / (334)|                           |
|   +-------------------------+              +-------------------------+                           |
|                                                                                                  |
+--------------------------------------------------------------------------------------------------+
                                                |
                                                v
+--------------------------------------------------------------------------------------------------+
|                             TRANSACTIONAL ENGINE & VOUCHERS (LAYER 3)                            |
|          Cash / Bank (111/112) | Sales / AR (131) | Purchase / AP (331) | Inventory (15x)        |
+--------------------------------------------------------------------------------------------------+
```

---

## 5. Domain Entities Specification

### 5.1 UnitOfMeasure & Conversion
```go
type UnitOfMeasure struct {
    ID               string    `json:"id"`
    CompanyProfileID string    `json:"company_profile_id"`
    Code             string    `json:"code"`             // e.g. "CAI", "KG", "THUNG"
    Name             string    `json:"name"`             // e.g. "Cái", "Kilogram", "Thùng"
    Description      string    `json:"description"`
    IsActive         bool      `json:"is_active"`
    CreatedAt        time.Time `json:"created_at"`
    UpdatedAt        time.Time `json:"updated_at"`
}

type UOMConversion struct {
    ID               string          `json:"id"`
    CompanyProfileID string          `json:"company_profile_id"`
    ItemID           string          `json:"item_id"`           // Specific to item or generic
    FromUOMID        string          `json:"from_uom_id"`       // e.g. "THUNG"
    ToUOMID          string          `json:"to_uom_id"`         // e.g. "LON" (Base UOM)
    Multiplier       decimal.Decimal `json:"multiplier"`        // e.g. 24.0 (1 Thùng = 24 Lon)
    ConversionType   string          `json:"conversion_type"`   // "MULTIPLY" or "DIVIDE"
    IsActive         bool            `json:"is_active"`
    CreatedAt        time.Time       `json:"created_at"`
    UpdatedAt        time.Time       `json:"updated_at"`
}
```

### 5.2 Warehouse
```go
type Warehouse struct {
    ID               string    `json:"id"`
    CompanyProfileID string    `json:"company_profile_id"`
    BranchID         *string   `json:"branch_id,omitempty"`
    Code             string    `json:"code"`             // e.g. "KHO_TONG", "KHO_NVL"
    Name             string    `json:"name"`             // e.g. "Kho tổng Hà Nội"
    Address          string    `json:"address"`
    DefaultAccountID string    `json:"default_account_id"` // FK to accounts (e.g. 1561, 152)
    IsActive         bool      `json:"is_active"`
    CreatedAt        time.Time `json:"created_at"`
    UpdatedAt        time.Time `json:"updated_at"`
}
```

### 5.3 BankAccount
```go
type BankAccount struct {
    ID               string    `json:"id"`
    CompanyProfileID string    `json:"company_profile_id"`
    BranchID         *string   `json:"branch_id,omitempty"`
    AccountNumber    string    `json:"account_number"`    // e.g. "19036888888888"
    BankName         string    `json:"bank_name"`         // e.g. "Techcombank"
    BankCode         string    `json:"bank_code"`         // Citad / Napas code: "TCB"
    BranchName       string    `json:"branch_name"`       // e.g. "Chi nhánh Ba Đình"
    CurrencyCode     string    `json:"currency_code"`     // "VND", "USD"
    GLAccountID      string    `json:"gl_account_id"`     // FK to accounts (1121, 1122 sub-account)
    IsActive         bool      `json:"is_active"`
    CreatedAt        time.Time `json:"created_at"`
    UpdatedAt        time.Time `json:"updated_at"`
}
```

### 5.4 Customer & Vendor (Counterparties)
```go
type Customer struct {
    ID                 string          `json:"id"`
    CompanyProfileID   string          `json:"company_profile_id"`
    Code               string          `json:"code"`               // e.g. "KH0012"
    Name               string          `json:"name"`               // Legal business or individual name
    TaxCode            string          `json:"tax_code"`           // 10 or 13-digit MST
    Address            string          `json:"address"`
    Phone              string          `json:"phone"`
    Email              string          `json:"email"`
    ContactPerson      string          `json:"contact_person"`
    PaymentTermDays    int             `json:"payment_term_days"`  // Default e.g. 30 days
    CreditLimit        decimal.Decimal `json:"credit_limit"`       // Max outstanding balance
    EnforceCreditLimit bool            `json:"enforce_credit_limit"`
    DefaultARAccountID string          `json:"default_ar_account_id"` // FK to accounts (131)
    IsActive           bool            `json:"is_active"`
    CreatedAt          time.Time       `json:"created_at"`
    UpdatedAt          time.Time       `json:"updated_at"`
}

type Vendor struct {
    ID                 string    `json:"id"`
    CompanyProfileID   string    `json:"company_profile_id"`
    Code               string    `json:"code"`               // e.g. "NCC0045"
    Name               string    `json:"name"`               // Legal vendor name
    TaxCode            string    `json:"tax_code"`           // 10 or 13-digit MST
    Address            string    `json:"address"`
    Phone              string    `json:"phone"`
    Email              string    `json:"email"`
    ContactPerson      string    `json:"contact_person"`
    BankAccountNumber  string    `json:"bank_account_number"` // Non-cash payment requirement
    BankName           string    `json:"bank_name"`
    BankBranch         string    `json:"bank_branch"`
    PaymentTermDays    int       `json:"payment_term_days"`
    DefaultAPAccountID string    `json:"default_ap_account_id"` // FK to accounts (331)
    IsActive           bool      `json:"is_active"`
    CreatedAt          time.Time `json:"created_at"`
    UpdatedAt          time.Time `json:"updated_at"`
}
```

### 5.5 Item / Product
```go
type ItemType string

const (
    ItemTypeMaterial     ItemType = "MATERIAL"      // Nguyên vật liệu (152)
    ItemTypeTool         ItemType = "TOOL"          // Công cụ dụng cụ (153)
    ItemTypeFinishedGood ItemType = "FINISHED_GOOD" // Thành phẩm (155)
    ItemTypeMerchandise  ItemType = "MERCHANDISE"   // Hàng hóa (156)
    ItemTypeService      ItemType = "SERVICE"       // Dịch vụ (Không theo dõi kho)
)

type Item struct {
    ID                 string          `json:"id"`
    CompanyProfileID   string          `json:"company_profile_id"`
    Code               string          `json:"code"`               // e.g. "VT-THEP-01"
    Name               string          `json:"name"`               // e.g. "Thép cuộn D6"
    Barcode            string          `json:"barcode,omitempty"`
    ItemType           ItemType        `json:"item_type"`
    BaseUOMID          string          `json:"base_uom_id"`        // FK to unit_of_measures
    DefaultWarehouseID *string         `json:"default_warehouse_id,omitempty"`
    InventoryAccountID *string         `json:"inventory_account_id,omitempty"` // 152, 1561
    COGSAccountID      string          `json:"cogs_account_id"`                // 632
    RevenueAccountID   string          `json:"revenue_account_id"`             // 5111, 5112
    DefaultVATRate     decimal.Decimal `json:"default_vat_rate"`               // 0, 5, 8, 10%
    StandardCostPrice  decimal.Decimal `json:"standard_cost_price"`
    StandardSalePrice  decimal.Decimal `json:"standard_sale_price"`
    IsActive           bool            `json:"is_active"`
    CreatedAt          time.Time       `json:"created_at"`
    UpdatedAt          time.Time       `json:"updated_at"`
}
```

### 5.6 Employee
```go
type Employee struct {
    ID                 string          `json:"id"`
    CompanyProfileID   string          `json:"company_profile_id"`
    BranchID           *string         `json:"branch_id,omitempty"`
    Code               string          `json:"code"`               // e.g. "NV001"
    FullName           string          `json:"full_name"`
    Department         string          `json:"department"`         // Phòng ban (Phòng Kế toán, Kinh doanh...)
    Position           string          `json:"position"`           // Chức vụ
    CitizenID          string          `json:"citizen_id"`         // CCCD (12 digits)
    TaxCode            string          `json:"tax_code,omitempty"` // MST cá nhân (10 digits)
    SocialInsuranceNo  string          `json:"social_insurance_no"`// Mã số BHXH (10 digits)
    BaseSalary         decimal.Decimal `json:"base_salary"`        // Lương căn bản đóng BHXH
    SalaryCoefficient  decimal.Decimal `json:"salary_coefficient"` // Hệ số lương
    BankAccountNumber  string          `json:"bank_account_number"`
    BankName           string          `json:"bank_name"`
    DefaultAdvanceAcc  string          `json:"default_advance_acc"` // FK to accounts (141)
    DefaultPayrollAcc  string          `json:"default_payroll_acc"` // FK to accounts (334)
    IsActive           bool            `json:"is_active"`
    CreatedAt          time.Time       `json:"created_at"`
    UpdatedAt          time.Time       `json:"updated_at"`
}
```

---

## 6. Business Rules & Algorithmic Workflows

### 6.1 Tax Code (MST) Modulo 11 Algorithm
```
Input: taxCode string
1. Clean string (remove spaces, hyphens).
2. If len == 10 (Headquarters / Primary Enterprise):
   - Each character must be ASCII digit '0'-'9'.
   - Sum = D1*31 + D2*29 + D3*23 + D4*19 + D5*17 + D6*13 + D7*7 + D8*5 + D9*3.
   - Remainder = Sum % 11.
   - Diff = 10 - Remainder.
   - If Diff == 10 -> Expected D10 = 0.
   - Else Expected D10 = Diff.
   - Match D10 against expected digit. Return true if equal, else false.
3. If len == 13 (Branch / Dependent Unit):
   - First 10 characters must satisfy Step 2.
   - Last 3 characters must be numeric string between "001" and "999".
4. If len != 10 && len != 13: Return false.
```

### 6.2 Multi-Tier UOM Conversion Engine
```
                  [ Pallet (1) ]
                        |  x 10
                        v
                  [ Thùng (10) ]
                        |  x 24
                        v
              [ Base UOM: Lon (240) ]

Conversion Formula:
- Forward (Transaction -> Base):
    Quantity_Base = Quantity_Transaction * Multiplier
- Reverse (Base -> Transaction):
    Quantity_Transaction = Quantity_Base / Multiplier (using Banker's Rounding)
```

### 6.3 Customer Credit Limit Invariant Check (INV-CAT-05)
```
                          [ Incoming Sales Order / Invoice ]
                                          |
                                          v
                         Is Customer EnforceCreditLimit == true?
                               /                     \
                             YES                      NO
                             /                         \
           Fetch Current AR Outstanding TK 131        Allow Posting
           (From Ledger Subledger Balance)
                            |
                            v
           New_Balance = Current_AR + Voucher_Amount
                            |
                            v
               New_Balance > CreditLimit?
                    /               \
                  YES                NO
                  /                   \
        Reject Transaction         Allow Posting
    (ErrCreditLimitExceeded)
```

---

## 7. MariaDB Schema DDL Blueprint

```sql
-- Migration: 00007_create_catalogs_master_data.sql

CREATE TABLE IF NOT EXISTS unit_of_measures (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_uom_code UNIQUE (company_profile_id, code),
    CONSTRAINT fk_uom_company FOREIGN KEY (company_profile_id) REFERENCES company_profiles(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS uom_conversions (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    item_id VARCHAR(36) NULL,
    from_uom_id VARCHAR(36) NOT NULL,
    to_uom_id VARCHAR(36) NOT NULL,
    multiplier DECIMAL(18, 6) NOT NULL,
    conversion_type ENUM('MULTIPLY', 'DIVIDE') NOT NULL DEFAULT 'MULTIPLY',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_uom_conversion UNIQUE (company_profile_id, item_id, from_uom_id, to_uom_id),
    CONSTRAINT fk_uom_conv_from FOREIGN KEY (from_uom_id) REFERENCES unit_of_measures(id),
    CONSTRAINT fk_uom_conv_to FOREIGN KEY (to_uom_id) REFERENCES unit_of_measures(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS warehouses (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    branch_id VARCHAR(36) NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(150) NOT NULL,
    address TEXT,
    default_account_id VARCHAR(36) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_warehouse_code UNIQUE (company_profile_id, code),
    CONSTRAINT fk_warehouse_company FOREIGN KEY (company_profile_id) REFERENCES company_profiles(id),
    CONSTRAINT fk_warehouse_account FOREIGN KEY (default_account_id) REFERENCES accounts(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS bank_accounts (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    branch_id VARCHAR(36) NULL,
    account_number VARCHAR(50) NOT NULL,
    bank_name VARCHAR(150) NOT NULL,
    bank_code VARCHAR(50) NOT NULL,
    branch_name VARCHAR(150),
    currency_code VARCHAR(3) NOT NULL DEFAULT 'VND',
    gl_account_id VARCHAR(36) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_bank_account_num UNIQUE (company_profile_id, account_number),
    CONSTRAINT fk_bank_acc_company FOREIGN KEY (company_profile_id) REFERENCES company_profiles(id),
    CONSTRAINT fk_bank_acc_currency FOREIGN KEY (currency_code) REFERENCES currencies(code),
    CONSTRAINT fk_bank_acc_gl FOREIGN KEY (gl_account_id) REFERENCES accounts(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS customers (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    tax_code VARCHAR(20),
    address TEXT,
    phone VARCHAR(50),
    email VARCHAR(100),
    contact_person VARCHAR(100),
    payment_term_days INT NOT NULL DEFAULT 30,
    credit_limit DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    enforce_credit_limit BOOLEAN NOT NULL DEFAULT FALSE,
    default_ar_account_id VARCHAR(36) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_customer_code UNIQUE (company_profile_id, code),
    CONSTRAINT fk_customer_company FOREIGN KEY (company_profile_id) REFERENCES company_profiles(id),
    CONSTRAINT fk_customer_ar_acc FOREIGN KEY (default_ar_account_id) REFERENCES accounts(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS vendors (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    tax_code VARCHAR(20),
    address TEXT,
    phone VARCHAR(50),
    email VARCHAR(100),
    contact_person VARCHAR(100),
    bank_account_number VARCHAR(50),
    bank_name VARCHAR(150),
    bank_branch VARCHAR(150),
    payment_term_days INT NOT NULL DEFAULT 30,
    default_ap_account_id VARCHAR(36) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_vendor_code UNIQUE (company_profile_id, code),
    CONSTRAINT fk_vendor_company FOREIGN KEY (company_profile_id) REFERENCES company_profiles(id),
    CONSTRAINT fk_vendor_ap_acc FOREIGN KEY (default_ap_account_id) REFERENCES accounts(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS items (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    barcode VARCHAR(100),
    item_type ENUM('MATERIAL', 'TOOL', 'FINISHED_GOOD', 'MERCHANDISE', 'SERVICE') NOT NULL,
    base_uom_id VARCHAR(36) NOT NULL,
    default_warehouse_id VARCHAR(36) NULL,
    inventory_account_id VARCHAR(36) NULL,
    cogs_account_id VARCHAR(36) NOT NULL,
    revenue_account_id VARCHAR(36) NOT NULL,
    default_vat_rate DECIMAL(5, 2) NOT NULL DEFAULT 10.00,
    standard_cost_price DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    standard_sale_price DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_item_code UNIQUE (company_profile_id, code),
    CONSTRAINT fk_item_company FOREIGN KEY (company_profile_id) REFERENCES company_profiles(id),
    CONSTRAINT fk_item_base_uom FOREIGN KEY (base_uom_id) REFERENCES unit_of_measures(id),
    CONSTRAINT fk_item_warehouse FOREIGN KEY (default_warehouse_id) REFERENCES warehouses(id),
    CONSTRAINT fk_item_inv_acc FOREIGN KEY (inventory_account_id) REFERENCES accounts(id),
    CONSTRAINT fk_item_cogs_acc FOREIGN KEY (cogs_account_id) REFERENCES accounts(id),
    CONSTRAINT fk_item_rev_acc FOREIGN KEY (revenue_account_id) REFERENCES accounts(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS employees (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    branch_id VARCHAR(36) NULL,
    code VARCHAR(50) NOT NULL,
    full_name VARCHAR(150) NOT NULL,
    department VARCHAR(100) NOT NULL,
    position VARCHAR(100) NOT NULL,
    citizen_id VARCHAR(20) NOT NULL,
    tax_code VARCHAR(20),
    social_insurance_no VARCHAR(20) NOT NULL,
    base_salary DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    salary_coefficient DECIMAL(6, 2) NOT NULL DEFAULT 1.00,
    bank_account_number VARCHAR(50),
    bank_name VARCHAR(150),
    default_advance_acc VARCHAR(36) NOT NULL,
    default_payroll_acc VARCHAR(36) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_employee_code UNIQUE (company_profile_id, code),
    CONSTRAINT uk_employee_citizen_id UNIQUE (company_profile_id, citizen_id),
    CONSTRAINT fk_employee_company FOREIGN KEY (company_profile_id) REFERENCES company_profiles(id),
    CONSTRAINT fk_employee_advance_acc FOREIGN KEY (default_advance_acc) REFERENCES accounts(id),
    CONSTRAINT fk_employee_payroll_acc FOREIGN KEY (default_payroll_acc) REFERENCES accounts(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

---

## 8. Desktop UI Wireframes & Component Layouts

### 8.1 Customer Catalog Management (`CustomerListView.vue`)
```
+--------------------------------------------------------------------------------------------------+
| FinGo Desktop > Danh mục > Khách hàng                                  [ + Thêm mới (F2) ] [ Xuất Excel ] |
+--------------------------------------------------------------------------------------------------+
| Tìm kiếm: [ Nhập mã, tên, MST...       ] | Nhóm KH: [ Tất cả      v ] | Trạng thái: [ Đang dùng v ]|
+--------------------------------------------------------------------------------------------------+
| Mã KH    | Tên khách hàng                 | Mã số thuế   | Hạn mức nợ   | Hạn nợ | ĐT liên hệ | Trạng thái |
+----------+--------------------------------+--------------+--------------+--------+------------+------------+
| KH0001   | CTY TNHH KỸ THUẬT ALPHA        | 0108999888   | 100,000,000  | 30     | 0903112233 | Hoạt động  |
| KH0002   | CTY CP ĐẦU TƯ BẢO AN           | 0312345678   | 500,000,000  | 45     | 0918889999 | Hoạt động  |
| KH0003   | DOANH NGHIỆP TƯ NHÂN MINH HẢI  | 0101234567-001|           0  | 15     | 0987654321 | Hoạt động  |
| KH0004   | ÔNG NGUYỄN VĂN AN (CÁ NHÂN)    |              |  20,000,000  |  7     | 0933445566 | Hoạt động  |
+----------+--------------------------------+--------------+--------------+--------+------------+------------+
| Đang chọn: KH0002 - CTY CP ĐẦU TƯ BẢO AN                                                         |
| - Dư nợ hiện tại: 245,000,000 đ | Hạn mức còn lại: 255,000,000 đ | TK Công nợ mặc định: 1311     |
+--------------------------------------------------------------------------------------------------+
```

### 8.2 Item / Product Edit Modal (`ItemFormModal.vue`)
```
+--------------------------------------------------------------------------------------------------+
| THÊM MỚI / CHỈNH SỬA VẬT TƯ, HÀNG HÓA, DỊCH VỤ                                            [ X ]  |
+--------------------------------------------------------------------------------------------------+
| Tính chất:     (*) [X] Hàng hóa (156)  [ ] Nguyên vật liệu (152)  [ ] Thành phẩm (155) [ ] Dịch vụ|
| Mã vật tư:     [ VT-THEP-D6         ]   Mã vạch:    [ 8934567890123      ]                    |
| Tên vật tư:    [ Thép cuộn xây dựng Hòa Phát D6 phi 6.0                                        ] |
| Đơn vị tính:   [ KG - Kilogram    v ]   Kho mặc định:[ KHO_TONG - Kho tổng Hải Phòng         v ] |
+--------------------------------------------------------------------------------------------------+
| [ Tài khoản ngầm định ]                                                                          |
| TK Kho:        [ 1561 - Hàng hóa   v ]  TK Giá vốn: [ 632 - Giá vốn hàng bán             v ]      |
| TK Doanh thu:  [ 5111 - Doanh thu  v ]  Thuế suất:  [ 8% (Nghị quyết 204/2025/QH15)      v ]      |
+--------------------------------------------------------------------------------------------------+
| [ Bảng quy đổi đơn vị tính (Multi-UOM) ]                                                         |
| ĐVT quy đổi    | Phép tính | Tỷ lệ quy đổi | Giá bán quy đổi | Mô tả                             |
| Cuộn           | Nhân      | 1,000.000000  | 16,500,000      | 1 Cuộn = 1,000 Kg                 |
| Tấn            | Nhân      | 1,000.000000  | 16,500,000      | 1 Tấn = 1,000 Kg                  |
+--------------------------------------------------------------------------------------------------+
|                                                      [ Hủy bỏ (Esc) ]  [ Lưu & Đóng (Ctrl+S) ]    |
+--------------------------------------------------------------------------------------------------+
```

---

## 9. Implementation Roadmap & Execution Plan

### Slice 3.1: Core Domain Models & Invariant TDD
- Define domain models in `internal/domain/catalog`:
  - `uom.go`: `UnitOfMeasure`, `UOMConversion`, multiplier math.
  - `warehouse.go`: `Warehouse`, asset account compatibility (`INV-CAT-07`).
  - `bank_account.go`: `BankAccount`, currency and GL link verification (`INV-CAT-06`).
  - `counterparty.go`: `Customer`, `Vendor`, Circular 105 MST Modulo-11 validator (`INV-CAT-02`).
  - `item.go`: `Item`, `ItemType`, default account checks (`INV-CAT-01`).
  - `employee.go`: `Employee`, CCCD/BHXH length check (`INV-CAT-08`), PDPL mask rules.
  - `repository.go`: Domain repository interfaces.
- TDD unit tests achieving **$\ge 95.0\%$ coverage**.

### Slice 3.2: MariaDB Migration & SQLC Persistence
- Author `db/migrations/00007_create_catalogs_master_data.sql`.
- Author `db/queries/catalogs.sql` with optimized CRUD and subledger queries.
- Run `goose up` and `sqlc generate`.
- Implement `CatalogRepo` adapter in `internal/adapter/mariadb/repository/catalog_repo.go`.
- Live integration tests against MariaDB 12.3 (`catalog_repo_test.go`).

### Slice 3.3: Usecase Workflows & Orchestration
- Implement `CatalogUseCase` in `internal/usecase/catalog`:
  - `RegisterCustomer`, `RegisterVendor` with automated MST verification and duplicate checks.
  - `RegisterItem` with leaf account validation and multi-UOM table binding.
  - `ValidateCustomerCreditLimit` against ledger balances.
  - `ValidateVendorNonCashPaymentEligibility` (Decree 181/2025 5M VND rule).
- Comprehensive usecase tests with stubs achieving **$\ge 85.0\%$ coverage**.

### Slice 3.4: Verification, Subagent Review & Sync
- Execute full test suite (`go test ./...`).
- Run CodeGraph sync (`codegraph sync`).
- Invoke dual-axis parallel code review (`Standards Reviewer` & `Spec Reviewer`).
- Git commit & push.
