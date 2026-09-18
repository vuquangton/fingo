namespace Accounting.Domain.Common;

public readonly record struct InventoryLayerId(Guid Value)
{
    public static InventoryLayerId New() => new(Guid.NewGuid());
    public static InventoryLayerId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct CostAllocationRunId(Guid Value)
{
    public static CostAllocationRunId New() => new(Guid.NewGuid());
    public static CostAllocationRunId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct FixedAssetId(Guid Value)
{
    public static FixedAssetId New() => new(Guid.NewGuid());
    public static FixedAssetId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct PrepaidExpenseId(Guid Value)
{
    public static PrepaidExpenseId New() => new(Guid.NewGuid());
    public static PrepaidExpenseId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct BomId(Guid Value)
{
    public static BomId New() => new(Guid.NewGuid());
    public static BomId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct BomLineId(Guid Value)
{
    public static BomLineId New() => new(Guid.NewGuid());
    public static BomLineId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct CostAbsorptionRunId(Guid Value)
{
    public static CostAbsorptionRunId New() => new(Guid.NewGuid());
    public static CostAbsorptionRunId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}
