using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Entities.GeneralLedger;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.PeriodEnd;

public record FiscalPeriodDto(
    int Year,
    int Month,
    DateTime StartDate,
    DateTime EndDate,
    bool IsSoftLocked,
    bool IsHardLocked,
    DateTime? LockedAtUtc);

public record GetFiscalPeriodsQuery : IRequest<Result<List<FiscalPeriodDto>>>;

public class GetFiscalPeriodsQueryHandler(IAccountingDbContext context) : IRequestHandler<GetFiscalPeriodsQuery, Result<List<FiscalPeriodDto>>>
{
    public async Task<Result<List<FiscalPeriodDto>>> Handle(GetFiscalPeriodsQuery request, CancellationToken cancellationToken)
    {
        var periods = await context.FiscalPeriods
            .AsNoTracking()
            .OrderBy(p => p.Year)
            .ThenBy(p => p.PeriodNumber)
            .Select(p => new FiscalPeriodDto(
                p.Year,
                p.PeriodNumber,
                p.StartDate,
                p.EndDate,
                p.IsSoftLocked,
                p.IsHardLocked,
                p.LockedAtUtc))
            .ToListAsync(cancellationToken);

        return Result<List<FiscalPeriodDto>>.Success(periods);
    }
}

public record ToggleFiscalPeriodLockCommand(int Year, int Month, bool Lock) : IRequest<Result<bool>>;

public class ToggleFiscalPeriodLockCommandHandler(
    IAccountingDbContext context,
    ICurrentUserService currentUser) : IRequestHandler<ToggleFiscalPeriodLockCommand, Result<bool>>
{
    public async Task<Result<bool>> Handle(ToggleFiscalPeriodLockCommand request, CancellationToken cancellationToken)
    {
        var period = await context.FiscalPeriods
            .FirstOrDefaultAsync(p => p.Year == request.Year && p.PeriodNumber == request.Month, cancellationToken);

        if (period == null)
        {
            return Result<bool>.Failure($"Kỳ kế toán {request.Month:D2}/{request.Year} không tồn tại.");
        }

        var userId = currentUser.UserId ?? Guid.Empty;
        if (request.Lock)
        {
            period.HardLock(userId);
        }
        else
        {
            period.Unlock();
        }

        await context.SaveChangesAsync(cancellationToken);
        return Result<bool>.Success(true);
    }
}
