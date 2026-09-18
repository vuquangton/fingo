using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Entities.Inventory;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.Inventory;

public record StocktakeItemLineDto(
    Guid ProductItemId,
    string ItemCode,
    string ItemName,
    string Unit,
    decimal BookQuantity,
    decimal PhysicalQuantity,
    decimal UnitCost,
    decimal VarianceQuantity,
    decimal VarianceAmount);

public record CreateStocktakeRecordCommand(
    string RecordNumber,
    DateTime AuditDate,
    Guid WarehouseId,
    string CommitteeMembers,
    List<StocktakeItemLineDto> Lines) : IRequest<Result<Guid>>;

public class CreateStocktakeRecordCommandHandler(IAccountingDbContext context) : IRequestHandler<CreateStocktakeRecordCommand, Result<Guid>>
{
    public async Task<Result<Guid>> Handle(CreateStocktakeRecordCommand request, CancellationToken cancellationToken)
    {
        var record = new StocktakeRecord(request.RecordNumber, request.AuditDate, request.WarehouseId, request.CommitteeMembers);
        foreach (var l in request.Lines)
        {
            record.AddLine(l.ProductItemId, l.BookQuantity, l.PhysicalQuantity, l.UnitCost);
        }

        context.AddEntity(record);
        await context.SaveChangesAsync(cancellationToken);

        return Result<Guid>.Success(record.Id);
    }
}
