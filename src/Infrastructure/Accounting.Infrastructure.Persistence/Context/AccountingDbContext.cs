using Accounting.Application.Common.Interfaces;
using Accounting.Domain.Common;
using Accounting.Domain.Entities.FixedAssets;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Entities.Inventory;
using Accounting.Domain.Entities.Payroll;
using Accounting.Domain.Entities.Purchasing;
using Accounting.Domain.Entities.Sales;
using Accounting.Domain.Entities.Security;
using Accounting.Domain.Entities.Tax;
using Accounting.Domain.Entities.Treasury;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Infrastructure.Persistence.Context;

public class AccountingDbContext : DbContext, IAccountingDbContext, IApplicationDbContext
{
    public AccountingDbContext(DbContextOptions<AccountingDbContext> options) : base(options) { }

    // General Ledger
    public DbSet<Account> Accounts => Set<Account>();
    public DbSet<Voucher> Vouchers => Set<Voucher>();
    public DbSet<VoucherLine> VoucherLines => Set<VoucherLine>();
    public DbSet<FiscalPeriod> FiscalPeriods => Set<FiscalPeriod>();
    public DbSet<PeriodClosingRule> PeriodClosingRules => Set<PeriodClosingRule>();

    // Treasury
    public DbSet<CashVoucher> CashVouchers => Set<CashVoucher>();
    public DbSet<CashCountRecord> CashCountRecords => Set<CashCountRecord>();
    public DbSet<CashCountLine> CashCountLines => Set<CashCountLine>();
    public DbSet<BankAccount> BankAccounts => Set<BankAccount>();
    public DbSet<PaymentOrder> PaymentOrders => Set<PaymentOrder>();
    public DbSet<BankStatementReconciliation> BankStatementReconciliations => Set<BankStatementReconciliation>();

    // Purchasing
    public DbSet<Vendor> Vendors => Set<Vendor>();
    public DbSet<PurchaseOrder> PurchaseOrders => Set<PurchaseOrder>();
    public DbSet<PurchaseOrderLine> PurchaseOrderLines => Set<PurchaseOrderLine>();
    public DbSet<GoodsReceiptNote> GoodsReceiptNotes => Set<GoodsReceiptNote>();
    public DbSet<GoodsReceiptNoteLine> GoodsReceiptNoteLines => Set<GoodsReceiptNoteLine>();
    public DbSet<VendorInvoice> VendorInvoices => Set<VendorInvoice>();

    // Sales
    public DbSet<Customer> Customers => Set<Customer>();
    public DbSet<SalesDeliveryNote> SalesDeliveryNotes => Set<SalesDeliveryNote>();
    public DbSet<SalesDeliveryNoteLine> SalesDeliveryNoteLines => Set<SalesDeliveryNoteLine>();
    public DbSet<SalesInvoice> SalesInvoices => Set<SalesInvoice>();

    // Inventory
    public DbSet<Warehouse> Warehouses => Set<Warehouse>();
    public DbSet<ProductItem> ProductItems => Set<ProductItem>();
    public DbSet<StockTransaction> StockTransactions => Set<StockTransaction>();
    public DbSet<WarehouseTransfer> WarehouseTransfers => Set<WarehouseTransfer>();
    public DbSet<WarehouseTransferLine> WarehouseTransferLines => Set<WarehouseTransferLine>();
    public DbSet<StocktakeRecord> StocktakeRecords => Set<StocktakeRecord>();
    public DbSet<StocktakeLine> StocktakeLines => Set<StocktakeLine>();

    // Fixed Assets
    public DbSet<FixedAsset> FixedAssets => Set<FixedAsset>();
    public DbSet<PrepaidExpense> PrepaidExpenses => Set<PrepaidExpense>();

    // Payroll
    public DbSet<Employee> Employees => Set<Employee>();
    public DbSet<SalarySlip> SalarySlips => Set<SalarySlip>();

    // Tax
    public DbSet<VatTransactionRecord> VatTransactionRecords => Set<VatTransactionRecord>();

    // Security & Audit
    public DbSet<AppUser> Users => Set<AppUser>();
    public DbSet<AppRole> Roles => Set<AppRole>();
    public DbSet<AuditLog> AuditLogs => Set<AuditLog>();
    public DbSet<AuditTrail> AuditTrails => Set<AuditTrail>();

    // Master Data Management (MDM)
    public DbSet<Accounting.Domain.MasterData.Accounts.Account> MasterAccounts => Set<Accounting.Domain.MasterData.Accounts.Account>();
    public DbSet<Accounting.Domain.MasterData.Partners.BusinessPartner> BusinessPartners => Set<Accounting.Domain.MasterData.Partners.BusinessPartner>();
    public DbSet<Accounting.Domain.MasterData.Currencies.Currency> Currencies => Set<Accounting.Domain.MasterData.Currencies.Currency>();
    public DbSet<Accounting.Domain.MasterData.Currencies.ExchangeRate> ExchangeRates => Set<Accounting.Domain.MasterData.Currencies.ExchangeRate>();
    public DbSet<Accounting.Domain.MasterData.Inventory.Warehouse> MasterWarehouses => Set<Accounting.Domain.MasterData.Inventory.Warehouse>();
    public DbSet<Accounting.Domain.MasterData.Inventory.UnitOfMeasure> UnitsOfMeasure => Set<Accounting.Domain.MasterData.Inventory.UnitOfMeasure>();
    public DbSet<Accounting.Domain.MasterData.Inventory.InventoryItem> InventoryItems => Set<Accounting.Domain.MasterData.Inventory.InventoryItem>();
    public DbSet<Accounting.Domain.MasterData.Dimensions.CostCenter> CostCenters => Set<Accounting.Domain.MasterData.Dimensions.CostCenter>();
    public DbSet<Accounting.Domain.MasterData.Dimensions.Department> Departments => Set<Accounting.Domain.MasterData.Dimensions.Department>();
    public DbSet<Accounting.Domain.MasterData.Dimensions.ExpenseItem> ExpenseItems => Set<Accounting.Domain.MasterData.Dimensions.ExpenseItem>();

    // General Ledger (GL) - Phase 3
    public DbSet<Accounting.Domain.Ledger.Voucher> GlVouchers => Set<Accounting.Domain.Ledger.Voucher>();
    public DbSet<Accounting.Domain.Ledger.VoucherLine> GlVoucherLines => Set<Accounting.Domain.Ledger.VoucherLine>();
    public DbSet<Accounting.Domain.Ledger.GeneralLedgerEntry> GeneralLedgerEntries => Set<Accounting.Domain.Ledger.GeneralLedgerEntry>();

    // Sub-Ledgers - Phase 4
    public DbSet<Accounting.Domain.Treasury.CashTransaction> SubCashTransactions => Set<Accounting.Domain.Treasury.CashTransaction>();
    public DbSet<Accounting.Domain.Payables.PurchaseInvoice> SubPurchaseInvoices => Set<Accounting.Domain.Payables.PurchaseInvoice>();
    public DbSet<Accounting.Domain.Payables.PurchaseInvoiceLine> SubPurchaseInvoiceLines => Set<Accounting.Domain.Payables.PurchaseInvoiceLine>();
    public DbSet<Accounting.Domain.Receivables.SalesInvoice> SubSalesInvoices => Set<Accounting.Domain.Receivables.SalesInvoice>();
    public DbSet<Accounting.Domain.Receivables.SalesInvoiceLine> SubSalesInvoiceLines => Set<Accounting.Domain.Receivables.SalesInvoiceLine>();
    public DbSet<Accounting.Domain.WarehouseOperations.WarehouseVoucher> SubWarehouseVouchers => Set<Accounting.Domain.WarehouseOperations.WarehouseVoucher>();
    public DbSet<Accounting.Domain.WarehouseOperations.WarehouseVoucherLine> SubWarehouseVoucherLines => Set<Accounting.Domain.WarehouseOperations.WarehouseVoucherLine>();
    public DbSet<Accounting.Domain.Settlement.InvoicePaymentAllocation> SubInvoicePaymentAllocations => Set<Accounting.Domain.Settlement.InvoicePaymentAllocation>();

    // Costing, Capital Assets & Manufacturing - Phase 5
    public DbSet<Accounting.Domain.Costing.InventoryLayer> CostInventoryLayers => Set<Accounting.Domain.Costing.InventoryLayer>();
    public DbSet<Accounting.Domain.Costing.CostingRun> CostAllocationRuns => Set<Accounting.Domain.Costing.CostingRun>();
    public DbSet<Accounting.Domain.CapitalAssets.FixedAsset> CapitalFixedAssets => Set<Accounting.Domain.CapitalAssets.FixedAsset>();
    public DbSet<Accounting.Domain.CapitalAssets.PrepaidExpense> CapitalPrepaidExpenses => Set<Accounting.Domain.CapitalAssets.PrepaidExpense>();
    public DbSet<Accounting.Domain.Manufacturing.BillOfMaterials> ManufacturingBoms => Set<Accounting.Domain.Manufacturing.BillOfMaterials>();
    public DbSet<Accounting.Domain.Manufacturing.BomLine> ManufacturingBomLines => Set<Accounting.Domain.Manufacturing.BomLine>();
    public DbSet<Accounting.Domain.Manufacturing.CostAbsorptionRun> ManufacturingCostAbsorptionRuns => Set<Accounting.Domain.Manufacturing.CostAbsorptionRun>();

    // Human Resources & Payroll - Phase 6
    public DbSet<Accounting.Domain.Payroll.PayrollEmployee> PayrollEmployees => Set<Accounting.Domain.Payroll.PayrollEmployee>();
    public DbSet<Accounting.Domain.Payroll.PayrollRun> PayrollRuns => Set<Accounting.Domain.Payroll.PayrollRun>();
    public DbSet<Accounting.Domain.Payroll.Payslip> Payslips => Set<Accounting.Domain.Payroll.Payslip>();

    // Period-End Closing & Statutory Reporting - Phase 7
    public DbSet<Accounting.Domain.PeriodEnd.PeriodClosingRun> PeriodClosingRuns => Set<Accounting.Domain.PeriodEnd.PeriodClosingRun>();
    public DbSet<Accounting.Domain.Reporting.ReportTemplate> ReportTemplates => Set<Accounting.Domain.Reporting.ReportTemplate>();
    public DbSet<Accounting.Domain.Reporting.ReportLine> ReportLines => Set<Accounting.Domain.Reporting.ReportLine>();
    public DbSet<Accounting.Domain.Tax.TaxDeclarationSnapshot> TaxDeclarationSnapshots => Set<Accounting.Domain.Tax.TaxDeclarationSnapshot>();

    // Ops & Maintenance - Phase 8
    public DbSet<Accounting.Domain.Ops.BackupHistory> BackupHistories => Set<Accounting.Domain.Ops.BackupHistory>();

    // Foundation Capabilities
    public DbSet<Accounting.Domain.Organization.CompanySetting> CompanySettings => Set<Accounting.Domain.Organization.CompanySetting>();
    public DbSet<Accounting.Domain.OpeningBalance.OpeningBalanceEntry> OpeningBalanceEntries => Set<Accounting.Domain.OpeningBalance.OpeningBalanceEntry>();

    // Explicit IAccountingDbContext IQueryable projections
    IQueryable<Account> IAccountingDbContext.Accounts => Accounts;
    IQueryable<Voucher> IAccountingDbContext.Vouchers => Vouchers;
    IQueryable<VoucherLine> IAccountingDbContext.VoucherLines => VoucherLines;
    IQueryable<FiscalPeriod> IAccountingDbContext.FiscalPeriods => FiscalPeriods;
    IQueryable<PeriodClosingRule> IAccountingDbContext.PeriodClosingRules => PeriodClosingRules;

    IQueryable<CashVoucher> IAccountingDbContext.CashVouchers => CashVouchers;
    IQueryable<CashCountRecord> IAccountingDbContext.CashCountRecords => CashCountRecords;
    IQueryable<BankAccount> IAccountingDbContext.BankAccounts => BankAccounts;
    IQueryable<PaymentOrder> IAccountingDbContext.PaymentOrders => PaymentOrders;
    IQueryable<BankStatementReconciliation> IAccountingDbContext.BankStatementReconciliations => BankStatementReconciliations;

    IQueryable<Vendor> IAccountingDbContext.Vendors => Vendors;
    IQueryable<PurchaseOrder> IAccountingDbContext.PurchaseOrders => PurchaseOrders;
    IQueryable<GoodsReceiptNote> IAccountingDbContext.GoodsReceiptNotes => GoodsReceiptNotes;
    IQueryable<VendorInvoice> IAccountingDbContext.VendorInvoices => VendorInvoices;

    IQueryable<Customer> IAccountingDbContext.Customers => Customers;
    IQueryable<SalesDeliveryNote> IAccountingDbContext.SalesDeliveryNotes => SalesDeliveryNotes;
    IQueryable<SalesInvoice> IAccountingDbContext.SalesInvoices => SalesInvoices;

    IQueryable<Warehouse> IAccountingDbContext.Warehouses => Warehouses;
    IQueryable<ProductItem> IAccountingDbContext.ProductItems => ProductItems;
    IQueryable<StockTransaction> IAccountingDbContext.StockTransactions => StockTransactions;
    IQueryable<WarehouseTransfer> IAccountingDbContext.WarehouseTransfers => WarehouseTransfers;
    IQueryable<StocktakeRecord> IAccountingDbContext.StocktakeRecords => StocktakeRecords;

    IQueryable<FixedAsset> IAccountingDbContext.FixedAssets => FixedAssets;
    IQueryable<PrepaidExpense> IAccountingDbContext.PrepaidExpenses => PrepaidExpenses;

    IQueryable<Employee> IAccountingDbContext.Employees => Employees;
    IQueryable<SalarySlip> IAccountingDbContext.SalarySlips => SalarySlips;

    IQueryable<VatTransactionRecord> IAccountingDbContext.VatTransactionRecords => VatTransactionRecords;

    IQueryable<AppUser> IAccountingDbContext.Users => Users;
    IQueryable<AppRole> IAccountingDbContext.Roles => Roles;
    IQueryable<AuditLog> IAccountingDbContext.AuditLogs => AuditLogs;
    IQueryable<AuditTrail> IAccountingDbContext.AuditTrails => AuditTrails;

    // Master Data Management (MDM) Projections
    IQueryable<Accounting.Domain.MasterData.Accounts.Account> IAccountingDbContext.MasterAccounts => MasterAccounts;
    IQueryable<Accounting.Domain.MasterData.Partners.BusinessPartner> IAccountingDbContext.BusinessPartners => BusinessPartners;
    IQueryable<Accounting.Domain.MasterData.Currencies.Currency> IAccountingDbContext.Currencies => Currencies;
    IQueryable<Accounting.Domain.MasterData.Currencies.ExchangeRate> IAccountingDbContext.ExchangeRates => ExchangeRates;
    IQueryable<Accounting.Domain.MasterData.Inventory.Warehouse> IAccountingDbContext.MasterWarehouses => MasterWarehouses;
    IQueryable<Accounting.Domain.MasterData.Inventory.UnitOfMeasure> IAccountingDbContext.UnitsOfMeasure => UnitsOfMeasure;
    IQueryable<Accounting.Domain.MasterData.Inventory.InventoryItem> IAccountingDbContext.InventoryItems => InventoryItems;
    IQueryable<Accounting.Domain.MasterData.Dimensions.CostCenter> IAccountingDbContext.CostCenters => CostCenters;
    IQueryable<Accounting.Domain.MasterData.Dimensions.Department> IAccountingDbContext.Departments => Departments;
    IQueryable<Accounting.Domain.MasterData.Dimensions.ExpenseItem> IAccountingDbContext.ExpenseItems => ExpenseItems;

    // General Ledger (GL) Projections
    IQueryable<Accounting.Domain.Ledger.Voucher> IAccountingDbContext.GlVouchers => GlVouchers;
    IQueryable<Accounting.Domain.Ledger.VoucherLine> IAccountingDbContext.GlVoucherLines => GlVoucherLines;
    IQueryable<Accounting.Domain.Ledger.GeneralLedgerEntry> IAccountingDbContext.GeneralLedgerEntries => GeneralLedgerEntries;

    // Sub-Ledgers - Phase 4 Projections
    IQueryable<Accounting.Domain.Treasury.CashTransaction> IAccountingDbContext.SubCashTransactions => SubCashTransactions;
    IQueryable<Accounting.Domain.Payables.PurchaseInvoice> IAccountingDbContext.SubPurchaseInvoices => SubPurchaseInvoices;
    IQueryable<Accounting.Domain.Payables.PurchaseInvoiceLine> IAccountingDbContext.SubPurchaseInvoiceLines => SubPurchaseInvoiceLines;
    IQueryable<Accounting.Domain.Receivables.SalesInvoice> IAccountingDbContext.SubSalesInvoices => SubSalesInvoices;
    IQueryable<Accounting.Domain.Receivables.SalesInvoiceLine> IAccountingDbContext.SubSalesInvoiceLines => SubSalesInvoiceLines;
    IQueryable<Accounting.Domain.WarehouseOperations.WarehouseVoucher> IAccountingDbContext.SubWarehouseVouchers => SubWarehouseVouchers;
    IQueryable<Accounting.Domain.WarehouseOperations.WarehouseVoucherLine> IAccountingDbContext.SubWarehouseVoucherLines => SubWarehouseVoucherLines;
    IQueryable<Accounting.Domain.Settlement.InvoicePaymentAllocation> IAccountingDbContext.SubInvoicePaymentAllocations => SubInvoicePaymentAllocations;

    // Costing, Capital Assets & Manufacturing - Phase 5 Projections
    IQueryable<Accounting.Domain.Costing.InventoryLayer> IAccountingDbContext.CostInventoryLayers => CostInventoryLayers;
    IQueryable<Accounting.Domain.Costing.CostingRun> IAccountingDbContext.CostAllocationRuns => CostAllocationRuns;
    IQueryable<Accounting.Domain.CapitalAssets.FixedAsset> IAccountingDbContext.CapitalFixedAssets => CapitalFixedAssets;
    IQueryable<Accounting.Domain.CapitalAssets.PrepaidExpense> IAccountingDbContext.CapitalPrepaidExpenses => CapitalPrepaidExpenses;
    IQueryable<Accounting.Domain.Manufacturing.BillOfMaterials> IAccountingDbContext.ManufacturingBoms => ManufacturingBoms;
    IQueryable<Accounting.Domain.Manufacturing.BomLine> IAccountingDbContext.ManufacturingBomLines => ManufacturingBomLines;
    IQueryable<Accounting.Domain.Manufacturing.CostAbsorptionRun> IAccountingDbContext.ManufacturingCostAbsorptionRuns => ManufacturingCostAbsorptionRuns;

    // Human Resources & Payroll - Phase 6 Projections
    IQueryable<Accounting.Domain.Payroll.PayrollEmployee> IAccountingDbContext.PayrollEmployees => PayrollEmployees;
    IQueryable<Accounting.Domain.Payroll.PayrollRun> IAccountingDbContext.PayrollRuns => PayrollRuns;
    IQueryable<Accounting.Domain.Payroll.Payslip> IAccountingDbContext.Payslips => Payslips;

    // Period-End Closing & Statutory Reporting - Phase 7 Projections
    IQueryable<Accounting.Domain.PeriodEnd.PeriodClosingRun> IAccountingDbContext.PeriodClosingRuns => PeriodClosingRuns;
    IQueryable<Accounting.Domain.Reporting.ReportTemplate> IAccountingDbContext.ReportTemplates => ReportTemplates;
    IQueryable<Accounting.Domain.Reporting.ReportLine> IAccountingDbContext.ReportLines => ReportLines;
    IQueryable<Accounting.Domain.Tax.TaxDeclarationSnapshot> IAccountingDbContext.TaxDeclarationSnapshots => TaxDeclarationSnapshots;
    IQueryable<Accounting.Domain.Ops.BackupHistory> IAccountingDbContext.BackupHistories => BackupHistories;
    IQueryable<Accounting.Domain.Organization.CompanySetting> IAccountingDbContext.CompanySettings => CompanySettings;
    IQueryable<Accounting.Domain.OpeningBalance.OpeningBalanceEntry> IAccountingDbContext.OpeningBalanceEntries => OpeningBalanceEntries;

    public void AddEntity<TEntity>(TEntity entity) where TEntity : class => Set<TEntity>().Add(entity);
    public void AddRangeEntities<TEntity>(IEnumerable<TEntity> entities) where TEntity : class => Set<TEntity>().AddRange(entities);
    public void RemoveEntity<TEntity>(TEntity entity) where TEntity : class => Set<TEntity>().Remove(entity);

    protected override void OnModelCreating(ModelBuilder modelBuilder)
    {
        base.OnModelCreating(modelBuilder);
        modelBuilder.ApplyConfigurationsFromAssembly(typeof(AccountingDbContext).Assembly);

        // Account
        modelBuilder.Entity<Account>(b =>
        {
            b.HasKey(a => a.Id);
            b.HasIndex(a => a.AccountNumber).IsUnique();
            b.Property(a => a.AccountNumber).HasMaxLength(20).IsRequired();
            b.Property(a => a.Name).HasMaxLength(200).IsRequired();
            b.HasOne(a => a.ParentAccount)
                .WithMany(a => a.SubAccounts)
                .HasForeignKey(a => a.ParentAccountId)
                .OnDelete(DeleteBehavior.Restrict);
        });

        // Voucher & Lines
        modelBuilder.Entity<Voucher>(b =>
        {
            b.HasKey(v => v.Id);
            b.HasIndex(v => v.VoucherNumber).IsUnique();
            b.Property(v => v.VoucherNumber).HasMaxLength(50).IsRequired();
            b.Property(v => v.Description).HasMaxLength(500).IsRequired();
            b.Property(v => v.Currency).HasMaxLength(10).IsRequired();
            b.Property(v => v.TotalDebitAmount).HasPrecision(18, 2);
            b.Property(v => v.TotalCreditAmount).HasPrecision(18, 2);
            b.Property(v => v.ExchangeRate).HasPrecision(18, 6);
            b.HasMany(v => v.Lines)
                .WithOne()
                .HasForeignKey(l => l.VoucherId)
                .OnDelete(DeleteBehavior.Cascade);
        });

        modelBuilder.Entity<VoucherLine>(b =>
        {
            b.HasKey(l => l.Id);
            b.Property(l => l.Amount).HasPrecision(18, 2);
            b.Property(l => l.ForeignAmount).HasPrecision(18, 4);
            b.Property(l => l.ExchangeRate).HasPrecision(18, 6);
            b.Property(l => l.Currency).HasMaxLength(10);
            b.Property(l => l.Description).HasMaxLength(500);

            b.HasOne(l => l.DebitAccount)
                .WithMany()
                .HasForeignKey(l => l.DebitAccountId)
                .OnDelete(DeleteBehavior.Restrict);

            b.HasOne(l => l.CreditAccount)
                .WithMany()
                .HasForeignKey(l => l.CreditAccountId)
                .OnDelete(DeleteBehavior.Restrict);
        });

        // CashVoucher
        modelBuilder.Entity<CashVoucher>(b =>
        {
            b.HasKey(c => c.Id);
            b.HasIndex(c => c.VoucherNumber).IsUnique();
            b.Property(c => c.Amount).HasPrecision(18, 2);
        });

        // BankAccount
        modelBuilder.Entity<BankAccount>(b =>
        {
            b.HasKey(a => a.Id);
            b.HasIndex(a => a.AccountNumber);
            b.Property(a => a.CurrentBalance).HasPrecision(18, 2);
        });

        // Vendor
        modelBuilder.Entity<Vendor>(b =>
        {
            b.HasKey(v => v.Id);
            b.HasIndex(v => v.Code).IsUnique();
            b.Property(v => v.CurrentPayableBalance).HasPrecision(18, 2);
        });

        // Customer
        modelBuilder.Entity<Customer>(b =>
        {
            b.HasKey(c => c.Id);
            b.HasIndex(c => c.Code).IsUnique();
            b.Property(c => c.CreditLimit).HasPrecision(18, 2);
            b.Property(c => c.CurrentReceivableBalance).HasPrecision(18, 2);
        });

        // ProductItem
        modelBuilder.Entity<ProductItem>(b =>
        {
            b.HasKey(p => p.Id);
            b.HasIndex(p => p.Code).IsUnique();
            b.Property(p => p.StandardCost).HasPrecision(18, 2);
            b.Property(p => p.CurrentStockQuantity).HasPrecision(18, 4);
            b.Property(p => p.CurrentStockValue).HasPrecision(18, 2);
        });

        // FixedAsset
        modelBuilder.Entity<FixedAsset>(b =>
        {
            b.HasKey(f => f.Id);
            b.HasIndex(f => f.AssetCode).IsUnique();
            b.Property(f => f.HistoricalCost).HasPrecision(18, 2);
            b.Property(f => f.AccumulatedDepreciation).HasPrecision(18, 2);
        });

        // PrepaidExpense
        modelBuilder.Entity<PrepaidExpense>(b =>
        {
            b.HasKey(p => p.Id);
            b.HasIndex(p => p.Code).IsUnique();
            b.Property(p => p.TotalAmount).HasPrecision(18, 2);
            b.Property(p => p.AllocatedAmount).HasPrecision(18, 2);
        });

        // Employee & SalarySlip
        modelBuilder.Entity<Employee>(b =>
        {
            b.HasKey(e => e.Id);
            b.HasIndex(e => e.Code).IsUnique();
            b.Property(e => e.BaseSalary).HasPrecision(18, 2);
            b.Property(e => e.InsuranceSalary).HasPrecision(18, 2);
            b.Property(e => e.Allowances).HasPrecision(18, 2);
        });

        modelBuilder.Entity<SalarySlip>(b =>
        {
            b.HasKey(s => s.Id);
            b.HasIndex(s => new { s.Year, s.Month, s.EmployeeId });
            b.Property(s => s.BaseSalary).HasPrecision(18, 2);
            b.Property(s => s.Allowances).HasPrecision(18, 2);
            b.Property(s => s.PersonalIncomeTax).HasPrecision(18, 2);
        });

        // AuditLog
        modelBuilder.Entity<AuditLog>(b =>
        {
            b.HasKey(a => a.Id);
            b.Property(a => a.Id).ValueGeneratedOnAdd();
            b.HasIndex(a => a.TimestampUtc);
            b.HasIndex(a => a.EntityName);
        });

        // Users & Roles
        modelBuilder.Entity<AppUser>(b =>
        {
            b.HasKey(u => u.Id);
            b.HasIndex(u => u.Username).IsUnique();
            b.HasOne(u => u.Role).WithMany().HasForeignKey(u => u.RoleId).OnDelete(DeleteBehavior.Restrict);
        });

        // FiscalPeriod with strongly-typed FiscalPeriodId
        modelBuilder.Entity<FiscalPeriod>(b =>
        {
            b.HasKey(p => p.Id);
            b.Property(p => p.Id)
                .HasConversion(id => id.Value, value => new FiscalPeriodId(value));
            b.Property(p => p.Year).IsRequired();
            b.Property(p => p.PeriodNumber).IsRequired();
            b.Property(p => p.StartDate).IsRequired();
            b.Property(p => p.EndDate).IsRequired();
            b.Property(p => p.IsSoftLocked).IsRequired();
            b.Property(p => p.IsHardLocked).IsRequired();
            b.HasIndex(p => new { p.Year, p.PeriodNumber }).IsUnique();
        });

        // AuditTrail with strongly-typed AuditTrailId
        modelBuilder.Entity<AuditTrail>(b =>
        {
            b.ToTable("audit_trails");
            b.HasKey(a => a.Id);
            b.Property(a => a.Id)
                .HasConversion(id => id.Value, value => new AuditTrailId(value));
            b.Property(a => a.EntityName).HasMaxLength(150).IsRequired();
            b.Property(a => a.Action).HasConversion<string>().HasMaxLength(50).IsRequired();
            b.Property(a => a.EntityId).HasMaxLength(100).IsRequired();
            b.Property(a => a.UserId).HasMaxLength(100);
            b.Property(a => a.Username).HasMaxLength(100);
            b.Property(a => a.MachineName).HasMaxLength(100).IsRequired();
            b.Property(a => a.TimestampUtc).IsRequired();
            b.HasIndex(a => a.TimestampUtc);
            b.HasIndex(a => a.EntityName);
        });

        // Global query filter for ISoftDeletable
        foreach (var entityType in modelBuilder.Model.GetEntityTypes())
        {
            if (typeof(ISoftDeletable).IsAssignableFrom(entityType.ClrType))
            {
                var parameter = System.Linq.Expressions.Expression.Parameter(entityType.ClrType, "e");
                var propertyMethodInfo = typeof(EF).GetMethod(nameof(EF.Property), System.Reflection.BindingFlags.Static | System.Reflection.BindingFlags.Public)?.MakeGenericMethod(typeof(bool));
                var isDeletedProperty = System.Linq.Expressions.Expression.Call(propertyMethodInfo!, parameter, System.Linq.Expressions.Expression.Constant(nameof(ISoftDeletable.IsDeleted)));
                var compareExpression = System.Linq.Expressions.Expression.MakeBinary(System.Linq.Expressions.ExpressionType.Equal, isDeletedProperty, System.Linq.Expressions.Expression.Constant(false));
                var lambda = System.Linq.Expressions.Expression.Lambda(compareExpression, parameter);
                modelBuilder.Entity(entityType.ClrType).HasQueryFilter(lambda);
            }
        }
    }
}
