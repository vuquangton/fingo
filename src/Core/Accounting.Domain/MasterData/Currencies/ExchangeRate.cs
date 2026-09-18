using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;

namespace Accounting.Domain.MasterData.Currencies;

public class ExchangeRate : Entity<ExchangeRateId>
{
    public CurrencyCode CurrencyCode { get; private set; }
    public CurrencyCode CurrencyId => CurrencyCode;
    public Currency? Currency { get; private set; }
    public DateTime ValidDate { get; private set; }
    public DateOnly ValidDateOnly => DateOnly.FromDateTime(ValidDate);
    public decimal BuyingRate { get; private set; }
    public decimal BuyRate => BuyingRate;
    public decimal SellingRate { get; private set; }
    public decimal SellRate => SellingRate;
    public decimal AverageRate { get; private set; }
    public decimal AccountingRate => AverageRate;

    private ExchangeRate() { }

    public ExchangeRate(
        ExchangeRateId id,
        CurrencyCode currencyCode,
        DateTime validDate,
        decimal buyingRate,
        decimal sellingRate,
        decimal averageRate)
    {
        if (buyingRate <= 0) throw new ArgumentOutOfRangeException(nameof(buyingRate), "Buying rate must be positive.");
        if (sellingRate <= 0) throw new ArgumentOutOfRangeException(nameof(sellingRate), "Selling rate must be positive.");
        if (averageRate <= 0) throw new ArgumentOutOfRangeException(nameof(averageRate), "Average rate must be positive.");

        Id = id;
        CurrencyCode = currencyCode;
        ValidDate = validDate.Date;
        BuyingRate = buyingRate;
        SellingRate = sellingRate;
        AverageRate = averageRate;
    }

    public void Update(decimal buyingRate, decimal sellingRate, decimal averageRate)
    {
        if (buyingRate <= 0) throw new ArgumentOutOfRangeException(nameof(buyingRate), "Buying rate must be positive.");
        if (sellingRate <= 0) throw new ArgumentOutOfRangeException(nameof(sellingRate), "Selling rate must be positive.");
        if (averageRate <= 0) throw new ArgumentOutOfRangeException(nameof(averageRate), "Average rate must be positive.");

        BuyingRate = buyingRate;
        SellingRate = sellingRate;
        AverageRate = averageRate;
    }
}
