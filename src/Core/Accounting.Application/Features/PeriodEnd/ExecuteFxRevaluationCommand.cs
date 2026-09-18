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

public record ExecuteFxRevaluationCommand(
    int Year,
    int Month,
    string CurrencyCode,
    decimal ClosingExchangeRate,
    string PerformedBy = "system") : IRequest<Result<PeriodClosingRunId>>;

public class ExecuteFxRevaluationCommandHandler(
    IAccountingDbContext context,
    IGlVoucherBridgeService glBridge) : IRequestHandler<ExecuteFxRevaluationCommand, Result<PeriodClosingRunId>>
{
    private readonly CurrencyCode _vnd = new("VND");

    public async Task<Result<PeriodClosingRunId>> Handle(ExecuteFxRevaluationCommand request, CancellationToken cancellationToken)
    {
        if (request.ClosingExchangeRate <= 0)
            throw new ArgumentException("Closing exchange rate must be positive.", nameof(request.ClosingExchangeRate));

        var fiscalPeriodId = FiscalPeriodId.FromYearMonth(request.Year, request.Month);
        var startDate = new DateOnly(request.Year, request.Month, 1);
        var endDate = startDate.AddMonths(1).AddDays(-1);

        // 1. Idempotency Guard
        var alreadyRevalued = await context.PeriodClosingRuns
            .AnyAsync(r => r.FiscalPeriodId == fiscalPeriodId &&
                           r.ClosingStep == ClosingStep.FxRevaluation &&
                           r.Status == ClosingStatus.Success &&
                           r.Notes != null && r.Notes.Contains(request.CurrencyCode), cancellationToken);

        if (alreadyRevalued)
        {
            throw new PeriodAlreadyClosedException(request.Year, request.Month, $"FxRevaluation_{request.CurrencyCode}");
        }

        // 2. Resolve statutory accounts: 4131/413, 515, 635
        var acc413 = await context.MasterAccounts.FirstOrDefaultAsync(a => a.Id == new AccountId("4131"), cancellationToken)
            ?? await context.MasterAccounts.FirstOrDefaultAsync(a => a.Id == new AccountId("413"), cancellationToken)
            ?? throw new StatutoryComplianceException("Account 4131 / 413 (Ch�nh l?ch t? gi� do d�nh gi� l?i) not found in Chart of Accounts.");

        var acc515 = await context.MasterAccounts.FirstOrDefaultAsync(a => a.Id == new AccountId("515"), cancellationToken)
            ?? throw new StatutoryComplianceException("Account 515 (Doanh thu ho?t d?ng t�i ch�nh) not found in Chart of Accounts.");

        var acc635 = await context.MasterAccounts.FirstOrDefaultAsync(a => a.Id == new AccountId("635"), cancellationToken)
            ?? throw new StatutoryComplianceException("Account 635 (Chi ph� t�i ch�nh) not found in Chart of Accounts.");

        // 3. Find posted vouchers in this foreign currency
        var targetCurrency = new CurrencyCode(request.CurrencyCode);
        var postedForeignVouchers = await context.GlVouchers
            .Include(v => v.Lines)
            .Where(v => v.CurrencyId == targetCurrency && v.Status == VoucherStatus.Posted && v.PostingDate <= endDate)
            .ToListAsync(cancellationToken);

        // Scan monetary accounts: Cash/Bank (1112, 1122), Receivables (131), Payables (331)
        var monetaryLines = postedForeignVouchers
            .SelectMany(v => v.Lines)
            .Where(l => l.AccountId.Value.StartsWith("1112") ||
                        l.AccountId.Value.StartsWith("1122") ||
                        l.AccountId.Value.StartsWith("131") ||
                        l.AccountId.Value.StartsWith("331"))
            .ToList();

        if (monetaryLines.Count == 0)
        {
            var emptyRun = new PeriodClosingRun(
                PeriodClosingRunId.New(),
                fiscalPeriodId,
                DateTime.UtcNow,
                request.PerformedBy,
                ClosingStep.FxRevaluation,
                $"No monetary foreign currency balances found for {request.CurrencyCode} in {request.Month:D2}/{request.Year}");

            context.AddEntity(emptyRun);
            await context.SaveChangesAsync(cancellationToken);
            return Result<PeriodClosingRunId>.Success(emptyRun.Id);
        }

        var voucherId = VoucherId.New();
        var voucherNumber = $"FX-{request.Year}{request.Month:D2}-{Guid.NewGuid():N}"[..18].ToUpperInvariant();
        var voucher = new Voucher(
            voucherId,
            voucherNumber,
            VoucherType.GeneralJournal,
            endDate,
            endDate,
            $"��nh gi� l?i ngo?i t? {request.CurrencyCode} k? {request.Month:D2}/{request.Year} theo t? gi� {request.ClosingExchangeRate:N2}",
            _vnd,
            1.0m);

        var grouped = monetaryLines
            .GroupBy(l => new { l.AccountId, l.PartnerId })
            .ToList();

        decimal totalGainOrLoss = 0m;

        foreach (var group in grouped)
        {
            var debitOriginal = group.Where(l => l.EntryType == LedgerEntryType.Debit).Sum(l => l.AmountOriginal);
            var creditOriginal = group.Where(l => l.EntryType == LedgerEntryType.Credit).Sum(l => l.AmountOriginal);
            var debitBase = group.Where(l => l.EntryType == LedgerEntryType.Debit).Sum(l => l.AmountBase);
            var creditBase = group.Where(l => l.EntryType == LedgerEntryType.Credit).Sum(l => l.AmountBase);

            bool isAsset = group.Key.AccountId.Value.StartsWith("1");
            var netOriginal = isAsset ? (debitOriginal - creditOriginal) : (creditOriginal - debitOriginal);
            var netBase = isAsset ? (debitBase - creditBase) : (creditBase - debitBase);

            if (netOriginal == 0) continue;

            var revaluedBase = decimal.Round(netOriginal * request.ClosingExchangeRate, 2, MidpointRounding.AwayFromZero);
            var diff = revaluedBase - netBase; // >0: Gain for asset or more debt for liability

            if (diff == 0) continue;

            if (isAsset)
            {
                if (diff > 0) // Asset increased: Gain
                {
                    // Step 1: N? TK T�i s?n / C� 413
                    voucher.AddLine(group.Key.AccountId, LedgerEntryType.Debit, diff, $"��nh gi� tang t�i s?n {group.Key.AccountId.Value}", partnerId: group.Key.PartnerId);
                    voucher.AddLine(acc413.Id, LedgerEntryType.Credit, diff, $"L�i t? gi� {group.Key.AccountId.Value} sang 413");

                    // Step 2: N? 413 / C� 515
                    voucher.AddLine(acc413.Id, LedgerEntryType.Debit, diff, $"K?t chuy?n l�i t? gi� sang 515");
                    voucher.AddLine(acc515.Id, LedgerEntryType.Credit, diff, $"Doanh thu t�i ch�nh t? ch�nh l?ch t? gi�");
                }
                else // Asset decreased: Loss
                {
                    var loss = Math.Abs(diff);
                    // Step 1: N? 413 / C� TK T�i s?n
                    voucher.AddLine(acc413.Id, LedgerEntryType.Debit, loss, $"L? t? gi� {group.Key.AccountId.Value} sang 413");
                    voucher.AddLine(group.Key.AccountId, LedgerEntryType.Credit, loss, $"��nh gi� gi?m t�i s?n {group.Key.AccountId.Value}", partnerId: group.Key.PartnerId);

                    // Step 2: N? 635 / C� 413
                    voucher.AddLine(acc635.Id, LedgerEntryType.Debit, loss, $"Chi ph� t�i ch�nh t? ch�nh l?ch t? gi�");
                    voucher.AddLine(acc413.Id, LedgerEntryType.Credit, loss, $"K?t chuy?n l? t? gi� t? 413 sang 635");
                }
            }
            else // Liability (331)
            {
                if (diff > 0) // Liability increased: Loss
                {
                    // Step 1: N? 413 / C� TK N? ph?i tr?
                    voucher.AddLine(acc413.Id, LedgerEntryType.Debit, diff, $"L? t? gi� tang c�ng n? {group.Key.AccountId.Value}");
                    voucher.AddLine(group.Key.AccountId, LedgerEntryType.Credit, diff, $"��nh gi� tang c�ng n? {group.Key.AccountId.Value}", partnerId: group.Key.PartnerId);

                    // Step 2: N? 635 / C� 413
                    voucher.AddLine(acc635.Id, LedgerEntryType.Debit, diff, $"Chi ph� t�i ch�nh t? ch�nh l?ch t? gi� c�ng n?");
                    voucher.AddLine(acc413.Id, LedgerEntryType.Credit, diff, $"K?t chuy?n l? t? gi� c�ng n? t? 413");
                }
                else // Liability decreased: Gain
                {
                    var gain = Math.Abs(diff);
                    // Step 1: N? TK N? ph?i tr? / C� 413
                    voucher.AddLine(group.Key.AccountId, LedgerEntryType.Debit, gain, $"��nh gi� gi?m c�ng n? {group.Key.AccountId.Value}", partnerId: group.Key.PartnerId);
                    voucher.AddLine(acc413.Id, LedgerEntryType.Credit, gain, $"L�i t? gi� gi?m c�ng n? {group.Key.AccountId.Value}");

                    // Step 2: N? 413 / C� 515
                    voucher.AddLine(acc413.Id, LedgerEntryType.Debit, gain, $"K?t chuy?n l�i t? gi� c�ng n? sang 515");
                    voucher.AddLine(acc515.Id, LedgerEntryType.Credit, gain, $"Doanh thu t�i ch�nh t? gi?m c�ng n?");
                }
            }

            totalGainOrLoss += diff;
        }

        if (voucher.Lines.Count == 0)
        {
            var noDiffRun = new PeriodClosingRun(
                PeriodClosingRunId.New(),
                fiscalPeriodId,
                DateTime.UtcNow,
                request.PerformedBy,
                ClosingStep.FxRevaluation,
                $"No FX difference for {request.CurrencyCode} in {request.Month:D2}/{request.Year}");

            context.AddEntity(noDiffRun);
            await context.SaveChangesAsync(cancellationToken);
            return Result<PeriodClosingRunId>.Success(noDiffRun.Id);
        }

        var postResult = await glBridge.PostOperationalVoucherAsync(voucher, request.PerformedBy, cancellationToken);
        if (!postResult.IsSuccess)
        {
            throw new InvalidOperationException($"Failed to post FX revaluation voucher: {postResult.ErrorMessage}");
        }

        var run = new PeriodClosingRun(
            PeriodClosingRunId.New(),
            fiscalPeriodId,
            DateTime.UtcNow,
            request.PerformedBy,
            ClosingStep.FxRevaluation,
            $"Revalued {request.CurrencyCode} @ {request.ClosingExchangeRate:N2}. Total net diff = {totalGainOrLoss:N2}");

        run.AddGeneratedVoucher(voucher.Id);
        context.AddEntity(run);
        await context.SaveChangesAsync(cancellationToken);

        return Result<PeriodClosingRunId>.Success(run.Id);
    }
}
