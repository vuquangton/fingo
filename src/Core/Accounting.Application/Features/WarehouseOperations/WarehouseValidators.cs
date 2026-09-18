using FluentValidation;

namespace Accounting.Application.Features.WarehouseOperations;

public class PostWarehouseVoucherCommandValidator : AbstractValidator<PostWarehouseVoucherCommand>
{
    public PostWarehouseVoucherCommandValidator()
    {
        RuleFor(x => x.VoucherNumber)
            .NotEmpty().WithMessage("Warehouse voucher number is required.")
            .MaximumLength(50).WithMessage("Voucher number cannot exceed 50 characters.");

        RuleFor(x => x.WarehouseId)
            .NotEmpty().WithMessage("Warehouse ID is required.");

        RuleFor(x => x.Lines)
            .NotNull().WithMessage("Lines cannot be null.")
            .Must(lines => lines != null && lines.Count > 0)
            .WithMessage("Warehouse voucher must contain at least one line item.");

        RuleForEach(x => x.Lines).ChildRules(line =>
        {
            line.RuleFor(l => l.Quantity)
                .GreaterThan(0).WithMessage("Line quantity must be strictly positive.");

            line.RuleFor(l => l.UnitPrice)
                .GreaterThanOrEqualTo(0).WithMessage("Line unit price cannot be negative.");

            line.RuleFor(l => l.DebitAccountCode)
                .NotEmpty().WithMessage("Debit account code is required.");

            line.RuleFor(l => l.CreditAccountCode)
                .NotEmpty().WithMessage("Credit account code is required.");
        });
    }
}
