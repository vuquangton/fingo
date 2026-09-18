using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Inventory;
using FluentValidation;

namespace Accounting.Application.MasterData.Validators;

public class InventoryItemValidator : AbstractValidator<InventoryItem>
{
    public InventoryItemValidator()
    {
        RuleFor(i => i.ItemCode)
            .NotEmpty().WithMessage("Item code is required.")
            .MaximumLength(50).WithMessage("Item code cannot exceed 50 characters.");

        RuleFor(i => i.ItemName)
            .NotEmpty().WithMessage("Item name is required.")
            .MaximumLength(200).WithMessage("Item name cannot exceed 200 characters.");

        RuleFor(i => i.BaseUomId)
            .Must(id => id != UomId.Empty)
            .WithMessage("Base Unit of Measure is required.");

        RuleFor(i => i.TaxRate)
            .GreaterThanOrEqualTo(0)
            .WithMessage("Tax rate cannot be negative.");
    }
}
