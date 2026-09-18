using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Ledger;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.PeriodEnd;

public class PreClosingCheckQueryHandlers : IRequestHandler<RunPreClosingCheckQuery, Result<PreClosingCheckReportDto>>
{
    private readonly IAccountingDbContext _context;

    public PreClosingCheckQueryHandlers(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<Result<PreClosingCheckReportDto>> Handle(RunPreClosingCheckQuery request, CancellationToken cancellationToken)
    {
        var startDate = new DateOnly(request.Year, request.Month, 1);
        var endDate = startDate.AddMonths(1).AddDays(-1);
        var checks = new List<HealthCheckItemDto>();

        // Check 1: Unposted / Draft / Invalid Vouchers in Period
        var unpostedVouchers = await _context.GlVouchers.AsNoTracking()
            .Where(v => v.PostingDate >= startDate && v.PostingDate <= endDate && v.Status != VoucherStatus.Posted && v.Status != VoucherStatus.Reversed)
            .Select(v => v.VoucherNumber)
            .ToListAsync(cancellationToken);

        checks.Add(new HealthCheckItemDto(
            "CHK_UNPOSTED_VOUCHERS",
            "Kiem tra chung tu chua ghi so trong ky",
            HealthCheckSeverity.Error,
            unpostedVouchers.Count == 0,
            unpostedVouchers.Count == 0
                ? "Tat ca chung tu trong ky da duoc ghi so."
                : $"Co {unpostedVouchers.Count} chung tu o trang thai Chua ghi so hoac Du thao.",
            unpostedVouchers.Count,
            unpostedVouchers.Take(10).ToList()
        ));

        // Check 2: Unbalanced General Ledger / Trial Balance
        var totalDebit = await _context.GeneralLedgerEntries.AsNoTracking()
            .Where(e => e.PostingDate <= endDate)
            .SumAsync(e => e.DebitAmount, cancellationToken);

        var totalCredit = await _context.GeneralLedgerEntries.AsNoTracking()
            .Where(e => e.PostingDate <= endDate)
            .SumAsync(e => e.CreditAmount, cancellationToken);

        var isGlBalanced = totalDebit == totalCredit;
        checks.Add(new HealthCheckItemDto(
            "CHK_GL_TRIAL_BALANCE",
            "Kiem tra can doi Tong No - Tong Co So Cai",
            HealthCheckSeverity.Error,
            isGlBalanced,
            isGlBalanced
                ? $"Can doi hoan hao: Tong No ({totalDebit:N2}) = Tong Co ({totalCredit:N2})."
                : $"Mat can doi: Tong No ({totalDebit:N2}) != Tong Co ({totalCredit:N2}). Chenh lech: {Math.Abs(totalDebit - totalCredit):N2}.",
            isGlBalanced ? 0 : 1
        ));

        // Check 3: Negative Cash / Fund Balance (TK 111x)
        var cashAccounts = await _context.GeneralLedgerEntries.AsNoTracking()
            .Where(e => EF.Functions.Like((string)e.AccountId, "111%") && e.PostingDate <= endDate)
            .GroupBy(e => (string)e.AccountId)
            .Select(g => new
            {
                AccountCode = g.Key,
                Balance = g.Sum(x => x.DebitAmount) - g.Sum(x => x.CreditAmount)
            })
            .ToListAsync(cancellationToken);

        var negativeCash = cashAccounts.Where(a => a.Balance < 0).ToList();
        checks.Add(new HealthCheckItemDto(
            "CHK_NEGATIVE_CASH",
            "Kiem tra am quy tien mat (TK 111)",
            HealthCheckSeverity.Error,
            negativeCash.Count == 0,
            negativeCash.Count == 0
                ? "So du quy tien mat binh thuong, khong am."
                : $"Phat hien {negativeCash.Count} tai khoan tien mat bi am quy: {string.Join(", ", negativeCash.Select(x => $"{x.AccountCode} ({x.Balance:N2})"))}.",
            negativeCash.Count,
            negativeCash.Select(x => x.AccountCode).ToList()
        ));

        // Check 4: Negative Bank Balance (TK 112x)
        var bankAccounts = await _context.GeneralLedgerEntries.AsNoTracking()
            .Where(e => EF.Functions.Like((string)e.AccountId, "112%") && e.PostingDate <= endDate)
            .GroupBy(e => (string)e.AccountId)
            .Select(g => new
            {
                AccountCode = g.Key,
                Balance = g.Sum(x => x.DebitAmount) - g.Sum(x => x.CreditAmount)
            })
            .ToListAsync(cancellationToken);

        var negativeBank = bankAccounts.Where(a => a.Balance < 0).ToList();
        checks.Add(new HealthCheckItemDto(
            "CHK_NEGATIVE_BANK",
            "Kiem tra am tien gui ngan hang (TK 112)",
            HealthCheckSeverity.Warning,
            negativeBank.Count == 0,
            negativeBank.Count == 0
                ? "So du tien gui ngan hang binh thuong."
                : $"Co {negativeBank.Count} tai khoan tien gui co so du am: {string.Join(", ", negativeBank.Select(x => $"{x.AccountCode} ({x.Balance:N2})"))}.",
            negativeBank.Count,
            negativeBank.Select(x => x.AccountCode).ToList()
        ));

        // Check 5: Negative Inventory Book (TK 15x)
        var inventoryAccounts = await _context.GeneralLedgerEntries.AsNoTracking()
            .Where(e => EF.Functions.Like((string)e.AccountId, "15%") && e.PostingDate <= endDate)
            .GroupBy(e => (string)e.AccountId)
            .Select(g => new
            {
                AccountCode = g.Key,
                Balance = g.Sum(x => x.DebitAmount) - g.Sum(x => x.CreditAmount)
            })
            .ToListAsync(cancellationToken);

        var negativeInv = inventoryAccounts.Where(a => a.Balance < 0).ToList();
        checks.Add(new HealthCheckItemDto(
            "CHK_NEGATIVE_INVENTORY",
            "Kiem tra am gia tri ton kho (TK 152, 153, 155, 156)",
            HealthCheckSeverity.Error,
            negativeInv.Count == 0,
            negativeInv.Count == 0
                ? "Gia tri ton kho binh thuong, khong co tai khoan kho am."
                : $"Co {negativeInv.Count} tai khoan hang ton kho co gia tri am: {string.Join(", ", negativeInv.Select(x => $"{x.AccountCode} ({x.Balance:N2})"))}.",
            negativeInv.Count,
            negativeInv.Select(x => x.AccountCode).ToList()
        ));

        // Check 6: AR/AP Sub-ledger vs General Ledger Reconciliation (TK 131 / 331)
        var arGlNet = await _context.GeneralLedgerEntries.AsNoTracking()
            .Where(e => EF.Functions.Like((string)e.AccountId, "131%") && e.PostingDate <= endDate)
            .SumAsync(e => e.DebitAmount - e.CreditAmount, cancellationToken);

        var arSubTotal = await _context.SubSalesInvoices.AsNoTracking()
            .Where(i => i.InvoiceDate <= endDate && i.Status != Domain.Receivables.SalesInvoiceStatus.Cancelled)
            .SumAsync(i => i.TotalAmount - i.ReceivedAmount, cancellationToken);

        var isArBalanced = Math.Abs(arGlNet - arSubTotal) < 0.01m;
        checks.Add(new HealthCheckItemDto(
            "CHK_AR_RECONCILIATION",
            "Doi chieu cong no phai thu KH (So Cai TK 131 vs So hoa don ban)",
            HealthCheckSeverity.Warning,
            isArBalanced,
            isArBalanced
                ? $"Khop cong no phai thu: So Cai ({arGlNet:N2}) = So hoa don ({arSubTotal:N2})."
                : $"Chenh lech cong no phai thu: So Cai ({arGlNet:N2}) vs So hoa don ({arSubTotal:N2}). Chenh lech: {Math.Abs(arGlNet - arSubTotal):N2}.",
            isArBalanced ? 0 : 1
        ));

        // Check 7: Input/Output VAT Reconciliation (TK 133 / TK 3331)
        var vatOutGl = await _context.GeneralLedgerEntries.AsNoTracking()
            .Where(e => EF.Functions.Like((string)e.AccountId, "3331%") && e.PostingDate >= startDate && e.PostingDate <= endDate)
            .SumAsync(e => e.CreditAmount - e.DebitAmount, cancellationToken);

        var vatOutInvoices = await _context.SubSalesInvoices.AsNoTracking()
            .Where(i => i.InvoiceDate >= startDate && i.InvoiceDate <= endDate && i.Status != Domain.Receivables.SalesInvoiceStatus.Cancelled)
            .SumAsync(i => i.VatAmount, cancellationToken);

        var isVatBalanced = Math.Abs(vatOutGl - vatOutInvoices) < 0.01m;
        checks.Add(new HealthCheckItemDto(
            "CHK_VAT_OUT_RECONCILIATION",
            "Doi chieu thue GTGT dau ra (TK 3331 vs Tong thue hoa don)",
            HealthCheckSeverity.Warning,
            isVatBalanced,
            isVatBalanced
                ? $"Thue GTGT dau ra hop le: So Cai ({vatOutGl:N2}) = Bang ke ban ra ({vatOutInvoices:N2})."
                : $"Chenh lech thue GTGT dau ra: So Cai ({vatOutGl:N2}) != Bang ke ({vatOutInvoices:N2}).",
            isVatBalanced ? 0 : 1
        ));

        int errorCount = checks.Count(c => !c.Passed && c.Severity == HealthCheckSeverity.Error);
        int warningCount = checks.Count(c => !c.Passed && c.Severity == HealthCheckSeverity.Warning);
        bool canProceed = errorCount == 0;

        return Result<PreClosingCheckReportDto>.Success(new PreClosingCheckReportDto(
            request.Year,
            request.Month,
            endDate,
            canProceed,
            errorCount,
            warningCount,
            checks
        ));
    }
}
