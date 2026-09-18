using Accounting.Domain.Entities.FixedAssets;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Entities.Inventory;
using Accounting.Domain.Entities.Payroll;
using Accounting.Domain.Entities.Purchasing;
using Accounting.Domain.Entities.Sales;
using Accounting.Domain.Entities.Security;
using Accounting.Domain.Entities.Tax;
using Accounting.Domain.Entities.Treasury;

namespace Accounting.Application.Common.Interfaces;

public interface IAccountingDbContext
{
    // General Ledger
    IQueryable<Account> Accounts { get; }
    IQueryable<Voucher> Vouchers { get; }
    IQueryable<VoucherLine> VoucherLines { get; }
    IQueryable<FiscalPeriod> FiscalPeriods { get; }
    IQueryable<PeriodClosingRule> PeriodClosingRules { get; }

    // Treasury
    IQueryable<CashVoucher> CashVouchers { get; }
    IQueryable<CashCountRecord> CashCountRecords { get; }
    IQueryable<BankAccount> BankAccounts { get; }
    IQueryable<PaymentOrder> PaymentOrders { get; }
    IQueryable<BankStatementReconciliation> BankStatementReconciliations { get; }

    // Purchasing & AP
    IQueryable<Vendor> Vendors { get; }
    IQueryable<PurchaseOrder> PurchaseOrders { get; }
    IQueryable<GoodsReceiptNote> GoodsReceiptNotes { get; }
    IQueryable<VendorInvoice> VendorInvoices { get; }

    // Sales & AR
    IQueryable<Customer> Customers { get; }
    IQueryable<SalesDeliveryNote> SalesDeliveryNotes { get; }
    IQueryable<SalesInvoice> SalesInvoices { get; }

    // Inventory
    IQueryable<Warehouse> Warehouses { get; }
    IQueryable<ProductItem> ProductItems { get; }
    IQueryable<StockTransaction> StockTransactions { get; }
    IQueryable<WarehouseTransfer> WarehouseTransfers { get; }
    IQueryable<StocktakeRecord> StocktakeRecords { get; }

    // Fixed Assets
    IQueryable<FixedAsset> FixedAssets { get; }
    IQueryable<PrepaidExpense> PrepaidExpenses { get; }

    // Payroll
    IQueryable<Employee> Employees { get; }
    IQueryable<SalarySlip> SalarySlips { get; }

    // Tax
    IQueryable<VatTransactionRecord> VatTransactionRecords { get; }

    // Security & Audit
    IQueryable<AppUser> Users { get; }
    IQueryable<AppRole> Roles { get; }
    IQueryable<AuditLog> AuditLogs { get; }
    IQueryable<AuditTrail> AuditTrails { get; }

    // Master Data Management (MDM)
    IQueryable<Accounting.Domain.MasterData.Accounts.Account> MasterAccounts { get; }
    IQueryable<Accounting.Domain.MasterData.Partners.BusinessPartner> BusinessPartners { get; }
    IQueryable<Accounting.Domain.MasterData.Currencies.Currency> Currencies { get; }
    IQueryable<Accounting.Domain.MasterData.Currencies.ExchangeRate> ExchangeRates { get; }
    IQueryable<Accounting.Domain.MasterData.Inventory.Warehouse> MasterWarehouses { get; }
    IQueryable<Accounting.Domain.MasterData.Inventory.UnitOfMeasure> UnitsOfMeasure { get; }
    IQueryable<Accounting.Domain.MasterData.Inventory.InventoryItem> InventoryItems { get; }
    IQueryable<Accounting.Domain.MasterData.Dimensions.CostCenter> CostCenters { get; }
    IQueryable<Accounting.Domain.MasterData.Dimensions.Department> Departments { get; }
    IQueryable<Accounting.Domain.MasterData.Dimensions.ExpenseItem> ExpenseItems { get; }

    // General Ledger (GL) - Phase 3
    IQueryable<Accounting.Domain.Ledger.Voucher> GlVouchers { get; }
    IQueryable<Accounting.Domain.Ledger.VoucherLine> GlVoucherLines { get; }
    IQueryable<Accounting.Domain.Ledger.GeneralLedgerEntry> GeneralLedgerEntries { get; }

    // Sub-Ledgers - Phase 4
    IQueryable<Accounting.Domain.Treasury.CashTransaction> SubCashTransactions { get; }
    IQueryable<Accounting.Domain.Payables.PurchaseInvoice> SubPurchaseInvoices { get; }
    IQueryable<Accounting.Domain.Payables.PurchaseInvoiceLine> SubPurchaseInvoiceLines { get; }
    IQueryable<Accounting.Domain.Receivables.SalesInvoice> SubSalesInvoices { get; }
    IQueryable<Accounting.Domain.Receivables.SalesInvoiceLine> SubSalesInvoiceLines { get; }
    IQueryable<Accounting.Domain.WarehouseOperations.WarehouseVoucher> SubWarehouseVouchers { get; }
    IQueryable<Accounting.Domain.WarehouseOperations.WarehouseVoucherLine> SubWarehouseVoucherLines { get; }
    IQueryable<Accounting.Domain.Settlement.InvoicePaymentAllocation> SubInvoicePaymentAllocations { get; }

    // Costing, Capital Assets & Manufacturing - Phase 5
    IQueryable<Accounting.Domain.Costing.InventoryLayer> CostInventoryLayers { get; }
    IQueryable<Accounting.Domain.Costing.CostingRun> CostAllocationRuns { get; }
    IQueryable<Accounting.Domain.CapitalAssets.FixedAsset> CapitalFixedAssets { get; }
    IQueryable<Accounting.Domain.CapitalAssets.PrepaidExpense> CapitalPrepaidExpenses { get; }
    IQueryable<Accounting.Domain.Manufacturing.BillOfMaterials> ManufacturingBoms { get; }
    IQueryable<Accounting.Domain.Manufacturing.BomLine> ManufacturingBomLines { get; }
    IQueryable<Accounting.Domain.Manufacturing.CostAbsorptionRun> ManufacturingCostAbsorptionRuns { get; }

    // Human Resources & Payroll - Phase 6
    IQueryable<Accounting.Domain.Payroll.PayrollEmployee> PayrollEmployees { get; }
    IQueryable<Accounting.Domain.Payroll.PayrollRun> PayrollRuns { get; }
    IQueryable<Accounting.Domain.Payroll.Payslip> Payslips { get; }

    // Period-End Closing & Statutory Reporting - Phase 7
    IQueryable<Accounting.Domain.PeriodEnd.PeriodClosingRun> PeriodClosingRuns { get; }
    IQueryable<Accounting.Domain.Reporting.ReportTemplate> ReportTemplates { get; }
    IQueryable<Accounting.Domain.Reporting.ReportLine> ReportLines { get; }
    IQueryable<Accounting.Domain.Tax.TaxDeclarationSnapshot> TaxDeclarationSnapshots { get; }

    // Ops & Maintenance - Phase 8
    IQueryable<Accounting.Domain.Ops.BackupHistory> BackupHistories { get; }

    // Foundation Capabilities (Company & Opening Balances)
    IQueryable<Accounting.Domain.Organization.CompanySetting> CompanySettings { get; }
    IQueryable<Accounting.Domain.OpeningBalance.OpeningBalanceEntry> OpeningBalanceEntries { get; }

    Task<int> SaveChangesAsync(CancellationToken cancellationToken = default);
    void AddEntity<TEntity>(TEntity entity) where TEntity : class;
    void AddRangeEntities<TEntity>(IEnumerable<TEntity> entities) where TEntity : class;
    void RemoveEntity<TEntity>(TEntity entity) where TEntity : class;
}
