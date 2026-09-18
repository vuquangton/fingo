using FluentValidation;

namespace Accounting.Application.Features.Manufacturing;

public class CreateBillOfMaterialsCommandValidator : AbstractValidator<CreateBillOfMaterialsCommand>
{
    public CreateBillOfMaterialsCommandValidator()
    {
        RuleFor(x => x.FinishedGoodItemId).NotEmpty().WithMessage("Finished good item ID is required.");
        RuleFor(x => x.Version).NotEmpty().WithMessage("Version is required.").MaximumLength(20);
        RuleFor(x => x.Lines).NotNull().Must(lines => lines != null && lines.Count > 0)
            .WithMessage("BOM must contain at least one line item.");
        RuleForEach(x => x.Lines).ChildRules(line =>
        {
            line.RuleFor(l => l.MaterialItemId).NotEmpty().WithMessage("Material item ID is required.");
            line.RuleFor(l => l.StandardQuantity).GreaterThan(0).WithMessage("Standard quantity must be strictly positive.");
            line.RuleFor(l => l.ScrapPercentage).InclusiveBetween(0, 100).WithMessage("Scrap percentage must be between 0 and 100.");
        });
    }
}

public class CalculateManufacturingCostCommandValidator : AbstractValidator<CalculateManufacturingCostCommand>
{
    public CalculateManufacturingCostCommandValidator()
    {
        RuleFor(x => x.Year).InclusiveBetween(2000, 2100).WithMessage("Invalid fiscal year.");
        RuleFor(x => x.Month).InclusiveBetween(1, 12).WithMessage("Month must be between 1 and 12.");
        RuleFor(x => x.FinishedGoodItemId).NotEmpty().WithMessage("Finished good item ID is required.");
        RuleFor(x => x.WarehouseId).NotEmpty().WithMessage("Warehouse ID is required.");
        RuleFor(x => x.ProducedQuantity).GreaterThan(0).WithMessage("Produced quantity must be strictly positive.");
        RuleFor(x => x.DirectMaterialCost).GreaterThanOrEqualTo(0).WithMessage("Direct material cost cannot be negative.");
        RuleFor(x => x.DirectLaborCost).GreaterThanOrEqualTo(0).WithMessage("Direct labor cost cannot be negative.");
        RuleFor(x => x.OverheadCost).GreaterThanOrEqualTo(0).WithMessage("Overhead cost cannot be negative.");
        RuleFor(x => x.WipBeginning).GreaterThanOrEqualTo(0).WithMessage("WIP beginning cannot be negative.");
        RuleFor(x => x.WipEnding).GreaterThanOrEqualTo(0).WithMessage("WIP ending cannot be negative.");
    }
}
