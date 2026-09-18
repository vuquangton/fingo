using Accounting.Domain.MasterData.Accounts;
using FluentValidation;

namespace Accounting.Application.MasterData.Validators;

public class AccountValidator : AbstractValidator<Account>
{
    public AccountValidator()
    {
        RuleFor(a => a.Id.Value)
            .NotEmpty().WithMessage("Account code is required.")
            .Matches(@"^\d{3,20}$").WithMessage("Account code must be numeric and between 3 and 20 digits.")
            .Must(code => !Accounting.Domain.MasterData.Compliance.BannedLegacyAccountCodes.IsBanned(code))
            .WithMessage(code => $"Account code '{code}' is abolished under Circular 99/2025/TT-BTC and cannot be used.");

        RuleFor(a => a.AccountName)
            .NotEmpty().WithMessage("Account name is required.")
            .MaximumLength(200).WithMessage("Account name cannot exceed 200 characters.");

        RuleFor(a => a)
            .Must(a =>
            {
                if (!a.ParentAccountId.HasValue || string.IsNullOrWhiteSpace(a.ParentAccountId.Value.Value))
                    return true;
                return a.Id.Value.StartsWith(a.ParentAccountId.Value.Value, StringComparison.OrdinalIgnoreCase);
            })
            .WithMessage("Child account code must strictly start with parent account code.");

        RuleFor(a => a)
            .Must(a => a.AccountLevel == Account.ComputeLevel(a.Id.Value))
            .WithMessage("Account level must be strictly calculated from account code length.");

        RuleFor(a => a)
            .Must(a => !a.IsParent || !a.CanPost)
            .WithMessage("Parent account cannot accept direct postings.");

        RuleFor(a => a)
            .Must(a => !a.EffectiveTo.HasValue || a.EffectiveTo.Value >= a.EffectiveFrom)
            .WithMessage("EffectiveTo date cannot be prior to EffectiveFrom date.");
    }
}
