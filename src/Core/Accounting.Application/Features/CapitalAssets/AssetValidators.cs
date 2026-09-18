using FluentValidation;

namespace Accounting.Application.Features.CapitalAssets;

public class RegisterFixedAssetCommandValidator : AbstractValidator<RegisterFixedAssetCommand>
{
    public RegisterFixedAssetCommandValidator()
    {
        RuleFor(x => x.AssetCode)
            .NotEmpty().WithMessage("Asset code is required.")
            .MaximumLength(50).WithMessage("Asset code cannot exceed 50 characters.");

        RuleFor(x => x.AssetName)
            .NotEmpty().WithMessage("Asset name is required.")
            .MaximumLength(250).WithMessage("Asset name cannot exceed 250 characters.");

        RuleFor(x => x.OriginalCost)
            .GreaterThan(0).WithMessage("Original cost must be strictly positive.");

        RuleFor(x => x.ResidualValue)
            .GreaterThanOrEqualTo(0).WithMessage("Residual value cannot be negative.")
            .LessThan(x => x.OriginalCost).WithMessage("Residual value must be less than original cost.");

        RuleFor(x => x.UsefulLifeMonths)
            .GreaterThan(0).WithMessage("Useful life in months must be strictly positive.");

        RuleFor(x => x.AssetAccountCode).NotEmpty().WithMessage("Asset account code is required.");
        RuleFor(x => x.DepreciationAccountCode).NotEmpty().WithMessage("Depreciation account code is required.");
        RuleFor(x => x.ExpenseAccountCode).NotEmpty().WithMessage("Expense account code is required.");
    }
}

public class RunMonthlyDepreciationCommandValidator : AbstractValidator<RunMonthlyDepreciationCommand>
{
    public RunMonthlyDepreciationCommandValidator()
    {
        RuleFor(x => x.Year).InclusiveBetween(2000, 2100).WithMessage("Invalid fiscal year.");
        RuleFor(x => x.Month).InclusiveBetween(1, 12).WithMessage("Month must be between 1 and 12.");
    }
}

public class RegisterPrepaidExpenseCommandValidator : AbstractValidator<RegisterPrepaidExpenseCommand>
{
    public RegisterPrepaidExpenseCommandValidator()
    {
        RuleFor(x => x.ExpenseCode)
            .NotEmpty().WithMessage("Expense code is required.")
            .MaximumLength(50).WithMessage("Expense code cannot exceed 50 characters.");

        RuleFor(x => x.Name)
            .NotEmpty().WithMessage("Expense name is required.")
            .MaximumLength(250).WithMessage("Expense name cannot exceed 250 characters.");

        RuleFor(x => x.TotalAmount)
            .GreaterThan(0).WithMessage("Total amount must be strictly positive.");

        RuleFor(x => x.TotalPeriods)
            .GreaterThan(0).WithMessage("Total periods must be strictly positive.");

        RuleFor(x => x.SourceAccountCode).NotEmpty().WithMessage("Source account code is required.");
        RuleFor(x => x.TargetExpenseAccountCode).NotEmpty().WithMessage("Target expense account code is required.");
    }
}

public class RunPrepaidAmortizationCommandValidator : AbstractValidator<RunPrepaidAmortizationCommand>
{
    public RunPrepaidAmortizationCommandValidator()
    {
        RuleFor(x => x.Year).InclusiveBetween(2000, 2100).WithMessage("Invalid fiscal year.");
        RuleFor(x => x.Month).InclusiveBetween(1, 12).WithMessage("Month must be between 1 and 12.");
    }
}
