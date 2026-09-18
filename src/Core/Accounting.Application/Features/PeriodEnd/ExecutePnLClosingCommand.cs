using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Common;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.PeriodEnd;
using MediatR;
using Microsoft.EntityFrameworkCore;
using AccountId = Accounting.Domain.MasterData.Common.AccountId;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;
using Voucher = Accounting.Domain.Ledger.Voucher;

namespace Accounting.Application.Features.PeriodEnd;

public record ExecutePnLClosingCommand(
    int Year,
    int Month,
    string PerformedBy = "system") : IRequest<Result<PeriodClosingRunId>>;

public class ExecutePnLClosingCommandHandler(
    IAccountingDbContext context,
    IGlVoucherBridgeService glBridge) : IRequestHandler<ExecutePnLClosingCommand, Result<PeriodClosingRunId>>
{
    private readonly CurrencyCode _vnd = new("VND");

    public async Task<Result<PeriodClosingRunId>> Handle(ExecutePnLClosingCommand request, CancellationToken cancellationToken)
    {
        var fiscalPeriodId = FiscalPeriodId.FromYearMonth(request.Year, request.Month);
        var startDate = new DateOnly(request.Year, request.Month, 1);
        var endDate = startDate.AddMonths(1).AddDays(-1);

        // 1. Idempotency Guard: Cannot close the same period twice for PnLClearance
        var alreadyClosed = await context.PeriodClosingRuns
            .AnyAsync(r => r.FiscalPeriodId == fiscalPeriodId &&
                           r.ClosingStep == ClosingStep.PnLClearance &&
                           r.Status == ClosingStatus.Success, cancellationToken);

        if (alreadyClosed)
        {
            throw new PeriodAlreadyClosedException(request.Year, request.Month, nameof(ClosingStep.PnLClearance));
        }

        // 2. Trial Balance Equality Guard before closing
        var trialBalanceDebit = await context.GeneralLedgerEntries
            .Where(e => e.PostingDate <= endDate)
            .SumAsync(e => e.DebitAmount, cancellationToken);

        var trialBalanceCredit = await context.GeneralLedgerEntries
            .Where(e => e.PostingDate <= endDate)
            .SumAsync(e => e.CreditAmount, cancellationToken);

        if (trialBalanceDebit != trialBalanceCredit)
        {
            throw new UnbalancedTrialBalanceException(trialBalanceDebit, trialBalanceCredit);
        }

        // 3. Find statutory clearance accounts: 911 and 4212 (or 421)
        var acc911 = await context.MasterAccounts.FirstOrDefaultAsync(a => a.Id == new AccountId("911"), cancellationToken)
            ?? throw new StatutoryComplianceException("Account 911 (X�c d?nh k?t qu? kinh doanh) not found in Chart of Accounts.");

        var acc4212 = await context.MasterAccounts.FirstOrDefaultAsync(a => a.Id == new AccountId("4212"), cancellationToken)
            ?? await context.MasterAccounts.FirstOrDefaultAsync(a => a.Id == new AccountId("421"), cancellationToken)
            ?? throw new StatutoryComplianceException("Account 4212 / 421 (L?i nhu?n sau thu? chua ph�n ph?i) not found in Chart of Accounts.");

        // 4. Query entries of Class 5, 6, 7, 8 in this period grouped by dimension
        var entries = await context.GeneralLedgerEntries
            .Where(e => e.PostingDate >= startDate && e.PostingDate <= endDate)
            .ToListAsync(cancellationToken);

        // Filter for classes 5, 6, 7, 8
        var closingEntries = entries
            .Where(e => e.AccountId.Value.StartsWith("5") ||
                        e.AccountId.Value.StartsWith("6") ||
                        e.AccountId.Value.StartsWith("7") ||
                        e.AccountId.Value.StartsWith("8"))
            .ToList();

        if (closingEntries.Count == 0)
        {
            // Nothing to close, create zero-voucher run
            var emptyRun = new PeriodClosingRun(
                PeriodClosingRunId.New(),
                fiscalPeriodId,
                DateTime.UtcNow,
                request.PerformedBy,
                ClosingStep.PnLClearance,
                $"No Class 5-8 activity in {request.Month:D2}/{request.Year}");

            context.AddEntity(emptyRun);
            await context.SaveChangesAsync(cancellationToken);
            return Result<PeriodClosingRunId>.Success(emptyRun.Id);
        }

        var voucherId = VoucherId.New();
        var voucherNumber = $"KC-{request.Year}{request.Month:D2}-{Guid.NewGuid():N}"[..18].ToUpperInvariant();
        var voucher = new Voucher(
            voucherId,
            voucherNumber,
            VoucherType.GeneralJournal,
            endDate,
            endDate,
            $"K?t chuy?n x�c d?nh KQKD th�ng {request.Month:D2}/{request.Year}",
            _vnd,
            1.0m);

        decimal totalRevenueTo911 = 0m;
        decimal totalExpenseTo911 = 0m;

        // Group by Account, Partner, CostCenter, Warehouse
        var grouped = closingEntries
            .GroupBy(e => new { e.AccountId, e.PartnerId, e.CostCenterId, e.WarehouseId })
            .Select(g => new
            {
                g.Key.AccountId,
                g.Key.PartnerId,
                g.Key.CostCenterId,
                g.Key.WarehouseId,
                TotalDebit = g.Sum(x => x.DebitAmount),
                TotalCredit = g.Sum(x => x.CreditAmount)
            })
            .ToList();

        // 5a. Clear Revenue & Other Income (Class 5 & 7, except 521)
        foreach (var item in grouped.Where(g => (g.AccountId.Value.StartsWith("5") && !g.AccountId.Value.StartsWith("521")) || g.AccountId.Value.StartsWith("7")))
        {
            var netCredit = item.TotalCredit - item.TotalDebit;
            if (netCredit > 0)
            {
                // Debit 5xx/7xx, Credit 911
                voucher.AddLine(
                    item.AccountId,
                    LedgerEntryType.Debit,
                    netCredit,
                    $"K?t chuy?n doanh thu/thu nh?p {item.AccountId.Value} sang 911",
                    partnerId: item.PartnerId,
                    costCenterId: item.CostCenterId,
                    warehouseId: item.WarehouseId);

                voucher.AddLine(
                    acc911.Id,
                    LedgerEntryType.Credit,
                    netCredit,
                    $"Nh?n k?t chuy?n doanh thu/thu nh?p {item.AccountId.Value}");

                totalRevenueTo911 += netCredit;
            }
            else if (netCredit < 0)
            {
                var netDebit = Math.Abs(netCredit);
                voucher.AddLine(
                    item.AccountId,
                    LedgerEntryType.Credit,
                    netDebit,
                    $"�i?u ch?nh gi?m doanh thu {item.AccountId.Value}",
                    partnerId: item.PartnerId,
                    costCenterId: item.CostCenterId,
                    warehouseId: item.WarehouseId);

                voucher.AddLine(
                    acc911.Id,
                    LedgerEntryType.Debit,
                    netDebit,
                    $"Gi?m doanh thu sang 911 t? {item.AccountId.Value}");

                totalRevenueTo911 -= netDebit;
            }
        }

        // 5b. Clear Revenue Deductions (521) to 911 or Revenue
        foreach (var item in grouped.Where(g => g.AccountId.Value.StartsWith("521")))
        {
            var netDebit = item.TotalDebit - item.TotalCredit;
            if (netDebit > 0)
            {
                // Credit 521, Debit 911
                voucher.AddLine(
                    acc911.Id,
                    LedgerEntryType.Debit,
                    netDebit,
                    $"K?t chuy?n gi?m tr? doanh thu {item.AccountId.Value} sang 911");

                voucher.AddLine(
                    item.AccountId,
                    LedgerEntryType.Credit,
                    netDebit,
                    $"K?t chuy?n gi?m tr? doanh thu {item.AccountId.Value}",
                    partnerId: item.PartnerId,
                    costCenterId: item.CostCenterId,
                    warehouseId: item.WarehouseId);

                totalRevenueTo911 -= netDebit;
            }
        }

        // 5c. Clear Expenses & Other Expenses (Class 6 & 8)
        foreach (var item in grouped.Where(g => g.AccountId.Value.StartsWith("6") || g.AccountId.Value.StartsWith("8")))
        {
            var netDebit = item.TotalDebit - item.TotalCredit;
            if (netDebit > 0)
            {
                // Debit 911, Credit 6xx/8xx
                voucher.AddLine(
                    acc911.Id,
                    LedgerEntryType.Debit,
                    netDebit,
                    $"K?t chuy?n chi ph� {item.AccountId.Value} sang 911");

                voucher.AddLine(
                    item.AccountId,
                    LedgerEntryType.Credit,
                    netDebit,
                    $"K?t chuy?n chi ph� {item.AccountId.Value}",
                    partnerId: item.PartnerId,
                    costCenterId: item.CostCenterId,
                    warehouseId: item.WarehouseId);

                totalExpenseTo911 += netDebit;
            }
            else if (netDebit < 0)
            {
                var netCredit = Math.Abs(netDebit);
                voucher.AddLine(
                    acc911.Id,
                    LedgerEntryType.Credit,
                    netCredit,
                    $"Gi?m chi ph� {item.AccountId.Value} sang 911");

                voucher.AddLine(
                    item.AccountId,
                    LedgerEntryType.Debit,
                    netCredit,
                    $"Gi?m chi ph� {item.AccountId.Value}",
                    partnerId: item.PartnerId,
                    costCenterId: item.CostCenterId,
                    warehouseId: item.WarehouseId);

                totalExpenseTo911 -= netCredit;
            }
        }

        // 5d. Net Profit / Loss transfer from 911 to 4212
        var netProfitOrLoss = totalRevenueTo911 - totalExpenseTo911;
        if (netProfitOrLoss > 0)
        {
            // L�i: N? 911 / C� 4212
            voucher.AddLine(
                acc911.Id,
                LedgerEntryType.Debit,
                netProfitOrLoss,
                $"K?t chuy?n l�i sau thu? th�ng {request.Month:D2}/{request.Year} sang 4212");

            voucher.AddLine(
                acc4212.Id,
                LedgerEntryType.Credit,
                netProfitOrLoss,
                $"L�i sau thu? th�ng {request.Month:D2}/{request.Year}");
        }
        else if (netProfitOrLoss < 0)
        {
            // L?: N? 4212 / C� 911
            var loss = Math.Abs(netProfitOrLoss);
            voucher.AddLine(
                acc4212.Id,
                LedgerEntryType.Debit,
                loss,
                $"K?t chuy?n l? sau thu? th�ng {request.Month:D2}/{request.Year}");

            voucher.AddLine(
                acc911.Id,
                LedgerEntryType.Credit,
                loss,
                $"K?t chuy?n l? t? 911 sang 4212");
        }

        // 6. Post voucher via GlVoucherBridgeService
        var postResult = await glBridge.PostOperationalVoucherAsync(voucher, request.PerformedBy, cancellationToken);
        if (!postResult.IsSuccess)
        {
            throw new InvalidOperationException($"Failed to post P&L closing voucher: {postResult.ErrorMessage}");
        }

        // 7. Record PeriodClosingRun
        var run = new PeriodClosingRun(
            PeriodClosingRunId.New(),
            fiscalPeriodId,
            DateTime.UtcNow,
            request.PerformedBy,
            ClosingStep.PnLClearance,
            $"P&L closing completed: Revenue = {totalRevenueTo911:N2}, Expenses = {totalExpenseTo911:N2}, Net = {netProfitOrLoss:N2}");

        run.AddGeneratedVoucher(voucher.Id);
        context.AddEntity(run);
        await context.SaveChangesAsync(cancellationToken);

        return Result<PeriodClosingRunId>.Success(run.Id);
    }
}
