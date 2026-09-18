using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Entities.Inventory;
using Accounting.Domain.Enums;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.Inventory;

public record CalculateInventoryCostingCommand(int Year, int Month) : IRequest<Result>;

public class CalculateInventoryCostingCommandHandler : IRequestHandler<CalculateInventoryCostingCommand, Result>
{
    private readonly IInventoryCostingEngine _costingEngine;

    public CalculateInventoryCostingCommandHandler(IInventoryCostingEngine costingEngine)
    {
        _costingEngine = costingEngine;
    }

    public async Task<Result> Handle(CalculateInventoryCostingCommand request, CancellationToken cancellationToken)
    {
        try
        {
            await _costingEngine.ReevaluateMonthlyPeriodicCostAsync(request.Year, request.Month, cancellationToken);
            return Result.Success();
        }
        catch (Exception ex)
        {
            return Result.Failure(ex.Message);
        }
    }
}

public record WarehouseTransferLineDto(Guid ProductItemId, decimal Quantity, decimal UnitCost);

public record CreateWarehouseTransferCommand(
    string TransferNumber,
    DateTime TransferDate,
    Guid SourceWarehouseId,
    Guid TargetWarehouseId,
    string Reason,
    List<WarehouseTransferLineDto> Lines) : IRequest<Result<Guid>>;

public class CreateWarehouseTransferCommandHandler : IRequestHandler<CreateWarehouseTransferCommand, Result<Guid>>
{
    private readonly IAccountingDbContext _context;

    public CreateWarehouseTransferCommandHandler(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<Result<Guid>> Handle(CreateWarehouseTransferCommand request, CancellationToken cancellationToken)
    {
        var transfer = new WarehouseTransfer(request.TransferNumber, request.TransferDate, request.SourceWarehouseId, request.TargetWarehouseId, request.Reason);
        foreach (var l in request.Lines)
        {
            transfer.AddLine(l.ProductItemId, l.Quantity, l.UnitCost);
        }

        _context.AddEntity(transfer);
        await _context.SaveChangesAsync(cancellationToken);
        return Result<Guid>.Success(transfer.Id);
    }
}

public record StockBalanceDto(
    Guid ProductItemId,
    string ItemCode,
    string ItemName,
    string Unit,
    decimal CurrentStockQuantity,
    decimal AverageUnitCost,
    decimal CurrentStockValue);

public record GetStockBalanceReportQuery : IRequest<Result<List<StockBalanceDto>>>;

public class GetStockBalanceReportQueryHandler : IRequestHandler<GetStockBalanceReportQuery, Result<List<StockBalanceDto>>>
{
    private readonly IAccountingDbContext _context;

    public GetStockBalanceReportQueryHandler(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<Result<List<StockBalanceDto>>> Handle(GetStockBalanceReportQuery request, CancellationToken cancellationToken)
    {
        var items = await _context.ProductItems.AsNoTracking().OrderBy(p => p.Code).ToListAsync(cancellationToken);
        var result = items.Select(i => new StockBalanceDto(
            i.Id,
            i.Code,
            i.Name,
            i.Unit,
            i.CurrentStockQuantity,
            i.AverageUnitCost,
            i.CurrentStockValue)).ToList();

        return Result<List<StockBalanceDto>>.Success(result);
    }
}
