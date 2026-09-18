using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Currencies;
using FluentValidation;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.MasterData;

public record CurrencyDto(
    string Code,
    string Name,
    string Symbol,
    int DecimalPlaces,
    bool IsBaseCurrency,
    bool IsActive);

public record ExchangeRateDto(
    Guid Id,
    string CurrencyCode,
    DateTime ValidDate,
    decimal BuyingRate,
    decimal SellingRate,
    decimal AverageRate);

public record CreateCurrencyCommand(
    string CurrencyCode,
    string CurrencyName,
    string Symbol,
    int DecimalPlaces = 0,
    bool IsBaseCurrency = false) : IRequest<Result<CurrencyCode>>;

public class CreateCurrencyCommandValidator : AbstractValidator<CreateCurrencyCommand>
{
    public CreateCurrencyCommandValidator()
    {
        RuleFor(x => x.CurrencyCode)
            .NotEmpty().WithMessage("Currency code is required.")
            .Matches(@"^[A-Z]{3}$").WithMessage("Currency code must be 3-letter ISO code.");
        RuleFor(x => x.CurrencyName).NotEmpty().MaximumLength(100);
        RuleFor(x => x.Symbol).NotEmpty().MaximumLength(10);
        RuleFor(x => x.DecimalPlaces).InclusiveBetween(0, 6);
    }
}

public class CreateCurrencyCommandHandler(IAccountingDbContext context) : IRequestHandler<CreateCurrencyCommand, Result<CurrencyCode>>
{
    public async Task<Result<CurrencyCode>> Handle(CreateCurrencyCommand request, CancellationToken cancellationToken)
    {
        var code = new CurrencyCode(request.CurrencyCode);
        var exists = await context.Currencies.AnyAsync(c => c.Id == code, cancellationToken);
        if (exists)
            return Result<CurrencyCode>.Failure($"Currency '{code.Value}' already exists.");

        var currency = new Currency(
            code,
            request.CurrencyName,
            request.Symbol,
            request.DecimalPlaces,
            request.IsBaseCurrency);

        context.AddEntity(currency);
        await context.SaveChangesAsync(cancellationToken);

        return Result<CurrencyCode>.Success(currency.Id);
    }
}

public record UpdateExchangeRateCommand(
    string CurrencyCode,
    DateTime ValidDate,
    decimal BuyingRate,
    decimal SellingRate,
    decimal AverageRate) : IRequest<Result<ExchangeRateId>>;

public class UpdateExchangeRateCommandValidator : AbstractValidator<UpdateExchangeRateCommand>
{
    public UpdateExchangeRateCommandValidator()
    {
        RuleFor(x => x.CurrencyCode).NotEmpty().Matches(@"^[A-Z]{3}$");
        RuleFor(x => x.BuyingRate).GreaterThan(0);
        RuleFor(x => x.SellingRate).GreaterThan(0);
        RuleFor(x => x.AverageRate).GreaterThan(0);
    }
}

public class UpdateExchangeRateCommandHandler(IAccountingDbContext context) : IRequestHandler<UpdateExchangeRateCommand, Result<ExchangeRateId>>
{
    public async Task<Result<ExchangeRateId>> Handle(UpdateExchangeRateCommand request, CancellationToken cancellationToken)
    {
        var code = new CurrencyCode(request.CurrencyCode);
        var currencyExists = await context.Currencies.AnyAsync(c => c.Id == code, cancellationToken);
        if (!currencyExists)
            return Result<ExchangeRateId>.Failure($"Currency '{code.Value}' not found.");

        var date = request.ValidDate.Date;
        var existingRate = await context.ExchangeRates
            .FirstOrDefaultAsync(r => r.CurrencyCode == code && r.ValidDate.Date == date, cancellationToken);

        if (existingRate != null)
        {
            existingRate.Update(request.BuyingRate, request.SellingRate, request.AverageRate);
            await context.SaveChangesAsync(cancellationToken);
            return Result<ExchangeRateId>.Success(existingRate.Id);
        }

        var rate = new ExchangeRate(
            ExchangeRateId.New(),
            code,
            date,
            request.BuyingRate,
            request.SellingRate,
            request.AverageRate);

        context.AddEntity(rate);
        await context.SaveChangesAsync(cancellationToken);

        return Result<ExchangeRateId>.Success(rate.Id);
    }
}

public record GetCurrenciesQuery(bool IncludeInactive = false) : IRequest<Result<IReadOnlyList<CurrencyDto>>>;

public class GetCurrenciesQueryHandler(IAccountingDbContext context) : IRequestHandler<GetCurrenciesQuery, Result<IReadOnlyList<CurrencyDto>>>
{
    public async Task<Result<IReadOnlyList<CurrencyDto>>> Handle(GetCurrenciesQuery request, CancellationToken cancellationToken)
    {
        var query = context.Currencies.AsNoTracking();
        if (!request.IncludeInactive)
            query = query.Where(c => c.IsActive);

        var list = await query
            .OrderBy(c => c.Id)
            .Select(c => new CurrencyDto(
                c.Id.Value,
                c.CurrencyName,
                c.Symbol,
                c.DecimalPlaces,
                c.IsBaseCurrency,
                c.IsActive))
            .ToListAsync(cancellationToken);

        return Result<IReadOnlyList<CurrencyDto>>.Success(list);
    }
}
