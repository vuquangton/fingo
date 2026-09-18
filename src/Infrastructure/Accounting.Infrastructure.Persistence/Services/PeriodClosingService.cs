using Accounting.Application.Common.Interfaces;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Enums;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Infrastructure.Persistence.Services;

public class PeriodClosingService : IPeriodClosingService
{
    private readonly IAccountingDbContext _context;
    private readonly IAuditLogService _auditLog;

    public PeriodClosingService(IAccountingDbContext context, IAuditLogService auditLog)
    {
        _context = context;
        _auditLog = auditLog;
    }

    public async Task<Guid> ExecutePeriodClosingAsync(int year, int month, Guid userId, CancellationToken cancellationToken = default)
    {
        var startDate = new DateTime(year, month, 1);
        var endDate = startDate.AddMonths(1).AddTicks(-1);

        var acc911 = await _context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "911", cancellationToken)
            ?? throw new InvalidOperationException("Account 911 (Xác định kết quả kinh doanh) not found in Chart of Accounts.");

        var acc4212 = await _context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "4212" || a.AccountNumber == "421", cancellationToken)
            ?? throw new InvalidOperationException("Account 4212 (Lợi nhuận sau thuế chưa phân phối) not found in Chart of Accounts.");

        // Fetch posted voucher lines in this period
        var postedLines = await _context.VoucherLines
            .Include(l => l.DebitAccount)
            .Include(l => l.CreditAccount)
            .Where(l => _context.Vouchers.Any(v => v.Id == l.VoucherId && v.Status == VoucherStatus.Posted && v.PostingDate >= startDate && v.PostingDate <= endDate))
            .ToListAsync(cancellationToken);

        var voucher = new Voucher(
            $"KC-{year}{month:D2}",
            endDate.Date,
            endDate.Date,
            VoucherType.PeriodClosing,
            $"Period closing and profit determination for {month:D2}/{year}",
            userId);

        decimal totalRevenueTransferred = 0m;
        decimal totalExpenseTransferred = 0m;

        // 1. Revenue accounts (511, 515, 711) - clear to Credit 911
        var revenueAccounts = await _context.Accounts
            .Where(a => a.Category == AccountCategory.Revenue || a.Category == AccountCategory.OtherIncome)
            .ToListAsync(cancellationToken);

        foreach (var acc in revenueAccounts)
        {
            var totalCredit = postedLines.Where(l => l.CreditAccountId == acc.Id).Sum(l => l.Amount);
            var totalDebit = postedLines.Where(l => l.DebitAccountId == acc.Id).Sum(l => l.Amount);
            var balance = totalCredit - totalDebit;

            if (balance > 0)
            {
                voucher.AddLine(acc.Id, acc911.Id, balance, $"Clear revenue {acc.AccountNumber} to 911");
                totalRevenueTransferred += balance;
            }
        }

        // 2. Expense accounts (632, 635, 641, 642, 811) - clear to Debit 911
        var expenseAccounts = await _context.Accounts
            .Where(a => a.Category == AccountCategory.Expense || a.Category == AccountCategory.OtherExpense)
            .ToListAsync(cancellationToken);

        foreach (var acc in expenseAccounts)
        {
            var totalDebit = postedLines.Where(l => l.DebitAccountId == acc.Id).Sum(l => l.Amount);
            var totalCredit = postedLines.Where(l => l.CreditAccountId == acc.Id).Sum(l => l.Amount);
            var balance = totalDebit - totalCredit;

            if (balance > 0)
            {
                voucher.AddLine(acc911.Id, acc.Id, balance, $"Clear expense {acc.AccountNumber} to 911");
                totalExpenseTransferred += balance;
            }
        }

        // 3. Net profit / loss determination (911 -> 4212)
        var netResult = totalRevenueTransferred - totalExpenseTransferred;
        if (netResult > 0)
        {
            // Net profit: Nợ 911 / Có 4212
            voucher.AddLine(acc911.Id, acc4212.Id, netResult, $"Net profit transferred to 4212 for {month:D2}/{year}");
        }
        else if (netResult < 0)
        {
            // Net loss: Nợ 4212 / Có 911
            voucher.AddLine(acc4212.Id, acc911.Id, Math.Abs(netResult), $"Net loss transferred from 4212 for {month:D2}/{year}");
        }

        if (voucher.Lines.Count > 0)
        {
            voucher.ValidateBalance();
            voucher.Post(userId);
            _context.AddEntity(voucher);
            await _context.SaveChangesAsync(cancellationToken);

            await _auditLog.LogAsync(
                AuditAction.Post,
                nameof(Voucher),
                voucher.Id.ToString(),
                diffSummary: $"Period closing {voucher.VoucherNumber} executed. Net Result: {netResult:N2} VND",
                cancellationToken: cancellationToken);
        }

        return voucher.Id;
    }

    public async Task LockPeriodAsync(int year, int month, bool isHardLock, Guid userId, CancellationToken cancellationToken = default)
    {
        var period = await _context.FiscalPeriods.FirstOrDefaultAsync(p => p.Year == year && p.Month == month, cancellationToken);
        if (period == null)
        {
            period = new FiscalPeriod(year, month);
            _context.AddEntity(period);
        }

        if (isHardLock) period.HardLock(userId);
        else period.SoftLock(userId);

        await _context.SaveChangesAsync(cancellationToken);
    }

    public async Task UnlockPeriodAsync(int year, int month, Guid userId, CancellationToken cancellationToken = default)
    {
        var period = await _context.FiscalPeriods.FirstOrDefaultAsync(p => p.Year == year && p.Month == month, cancellationToken);
        if (period != null)
        {
            period.Unlock();
            await _context.SaveChangesAsync(cancellationToken);
        }
    }
}
