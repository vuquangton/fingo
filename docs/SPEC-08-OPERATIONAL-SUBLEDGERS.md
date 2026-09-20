# FinGo Technical Specification (SPEC)
## Module 08: Operational Subledgers (Transactions - Layer 4)
**Document ID**: `FINGO-SPEC-08-OPERATIONAL-SUBLEDGERS`  
**Standard Authority**: `FINGO-QA-STRATEGY-2026-V1` / `VAS` / `Circular 99/2025/TT-BTC` / `Circular 133/2016/TT-BTC`  
**Classification**: Core Financial Engine / Production Architecture  
**Status**: DRAFT FOR ARCHITECTURAL APPROVAL  

---

## 1. Architectural Principles & Onion Boundaries

Operational Subledgers (Layer 4) consume Master Data (Layer 2) and the Accounting Spine (Layer 1), and mutate balances established during Cutover (Layer 3).

```
                      +------------------------------------------+
                      |         UI / ADAPTER LAYER (WAILS)       |
                      +------------------------------------------+
                                           |
                                           v
                      +------------------------------------------+
                      |               USECASE LAYER              |
                      | - CashUseCase        - SalesUseCase      |
                      | - BankUseCase        - PurchaseUseCase   |
                      | - InventoryUseCase   - PayrollUseCase    |
                      | - AssetUseCase       - GLVoucherUseCase  |
                      +------------------------------------------+
                                           |
                                           v
                      +------------------------------------------+
                      |               DOMAIN LAYER               |
                      | Pure Go structs, Zero float, Zero I/O    |
                      | Double-Entry Invariants, State Machines  |
                      +------------------------------------------+
                                           |
                                           v
                      +------------------------------------------+
                      |       PERSISTENCE ADAPTER (MARIADB)      |
                      | sqlc queries, RepeatableRead tx, DDL     |
                      +------------------------------------------+
```

---

## 2. Invariant Specifications (INV-OPS-01 through INV-OPS-15)

- **INV-OPS-01 (General Voucher Equilibrium)**:
  Every general or operational voucher MUST balance: $\sum \text{DebitAmountVND} - \sum \text{CreditAmountVND} \equiv 0$. Zero tolerance for variance.
- **INV-OPS-02 (Posting Account Leaf Check)**:
  All ledger accounts in voucher lines MUST have `is_leaf = true` and `is_active = true` in the Chart of Accounts (`ErrNonLeafPostingAccount`).
- **INV-OPS-03 (Cost Center Mandatory on Expense/Cost Accounts)**:
  Voucher lines posting to accounts starting with `642`, `641`, `621`, `622`, `627`, or `154` MUST have a non-empty `CostCenterID` (`ErrCostCenterRequired`).
- **INV-OPS-04 (Period Lock Barrier)**:
  Transactions with `VoucherDate <= CompanyProfile.LockDate` are strictly rejected for creation, modification, or deletion (`ErrPeriodLocked`).
- **INV-OPS-05 (Decree 181 Non-Cash Payment Gate)**:
  Cash payments (`CashPayment`) settling purchases/bills $\ge 5,000,000\text{ VND}$ must be flagged and require explicit Chief Accountant confirmation (`NON_CASH_VIOLATION_CONFIRMED`).
- **INV-OPS-06 (Multi-Currency Bank Account Currency Seam)**:
  Bank transactions on bank account linked to `1121` MUST use base currency `VND`. Bank transactions on accounts linked to `1122` MUST use foreign currency (e.g. `USD`), converting to VND via SBV/Commercial bank spot rate:
  $$\text{AmountVND} = (\text{AmountFC} \times \text{ExchangeRate}).\text{RoundBank}(0)$$
- **INV-OPS-07 (VAT Math & Resolution 204 Expiry)**:
  - $\text{VATAmountVND} = (\text{LineNetVND} \times \text{VATRate}).\text{RoundBank}(0)$.
  - 8% VAT rate is strictly valid for dates $2025\text{-}07\text{-}01 \le \text{VoucherDate} \le 2026\text{-}12\text{-}31$. Postings with 8% VAT after $2026\text{-}12\text{-}31$ are rejected with `ErrVATRateExpired`.
- **INV-OPS-08 (Inventory Non-Negative Quantity Rule)**:
  Stock outward movements cannot reduce the physical warehouse inventory of any SKU below zero:
  $$\text{CurrentBalance} - \text{OutwardQty} \ge 0$$
  Attempts to overdraw inventory produce `ErrInsufficientInventory`.
- **INV-OPS-09 (Inventory Moving Weighted Average Costing)**:
  Unit cost for outward movement is calculated at transaction time:
  $$\text{UnitCost}_{\text{out}} = \frac{\text{CurrentInventoryValue}}{\text{CurrentInventoryQty}}$$
  Total outward amount is rounded banker's even: $\text{TotalCost} = (\text{OutwardQty} \times \text{UnitCost}_{\text{out}}).\text{RoundBank}(0)$.
- **INV-OPS-10 (Circular 45 Fixed Asset Threshold)**:
  Fixed assets (TK `211`) require historical cost $\ge 30,000,000\text{ VND}$ and useful life $\ge 12\text{ months}$. Assets $< 30\text{M}$ must be recorded as Tools & Supplies (CCDC TK `153`/`242`).
- **INV-OPS-11 (Tool & Supply 36-Month Allocation Cap)**:
  Tools and supplies allocated via Account `242` cannot exceed 36 months of allocation (`ErrAllocationPeriodExceeded`).
- **INV-OPS-12 (Statutory Payroll Rates 2026)**:
  Statutory employer social contributions: BHXH 17.5%, BHYT 3.0%, BHTN 1.0%, KPCĐ 2.0%.
  Employee deductions: BHXH 8.0%, BHYT 1.5%, BHTN 1.0%.
  Calculated per employee per month, rounded half-even to integer VND.
- **INV-OPS-13 (Voucher Numbering Continuity)**:
  Voucher numbers must follow atomic sequence rules per voucher type and fiscal year (e.g. `PT-2026-000001`, `PC-2026-000001`, `BH-2026-000001`). No gaps permitted.
- **INV-OPS-14 (Idempotent Mutation)**:
  All operational creation endpoints accept an `IdempotencyKey`. Duplicate submissions return the existing record without duplicate financial postings.
- **INV-OPS-15 (Immutability & Soft Delete)**:
  Posted vouchers cannot be physically deleted (`DELETE FROM`). Cancellations require `Status = CANCELLED`, recording an audit entry, and generating reversing GL entries (storno / ghi số âm).

---

## 3. MariaDB DDL Schemas (Migration 00009)

```sql
-- Migration: 00009_create_operational_subledgers.sql

-- 1. General Vouchers & GL Lines (The Master Journal)
CREATE TABLE IF NOT EXISTS vouchers (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    branch_id VARCHAR(36) NULL,
    voucher_no VARCHAR(50) NOT NULL,
    voucher_date DATE NOT NULL,
    posted_date DATE NOT NULL,
    voucher_type ENUM('GENERAL', 'CASH_RECEIPT', 'CASH_PAYMENT', 'BANK_RECEIPT', 'BANK_PAYMENT', 'SALES', 'SALES_RETURN', 'PURCHASE', 'PURCHASE_RETURN', 'STOCK_INWARD', 'STOCK_OUTWARD', 'ASSET', 'PAYROLL') NOT NULL,
    description VARCHAR(500) NOT NULL,
    status ENUM('DRAFT', 'POSTED', 'CANCELLED') NOT NULL DEFAULT 'DRAFT',
    total_debit DECIMAL(18, 0) NOT NULL DEFAULT 0,
    total_credit DECIMAL(18, 0) NOT NULL DEFAULT 0,
    currency_code VARCHAR(3) NOT NULL DEFAULT 'VND',
    exchange_rate DECIMAL(18, 6) NOT NULL DEFAULT 1.000000,
    source_document_id VARCHAR(36) NULL,
    source_document_type VARCHAR(50) NULL,
    idempotency_key VARCHAR(64) NULL UNIQUE,
    created_by VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_voucher_company FOREIGN KEY (company_profile_id) REFERENCES company_profile (id) ON DELETE RESTRICT,
    UNIQUE KEY uq_voucher_no_company (company_profile_id, voucher_type, voucher_no),
    INDEX idx_voucher_date (company_profile_id, voucher_date),
    INDEX idx_voucher_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS voucher_lines (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL,
    line_order INT NOT NULL,
    debit_account_id VARCHAR(36) NOT NULL,
    credit_account_id VARCHAR(36) NOT NULL,
    debit_account_code VARCHAR(20) NOT NULL,
    credit_account_code VARCHAR(20) NOT NULL,
    amount_fc DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    note VARCHAR(255) NULL,
    customer_id VARCHAR(36) NULL,
    vendor_id VARCHAR(36) NULL,
    employee_id VARCHAR(36) NULL,
    item_id VARCHAR(36) NULL,
    warehouse_id VARCHAR(36) NULL,
    cost_center_id VARCHAR(36) NULL,
    expense_item_id VARCHAR(36) NULL,
    invoice_no VARCHAR(50) NULL,
    invoice_date DATE NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_vl_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers (id) ON DELETE CASCADE,
    CONSTRAINT fk_vl_debit_acc FOREIGN KEY (debit_account_id) REFERENCES accounts (id) ON DELETE RESTRICT,
    CONSTRAINT fk_vl_credit_acc FOREIGN KEY (credit_account_id) REFERENCES accounts (id) ON DELETE RESTRICT,
    INDEX idx_vl_accounts (debit_account_code, credit_account_code),
    INDEX idx_vl_counterparty (customer_id, vendor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 2. Cash Subledger (01-TT, 02-TT)
CREATE TABLE IF NOT EXISTS cash_receipts (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL UNIQUE,
    company_profile_id VARCHAR(36) NOT NULL,
    payer_name VARCHAR(150) NOT NULL,
    payer_address VARCHAR(255) NULL,
    reason VARCHAR(255) NOT NULL,
    cash_account_id VARCHAR(36) NOT NULL,
    total_amount_vnd DECIMAL(18, 0) NOT NULL,
    accompanying_documents VARCHAR(100) NULL,
    CONSTRAINT fk_cr_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS cash_payments (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL UNIQUE,
    company_profile_id VARCHAR(36) NOT NULL,
    receiver_name VARCHAR(150) NOT NULL,
    receiver_address VARCHAR(255) NULL,
    reason VARCHAR(255) NOT NULL,
    cash_account_id VARCHAR(36) NOT NULL,
    total_amount_vnd DECIMAL(18, 0) NOT NULL,
    is_non_cash_override BOOLEAN NOT NULL DEFAULT FALSE,
    override_reason VARCHAR(255) NULL,
    accompanying_documents VARCHAR(100) NULL,
    CONSTRAINT fk_cp_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 3. Bank Subledger (Báo có / Báo nợ / UNC)
CREATE TABLE IF NOT EXISTS bank_transactions (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL UNIQUE,
    company_profile_id VARCHAR(36) NOT NULL,
    bank_account_id VARCHAR(36) NOT NULL,
    transaction_type ENUM('CREDIT_ADVICE', 'DEBIT_ADVICE', 'TRANSFER_ORDER') NOT NULL,
    counterparty_account_no VARCHAR(50) NULL,
    counterparty_bank_name VARCHAR(150) NULL,
    counterparty_name VARCHAR(150) NULL,
    bank_reference_no VARCHAR(100) NULL,
    fee_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    vat_fee_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    total_amount_vnd DECIMAL(18, 0) NOT NULL,
    CONSTRAINT fk_bt_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers (id) ON DELETE RESTRICT,
    CONSTRAINT fk_bt_bank FOREIGN KEY (bank_account_id) REFERENCES bank_accounts (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 4. Sales Subledger (Hóa đơn bán hàng)
CREATE TABLE IF NOT EXISTS sales_invoices (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL UNIQUE,
    company_profile_id VARCHAR(36) NOT NULL,
    customer_id VARCHAR(36) NOT NULL,
    invoice_template VARCHAR(20) NOT NULL DEFAULT '1',
    invoice_series VARCHAR(10) NOT NULL,
    invoice_no VARCHAR(20) NOT NULL,
    invoice_date DATE NOT NULL,
    due_date DATE NOT NULL,
    payment_method VARCHAR(20) NOT NULL DEFAULT 'CK',
    einvoice_status ENUM('DRAFT', 'SIGNED', 'CQT_SENT', 'CQT_ACCEPTED', 'CQT_REJECTED') NOT NULL DEFAULT 'DRAFT',
    einvoice_code_cqt VARCHAR(50) NULL,
    subtotal_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    discount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    vat_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    total_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    is_stock_outward_auto BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT fk_si_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers (id) ON DELETE RESTRICT,
    CONSTRAINT fk_si_customer FOREIGN KEY (customer_id) REFERENCES customers (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS sales_invoice_lines (
    id VARCHAR(36) PRIMARY KEY,
    sales_invoice_id VARCHAR(36) NOT NULL,
    line_order INT NOT NULL,
    item_id VARCHAR(36) NOT NULL,
    warehouse_id VARCHAR(36) NOT NULL,
    uom_id VARCHAR(36) NOT NULL,
    quantity DECIMAL(18, 4) NOT NULL,
    unit_price DECIMAL(18, 4) NOT NULL,
    discount_rate DECIMAL(5, 2) NOT NULL DEFAULT 0.00,
    discount_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    net_amount_vnd DECIMAL(18, 0) NOT NULL,
    vat_rate VARCHAR(10) NOT NULL, -- '0%', '5%', '8%', '10%', 'KCT'
    vat_amount_vnd DECIMAL(18, 0) NOT NULL,
    total_amount_vnd DECIMAL(18, 0) NOT NULL,
    cogs_unit_price DECIMAL(18, 4) NOT NULL DEFAULT 0,
    total_cogs_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    revenue_account_id VARCHAR(36) NOT NULL,
    cogs_account_id VARCHAR(36) NOT NULL,
    inventory_account_id VARCHAR(36) NOT NULL,
    vat_account_id VARCHAR(36) NOT NULL,
    CONSTRAINT fk_sil_invoice FOREIGN KEY (sales_invoice_id) REFERENCES sales_invoices (id) ON DELETE CASCADE,
    CONSTRAINT fk_sil_item FOREIGN KEY (item_id) REFERENCES items (id) ON DELETE RESTRICT,
    CONSTRAINT fk_sil_wh FOREIGN KEY (warehouse_id) REFERENCES warehouses (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 5. Purchase Subledger (Hóa đơn mua hàng)
CREATE TABLE IF NOT EXISTS purchase_invoices (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL UNIQUE,
    company_profile_id VARCHAR(36) NOT NULL,
    vendor_id VARCHAR(36) NOT NULL,
    invoice_series VARCHAR(10) NULL,
    invoice_no VARCHAR(20) NOT NULL,
    invoice_date DATE NOT NULL,
    due_date DATE NOT NULL,
    subtotal_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    discount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    vat_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    total_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    is_stock_inward_auto BOOLEAN NOT NULL DEFAULT TRUE,
    is_non_cash_compliant BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT fk_pi_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers (id) ON DELETE RESTRICT,
    CONSTRAINT fk_pi_vendor FOREIGN KEY (vendor_id) REFERENCES vendors (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS purchase_invoice_lines (
    id VARCHAR(36) PRIMARY KEY,
    purchase_invoice_id VARCHAR(36) NOT NULL,
    line_order INT NOT NULL,
    item_id VARCHAR(36) NOT NULL,
    warehouse_id VARCHAR(36) NOT NULL,
    uom_id VARCHAR(36) NOT NULL,
    quantity DECIMAL(18, 4) NOT NULL,
    unit_price DECIMAL(18, 4) NOT NULL,
    net_amount_vnd DECIMAL(18, 0) NOT NULL,
    vat_rate VARCHAR(10) NOT NULL,
    vat_amount_vnd DECIMAL(18, 0) NOT NULL,
    total_amount_vnd DECIMAL(18, 0) NOT NULL,
    asset_account_id VARCHAR(36) NOT NULL,
    vat_account_id VARCHAR(36) NOT NULL,
    CONSTRAINT fk_pil_invoice FOREIGN KEY (purchase_invoice_id) REFERENCES purchase_invoices (id) ON DELETE CASCADE,
    CONSTRAINT fk_pil_item FOREIGN KEY (item_id) REFERENCES items (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 6. Inventory Movements (Phiếu nhập / Phiếu xuất)
CREATE TABLE IF NOT EXISTS stock_movements (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL UNIQUE,
    company_profile_id VARCHAR(36) NOT NULL,
    movement_type ENUM('INWARD', 'OUTWARD') NOT NULL,
    source_type ENUM('PURCHASE', 'SALES', 'MANUFACTURING', 'TRANSFER', 'ADJUSTMENT') NOT NULL,
    warehouse_id VARCHAR(36) NOT NULL,
    delivered_by VARCHAR(100) NULL,
    received_by VARCHAR(100) NULL,
    total_quantity DECIMAL(18, 4) NOT NULL,
    total_amount_vnd DECIMAL(18, 0) NOT NULL,
    CONSTRAINT fk_sm_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers (id) ON DELETE RESTRICT,
    CONSTRAINT fk_sm_wh FOREIGN KEY (warehouse_id) REFERENCES warehouses (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS stock_movement_lines (
    id VARCHAR(36) PRIMARY KEY,
    movement_id VARCHAR(36) NOT NULL,
    line_order INT NOT NULL,
    item_id VARCHAR(36) NOT NULL,
    uom_id VARCHAR(36) NOT NULL,
    quantity DECIMAL(18, 4) NOT NULL,
    unit_cost DECIMAL(18, 4) NOT NULL,
    total_cost_vnd DECIMAL(18, 0) NOT NULL,
    batch_number VARCHAR(50) NULL,
    expiry_date DATE NULL,
    CONSTRAINT fk_sml_movement FOREIGN KEY (movement_id) REFERENCES stock_movements (id) ON DELETE CASCADE,
    CONSTRAINT fk_sml_item FOREIGN KEY (item_id) REFERENCES items (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 7. Fixed Assets & Tool Allocations (Ghi tăng TSCĐ / Xuất dùng CCDC)
CREATE TABLE IF NOT EXISTS asset_transactions (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL UNIQUE,
    company_profile_id VARCHAR(36) NOT NULL,
    transaction_type ENUM('INCREASE_ASSET', 'DEPRECIATION_RUN', 'DISPOSAL', 'TOOL_ALLOCATION') NOT NULL,
    asset_id VARCHAR(36) NULL,
    tool_id VARCHAR(36) NULL,
    original_cost DECIMAL(18, 0) NOT NULL,
    accumulated_depreciation DECIMAL(18, 0) NOT NULL DEFAULT 0,
    allocation_period_months INT NOT NULL,
    monthly_amount_vnd DECIMAL(18, 0) NOT NULL,
    CONSTRAINT fk_at_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 8. Payroll Subledger (Bảng lương & Chấm công)
CREATE TABLE IF NOT EXISTS payroll_runs (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL UNIQUE,
    company_profile_id VARCHAR(36) NOT NULL,
    period_month INT NOT NULL,
    period_year INT NOT NULL,
    total_gross_salary DECIMAL(18, 0) NOT NULL,
    total_insurance_company DECIMAL(18, 0) NOT NULL,
    total_insurance_employee DECIMAL(18, 0) NOT NULL,
    total_pit_tax DECIMAL(18, 0) NOT NULL,
    total_net_salary DECIMAL(18, 0) NOT NULL,
    CONSTRAINT fk_pr_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers (id) ON DELETE RESTRICT,
    UNIQUE KEY uq_payroll_period (company_profile_id, period_year, period_month)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS payroll_run_items (
    id VARCHAR(36) PRIMARY KEY,
    payroll_run_id VARCHAR(36) NOT NULL,
    employee_id VARCHAR(36) NOT NULL,
    base_salary DECIMAL(18, 0) NOT NULL,
    working_days DECIMAL(5, 2) NOT NULL,
    actual_salary DECIMAL(18, 0) NOT NULL,
    allowances DECIMAL(18, 0) NOT NULL DEFAULT 0,
    gross_income DECIMAL(18, 0) NOT NULL,
    bhxh_employee DECIMAL(18, 0) NOT NULL,
    bhyt_employee DECIMAL(18, 0) NOT NULL,
    bhtn_employee DECIMAL(18, 0) NOT NULL,
    pit_deduction_personal DECIMAL(18, 0) NOT NULL,
    pit_deduction_dependents DECIMAL(18, 0) NOT NULL,
    pit_tax_amount DECIMAL(18, 0) NOT NULL,
    net_salary DECIMAL(18, 0) NOT NULL,
    bhxh_company DECIMAL(18, 0) NOT NULL,
    bhyt_company DECIMAL(18, 0) NOT NULL,
    bhtn_company DECIMAL(18, 0) NOT NULL,
    kpcd_company DECIMAL(18, 0) NOT NULL,
    cost_account_id VARCHAR(36) NOT NULL,
    cost_center_id VARCHAR(36) NULL,
    CONSTRAINT fk_pri_run FOREIGN KEY (payroll_run_id) REFERENCES payroll_runs (id) ON DELETE CASCADE,
    CONSTRAINT fk_pri_employee FOREIGN KEY (employee_id) REFERENCES employees (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

---

## 4. Implementation Slices & Execution Plan

To guarantee modular reliability, Phase 5 will be implemented incrementally across **8 TDD vertical slices**:

- **Slice 5.1: General Ledger Core Voucher Engine (`internal/domain/gl`, `db/migrations/00009`, `internal/usecase/gl`)**
  - DDL setup, `Voucher` + `VoucherLines`, equilibrium check, leaf account check, lock date check, atomic sequencing.
- **Slice 5.2: Cash Subledger (`internal/domain/cash`, `internal/usecase/cash`)**
  - `CashReceipt` (01-TT) & `CashPayment` (02-TT), Decree 181 $\ge 5\text{M}$ non-cash gate, GL auto-journaling.
- **Slice 5.3: Bank Subledger (`internal/domain/bank`, `internal/usecase/bank`)**
  - `BankTransaction` (Báo có / Báo nợ / UNC), currency alignment (1121 vs 1122), fee VAT lines.
- **Slice 5.4: Sales Subledger & Auto-COGS (`internal/domain/sales`, `internal/usecase/sales`)**
  - `SalesInvoice` + lines, VAT engine (0%, 5%, 8%, 10%, KCT), Resolution 204 expiry check, auto-outward linking.
- **Slice 5.5: Purchase Subledger (`internal/domain/purchase`, `internal/usecase/purchase`)**
  - `PurchaseInvoice` + lines, Decree 181 non-cash compliance, auto-inward linking.
- **Slice 5.6: Inventory Subledger & Costing (`internal/domain/inventory`, `internal/usecase/inventory`)**
  - `StockInward` (01-VT) & `StockOutward` (02-VT), Moving Weighted Average costing, non-negative inventory barrier.
- **Slice 5.7: Fixed Assets & CCDC Tools (`internal/domain/asset`, `internal/usecase/asset`)**
  - Circular 45 30M threshold check, monthly straight-line depreciation run, 36-month CCDC allocation limit.
- **Slice 5.8: Payroll & Timesheet Engine (`internal/domain/payroll`, `internal/usecase/payroll`)**
  - `PayrollRun` + employee items, statutory 2026 contribution rates (10.5% EE / 21.5% ER), PIT withholding, GL dual-posting.

---

## 5. Verification & Test Plan

- **Unit Tests (`internal/domain/...`)**: Table-driven tests verifying equilibrium, tax calculations, statutory rates, and state machines ($\ge 95.0\%$ coverage).
- **Integration Tests (`internal/adapter/mariadb/repository/...`)**: Live MariaDB 12.3 testing with `RepeatableRead` transactions, foreign key integrity, and concurrent voucher number allocation.
- **Regression Tests**: `go test ./...` across all 20 packages in `fingo`.
