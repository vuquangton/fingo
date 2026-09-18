using Accounting.Domain.Common;
using Accounting.Domain.Exceptions;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.CapitalAssets;

public class FixedAsset : AggregateRoot<FixedAssetId>, IAuditableEntity, ISoftDeletable
{
    public string AssetCode { get; private set; } = string.Empty;
    public string AssetName { get; private set; } = string.Empty;
    public FixedAssetType AssetType { get; private set; }
    public decimal OriginalCost { get; private set; }
    public decimal ResidualValue { get; private set; }
    public decimal DepreciableAmount => OriginalCost - ResidualValue;
    public decimal AccumulatedDepreciation { get; private set; }
    public decimal RemainingBookValue => OriginalCost - AccumulatedDepreciation;
    public int UsefulLifeMonths { get; private set; }
    public int RemainingMonths { get; private set; }
    public DateOnly CapitalizationDate { get; private set; }
    public DateOnly DepreciationStartDate { get; private set; }
    public AccountId AssetAccountId { get; private set; }
    public AccountId DepreciationAccountId { get; private set; }
    public AccountId ExpenseAccountId { get; private set; }
    public CostCenterId? CostCenterId { get; private set; }
    public FixedAssetStatus Status { get; private set; }

    // IAuditableEntity
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    // ISoftDeletable
    public bool IsDeleted { get; set; }
    public DateTime? DeletedAtUtc { get; set; }
    public string? DeletedBy { get; set; }

    private FixedAsset() { }

    public FixedAsset(
        FixedAssetId id,
        string assetCode,
        string assetName,
        FixedAssetType assetType,
        decimal originalCost,
        int usefulLifeMonths,
        DateOnly capitalizationDate,
        DateOnly depreciationStartDate,
        AccountId assetAccountId,
        AccountId depreciationAccountId,
        AccountId expenseAccountId,
        decimal residualValue = 0m,
        CostCenterId? costCenterId = null)
    {
        if (id == FixedAssetId.Empty)
            throw new ArgumentException("Asset ID cannot be empty.", nameof(id));

        var code = (assetCode ?? string.Empty).Trim().ToUpperInvariant();
        if (string.IsNullOrWhiteSpace(code))
            throw new ArgumentException("Asset code cannot be empty.", nameof(assetCode));

        var name = (assetName ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(name))
            throw new ArgumentException("Asset name cannot be empty.", nameof(assetName));

        if (originalCost <= 0m)
            throw new ArgumentException("Original cost must be strictly positive.", nameof(originalCost));

        if (residualValue < 0m || residualValue >= originalCost)
            throw new ArgumentException("Residual value must be non-negative and less than original cost.", nameof(residualValue));

        if (usefulLifeMonths <= 0)
            throw new ArgumentException("Useful life in months must be strictly positive.", nameof(usefulLifeMonths));

        Id = id;
        AssetCode = code;
        AssetName = name;
        AssetType = assetType;
        OriginalCost = originalCost;
        ResidualValue = residualValue;
        AccumulatedDepreciation = 0m;
        UsefulLifeMonths = usefulLifeMonths;
        RemainingMonths = usefulLifeMonths;
        CapitalizationDate = capitalizationDate;
        DepreciationStartDate = depreciationStartDate;
        AssetAccountId = assetAccountId;
        DepreciationAccountId = depreciationAccountId;
        ExpenseAccountId = expenseAccountId;
        CostCenterId = costCenterId;
        Status = FixedAssetStatus.Active;
    }

    public decimal CalculateMonthlyDepreciation(int year, int month)
    {
        if (Status != FixedAssetStatus.Active)
            return 0m;

        var periodDate = new DateOnly(year, month, 1);
        var startMonth = new DateOnly(DepreciationStartDate.Year, DepreciationStartDate.Month, 1);
        if (periodDate < startMonth)
            return 0m;

        var remainingDepreciable = DepreciableAmount - AccumulatedDepreciation;
        if (remainingDepreciable <= 0m)
            return 0m;

        if (RemainingMonths <= 1)
            return remainingDepreciable;

        var standardMonthly = decimal.Round(DepreciableAmount / UsefulLifeMonths, 2, MidpointRounding.AwayFromZero);
        return Math.Min(standardMonthly, remainingDepreciable);
    }

    public decimal ApplyDepreciation(decimal amount, int year, int month)
    {
        if (Status == FixedAssetStatus.FullyDepreciated)
            throw new AssetAlreadyDepreciatedException(AssetCode, "asset is already fully depreciated");

        if (Status == FixedAssetStatus.Disposed)
            throw new AssetAlreadyDepreciatedException(AssetCode, "asset is already disposed");

        if (amount <= 0m)
            throw new ArgumentException("Depreciation amount must be strictly positive.", nameof(amount));

        var maxAllowed = DepreciableAmount - AccumulatedDepreciation;
        var actualAmount = Math.Min(amount, maxAllowed);

        AccumulatedDepreciation += actualAmount;
        RemainingMonths = Math.Max(0, RemainingMonths - 1);

        if (AccumulatedDepreciation >= DepreciableAmount || RemainingMonths == 0)
        {
            Status = FixedAssetStatus.FullyDepreciated;
        }

        return actualAmount;
    }

    public void DisposeAsset()
    {
        if (Status == FixedAssetStatus.Disposed)
            throw new InvalidOperationException($"Fixed asset '{AssetCode}' is already disposed.");

        Status = FixedAssetStatus.Disposed;
    }
}
