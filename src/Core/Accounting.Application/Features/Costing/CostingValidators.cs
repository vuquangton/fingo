using FluentValidation;

namespace Accounting.Application.Features.Costing;

public class RecordInwardInventoryLayerCommandValidator : AbstractValidator<RecordInwardInventoryLayerCommand>
{
    public RecordInwardInventoryLayerCommandValidator()
    {
        RuleFor(x => x.WarehouseId).NotEmpty().WithMessage("Warehouse ID is required.");
        RuleFor(x => x.InventoryItemId).NotEmpty().WithMessage("Inventory Item ID is required.");
        RuleFor(x => x.Quantity).GreaterThan(0).WithMessage("Quantity must be strictly positive.");
        RuleFor(x => x.UnitCost).GreaterThanOrEqualTo(0).WithMessage("Unit cost cannot be negative.");
    }
}

public class RunMonthlyWeightedAverageCostingCommandValidator : AbstractValidator<RunMonthlyWeightedAverageCostingCommand>
{
    public RunMonthlyWeightedAverageCostingCommandValidator()
    {
        RuleFor(x => x.Year).InclusiveBetween(2000, 2100).WithMessage("Invalid fiscal year.");
        RuleFor(x => x.Month).InclusiveBetween(1, 12).WithMessage("Month must be between 1 and 12.");
        RuleFor(x => x.WarehouseId).NotEmpty().WithMessage("Warehouse ID is required.");
        RuleFor(x => x.InventoryItemId).NotEmpty().WithMessage("Inventory Item ID is required.");
    }
}
