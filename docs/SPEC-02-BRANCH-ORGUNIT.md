# Business Requirements Document (BRD) & Statutory Specification
## Module: `Branch / OrgUnit` (Chi nhánh, Đơn vị trực thuộc & Phòng ban)

**Author**: Senior Solution Architect & Chief Accountant (20+ years VAS, Circular 99/2025, Circular 80/2021, Decree 123/2020 Lead)  
**Document Code**: BRD-FIN-SYS-002  
**Status**: APPROVED / PRODUCTION-READY SPECIFICATION  
**Target Application**: FinGo (SME Accounting Desktop)  

---

# 1. Production Readiness Audit: Current App vs. Statutory Reality

### Executive Verdict: **CANNOT OPERATE IN PRODUCTION (FAIL)**

### Current Codebase Assessment:
* **Domain State**: `0%` coverage. There is NO `Branch` or `OrgUnit` entity anywhere in `internal/domain`.
* **Database State**: Zero tables (`branches` or `org_units` do not exist in MariaDB schema).
* **General Ledger**: Vouchers only support flat single-entity posting; no organizational dimension (`branch_id` / `department_id`) on vouchers or lines.
* **Internal Accounts**: No internal clearing engine for accounts **TK 136** (*Phải thu nội bộ*) and **TK 336** (*Phải trả nội bộ*).

### Critical Business & Statutory Failure Points:
1. **Tax Registration Breach (Luật Quản lý thuế 2019, TT 105/2020/TT-BTC & TT 86/2024/TT-BTC)**:
   - Enterprises with branches cannot assign the mandatory 13-digit tax code (`XXXXXXXXXX-YYY`) or 5-digit business location code (`00001` to `99999`).
2. **Tax Allocation Inoperability (Circular 80/2021/TT-BTC - Điều 12, 13, 14, 15)**:
   - Vietnamese law requires head offices to allocate VAT and CIT to provincial tax departments where dependent manufacturing branches or sales outlets reside (Form `01-6/GTGT` and `01-1/TNDN`). The app has zero allocation data tracking.
3. **E-Invoicing & Movement Violation (Decree 123/2020/ND-CP & Circular 32/2025/TT-BTC)**:
   - Internal goods transfers between head office and branches cannot issue statutory **Internal Stock Transfer Orders & Delivery Notes (Phiếu xuất kho kiêm vận chuyển nội bộ điện tử - Mẫu 03/XKNB)**.
4. **General Ledger & Financial Reporting Collapse (Circular 99/2025/TT-BTC & Circular 200/2014/TT-BTC)**:
   - Cannot generate **Consolidated Financial Statements (Báo cáo tài chính tổng hợp)** because internal receivables/payables (TK 136/336) and internal revenues/costs (TK 511/632) cannot be tagged or eliminated.

---

# 2. Vietnamese Legal & Regulatory Framework

This specification synthesizes mandatory requirements from primary authorities:

| Legislation / Circular | Regulatory Authority | Mandatory Requirement for Accounting & Software |
| :--- | :--- | :--- |
| **Luật Doanh nghiệp 2020** (Điều 44, 45) | Quốc hội | Establishes legal status: (1) **Chi nhánh** (Branch - conducts business), (2) **Văn phòng đại diện** (Rep Office - non-profit liaison), (3) **Địa điểm kinh doanh** (Business Location - operational site). |
| **Thông tư 80/2021/TT-BTC** (Điều 12, 13) | Bộ Tài chính | Governs tax declaration & allocation: Dependent branches in different provinces must be allocated VAT (1.5% or 2% of un-taxed revenue) and CIT based on expense ratios. |
| **Thông tư 105/2020 & TT 86/2024** | Bộ Tài chính | Tax code taxonomy: Head office = 10 digits (`XXXXXXXXXX`); Branch = 13 digits (`XXXXXXXXXX-YYY`); Business Location = 5-digit sequence code (`00001`–`99999`). |
| **Thông tư 99/2025 & TT 200/2014** | Bộ Tài chính | Dual accounting options: Independent accounting (*Hạch toán độc lập*) vs Dependent accounting (*Hạch toán phụ thuộc*) via TK 136 & TK 336. Internal transaction elimination for BCTC. |
| **Nghị định 123/2020 & TT 32/2025** | Chính phủ / BTC | E-invoice rules for branches: Branch with own digital cert issues own e-invoices, or uses head office cert. Internal transfers MUST use e-document Mẫu 03/XKNB. |
| **Luật BHXH 2024** | BHXH Việt Nam | Branches in different provinces MUST register and pay social insurance directly to the provincial social security office of their jurisdiction. |

---

# 3. Domain Model & Class Taxonomy

### 3.1 Organizational Unit Architecture

```go
package system

import (
	"errors"
	"strings"
	"time"
)

// OrgUnitType defines the legal and operational classification of an organizational unit
type OrgUnitType string

const (
	OrgUnitHeadOffice       OrgUnitType = "HEAD_OFFICE"        // Trụ sở chính (MST 10 số)
	OrgUnitBranch           OrgUnitType = "BRANCH"             // Chi nhánh (MST 13 số)
	OrgUnitRepOffice        OrgUnitType = "REP_OFFICE"         // Văn phòng đại diện (MST 13 số - Không kinh doanh)
	OrgUnitBusinessLocation OrgUnitType = "BUSINESS_LOCATION"  // Địa điểm kinh doanh (Mã 5 số)
	OrgUnitDepartment       OrgUnitType = "DEPARTMENT"         // Phòng ban nội bộ (Cost Center)
)

// AccountingGovernance defines how financial transactions are recorded
type AccountingGovernance string

const (
	GovIndependent AccountingGovernance = "INDEPENDENT" // Hạch toán độc lập (Tự lập BCTC, tự khai thuế)
	GovDependent   AccountingGovernance = "DEPENDENT"   // Hạch toán phụ thuộc (Dùng TK 136/336, BCTC tổng hợp)
	GovCostCenter  AccountingGovernance = "COST_CENTER"  // Chỉ theo dõi chi phí (Phòng ban/Địa điểm KD)
)

// TaxFilingMechanism defines how tax returns are filed under Circular 80/2021/TT-BTC
type TaxFilingMechanism string

const (
	TaxFilingCentralized   TaxFilingMechanism = "CENTRALIZED"   // Khai thuế tập trung tại Trụ sở chính
	TaxFilingDecentralized TaxFilingMechanism = "DECENTRALIZED" // Khai và nộp thuế trực tiếp tại CQT quản lý chi nhánh
	TaxFilingAllocated     TaxFilingMechanism = "ALLOCATED"     // Phân bổ thuế vãng lai / nhà máy theo TT 80
)

type BranchOrgUnit struct {
	ID                     string                `json:"id"`
	ParentID               *string               `json:"parent_id,omitempty"` // Trụ sở chính hoặc Chi nhánh cấp trên
	CompanyProfileID       string                `json:"company_profile_id"`
	Code                   string                `json:"code"`                // e.g. "CN_HCM", "KHO_DANANG", "PB_KE_TOAN"
	Name                   string                `json:"name"`                // Tên pháp lý hoặc tên phòng ban
	UnitType               OrgUnitType           `json:"unit_type"`
	AccountingGovernance   AccountingGovernance `json:"accounting_governance"`
	TaxFilingMechanism     TaxFilingMechanism    `json:"tax_filing_mechanism"`
	
	// Tax & Legal Identification
	TaxCode                string                `json:"tax_code,omitempty"`  // MST 13 số (chi nhánh) hoặc mã 5 số (ĐĐKD)
	TaxAuthorityCode       string                `json:"tax_authority_code,omitempty"`
	TaxAuthorityName       string                `json:"tax_authority_name,omitempty"`
	ProvinceCityCode       string                `json:"province_city_code"`  // Mã tỉnh/thành để đối chiếu cùng/khác tỉnh
	Address                string                `json:"address"`
	ManagerName            string                `json:"manager_name,omitempty"` // Người đứng đầu chi nhánh / Trưởng phòng
	ChiefAccountant        string                `json:"chief_accountant,omitempty"`
	
	// Operational & Accounting Configuration
	InternalReceivableAccount string             `json:"internal_receivable_account"` // Default: 1361 hoặc 1368
	InternalPayableAccount    string             `json:"internal_payable_account"`    // Default: 3361 hoặc 3368
	HasOwnEInvoice            bool               `json:"has_own_einvoice"`            // Chi nhánh tự xuất hóa đơn điện tử
	IsActive                  bool               `json:"is_active"`
	CreatedAt                 time.Time          `json:"created_at"`
	UpdatedAt                 time.Time          `json:"updated_at"`
}
```

---

# 4. Detailed Business Rules (Invariants)

| Rule ID | Name | Formal Invariant Specification |
| :--- | :--- | :--- |
| **BR-ORG-01** | Tax Code Structure Enforcement | If `UnitType == BRANCH` or `REP_OFFICE`, `TaxCode` MUST be 14 characters (`XXXXXXXXXX-YYY`) where the first 10 digits match the company's head office tax code, and suffix `YYY` is between `001` and `999`. If `UnitType == BUSINESS_LOCATION`, `TaxCode` MUST be a 5-digit numeric string (`00001`–`99999`). |
| **BR-ORG-02** | Independent vs Dependent Accounting Constraint | If `UnitType == BRANCH` and `AccountingGovernance == INDEPENDENT`, `TaxFilingMechanism` MUST be `DECENTRALIZED` (Independent branches must file taxes locally per TT 80/2021). `AccountingGovernance == INDEPENDENT` is FORBIDDEN for `BUSINESS_LOCATION` and `DEPARTMENT`. |
| **BR-ORG-03** | Province-Based Tax Allocation Invariant | If `AccountingGovernance == DEPENDENT` and `ProvinceCityCode != HeadOffice.ProvinceCityCode`: (a) If the unit engages in production, system MUST flag transactions for **Form 01-6/GTGT** allocation (1.5% or 2% VAT allocation); (b) If the unit is an administrative office, VAT is declared centralized at Head Office. |
| **BR-ORG-04** | Internal Accounts Reciprocity (TK 136 / 336) | For every transaction between Head Office and a dependent unit involving TK 136, a reciprocal balanced entry on TK 336 MUST be recorded on the recipient's ledger. At any closing date: $\sum \text{Balance}(136) == \sum \text{Balance}(336)$. |
| **BR-ORG-05** | Elimination of Internal Balances on Consolidation | When generating the **Consolidated Trial Balance (Bảng CĐS phát sinh tổng hợp)**, the reporting engine MUST automatically eliminate: (1) Balances of TK 136 and TK 336 between units, (2) Internal revenue and internal COGS (TK 511 / TK 632) arising from internal goods movements. |

---

# 5. Use Cases & Behavioral Workflows

## UC-01: Branch / OrgUnit Registration & Hierarchy Setup
* **Actor**: Administrator / Chief Accountant.
* **Preconditions**: Head office `CompanyProfile` exists.
* **Happy Path (Main Flow)**:
  1. User selects "Add Organizational Unit".
  2. User selects Unit Type (e.g. `BRANCH`).
  3. System prompts for Code, Name, Address, and Province.
  4. System prompts for Tax Code. User inputs `0101243150-001`.
  5. System validates: (a) First 10 digits match company profile MST, (b) Suffix is numeric `001`, (c) Modulo-11 checksum passes.
  6. User selects Accounting Regime: `DEPENDENT` (Hạch toán phụ thuộc).
  7. System checks Province: Branch is in Đà Nẵng, Head Office is in Hà Nội (Different Province). System sets `TaxFilingMechanism = ALLOCATED` and configures internal accounts (1361 / 3361).
  8. System persists unit and creates organizational ledger partition.
* **Alternative Path (A1 - Internal Department Creation)**:
  - User selects `DEPARTMENT` (e.g. `Phòng Kinh doanh`). Tax code fields are disabled. Unit is tagged solely as a Cost Center for GL expense tracking (TK 641, 642).
* **Exception Path (E1 - Tax Code Suffix Collision)**:
  - User enters an existing branch suffix (`-001`). System returns `ERR_DUPLICATE_BRANCH_TAX_CODE`. Save blocked.
* **Exception Path (E2 - Mismatched Parent Tax Code)**:
  - User enters branch tax code with base MST differing from company profile. System returns `ERR_INVALID_PARENT_TAX_CODE: Chi nhánh phải trực thuộc MST của Trụ sở chính`.

## UC-02: Inter-Branch Stock Movement (Mẫu 03/XKNB)
* **Actor**: Warehouse Keeper / Logistics Accountant.
* **Happy Path**:
  1. Head Office initiates transfer of 100 laptops to Da Nang Branch.
  2. System issues **Internal Dispatch Order (Lệnh điều động nội bộ)** and generates **Phiếu xuất kho kiêm vận chuyển nội bộ điện tử (Mẫu 03/XKNB)**:
     - Header: Sender MST `0101243150`, Receiver MST `0101243150-001`.
     - Entry at Head Office: $Nợ\ TK\ 1368\ (\text{or}\ 157)\ /\ Có\ TK\ 156$.
  3. Da Nang Branch confirms physical receipt.
  4. Entry at Da Nang Branch: $Nợ\ TK\ 156\ /\ Có\ TK\ 3368$.
  5. Reciprocal balance verified: $TK\ 1368 == TK\ 3368$.

## UC-03: Consolidated Financial Statement & Elimination Run
* **Actor**: Chief Accountant.
* **Preconditions**: Period closed across Head Office and all dependent branches.
* **Happy Path**:
  1. Chief Accountant triggers "Generate Consolidated Financial Statements" (Lập BCTC tổng hợp).
  2. System aggregates trial balances of all units.
  3. Elimination Engine executes automated contra-entries:
     $$Nợ\ TK\ 336\ /\ Có\ TK\ 136 \quad (\text{Clears internal receivables/payables})$$
     $$Nợ\ TK\ 511\ /\ Có\ TK\ 632 \quad (\text{Clears internal transfers recorded as sales})$$
  4. System generates consolidated B01-DNN, B02-DNN, and B03-DNN with zero internal distortion.

---

# 6. Data Flow Architecture (Mermaid)

```mermaid
flowchart TD
    A["CompanyProfile (Head Office - MST 10 số)"] -->|Has Many| B["BranchOrgUnit (Chi nhánh / Phòng ban)"]
    
    subgraph S1["Legal Classification"]
        B -->|Branch (13 số)| C1["Hạch toán độc lập (Khai thuế riêng)"]
        B -->|Branch (13 số)| C2["Hạch toán phụ thuộc (TK 136/336)"]
        B -->|Địa điểm KD (5 số)| C3["Cơ sở sản xuất / Bán lẻ (Phân bổ TT 80)"]
        B -->|Phòng ban| C4["Cost Center (TK 641, 642, 154)"]
    end
    
    subgraph S2["Transaction Engine"]
        C2 --> D["Voucher with OrgUnit Dimension"]
        D -->|Internal Transfer| E["Phiếu XK kiêm VCNB (Mẫu 03/XKNB)"]
        D -->|Daily Posting| F["GL Control Accounts (TK 1361/3361)"]
    end
    
    subgraph S3["Closing & Statutory Output"]
        F --> G["Consolidation & Elimination Engine"]
        G -->|Eliminate 136/336| H["Consolidated Trial Balance (Bảng CĐS Tổng hợp)"]
        G -->|Generate Phụ lục TT 80| I["Form 01-6/GTGT & 01-1/TNDN (Phân bổ tỉnh)"]
    end
```

---

# 7. User Journey & Screen Templates

```
[Screen 1: Organizational Hierarchy Tree]
┌─────────────────────────────────────────────────────────────┐
│ Công ty Cổ phần FinGo (Trụ sở chính - 0101243150)           │
│  ├── Chi nhánh TP. Hồ Chí Minh (0101243150-001 - Độc lập)   │
│  │    └── Showroom Quận 1 (Mã ĐĐKD: 00001)                 │
│  ├── Chi nhánh Đà Nẵng (0101243150-002 - Phụ thuộc)        │
│  │    └── Nhà máy Hòa Khánh (Phân bổ thuế TT 80)           │
│  └── Khối Văn phòng Trụ sở (Hà Nội)                        │
│       ├── Phòng Kế toán (Cost Center: 642)                 │
│       └── Phòng Kinh doanh (Cost Center: 641)               │
└─────────────────────────────────────────────────────────────┘
```

---

# 8. Implementation Roadmap & Execution Plan

### Milestone 1: Domain Core & Invariants (`TDD`)
1. Value Objects: `OrgUnitType`, `AccountingGovernance`, `TaxFilingMechanism`.
2. Entity `BranchOrgUnit`: Enforcing `BR-ORG-01` to `BR-ORG-03` (MST 13-digit parent matching, 5-digit location check, local vs centralized filing).
3. Domain Seam `BranchOrgUnitRepository`.

### Milestone 2: MariaDB Database Persistence
4. Goose migration `00003_create_branch_org_unit.sql`.
5. SQLC queries in `db/queries/org_unit.sql`:
   - `CreateBranchOrgUnit`, `GetBranchByID`, `ListOrgUnitsByCompany`, `UpdateOrgUnit`.
6. Deep Repository Adapter `internal/adapter/mariadb/repository/org_unit_repo.go`.

### Milestone 3: General Ledger Tagging & Internal Clearing Engine
7. Add `branch_id` and `department_id` foreign keys to `vouchers` and `voucher_lines`.
8. Implement Inter-unit Reciprocal Reconciliation ($TK\ 136 == TK\ 336$).

### Milestone 4: Consolidated BCTC & Tax Allocation Engine (Circular 80)
9. Internal transaction elimination engine for reporting.
10. Form 01-6/GTGT provincial VAT allocation calculator.

---

# 9. Obsidian Mind Map Wrap-up

```text
# [[Branch-OrgUnit-Architecture]]
- **Legal Entity Topology**
  - [[HeadOffice]] (Trụ sở chính - MST 10 số)
  - [[Branch]] (Chi nhánh - MST 13 số `XXXXXXXXXX-YYY`)
  - [[RepOffice]] (VPĐD - MST 13 số, không phát sinh doanh thu)
  - [[BusinessLocation]] (Địa điểm kinh doanh - Mã 5 số `00001`)
  - [[Department]] (Phòng ban - Cost Center)
- **Statutory Tax Allocation (Thông tư 80/2021/TT-BTC)**
  - [[CentralizedFiling]] (Cùng tỉnh - Kê khai tập trung tại Trụ sở chính)
  - [[DecentralizedFiling]] (Độc lập - Kê khai trực tiếp tại địa phương)
  - [[AllocatedFiling]] (Khác tỉnh - Phân bổ VAT 1.5%/2% Mẫu 01-6/GTGT, TNDN Mẫu 01-1/TNDN)
- **Internal Accounting Engine (Thông tư 99/2025 & TT 200/2014)**
  - [[Account136]] (Phải thu nội bộ: 1361 vốn, 1368 vãng lai)
  - [[Account336]] (Phải trả nội bộ: 3361 vốn, 3368 vãng lai)
  - [[InternalElimination]] (Bù trừ công nợ & loại trừ doanh thu nội bộ trên BCTC)
  - [[E-Doc03-XKNB]] (Phiếu xuất kho kiêm vận chuyển nội bộ điện tử)
```
