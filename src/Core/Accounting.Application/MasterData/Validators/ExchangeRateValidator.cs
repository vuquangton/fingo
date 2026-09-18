using Accounting.Domain.MasterData.Currencies;
using FluentValidation;

namespace Accounting.Application.MasterData.Validators;

public class ExchangeRateValidator : AbstractValidator<ExchangeRate>
{
    public ExchangeRateValidator()
    {
        RuleFor(r => r.CurrencyCode.Value)
            .NotEmpty().WithMessage("Currency code is required.")
            .Matches(@"^[A-Z]{3}$").WithMessage("Currency code must be a 3-letter ISO code.");

        RuleFor(r => r.BuyingRate)
            .GreaterThan(0)
            .WithMessage("Buying exchange rate must be strictly positive.");

        RuleFor(r => r.SellingRate)
            .GreaterThan(0)
            .WithMessage("Selling exchange rate must be strictly positive.");

        RuleFor(r => r.AverageRate)
            .GreaterThan(0)
            .WithMessage("Average exchange rate must be strictly positive.");

        RuleFor(r => r.ValidDate)
            .Must(d => d.Date <= DateTime.UtcNow.Date.AddDays(1))
            .WithMessage("Exchange rate cannot be set for distant future dates.");
    }
}
