using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Entities.FixedAssets;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Enums;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.FixedAssets;

public record CalculateDepreciationCommand(
    int Year,
    int Month,
    Guid DepreciationExpenseAccountId, // TK 642, 641...
    Guid AccumulatedDepreciationAccountId) // TK 214
    : IRequest<Result<Guid>>;

public class CalculateDepreciationCommandHandler : IRequestHandler<CalculateDepreciationCommand, Result<Guid>>
{
    private readonly IAccountingDbContext _context;
    private readonly ICurrentUserService _currentUser;

    public CalculateDepreciationCommandHandler(IAccountingDbContext context, ICurrentUserService currentUser)
    {
        _context = context;
        _currentUser = currentUser;
    }

    public async Task<Result<Guid>> Handle(CalculateDepreciationCommand request, CancellationToken cancellationToken)
    {
        var assets = await _context.FixedAssets.Where(a => !a.IsDisposed && a.NetBookValue > 0).ToListAsync(cancellationToken);
        decimal totalDepreciation = 0m;

        foreach (var asset in assets)
        {
            totalDepreciation += asset.ApplyMonthlyDepreciation();
        }

        if (totalDepreciation <= 0)
        {
            return Result<Guid>.Failure("No active assets require depreciation for this period.");
        }

        var userId = _currentUser.UserId ?? Guid.Empty;
        var voucherDate = new DateTime(request.Year, request.Month, DateTime.DaysInMonth(request.Year, request.Month));
        var voucher = new Voucher(
            $"KH-{request.Year}{request.Month:D2}",
            voucherDate,
            voucherDate,
            VoucherType.Depreciation,
            $"Monthly depreciation for {request.Month:D2}/{request.Year}",
            userId);

        voucher.AddLine(request.DepreciationExpenseAccountId, request.AccumulatedDepreciationAccountId, totalDepreciation, $"Depreciation {request.Month:D2}/{request.Year}");
        voucher.ValidateBalance();
        _context.AddEntity(voucher);

        await _context.SaveChangesAsync(cancellationToken);
        return Result<Guid>.Success(voucher.Id);
    }
}

public record FixedAssetSummaryDto(
    Guid Id,
    string AssetCode,
    string Name,
    string Department,
    decimal HistoricalCost,
    decimal AccumulatedDepreciation,
    decimal NetBookValue,
    int UsefulLifeMonths,
    decimal MonthlyDepreciationAmount);

public record GetFixedAssetLedgerQuery : IRequest<Result<List<FixedAssetSummaryDto>>>;

public class GetFixedAssetLedgerQueryHandler : IRequestHandler<GetFixedAssetLedgerQuery, Result<List<FixedAssetSummaryDto>>>
{
    private readonly IAccountingDbContext _context;

    public GetFixedAssetLedgerQueryHandler(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<Result<List<FixedAssetSummaryDto>>> Handle(GetFixedAssetLedgerQuery request, CancellationToken cancellationToken)
    {
        var list = await _context.FixedAssets.AsNoTracking()
            .OrderBy(a => a.AssetCode)
            .Select(a => new FixedAssetSummaryDto(
                a.Id,
                a.AssetCode,
                a.Name,
                a.Department,
                a.HistoricalCost,
                a.AccumulatedDepreciation,
                a.NetBookValue,
                a.UsefulLifeMonths,
                a.MonthlyDepreciationAmount))
            .ToListAsync(cancellationToken);

        return Result<List<FixedAssetSummaryDto>>.Success(list);
    }
}
