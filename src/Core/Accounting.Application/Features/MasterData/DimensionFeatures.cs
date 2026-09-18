using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Dimensions;
using Accounting.Domain.MasterData.Inventory;
using FluentValidation;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.MasterData;

// Warehouse DTO & Commands
public record WarehouseDto(string Id, string Name, string? Address, bool IsActive);

public record CreateWarehouseCommand(string Id, string Name, string? Address = null) : IRequest<Result<WarehouseId>>;

public class CreateWarehouseCommandValidator : AbstractValidator<CreateWarehouseCommand>
{
    public CreateWarehouseCommandValidator()
    {
        RuleFor(x => x.Id).NotEmpty().MaximumLength(50);
        RuleFor(x => x.Name).NotEmpty().MaximumLength(200);
    }
}

public class CreateWarehouseCommandHandler(IAccountingDbContext context) : IRequestHandler<CreateWarehouseCommand, Result<WarehouseId>>
{
    public async Task<Result<WarehouseId>> Handle(CreateWarehouseCommand request, CancellationToken cancellationToken)
    {
        var id = new WarehouseId(request.Id);
        var exists = await context.MasterWarehouses.AnyAsync(w => w.Id == id, cancellationToken);
        if (exists)
            return Result<WarehouseId>.Failure($"Warehouse '{id.Value}' already exists.");

        var warehouse = new Warehouse(id, request.Name, request.Address);
        context.AddEntity(warehouse);
        await context.SaveChangesAsync(cancellationToken);

        return Result<WarehouseId>.Success(warehouse.Id);
    }
}

// Unit of Measure DTO & Commands
public record UnitOfMeasureDto(Guid Id, string Code, string Name, string? Description, bool IsActive);

public record CreateUnitOfMeasureCommand(string Code, string Name, string? Description = null) : IRequest<Result<UomId>>;

public class CreateUnitOfMeasureCommandValidator : AbstractValidator<CreateUnitOfMeasureCommand>
{
    public CreateUnitOfMeasureCommandValidator()
    {
        RuleFor(x => x.Code).NotEmpty().MaximumLength(20);
        RuleFor(x => x.Name).NotEmpty().MaximumLength(100);
    }
}

public class CreateUnitOfMeasureCommandHandler(IAccountingDbContext context) : IRequestHandler<CreateUnitOfMeasureCommand, Result<UomId>>
{
    public async Task<Result<UomId>> Handle(CreateUnitOfMeasureCommand request, CancellationToken cancellationToken)
    {
        var code = request.Code.Trim().ToUpperInvariant();
        var exists = await context.UnitsOfMeasure.AnyAsync(u => u.UomCode == code, cancellationToken);
        if (exists)
            return Result<UomId>.Failure($"Unit of measure '{code}' already exists.");

        var uom = new UnitOfMeasure(UomId.New(), code, request.Name, request.Description);
        context.AddEntity(uom);
        await context.SaveChangesAsync(cancellationToken);

        return Result<UomId>.Success(uom.Id);
    }
}

// Inventory Item DTO & Commands
public record InventoryItemDto(
    Guid Id,
    string Code,
    string Name,
    ItemType ItemType,
    Guid BaseUomId,
    CostingMethod CostingMethod,
    string? InventoryAccountId,
    string? CogsAccountId,
    string? RevenueAccountId,
    decimal TaxRate,
    bool IsActive);

public record CreateInventoryItemCommand(
    string Code,
    string Name,
    ItemType ItemType,
    Guid BaseUomId,
    CostingMethod CostingMethod = CostingMethod.MovingAverage,
    string? InventoryAccountId = null,
    string? CogsAccountId = null,
    string? RevenueAccountId = null,
    decimal TaxRate = 0m) : IRequest<Result<InventoryItemId>>;

public class CreateInventoryItemCommandValidator : AbstractValidator<CreateInventoryItemCommand>
{
    public CreateInventoryItemCommandValidator()
    {
        RuleFor(x => x.Code).NotEmpty().MaximumLength(50);
        RuleFor(x => x.Name).NotEmpty().MaximumLength(200);
        RuleFor(x => x.BaseUomId).NotEmpty();
        RuleFor(x => x.TaxRate).GreaterThanOrEqualTo(0);
    }
}

public class CreateInventoryItemCommandHandler(IAccountingDbContext context) : IRequestHandler<CreateInventoryItemCommand, Result<InventoryItemId>>
{
    public async Task<Result<InventoryItemId>> Handle(CreateInventoryItemCommand request, CancellationToken cancellationToken)
    {
        var code = request.Code.Trim().ToUpperInvariant();
        var exists = await context.InventoryItems.AnyAsync(i => i.ItemCode == code, cancellationToken);
        if (exists)
            return Result<InventoryItemId>.Failure($"Inventory item '{code}' already exists.");

        var uomId = new UomId(request.BaseUomId);
        var uomExists = await context.UnitsOfMeasure.AnyAsync(u => u.Id == uomId, cancellationToken);
        if (!uomExists)
            return Result<InventoryItemId>.Failure($"Unit of measure '{request.BaseUomId}' does not exist.");

        var item = new InventoryItem(
            InventoryItemId.New(),
            code,
            request.Name,
            request.ItemType,
            uomId,
            request.CostingMethod,
            request.InventoryAccountId != null ? new AccountId(request.InventoryAccountId) : null,
            request.CogsAccountId != null ? new AccountId(request.CogsAccountId) : null,
            request.RevenueAccountId != null ? new AccountId(request.RevenueAccountId) : null,
            request.TaxRate);

        context.AddEntity(item);
        await context.SaveChangesAsync(cancellationToken);

        return Result<InventoryItemId>.Success(item.Id);
    }
}

// Cost Center & Department DTOs
public record CostCenterDto(string Id, string Name, string? ParentId, bool IsActive);
public record DepartmentDto(string Id, string Name, string? ParentId, bool IsActive);
