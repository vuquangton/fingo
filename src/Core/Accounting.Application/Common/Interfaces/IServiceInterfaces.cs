using System.Data;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Enums;

namespace Accounting.Application.Common.Interfaces;

public interface IDbConnectionFactory : ISqlConnectionFactory
{
}

public interface ICurrentUserService
{
    Guid? UserId { get; }
    string Username { get; }
    string MachineName { get; }
    string? IpAddress { get; }
}

public interface IDateTimeService
{
    DateTime UtcNow { get; }
    DateTime Now { get; }
}

public interface IVoucherPostingService
{
    Task<Voucher> PostVoucherAsync(Guid voucherId, Guid userId, CancellationToken cancellationToken = default);
    Task<Voucher> UnpostVoucherAsync(Guid voucherId, Guid userId, CancellationToken cancellationToken = default);
    Task<Voucher> ReverseVoucherAsync(Guid voucherId, string newVoucherNumber, Guid userId, DateTime reversalDate, string reason, CancellationToken cancellationToken = default);
}

public interface IInventoryCostingEngine
{
    Task<decimal> CalculateOutflowCostAsync(Guid productItemId, decimal quantity, DateTime transactionDate, CostingMethod method, CancellationToken cancellationToken = default);
    Task ReevaluateMonthlyPeriodicCostAsync(int year, int month, CancellationToken cancellationToken = default);
}

public interface IPeriodClosingService
{
    Task<Guid> ExecutePeriodClosingAsync(int year, int month, Guid userId, CancellationToken cancellationToken = default);
    Task LockPeriodAsync(int year, int month, bool isHardLock, Guid userId, CancellationToken cancellationToken = default);
    Task UnlockPeriodAsync(int year, int month, Guid userId, CancellationToken cancellationToken = default);
}

public interface IStatutoryReportService
{
    Task<FinancialPositionReportDto> GetFinancialPositionReportAsync(DateTime asOfDate, CancellationToken cancellationToken = default);
    Task<IncomeStatementReportDto> GetIncomeStatementReportAsync(DateTime fromDate, DateTime toDate, CancellationToken cancellationToken = default);
    Task<CashFlowReportDto> GetCashFlowReportAsync(DateTime fromDate, DateTime toDate, bool isDirectMethod, CancellationToken cancellationToken = default);
}

public interface ITaxReportingService
{
    Task<VatDeclarationDto> GetVatDeclarationAsync(int year, int quarterOrMonth, bool isQuarter, CancellationToken cancellationToken = default);
    Task<string> GenerateTaxSubmissionXmlAsync(int year, int quarterOrMonth, bool isQuarter, CancellationToken cancellationToken = default);
}

public interface IAuditLogService
{
    Task LogAsync(AuditAction action, string entityName, string entityId, string? oldValuesJson = null, string? newValuesJson = null, string? diffSummary = null, CancellationToken cancellationToken = default);
}

public interface IDatabaseBackupService
{
    Task<string> BackupDatabaseAsync(string destinationPath, CancellationToken cancellationToken = default);
    Task RestoreDatabaseAsync(string backupFilePath, CancellationToken cancellationToken = default);
    Task OptimizeDatabaseAsync(CancellationToken cancellationToken = default);
}

public record FinancialPositionReportDto(
    DateTime AsOfDate,
    decimal TotalShortTermAssets,
    decimal TotalLongTermAssets,
    decimal TotalAssets,
    decimal TotalLiabilities,
    decimal TotalEquity,
    decimal TotalResources,
    List<ReportLineDto> Lines);

public record IncomeStatementReportDto(
    DateTime FromDate,
    DateTime ToDate,
    decimal GrossRevenue,
    decimal RevenueDeductions,
    decimal NetRevenue,
    decimal CostOfGoodsSold,
    decimal GrossProfit,
    decimal FinancialIncome,
    decimal FinancialExpenses,
    decimal SellingExpenses,
    decimal GeneralAdminExpenses,
    decimal OperatingProfit,
    decimal OtherIncome,
    decimal OtherExpenses,
    decimal OtherProfit,
    decimal ProfitBeforeTax,
    decimal CorporateIncomeTax,
    decimal NetProfitAfterTax,
    List<ReportLineDto> Lines);

public record CashFlowReportDto(
    DateTime FromDate,
    DateTime ToDate,
    bool IsDirectMethod,
    decimal NetCashFromOperating,
    decimal NetCashFromInvesting,
    decimal NetCashFromFinancing,
    decimal NetCashChange,
    decimal BeginningCash,
    decimal EndingCash,
    List<ReportLineDto> Lines);

public record ReportLineDto(string Code, string Description, string Note, decimal CurrentPeriodAmount, decimal PreviousPeriodAmount);

public record VatDeclarationDto(
    int Year,
    int Period,
    bool IsQuarter,
    decimal TotalTaxablePurchase,
    decimal TotalVatPurchase,
    decimal TotalTaxableSales,
    decimal TotalVatSales,
    decimal PayableVatAmount,
    List<VatRecordDto> InputInvoices,
    List<VatRecordDto> OutputInvoices);

public record VatRecordDto(
    string InvoiceSeries,
    string InvoiceNumber,
    DateTime InvoiceDate,
    string PartnerTaxCode,
    string PartnerName,
    string Description,
    decimal TaxableAmount,
    int VatRatePercent,
    decimal VatAmount);
