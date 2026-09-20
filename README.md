# FinGo - SME Accounting Desktop Application

On-premise enterprise accounting desktop application built with **Go (v1.25+)**, **Wails v2**, **Vue 3 / TypeScript**, and **MariaDB 12.3**. Designed using strict **Onion Architecture** (Modular Monolith) with zero external dependency leaks in domain rules. Fully aligned with Vietnamese Accounting Standards (VAS - Circular 133/2016/TT-BTC & Circular 200/2014/TT-BTC) and electronic invoicing standards (Decree 123/2020/NĐ-CP).

---

## 1. Solution Architecture (Onion Architecture)

```text
d:\accounting\
├── .agents/                 # AI skill runbooks & guidelines (5W1H decision matrix)
├── .codegraph/              # Fast AST knowledge graph for codebase symbol navigation
├── GEMINI.md                # Permanent project rules, context, and operational commands
└── fingo/                   # Core application
    ├── .env                 # Local MariaDB configuration
    ├── go.mod               # Go module: fingo (Go 1.25+)
    ├── wails.json           # Wails desktop configuration (Output: FinGo.exe)
    ├── sqlc.yaml            # Compile-time safe SQL code generation config
    │
    ├── main.go              # Wails application entry point & lifecycle hooks
    ├── app.go               # Seam between frontend and backend use cases
    │
    ├── internal/            # Private Application Architecture
    │   ├── domain/          # THE CORE (Zero external dependencies, pure Go structs)
    │   │   ├── system/      # User, Role, Permission, CompanyProfile, AuditLog
    │   │   ├── catalog/     # Customer, Vendor, Item, BankAccount, Warehouse
    │   │   ├── gl/          # Voucher, VoucherLine, Double-entry validation
    │   │   ├── cash/        # CashReceipt (111), CashPayment (111), BankTransaction (112)
    │   │   ├── sales/       # SalesInvoice, SalesInvoiceLine, VAT math
    │   │   ├── purchase/    # PurchaseInvoice, PurchaseInvoiceLine
    │   │   ├── inventory/   # StockInward, StockOutward, StockMovementLine
    │   │   ├── asset/       # FixedAsset (211/214), Monthly straight-line depreciation
    │   │   ├── opening/     # Account, Customer, Vendor, Inventory Opening Balances
    │   │   ├── tax/         # VATDeclaration (Form 01/GTGT Box 40/43), EInvoice (NĐ123)
    │   │   ├── closing/     # PeriodClosing (511 -> 911 <- 632, 642 -> 421), Lock Date
    │   │   ├── report/      # TrialBalance (Opening + Movement = Closing), B01/B02/B03
    │   │   ├── payroll/     # Employee, Statutory insurance (10.5% / 21.5%), Net salary
    │   │   └── costing/     # ProductionCostCard (WIP allocation, Unit cost)
    │   │
    │   ├── usecase/         # APPLICATION SERVICES (Flow orchestration & transaction boundaries)
    │   │
    │   └── adapter/         # INFRASTRUCTURE & EXTERNAL ADAPTERS
    │       ├── mariadb/     # Database repository implementations & sqlc generated queries
    │       └── wails/       # Desktop bindings & UI event handling
    │
    ├── pkg/
    │   └── logger/          # Context-aware structured logger (Standard library log/slog)
    │
    ├── db/
    │   ├── migrations/      # Goose versioned SQL migrations
    │   └── queries/         # sqlc type-safe query templates
    │
    └── frontend/            # DESKTOP UI (Vue 3 + Vite + TypeScript)
        ├── src/             # Views, accounting grids, and components
        └── wailsjs/         # Auto-generated Go-to-TypeScript runtime bridge
```

---

## 2. Tech Stack

- **Backend Language**: Go (v1.25+)
- **Desktop Runtime**: Wails v2 (`github.com/wailsapp/wails/v2`)
- **Frontend Framework**: Vue 3 + Vite + TypeScript
- **Database**: MariaDB 12.3 (On-premise / Local LAN)
  - Driver: `github.com/go-sql-driver/mysql`
  - Generator: `sqlc` (raw SQL to type-safe Go code)
  - Migrations: `goose` (`db/migrations`)
- **Arbitrary Precision Math**: `github.com/shopspring/decimal` (Zero floating point errors)
- **Logging**: `log/slog` (Standard library structured logging with `trace_id` and `user_id`)
- **Knowledge Graph**: CodeGraph (`.codegraph/`) for instant AST exploration

---

## 3. Database & Dev Environment

- **Database Name**: `fingo`
- **Default Dev Connection**: `dev:123456@tcp(127.0.0.1:3306)/fingo?parseTime=true`
- **Configuration**: `.env` located in `d:\accounting\fingo\.env`

---

## 4. Essential CLI Commands

Execute all commands from `d:\accounting\fingo`:

```powershell
# Run Live Dev Desktop Server
wails dev

# Compile Production Executable (FinGo.exe)
wails build

# Run All Domain Unit Tests (14 Packages)
go test -v ./internal/domain/...

# Run Logger Unit Tests
go test -v ./pkg/logger/...

# Generate Type-Safe SQLC Go Queries
sqlc generate

# Run Database Migrations
goose -dir db/migrations mysql "dev:123456@tcp(127.0.0.1:3306)/fingo?parseTime=true" up

# Create New Database Migration
goose -dir db/migrations create <migration_name> sql
```

To sync CodeGraph index (run from `d:\accounting`):
```powershell
codegraph sync
```

---

## 5. Domain Invariants Enforced

1. **Double-Entry Equilibrium**: Every voucher and opening balance requires $\sum \text{Debit} == \sum \text{Credit}$.
2. **Fixed-Point Financial Math**: All amounts, unit costs, VAT rates, and taxes use `decimal.Decimal`. No `float32`/`float64`.
3. **Period Locking**: Modifications and new vouchers are blocked if `VoucherDate <= LockDate`.
4. **VAS Compliance**: Automated P&L transfer paths ($511/515/711 \rightarrow 911 \leftarrow 632/635/641/642/811 \rightarrow 421$) and Statutory VAT declaration calculations.
