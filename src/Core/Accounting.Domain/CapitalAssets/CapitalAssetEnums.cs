namespace Accounting.Domain.CapitalAssets;

public enum FixedAssetType
{
    Tangible = 1,
    Intangible = 2,
    FinanceLease = 3
}

public enum FixedAssetStatus
{
    Active = 1,
    FullyDepreciated = 2,
    Disposed = 3
}

public enum PrepaidExpenseStatus
{
    Active = 1,
    Completed = 2
}
