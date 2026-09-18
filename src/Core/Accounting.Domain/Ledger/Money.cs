using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.Ledger;

/// <summary>
/// Immutable value object representing a monetary amount bound to a specific ISO currency.
/// Prevents currency mixing without explicit exchange rate conversion.
/// </summary>
public readonly record struct Money(decimal Amount, CurrencyCode Currency) : IComparable<Money>
{
    public static Money Zero(CurrencyCode currency) => new(0m, currency);

    public static Money Vnd(decimal amount) => new(amount, CurrencyCode.Vnd);
    public static Money Usd(decimal amount) => new(amount, CurrencyCode.Usd);
    public static Money Eur(decimal amount) => new(amount, CurrencyCode.Eur);

    public Money Add(Money other)
    {
        EnsureSameCurrency(other);
        return new Money(Amount + other.Amount, Currency);
    }

    public Money Subtract(Money other)
    {
        EnsureSameCurrency(other);
        return new Money(Amount - other.Amount, Currency);
    }

    public Money ConvertToBase(decimal exchangeRate, CurrencyCode baseCurrency = default)
    {
        if (exchangeRate <= 0)
            throw new ArgumentOutOfRangeException(nameof(exchangeRate), "Exchange rate must be strictly positive.");

        var targetCurrency = baseCurrency == default ? CurrencyCode.Vnd : baseCurrency;
        if (Currency == targetCurrency)
            return new Money(Amount, targetCurrency);

        var convertedAmount = decimal.Round(Amount * exchangeRate, 2, MidpointRounding.AwayFromZero);
        return new Money(convertedAmount, targetCurrency);
    }

    public static Money operator +(Money left, Money right) => left.Add(right);
    public static Money operator -(Money left, Money right) => left.Subtract(right);

    public int CompareTo(Money other)
    {
        EnsureSameCurrency(other);
        return Amount.CompareTo(other.Amount);
    }

    public override string ToString() => $"{Amount:N2} {Currency.Value}";

    private void EnsureSameCurrency(Money other)
    {
        if (Currency != other.Currency)
            throw new InvalidOperationException($"Cannot operate on money with differing currencies: '{Currency.Value}' vs '{other.Currency.Value}'. Convert first.");
    }
}
