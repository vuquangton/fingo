using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;

namespace Accounting.Domain.MasterData.Currencies;

public class Currency : Entity<CurrencyCode>
{
    public string CurrencyName { get; private set; } = string.Empty;
    public string Symbol { get; private set; } = string.Empty;
    public bool IsBaseCurrency { get; private set; }
    public int DecimalPlaces { get; private set; }
    public bool IsActive { get; private set; } = true;

    private Currency() { }

    public Currency(CurrencyCode code, string currencyName, string symbol, int decimalPlaces = 2, bool isBaseCurrency = false)
    {
        if (string.IsNullOrWhiteSpace(code.Value) || code.Value.Length != 3)
            throw new ArgumentException("Currency code must be a 3-letter ISO code.", nameof(code));

        if (decimalPlaces < 0 || decimalPlaces > 6)
            throw new ArgumentOutOfRangeException(nameof(decimalPlaces), "Decimal places must be between 0 and 6.");

        Id = code;
        CurrencyName = currencyName.Trim();
        Symbol = symbol.Trim();
        IsBaseCurrency = isBaseCurrency;
        DecimalPlaces = decimalPlaces;
    }

    public void MarkAsBaseCurrency() => IsBaseCurrency = true;
    public void UnmarkAsBaseCurrency() => IsBaseCurrency = false;
    public void SetActive(bool isActive) => IsActive = isActive;
    public void UpdateMetadata(string currencyName, string symbol, int decimalPlaces)
    {
        CurrencyName = currencyName.Trim();
        Symbol = symbol.Trim();
        DecimalPlaces = decimalPlaces;
    }
}
