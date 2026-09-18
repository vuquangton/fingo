using FluentValidation;

namespace Accounting.Application.Features.Payables;

public class RegisterPurchaseInvoiceCommandValidator : AbstractValidator<RegisterPurchaseInvoiceCommand>
{
    private static readonly HashSet<decimal> ValidVatRates = [0m, 5m, 8m, 10m];

    public RegisterPurchaseInvoiceCommandValidator()
    {
        RuleFor(x => x.InvoiceNumber)
            .NotEmpty().WithMessage("Invoice number is required.")
            .MaximumLength(50).WithMessage("Invoice number cannot exceed 50 characters.");

        RuleFor(x => x.DueDate)
            .GreaterThanOrEqualTo(x => x.InvoiceDate)
            .WithMessage("Due date cannot precede invoice date.");

        RuleFor(x => x.Lines)
            .NotNull().WithMessage("Lines cannot be null.")
            .Must(lines => lines != null && lines.Count > 0)
            .WithMessage("Purchase invoice must contain at least one line item.");

        RuleForEach(x => x.Lines).ChildRules(line =>
        {
            line.RuleFor(l => l.AccountCode)
                .NotEmpty().WithMessage("Line account code is required.");

            line.RuleFor(l => l.Quantity)
                .GreaterThan(0).WithMessage("Quantity must be strictly positive.");

            line.RuleFor(l => l.UnitPrice)
                .GreaterThanOrEqualTo(0).WithMessage("Unit price cannot be negative.");

            line.RuleFor(l => l.VatRate)
                .Must(rate => ValidVatRates.Contains(rate))
                .WithMessage("VAT rate must be a valid statutory bracket (0%, 5%, 8%, 10%).");
        });
    }
}

public class SettleVendorInvoiceCommandValidator : AbstractValidator<SettleVendorInvoiceCommand>
{
    public SettleVendorInvoiceCommandValidator()
    {
        RuleFor(x => x.PaymentVoucherNumber)
            .NotEmpty().WithMessage("Payment voucher number is required.")
            .MaximumLength(50).WithMessage("Payment voucher number cannot exceed 50 characters.");

        RuleFor(x => x.Amount)
            .GreaterThan(0).WithMessage("Settlement amount must be strictly positive.");
    }
}
