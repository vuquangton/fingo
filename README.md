# Battle-Tested On-Premise Enterprise Accounting Desktop Application (.NET 10 LTS)

Production-grade architectural scaffolding and complete module contracts for a high-reliability, on-premise Desktop Accounting System running on **.NET 10 (LTS)**, fully compliant with the Vietnamese Accounting System (VAS - Circular 200/2014/TT-BTC & Circular 133/2016/TT-BTC).

---

## 1. Solution Architecture

```
d:\accounting\
├── src\
│   ├── Core\
│   │   ├── Accounting.Domain\                 # Domain-Driven Design (Aggregates, Value Objects, Domain Events, Rules, Exceptions)
│   │   └── Accounting.Application\            # Clean Architecture CQRS (MediatR Handlers, DTOs, FluentValidation, Pipeline Behaviors)
│   ├── Infrastructure\
│   │   ├── Accounting.Infrastructure.Persistence\ # EF Core 10 (Dynamic SQLite/PostgreSQL provider), Dapper Connection Factory, DbInitializer
│   │   └── Accounting.Infrastructure.Compliance\  # VAS BCTC (B01-DN, B02-DN, B03-DN), Tax Engine (PIT, VAT), HTKK XML Formatter
│   └── UI\
│       └── Accounting.WpfApp\                 # Keyboard-first WPF Desktop App (.NET 10), CommunityToolkit.Mvvm, Virtualized UI Grids
└── tests\
    └── Accounting.Domain.Tests\               # xUnit unit & integration test suite verifying double-entry, costing, PIT, seeding
```

---

## 2. Dynamic Database Switching (Zero-Churn Database Strategy)

The system supports seamless switching between embedded SQLite (for single-user / branch stations / offline setups) and client-server PostgreSQL (for multi-user enterprise deployments) via `appsettings.json` without requiring any code changes.

```json
{
  "DatabaseProvider": "Sqlite",
  "ConnectionStrings": {
    "SqliteConnection": "Data Source=accounting.db",
    "PostgreSqlConnection": "Host=localhost;Port=5432;Database=accounting;Username=postgres;Password=postgres"
  }
}
```

* **SQLite v3+:** Zero-configuration embedded storage. Automatically initializes schema and seeds circular 200/133 Chart of Accounts on first launch.
* **PostgreSQL v16+:** Switch `DatabaseProvider` to `"PostgreSql"` to route EF Core and Dapper to enterprise PostgreSQL instances.

---

## 3. Core Accounting Modules

1. **General Ledger & Double-Entry Journal Engine (Sổ Cái & Định Khoản):**
   - Tiered Chart of Accounts (COA) compliant with Circulars 200 & 133 (TK 111, 112, 131, 133, 152, 156, 211, 214, 242, 331, 333, 334, 338, 411, 421, 511, 632, 641, 642, 811, 911).
   - Strict `SUM(Debit) == SUM(Credit)` validation before persistence.
   - Voucher lifecycle (`Draft` -> `Pending` -> `Approved` -> `Posted`).
   - Immutable posted vouchers with automated reversing entries (`CreateReversal`).
   - Fiscal period soft/hard locking and automated revenue/expense clearance (`TK 911`).

2. **Cash & Treasury (Quỹ Tiền Mặt & Tiền Gửi Ngân Hàng):**
   - Receipts (Phiếu thu), Payments (Phiếu chi), Cash counting log (Biên bản kiểm kê quỹ).
   - Multi-bank account tracking, Payment Orders (Ủy nhiệm chi - UNC), Bank statement reconciliation (Đối chiếu sổ phụ).

3. **Purchasing & Accounts Payable - AP (Mua Hàng & Phải Trả):**
   - Purchase orders, Goods receipts (Phiếu nhập kho), Three-way matching (PO vs. GRN vs. Invoice), Vendor aging analysis.

4. **Sales & Accounts Receivable - AR (Bán Hàng & Phải Thu):**
   - Quotations, Delivery notes (Phiếu xuất kho), Sales invoices, Dynamic credit limit enforcement, Customer aging schedules.

5. **Inventory Valuation & Logistics (Kho & Giá Vốn):**
   - Multi-warehouse topology, warehouse transfers (Điều chuyển kho), stocktaking (Kiểm kê).
   - Valuation engines: FIFO, Perpetual Moving Average (Bình quân gia quyền tức thời), Monthly Periodic Weighted Average.

6. **Fixed Assets & Tool Amortization (Tài Sản Cố Định & CCDC):**
   - Fixed asset registry, Straight-line & Accelerated depreciation, Monthly expense allocation for TK 242 (Chi phí trả trước).

7. **Payroll & Statutory Deductions (Tiền Lương & Trích Theo Lương):**
   - Employee timesheet aggregation, Statutory insurance (Employee: 8% BHXH, 1.5% BHYT, 1% BHTN; Employer: 17.5% BHXH, 3% BHYT, 1% BHTN, 2% Trade Union).
   - Progressive Personal Income Tax (PIT) withholding calculation with statutory deductions (11M personal, 4.4M dependent).

8. **Tax & Statutory Financial Statements (Thuế & BCTC):**
   - B01-DN: Báo cáo tình hình tài chính (Statement of Financial Position).
   - B02-DN: Báo cáo kết quả hoạt động kinh doanh (Income Statement).
   - B03-DN: Báo cáo lưu chuyển tiền tệ (Cash Flows Statement - Direct & Indirect).
   - Input/Output VAT ledgers and General Department of Taxation XML submission schema.

9. **System, Security & Immutable Audit Trail (Hệ Thống & Nhật Ký):**
   - Role-Based Access Control (RBAC).
   - Append-only immutable audit trail capturing user, machine fingerprint, UTC timestamp, and delta payloads.
   - Online database backup (SQLite VACUUM INTO / PostgreSQL pg_dump wrapper).

---

## 4. Keyboard-First Desktop UX

| Shortcut | Action | Vietnamese Description |
|---|---|---|
| `F2` | New Voucher | Tạo mới chứng từ kế toán |
| `F3` | Search / Find | Tìm kiếm danh mục / chứng từ |
| `F5` | Refresh Data | Nạp lại dữ liệu từ cơ sở dữ liệu |
| `F8` | Post Voucher | Ghi sổ kế toán |
| `F9` | Unpost Voucher | Bỏ ghi sổ kế toán |
| `F12` / `Ctrl+S` | Save | Lưu chứng từ |
| `Enter` / `Tab` | Next Field / Commit Row | Chuyển ô nhập liệu / Nhập dòng chi tiết |
| `Esc` | Cancel | Đóng hộp thoại / Hủy thao tác |

---

## 5. Verification & Test Execution

```bash
# Build entire solution across all 6 projects
dotnet build Accounting.slnx -c Release

# Run automated test suite (11 unit & integration tests)
dotnet test Accounting.slnx -c Release
```
