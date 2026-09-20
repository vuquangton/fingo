# SPEC-07: INITIALIZATION & CUTOVER (OPENING BALANCES - LAYER 3)
## Standard Authority: `FINGO-SPEC-OPENING-2026-V1`
### Domain Seam: `internal/domain/opening` | Usecase Seam: `internal/usecase/opening` | Persistence: `internal/adapter/mariadb`
**Role Authority**: BA Lead (20+ Years Enterprise Architecture) & Chief Accountant (20+ Years VAS/Circular 99/2025/TT-BTC)  
**Standard Compliance**:
- Luật Kế toán số 88/2015/QH13 (Điều 10, 12, 13, 50, 52)
- Thông tư 99/2025/TT-BTC (Hiệu lực 01/01/2026) & Thông tư 133/2016/TT-BTC / Thông tư 200/2014/TT-BTC
- Thông tư 45/2013/TT-BTC & Thông tư 147/2016/TT-BTC (Chế độ quản lý, sử dụng và trích khấu hao TSCĐ)
- Nghị định 123/2020/NĐ-CP & Thông tư 32/2025/TT-BTC (Hóa đơn điện tử - Quản lý hóa đơn mở đầu kỳ)
- Nghị định 181/2025/NĐ-CP & Luật số 48/2024/QH15 (Thanh toán không dùng tiền mặt)
- `FINGO-QA-STRATEGY-2026-V1` (Zero-Float Policy, Byte-Identical Reproducibility)

---

# 1. Production Readiness Audit & Gap Analysis

### Verdict: **FAILED (CANNOT OPERATE IN PROD AS-IS)**

| Dimension | Current State | Required Production State | Severity |
| :--- | :--- | :--- | :--- |
| **Persistence (MariaDB)** | 0 tables. Stubs only. | 6 normalized tables with tenant isolation, audit stamps, and unique integrity keys. | **CRITICAL (P1)** |
| **Trial Balance Equilibrium** | Basic slice comparison. | Strict double-entry equilibrium invariant ($\sum \text{Debit} \equiv \sum \text{Credit}$) across all balance sheet accounts (111-421). | **CRITICAL (P1)** |
| **Subledger Reconciliation** | None. | Cross-layer reconciliation engine: Subledgers (131, 331, 15x, 211/214) must reconcile 100.00% to General Ledger opening balances. | **CRITICAL (P1)** |
| **Multi-Currency Opening** | VND only. | Foreign currency balances (USD, EUR) tracking `AmountFC`, historical exchange rate, and base VND book value. | **CRITICAL (P1)** |
| **AR/AP Invoice-Level Aging** | Lump sum amounts only. | Invoice-level breakdown (Invoice No, Date, Original Amount, Remaining Balance) for FIFO settlement. | **HIGH (P2)** |
| **Fixed Asset Depreciation** | Absent. | Circular 45/2013 schedule tracking historical cost, accumulated depreciation, remaining life, and monthly allocation. | **HIGH (P2)** |
| **Cutover Locking & Audit** | In-memory boolean flag. | State machine (`DRAFT` $\to$ `VALIDATED` $\to$ `COMMITTED` $\to$ `LOCKED`) with Chief Accountant electronic sign-off. | **HIGH (P2)** |

---

# 2. Statutory Legal & Accounting Framework

```
+----------------------------------------------------------------------------------------------------+
|                                    STATUTORY CUTOVER FRAMEWORK                                     |
|                                                                                                    |
|    [LUẬT KẾ TOÁN 88/2015/QH13]          [THÔNG TƯ 99/2025/TT-BTC]        [THÔNG TƯ 45/2013/TT-BTC] |
|    - Điều 13: Cấm số dư khống           - Bảng cân đối phát sinh mở      - Nguyên giá TSCĐ >= 30M  |
|    - Cân đối tuyệt đối Nợ = Có          - Tài khoản lưỡng tính 131/331   - Khung thời gian khấu hao|
|                                                                                                    |
|    [NGHỊ ĐỊNH 123/2020/NĐ-CP]           [VAS 10 / CHÊNH LỆCH TỶ GIÁ]     [CIRCULAR 133 & 200]      |
|    - Hóa đơn mở đầu kỳ chưa TT          - Tỷ giá ghi sổ công nợ NT       - Sổ chi tiết tồn kho 15x |
|    - Ký hiệu, Số HĐ, Ngày HĐ            - Đánh giá lại ngoại tệ 111/112  - Thẻ kho và giá trị tồn  |
+----------------------------------------------------------------------------------------------------+
```

### 2.1 The Seven Statutory Invariants of Layer 3
1. **INV-OPEN-01 (General Ledger Double-Entry Equilibrium)**:
   $$\sum_{a \in \text{COA}} \text{OpeningDebit}(a) - \sum_{a \in \text{COA}} \text{OpeningCredit}(a) \equiv 0$$
   Nominal P&L accounts (511, 632, 641, 642, 711, 811, 911) must have ZERO opening balance.
2. **INV-OPEN-02 (Customer AR Subledger Reconciliation)**:
   $$\sum_{c \in \text{Customers}} \text{Debit}(c) \equiv \text{OpeningDebit}(\text{TK 131}) \quad \text{and} \quad \sum_{c \in \text{Customers}} \text{Credit}(c) \equiv \text{OpeningCredit}(\text{TK 131})$$
   Hermaphroditic rule: Debit (receivable) and Credit (advance customer payments) must be segregated; no netting allowed.
3. **INV-OPEN-03 (Vendor AP Subledger Reconciliation)**:
   $$\sum_{v \in \text{Vendors}} \text{Credit}(v) \equiv \text{OpeningCredit}(\text{TK 331}) \quad \text{and} \quad \sum_{v \in \text{Vendors}} \text{Debit}(v) \equiv \text{OpeningDebit}(\text{TK 331})$$
   Debit (advance payments to suppliers) and Credit (unpaid bills) must be segregated.
4. **INV-OPEN-04 (Inventory Valuation Reconciliation)**:
   $$\sum_{w \in \text{Warehouses}, i \in \text{Items}} (\text{Quantity}_{w,i} \times \text{UnitCost}_{w,i}) \equiv \text{OpeningDebit}(\text{TK } 152 + 153 + 155 + 156)$$
   Negative inventory quantities or unit costs $\le 0$ are strictly prohibited (`ErrInvalidInventoryBalance`).
5. **INV-OPEN-05 (Fixed Asset Schedule Equilibrium)**:
   $$\sum \text{Asset.OriginalCost} \equiv \text{OpeningDebit}(\text{TK 211})$$
   $$\sum \text{Asset.AccumulatedDepreciation} \equiv \text{OpeningCredit}(\text{TK 214})$$
   Net Book Value $\equiv \text{OriginalCost} - \text{AccumulatedDepreciation} \ge 0$.
6. **INV-OPEN-06 (Multi-Currency Valuation Integrity)**:
   For foreign currency accounts (e.g. `1112`, `1122`, `1312`, `3312`):
   $$\text{AmountVND} = (\text{AmountFC} \times \text{ExchangeRate}).\text{RoundBank}(0)$$
7. **INV-OPEN-07 (Cutover Commitment Immutability)**:
   Once the Opening Batch is transitioned to `COMMITTED` or `LOCKED`, all opening records become permanently read-only. Modification requires formal reversal by Chief Accountant.

---

# 3. Architecture & Domain Topology

```
+----------------------------------------------------------------------------------------------------+
|                                    ACCOUNTING SPINE (LAYER 1)                                      |
|            Currencies             |        Chart of Accounts        |         Fiscal Year          |
+-----------------------------------+---------------------------------+------------------------------+
                                                     |
                                                     | Validates Leaf & Active COA
                                                     v
+----------------------------------------------------------------------------------------------------+
|                                  CATALOGS MASTER DATA (LAYER 2)                                    |
|   UnitOfMeasure | Warehouses (15x) | BankAccounts (112) | Customers (131) | Vendors (331) | Items  |
+----------------------------------------------------+-----------------------------------------------+
                                                     |
                                                     | References Catalog Dimensions
                                                     v
+----------------------------------------------------------------------------------------------------+
|                             INITIALIZATION & CUTOVER ENGINE (LAYER 3)                              |
|                                                                                                    |
|                                    +-----------------------+                                       |
|                                    |     OpeningBatch      |                                       |
|                                    | - CompanyProfileID    |                                       |
|                                    | - AsOfDate (2025-12-31|                                       |
|                                    | - Status: COMMITTED   |                                       |
|                                    +-----------+-----------+                                       |
|                                                |                                                   |
|         +-------------------+------------------+-------------------+------------------+            |
|         |                   |                  |                   |                  |            |
|         v                   v                  v                   v                  v            |
|  +--------------+    +--------------+   +--------------+    +--------------+   +--------------+    |
|  |AccountOpening|    |CustomerOpen  |   | VendorOpen   |    |InventoryOpen |   | FixedAsset   |    |
|  |Balances (GL) |    |Balances (131)|   |Balances (331)|    |Balances (15x)|   |Opening (211) |    |
|  +--------------+    +--------------+   +--------------+    +--------------+   +--------------+    |
|         |                   |                  |                   |                  |            |
|         +-------------------+------------------+-------------------+------------------+            |
|                                                |                                                   |
|                                                v                                                   |
|                               +---------------------------------+                                  |
|                               | Reconciliation Engine (INV-01..5|                                  |
|                               | - Trial Balance balanced        |                                  |
|                               | - Subledgers match GL           |                                  |
|                               +---------------------------------+                                  |
+----------------------------------------------------------------------------------------------------+
                                                 |
                                                 v
+----------------------------------------------------------------------------------------------------+
|                                 TRANSACTIONAL LEDGER (LAYER 4)                                     |
|    General Journal Posting | Vouchers (111, 112, 131, 331, 15x) | Period-End Financial Reports     |
+----------------------------------------------------------------------------------------------------+
```

---

# 4. Database Schema Specification (DDL)

```sql
-- Migration: 00008_create_opening_balances.sql

CREATE TABLE IF NOT EXISTS opening_batches (
    id VARCHAR(36) PRIMARY KEY,
    company_profile_id VARCHAR(36) NOT NULL,
    as_of_date DATE NOT NULL,
    status ENUM('DRAFT', 'VALIDATED', 'COMMITTED', 'LOCKED') NOT NULL DEFAULT 'DRAFT',
    total_debit DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    total_credit DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    committed_at TIMESTAMP NULL,
    committed_by VARCHAR(36) NULL,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_opening_batch_company_date UNIQUE (company_profile_id, as_of_date),
    CONSTRAINT fk_opening_batch_company FOREIGN KEY (company_profile_id) REFERENCES company_profile(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS account_opening_balances (
    id VARCHAR(36) PRIMARY KEY,
    batch_id VARCHAR(36) NOT NULL,
    account_id VARCHAR(36) NOT NULL,
    currency_code VARCHAR(3) NOT NULL DEFAULT 'VND',
    debit_amount_fc DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    credit_amount_fc DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    exchange_rate DECIMAL(18, 6) NOT NULL DEFAULT 1.000000,
    debit_amount_vnd DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    credit_amount_vnd DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_account_opening UNIQUE (batch_id, account_id, currency_code),
    CONSTRAINT fk_acc_open_batch FOREIGN KEY (batch_id) REFERENCES opening_batches(id) ON DELETE CASCADE,
    CONSTRAINT fk_acc_open_acc FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS customer_opening_balances (
    id VARCHAR(36) PRIMARY KEY,
    batch_id VARCHAR(36) NOT NULL,
    customer_id VARCHAR(36) NOT NULL,
    invoice_no VARCHAR(50) NULL,
    invoice_date DATE NULL,
    due_date DATE NULL,
    currency_code VARCHAR(3) NOT NULL DEFAULT 'VND',
    debit_amount_fc DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    credit_amount_fc DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    exchange_rate DECIMAL(18, 6) NOT NULL DEFAULT 1.000000,
    debit_amount_vnd DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    credit_amount_vnd DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_cust_open_batch FOREIGN KEY (batch_id) REFERENCES opening_batches(id) ON DELETE CASCADE,
    CONSTRAINT fk_cust_open_cust FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS vendor_opening_balances (
    id VARCHAR(36) PRIMARY KEY,
    batch_id VARCHAR(36) NOT NULL,
    vendor_id VARCHAR(36) NOT NULL,
    bill_no VARCHAR(50) NULL,
    bill_date DATE NULL,
    due_date DATE NULL,
    currency_code VARCHAR(3) NOT NULL DEFAULT 'VND',
    debit_amount_fc DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    credit_amount_fc DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    exchange_rate DECIMAL(18, 6) NOT NULL DEFAULT 1.000000,
    debit_amount_vnd DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    credit_amount_vnd DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_vend_open_batch FOREIGN KEY (batch_id) REFERENCES opening_batches(id) ON DELETE CASCADE,
    CONSTRAINT fk_vend_open_vend FOREIGN KEY (vendor_id) REFERENCES vendors(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS inventory_opening_balances (
    id VARCHAR(36) PRIMARY KEY,
    batch_id VARCHAR(36) NOT NULL,
    warehouse_id VARCHAR(36) NOT NULL,
    item_id VARCHAR(36) NOT NULL,
    uom_id VARCHAR(36) NOT NULL,
    quantity DECIMAL(18, 4) NOT NULL DEFAULT 0.0000,
    unit_cost DECIMAL(18, 4) NOT NULL DEFAULT 0.0000,
    total_amount_vnd DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    batch_number VARCHAR(50) NULL,
    expiry_date DATE NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_inventory_open UNIQUE (batch_id, warehouse_id, item_id, batch_number),
    CONSTRAINT fk_inv_open_batch FOREIGN KEY (batch_id) REFERENCES opening_batches(id) ON DELETE CASCADE,
    CONSTRAINT fk_inv_open_wh FOREIGN KEY (warehouse_id) REFERENCES warehouses(id) ON DELETE RESTRICT,
    CONSTRAINT fk_inv_open_item FOREIGN KEY (item_id) REFERENCES items(id) ON DELETE RESTRICT,
    CONSTRAINT fk_inv_open_uom FOREIGN KEY (uom_id) REFERENCES unit_of_measures(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS asset_opening_balances (
    id VARCHAR(36) PRIMARY KEY,
    batch_id VARCHAR(36) NOT NULL,
    asset_code VARCHAR(50) NOT NULL,
    asset_name VARCHAR(255) NOT NULL,
    asset_account_id VARCHAR(36) NOT NULL,       -- TK 211x
    depreciation_account_id VARCHAR(36) NOT NULL, -- TK 214x
    cost_account_id VARCHAR(36) NOT NULL,         -- TK 627/641/642
    department_id VARCHAR(36) NULL,
    acquisition_date DATE NOT NULL,
    start_depreciation_date DATE NOT NULL,
    original_cost DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    accumulated_depreciation DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    net_book_value DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    useful_life_months INT NOT NULL,
    remaining_life_months INT NOT NULL,
    monthly_depreciation DECIMAL(18, 2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT uk_asset_open_code UNIQUE (batch_id, asset_code),
    CONSTRAINT fk_asset_open_batch FOREIGN KEY (batch_id) REFERENCES opening_batches(id) ON DELETE CASCADE,
    CONSTRAINT fk_asset_open_asset_acc FOREIGN KEY (asset_account_id) REFERENCES accounts(id) ON DELETE RESTRICT,
    CONSTRAINT fk_asset_open_depr_acc FOREIGN KEY (depreciation_account_id) REFERENCES accounts(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

---

# 5. Core Reconciliation Algorithms & Business Logic

### 5.1 Trial Balance Equilibrium Check
```go
func (b *OpeningBatch) ValidateEquilibrium() error {
    totalDebit := decimal.Zero
    totalCredit := decimal.Zero

    for _, acc := range b.Accounts {
        // Disallow nominal P&L accounts (Class 5, 6, 7, 8, 9)
        prefix := acc.AccountCode[0:1]
        if prefix == "5" || prefix == "6" || prefix == "7" || prefix == "8" || prefix == "9" {
            if !acc.DebitAmountVND.IsZero() || !acc.CreditAmountVND.IsZero() {
                return fmt.Errorf("%w: P&L account %s cannot have opening balance", ErrInvalidOpeningAccount, acc.AccountCode)
            }
        }
        totalDebit = totalDebit.Add(acc.DebitAmountVND)
        totalCredit = totalCredit.Add(acc.CreditAmountVND)
    }

    if !totalDebit.Equal(totalCredit) {
        diff := totalDebit.Sub(totalCredit).Abs()
        return fmt.Errorf("%w: total debit %s != total credit %s (difference: %s VND)", 
            ErrTrialBalanceUnbalanced, totalDebit.String(), totalCredit.String(), diff.String())
    }
    return nil
}
```

### 5.2 Subledger Reconciliation Check
```go
func (b *OpeningBatch) ValidateSubledgerReconciliation() error {
    // 1. Reconcile Customer AR (TK 131)
    custTotalDebit := decimal.Zero
    custTotalCredit := decimal.Zero
    for _, c := range b.Customers {
        custTotalDebit = custTotalDebit.Add(c.DebitAmountVND)
        custTotalCredit = custTotalCredit.Add(c.CreditAmountVND)
    }
    gl131Debit, gl131Credit := b.GetGLBalance("131")
    if !custTotalDebit.Equal(gl131Debit) || !custTotalCredit.Equal(gl131Credit) {
        return fmt.Errorf("%w: Customer subledger (Dr: %s, Cr: %s) does not match TK 131 (Dr: %s, Cr: %s)",
            ErrSubledgerReconciliationFailed, custTotalDebit, custTotalCredit, gl131Debit, gl131Credit)
    }

    // 2. Reconcile Vendor AP (TK 331)
    vendTotalDebit := decimal.Zero
    vendTotalCredit := decimal.Zero
    for _, v := range b.Vendors {
        vendTotalDebit = vendTotalDebit.Add(v.DebitAmountVND)
        vendTotalCredit = vendTotalCredit.Add(v.CreditAmountVND)
    }
    gl331Debit, gl331Credit := b.GetGLBalance("331")
    if !vendTotalDebit.Equal(gl331Debit) || !vendTotalCredit.Equal(gl331Credit) {
        return fmt.Errorf("%w: Vendor subledger (Dr: %s, Cr: %s) does not match TK 331 (Dr: %s, Cr: %s)",
            ErrSubledgerReconciliationFailed, vendTotalDebit, vendTotalCredit, gl331Debit, gl331Credit)
    }

    // 3. Reconcile Inventory (TK 152, 153, 155, 156)
    invTotalVND := decimal.Zero
    for _, item := range b.Inventory {
        invTotalVND = invTotalVND.Add(item.TotalAmountVND)
    }
    glInvDebit := b.GetGLInventoryTotal()
    if !invTotalVND.Equal(glInvDebit) {
        return fmt.Errorf("%w: Inventory subledger (%s VND) does not match TK 15x (%s VND)",
            ErrSubledgerReconciliationFailed, invTotalVND, glInvDebit)
    }

    // 4. Reconcile Fixed Assets (TK 211 & 214)
    assetTotalCost := decimal.Zero
    assetTotalDepr := decimal.Zero
    for _, a := range b.Assets {
        assetTotalCost = assetTotalCost.Add(a.OriginalCost)
        assetTotalDepr = assetTotalDepr.Add(a.AccumulatedDepreciation)
    }
    gl211Debit, _ := b.GetGLBalance("211")
    _, gl214Credit := b.GetGLBalance("214")
    if !assetTotalCost.Equal(gl211Debit) || !assetTotalDepr.Equal(gl214Credit) {
        return fmt.Errorf("%w: Fixed asset schedule (Cost: %s, Depr: %s) does not match TK 211/214 (Dr 211: %s, Cr 214: %s)",
            ErrSubledgerReconciliationFailed, assetTotalCost, assetTotalDepr, gl211Debit, gl214Credit)
    }

    return nil
}
```

---

# 6. Verification Plan & Test Strategy

1. **Unit Tests (`internal/domain/opening`)**:
   - Verify `ValidateEquilibrium` rejects unbalanced batches.
   - Verify rejection of nominal P&L accounts (Class 5-9).
   - Verify hermaphroditic customer and vendor subledger balance matching.
   - Verify inventory subledger value matches sum of accounts 151-158.
   - Verify fixed asset cost/depreciation matches accounts 211/214.
   - Target line coverage: **$\ge 95.0\%$**.
2. **Integration Tests (`internal/adapter/mariadb`)**:
   - Execute against live MariaDB 12.3 via Goose migration 00008.
   - Verify batch save, fetch, status update, and child row cascades.
3. **Usecase Tests (`internal/usecase/opening`)**:
   - `CreateOpeningBatch`: Idempotency, draft creation.
   - `ImportAccountOpeningBalances`: Leaf-only account validation (`INV-SPINE-01`).
   - `ImportCustomerOpeningBalances`: Counterparty validation + invoice tracking.
   - `CommitOpeningBatch`: Full reconciliation gate; locks batch and logs audit entry.
   - Target line coverage: **$\ge 85.0\%$**.
