using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Partners;
using FluentValidation;

namespace Accounting.Application.MasterData.Validators;

public class BusinessPartnerValidator : AbstractValidator<BusinessPartner>
{
    public BusinessPartnerValidator()
    {
        RuleFor(p => p.PartnerCode)
            .NotEmpty().WithMessage("Partner code is required.")
            .MaximumLength(50).WithMessage("Partner code cannot exceed 50 characters.");

        RuleFor(p => p.Name)
            .NotEmpty().WithMessage("Partner name is required.")
            .MaximumLength(200).WithMessage("Partner name cannot exceed 200 characters.");

        RuleFor(p => p.PartnerType)
            .Must(t => t != PartnerType.None)
            .WithMessage("At least one partner type role must be assigned.");

        RuleFor(p => p.TaxCode)
            .Matches(@"^\d{10}(-\d{3})?$")
            .When(p => !string.IsNullOrWhiteSpace(p.TaxCode))
            .WithMessage("Tax code must match Vietnamese statutory format: 10 digits or 10 digits followed by a hyphen and 3 branch digits (e.g. 0101234567 or 0101234567-001).");

        RuleFor(p => p.CreditLimit)
            .GreaterThanOrEqualTo(0)
            .WithMessage("Credit limit cannot be negative.");

        RuleFor(p => p.PaymentTermDays)
            .GreaterThanOrEqualTo(0)
            .WithMessage("Payment term days cannot be negative.");
    }
}
