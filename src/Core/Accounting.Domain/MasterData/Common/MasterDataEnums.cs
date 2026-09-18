namespace Accounting.Domain.MasterData.Common;

public enum AccountType
{
    Asset = 1,
    Liability = 2,
    Equity = 3,
    Revenue = 4,
    CostOfSales = 5,
    Expense = 6,
    OtherIncome = 7,
    OtherExpense = 8,
    OffBalanceSheet = 9
}

public enum BalanceNature
{
    DebitBalance = 1,
    CreditBalance = 2,
    Bilateral = 3,
    ZeroBalance = 4
}

[Flags]
public enum PartnerType
{
    None = 0,
    Customer = 1,
    Vendor = 2,
    Employee = 4,
    Other = 8
}

public enum LegalEntityType
{
    Corporate = 1,     // Doanh nghiệp / Tổ chức (có MST)
    Individual = 2,    // Cá nhân / Hộ kinh doanh (CCCD / MST cá nhân)
    Foreign = 3        // Nhà thầu / Khách hàng nước ngoài
}

public enum PartnerRiskTier
{
    Low = 1,
    Medium = 2,
    High = 3,
    Critical = 4
}

public enum CostingMethod
{
    FIFO = 1,
    MovingAverage = 2,
    MonthlyWeightedAverage = 3,
    SpecificIdentification = 4
}

public enum ItemType
{
    RawMaterial = 1,
    FinishedGoods = 2,
    Merchandise = 3,
    Service = 4,
    ToolEquipment = 5
}

public enum GoverningCircular
{
    TT99_2025_BTC = 1,
    TT89_2026_BTC = 2
}
