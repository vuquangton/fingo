using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.CapitalAssets;
using Accounting.Domain.Common;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using MediatR;
using Microsoft.EntityFrameworkCore;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;
using Voucher = Accounting.Domain.Ledger.Voucher;
using VoucherType = Accounting.Domain.Ledger.VoucherType;

namespace Accounting.Application.Features.CapitalAssets;

public record RegisterFixedAssetCommand(
    string AssetCode,
    string AssetName,
    FixedAssetType AssetType,
    decimal OriginalCost,
    int UsefulLifeMonths,
    DateOnly CapitalizationDate,
    DateOnly DepreciationStartDate,
    string AssetAccountCode = "2111",
    string DepreciationAccountCode = "2141",
    string ExpenseAccountCode = "6424",
    decimal ResidualValue = 0m,
    string? CostCenterId = null) : IRequest<Result<FixedAssetId>>;

public class RegisterFixedAssetCommandHandler(
    IAccountingDbContext context) : IRequestHandler<RegisterFixedAssetCommand, Result<FixedAssetId>>
{
    public async Task<Result<FixedAssetId>> Handle(RegisterFixedAssetCommand request, CancellationToken cancellationToken)
    {
        var existing = await context.CapitalFixedAssets
            .AnyAsync(a => a.AssetCode == request.AssetCode.Trim().ToUpperInvariant(), cancellationToken);

        if (existing)
            return Result<FixedAssetId>.Failure($"Fixed asset code '{request.AssetCode}' already exists.");

        var assetId = FixedAssetId.New();
        var asset = new FixedAsset(
            assetId,
            request.AssetCode,
            request.AssetName,
            request.AssetType,
            request.OriginalCost,
            request.UsefulLifeMonths,
            request.CapitalizationDate,
            request.DepreciationStartDate,
            new AccountId(request.AssetAccountCode),
            new AccountId(request.DepreciationAccountCode),
            new AccountId(request.ExpenseAccountCode),
            request.ResidualValue,
            !string.IsNullOrWhiteSpace(request.CostCenterId) ? new CostCenterId(request.CostCenterId) : null);

        context.AddEntity(asset);
        await context.SaveChangesAsync(cancellationToken);

        return Result<FixedAssetId>.Success(asset.Id);
    }
}

public record MonthlyDepreciationResultDto(
    int DepreciatedAssetCount,
    decimal TotalDepreciationAmount,
    Guid? GlVoucherId);

public record RunMonthlyDepreciationCommand(
    int Year,
    int Month,
    string PostedBy = "asset_accountant") : IRequest<Result<MonthlyDepreciationResultDto>>;

public class RunMonthlyDepreciationCommandHandler(
    IAccountingDbContext context,
    IGlVoucherBridgeService glBridge) : IRequestHandler<RunMonthlyDepreciationCommand, Result<MonthlyDepreciationResultDto>>
{
    public async Task<Result<MonthlyDepreciationResultDto>> Handle(RunMonthlyDepreciationCommand request, CancellationToken cancellationToken)
    {
        var activeAssets = await context.CapitalFixedAssets
            .Where(a => a.Status == FixedAssetStatus.Active)
            .ToListAsync(cancellationToken);

        if (activeAssets.Count == 0)
            return Result<MonthlyDepreciationResultDto>.Success(new MonthlyDepreciationResultDto(0, 0m, null));

        var periodEndDate = new DateOnly(request.Year, request.Month, 1).AddMonths(1).AddDays(-1);
        var voucherNum = $"KH-{request.Year}{request.Month:D2}-{Guid.NewGuid():N}"[..18].ToUpperInvariant();
        var glVoucher = new Voucher(
            VoucherId.New(),
            voucherNum,
            VoucherType.GeneralJournal,
            periodEndDate,
            periodEndDate,
            $"Monthly fixed asset depreciation for period {request.Month:D2}/{request.Year}",
            new CurrencyCode("VND"),
            1.0m);

        var totalAmount = 0m;
        var depreciatedCount = 0;

        foreach (var asset in activeAssets)
        {
            var amount = asset.CalculateMonthlyDepreciation(request.Year, request.Month);
            if (amount <= 0m)
                continue;

            asset.ApplyDepreciation(amount, request.Year, request.Month);
            totalAmount += amount;
            depreciatedCount++;

            // Debit Depreciation Expense Account (6424/6274)
            glVoucher.AddLine(
                asset.ExpenseAccountId,
                LedgerEntryType.Debit,
                amount,
                $"Depreciation for asset {asset.AssetCode} ({asset.AssetName})",
                costCenterId: asset.CostCenterId);

            // Credit Accumulated Depreciation Account (2141)
            glVoucher.AddLine(
                asset.DepreciationAccountId,
                LedgerEntryType.Credit,
                amount,
                $"Accumulated depreciation for asset {asset.AssetCode}");
        }

        if (totalAmount <= 0m)
            return Result<MonthlyDepreciationResultDto>.Success(new MonthlyDepreciationResultDto(0, 0m, null));

        var glResult = await glBridge.PostOperationalVoucherAsync(glVoucher, request.PostedBy, cancellationToken);
        if (!glResult.IsSuccess)
            return Result<MonthlyDepreciationResultDto>.Failure(glResult.ErrorMessage ?? "Failed to post depreciation GL voucher.");

        await context.SaveChangesAsync(cancellationToken);

        return Result<MonthlyDepreciationResultDto>.Success(new MonthlyDepreciationResultDto(
            depreciatedCount,
            totalAmount,
            glVoucher.Id.Value));
    }
}

public record RegisterPrepaidExpenseCommand(
    string ExpenseCode,
    string Name,
    decimal TotalAmount,
    int TotalPeriods,
    DateOnly StartDate,
    string SourceAccountCode = "242",
    string TargetExpenseAccountCode = "6427",
    string? CostCenterId = null) : IRequest<Result<PrepaidExpenseId>>;

public class RegisterPrepaidExpenseCommandHandler(
    IAccountingDbContext context) : IRequestHandler<RegisterPrepaidExpenseCommand, Result<PrepaidExpenseId>>
{
    public async Task<Result<PrepaidExpenseId>> Handle(RegisterPrepaidExpenseCommand request, CancellationToken cancellationToken)
    {
        var existing = await context.CapitalPrepaidExpenses
            .AnyAsync(p => p.ExpenseCode == request.ExpenseCode.Trim().ToUpperInvariant(), cancellationToken);

        if (existing)
            return Result<PrepaidExpenseId>.Failure($"Prepaid expense code '{request.ExpenseCode}' already exists.");

        var expenseId = PrepaidExpenseId.New();
        var expense = new PrepaidExpense(
            expenseId,
            request.ExpenseCode,
            request.Name,
            request.TotalAmount,
            request.TotalPeriods,
            request.StartDate,
            new AccountId(request.SourceAccountCode),
            new AccountId(request.TargetExpenseAccountCode),
            !string.IsNullOrWhiteSpace(request.CostCenterId) ? new CostCenterId(request.CostCenterId) : null);

        context.AddEntity(expense);
        await context.SaveChangesAsync(cancellationToken);

        return Result<PrepaidExpenseId>.Success(expense.Id);
    }
}

public record MonthlyAmortizationResultDto(
    int AmortizedCount,
    decimal TotalAmortizationAmount,
    Guid? GlVoucherId);

public record RunPrepaidAmortizationCommand(
    int Year,
    int Month,
    string PostedBy = "general_accountant") : IRequest<Result<MonthlyAmortizationResultDto>>;

public class RunPrepaidAmortizationCommandHandler(
    IAccountingDbContext context,
    IGlVoucherBridgeService glBridge) : IRequestHandler<RunPrepaidAmortizationCommand, Result<MonthlyAmortizationResultDto>>
{
    public async Task<Result<MonthlyAmortizationResultDto>> Handle(RunPrepaidAmortizationCommand request, CancellationToken cancellationToken)
    {
        var activeExpenses = await context.CapitalPrepaidExpenses
            .Where(p => p.Status == PrepaidExpenseStatus.Active)
            .ToListAsync(cancellationToken);

        if (activeExpenses.Count == 0)
            return Result<MonthlyAmortizationResultDto>.Success(new MonthlyAmortizationResultDto(0, 0m, null));

        var periodEndDate = new DateOnly(request.Year, request.Month, 1).AddMonths(1).AddDays(-1);
        var voucherNum = $"PB-{request.Year}{request.Month:D2}-{Guid.NewGuid():N}"[..18].ToUpperInvariant();
        var glVoucher = new Voucher(
            VoucherId.New(),
            voucherNum,
            VoucherType.GeneralJournal,
            periodEndDate,
            periodEndDate,
            $"Monthly prepaid expense amortization for period {request.Month:D2}/{request.Year}",
            new CurrencyCode("VND"),
            1.0m);

        var totalAmount = 0m;
        var amortizedCount = 0;

        foreach (var expense in activeExpenses)
        {
            var amount = expense.CalculateMonthlyAmortization();
            if (amount <= 0m)
                continue;

            expense.ApplyAmortization(amount);
            totalAmount += amount;
            amortizedCount++;

            // Debit Target Expense Account (6427/6417/627)
            glVoucher.AddLine(
                expense.TargetExpenseAccountId,
                LedgerEntryType.Debit,
                amount,
                $"Amortization of prepaid expense {expense.ExpenseCode} ({expense.Name})",
                costCenterId: expense.CostCenterId);

            // Credit Source Account (242)
            glVoucher.AddLine(
                expense.SourceAccountId,
                LedgerEntryType.Credit,
                amount,
                $"Allocation from prepaid expense {expense.ExpenseCode}");
        }

        if (totalAmount <= 0m)
            return Result<MonthlyAmortizationResultDto>.Success(new MonthlyAmortizationResultDto(0, 0m, null));

        var glResult = await glBridge.PostOperationalVoucherAsync(glVoucher, request.PostedBy, cancellationToken);
        if (!glResult.IsSuccess)
            return Result<MonthlyAmortizationResultDto>.Failure(glResult.ErrorMessage ?? "Failed to post amortization GL voucher.");

        await context.SaveChangesAsync(cancellationToken);

        return Result<MonthlyAmortizationResultDto>.Success(new MonthlyAmortizationResultDto(
            amortizedCount,
            totalAmount,
            glVoucher.Id.Value));
    }
}
