using Accounting.Domain.Common;
using Accounting.Domain.Enums;

namespace Accounting.Domain.Entities.FixedAssets;

public class FixedAsset : AggregateRoot<Guid>
{
    public string AssetCode { get; private set; } = string.Empty; // TSCĐ-001
    public string Name { get; private set; } = string.Empty;
    public string Department { get; private set; } = string.Empty;
    public DateTime PurchaseDate { get; private set; }
    public DateTime CapitalizationDate { get; private set; }
    public decimal HistoricalCost { get; private set; }             // Nguyên giá (TK 211)
    public decimal AccumulatedDepreciation { get; private set; }    // Hao mòn lũy kế (TK 214)
    public decimal NetBookValue => HistoricalCost - AccumulatedDepreciation; // Giá trị còn lại
    public int UsefulLifeMonths { get; private set; }               // Thời gian trích KH (tháng)
    public DepreciationMethod Method { get; private set; } = DepreciationMethod.StraightLine;
    public decimal MonthlyDepreciationAmount => UsefulLifeMonths > 0 ? decimal.Round(HistoricalCost / UsefulLifeMonths, 2, MidpointRounding.AwayFromZero) : 0m;
    public bool IsDisposed { get; private set; }

    private FixedAsset() { }

    public FixedAsset(
        string assetCode,
        string name,
        string department,
        DateTime purchaseDate,
        DateTime capitalizationDate,
        decimal historicalCost,
        int usefulLifeMonths,
        DepreciationMethod method = DepreciationMethod.StraightLine)
    {
        Id = Guid.NewGuid();
        AssetCode = assetCode.Trim().ToUpperInvariant();
        Name = name.Trim();
        Department = department.Trim();
        PurchaseDate = purchaseDate;
        CapitalizationDate = capitalizationDate;
        HistoricalCost = historicalCost;
        UsefulLifeMonths = usefulLifeMonths;
        Method = method;
        AccumulatedDepreciation = 0m;
    }

    public decimal ApplyMonthlyDepreciation()
    {
        if (IsDisposed || NetBookValue <= 0) return 0m;

        var amount = Math.Min(MonthlyDepreciationAmount, NetBookValue);
        AccumulatedDepreciation += amount;
        return amount;
    }

    public void DisposeAsset() => IsDisposed = true;
}

public class PrepaidExpense : AggregateRoot<Guid>
{
    public string Code { get; private set; } = string.Empty; // CCDC / TK 242
    public string Name { get; private set; } = string.Empty;
    public decimal TotalAmount { get; private set; }
    public decimal AllocatedAmount { get; private set; }
    public decimal RemainingAmount => TotalAmount - AllocatedAmount;
    public int TotalPeriodsMonths { get; private set; }
    public int AllocatedPeriodsMonths { get; private set; }
    public decimal MonthlyAllocationAmount => TotalPeriodsMonths > 0 ? decimal.Round(TotalAmount / TotalPeriodsMonths, 2, MidpointRounding.AwayFromZero) : 0m;
    public DateTime StartDate { get; private set; }
    public bool IsFullyAllocated => AllocatedAmount >= TotalAmount || AllocatedPeriodsMonths >= TotalPeriodsMonths;

    private PrepaidExpense() { }

    public PrepaidExpense(string code, string name, decimal totalAmount, int totalPeriodsMonths, DateTime startDate)
    {
        Id = Guid.NewGuid();
        Code = code.Trim().ToUpperInvariant();
        Name = name.Trim();
        TotalAmount = totalAmount;
        TotalPeriodsMonths = totalPeriodsMonths;
        StartDate = startDate;
        AllocatedAmount = 0m;
        AllocatedPeriodsMonths = 0;
    }

    public decimal AllocateNextMonth()
    {
        if (IsFullyAllocated) return 0m;

        var amount = Math.Min(MonthlyAllocationAmount, RemainingAmount);
        AllocatedAmount += amount;
        AllocatedPeriodsMonths++;
        return amount;
    }
}
