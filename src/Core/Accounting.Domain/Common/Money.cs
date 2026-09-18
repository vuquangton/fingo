namespace Accounting.Domain.Common;

public sealed class Money : ValueObject, IComparable<Money>
{
    public decimal Amount { get; }
    public string Currency { get; }

    public Money(decimal amount, string currency = CurrencyCode.VND)
    {
        if (string.IsNullOrWhiteSpace(currency))
            throw new ArgumentException("Currency code cannot be empty.", nameof(currency));

        Amount = decimal.Round(amount, 4, MidpointRounding.AwayFromZero);
        Currency = currency.ToUpperInvariant();
    }

    public static Money Zero(string currency = CurrencyCode.VND) => new(0m, currency);

    public static Money FromVnd(decimal amount) => new(amount, CurrencyCode.VND);
    public static Money FromUsd(decimal amount) => new(amount, CurrencyCode.USD);

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

    public Money Multiply(decimal multiplier) => new(Amount * multiplier, Currency);

    public Money Divide(decimal divisor)
    {
        if (divisor == 0) throw new DivideByZeroException("Cannot divide Money by zero.");
        return new Money(Amount / divisor, Currency);
    }

    public Money ConvertTo(string targetCurrency, decimal exchangeRate)
    {
        if (Currency == targetCurrency) return this;
        if (exchangeRate <= 0) throw new ArgumentOutOfRangeException(nameof(exchangeRate), "Exchange rate must be positive.");
        return new Money(Amount * exchangeRate, targetCurrency);
    }

    private void EnsureSameCurrency(Money other)
    {
        if (Currency != other.Currency)
            throw new InvalidOperationException($"Currency mismatch: cannot operate between {Currency} and {other.Currency}.");
    }

    public static Money operator +(Money a, Money b) => a.Add(b);
    public static Money operator -(Money a, Money b) => a.Subtract(b);
    public static Money operator *(Money a, decimal b) => a.Multiply(b);
    public static Money operator /(Money a, decimal b) => a.Divide(b);

    public static bool operator <(Money a, Money b)
    {
        a.EnsureSameCurrency(b);
        return a.Amount < b.Amount;
    }

    public static bool operator >(Money a, Money b)
    {
        a.EnsureSameCurrency(b);
        return a.Amount > b.Amount;
    }

    public static bool operator <=(Money a, Money b) => !(a > b);
    public static bool operator >=(Money a, Money b) => !(a < b);

    public int CompareTo(Money? other)
    {
        if (other is null) return 1;
        EnsureSameCurrency(other);
        return Amount.CompareTo(other.Amount);
    }

    protected override IEnumerable<object?> GetEqualityComponents()
    {
        yield return Amount;
        yield return Currency;
    }

    public override string ToString() => $"{Amount:N2} {Currency}";
}

public static class CurrencyCode
{
    public const string VND = "VND";
    public const string USD = "USD";
    public const string EUR = "EUR";
    public const string JPY = "JPY";
    public const string CNY = "CNY";
}

public sealed class ExchangeRate : ValueObject
{
    public string BaseCurrency { get; }
    public string QuoteCurrency { get; }
    public decimal Rate { get; }
    public DateTime EffectiveDate { get; }

    public ExchangeRate(string baseCurrency, string quoteCurrency, decimal rate, DateTime effectiveDate)
    {
        if (rate <= 0) throw new ArgumentOutOfRangeException(nameof(rate), "Exchange rate must be positive.");
        BaseCurrency = baseCurrency.ToUpperInvariant();
        QuoteCurrency = quoteCurrency.ToUpperInvariant();
        Rate = rate;
        EffectiveDate = effectiveDate.Date;
    }

    public Money Convert(Money amount)
    {
        if (amount.Currency != BaseCurrency)
            throw new InvalidOperationException($"Amount currency {amount.Currency} does not match base currency {BaseCurrency}.");
        return new Money(amount.Amount * Rate, QuoteCurrency);
    }

    protected override IEnumerable<object?> GetEqualityComponents()
    {
        yield return BaseCurrency;
        yield return QuoteCurrency;
        yield return Rate;
        yield return EffectiveDate;
    }
}
