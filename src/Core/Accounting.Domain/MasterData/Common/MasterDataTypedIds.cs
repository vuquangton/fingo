namespace Accounting.Domain.MasterData.Common;

public readonly record struct AccountId : IComparable<AccountId>
{
    public string Value { get; init; }

    public AccountId(string value)
    {
        Value = (value ?? string.Empty).Trim();
    }

    public static AccountId Empty => new(string.Empty);
    public int CompareTo(AccountId other) => string.Compare(Value, other.Value, StringComparison.OrdinalIgnoreCase);
    public override string ToString() => Value;
    public static implicit operator string(AccountId id) => id.Value;
    public static explicit operator AccountId(string value) => new(value);
}

public readonly record struct PartnerId : IComparable<PartnerId>
{
    public Guid Value { get; init; }

    public PartnerId(Guid value)
    {
        Value = value;
    }

    public static PartnerId New() => new(Guid.NewGuid());
    public static PartnerId Empty => new(Guid.Empty);
    public int CompareTo(PartnerId other) => Value.CompareTo(other.Value);
    public override string ToString() => Value.ToString("D");
    public static implicit operator Guid(PartnerId id) => id.Value;
    public static explicit operator PartnerId(Guid value) => new(value);
}

public readonly record struct CurrencyCode : IComparable<CurrencyCode>
{
    public string Value { get; init; }

    public CurrencyCode(string value)
    {
        Value = (value ?? string.Empty).Trim().ToUpperInvariant();
    }

    public static CurrencyCode Vnd => new("VND");
    public static CurrencyCode Usd => new("USD");
    public static CurrencyCode Eur => new("EUR");

    public int CompareTo(CurrencyCode other) => string.Compare(Value, other.Value, StringComparison.OrdinalIgnoreCase);
    public override string ToString() => Value;
    public static implicit operator string(CurrencyCode code) => code.Value;
    public static explicit operator CurrencyCode(string value) => new(value);
}

public readonly record struct WarehouseId : IComparable<WarehouseId>
{
    public string Value { get; init; }

    public WarehouseId(string value)
    {
        Value = (value ?? string.Empty).Trim().ToUpperInvariant();
    }

    public int CompareTo(WarehouseId other) => string.Compare(Value, other.Value, StringComparison.OrdinalIgnoreCase);
    public override string ToString() => Value;
    public static implicit operator string(WarehouseId id) => id.Value;
    public static explicit operator WarehouseId(string value) => new(value);
}

public readonly record struct UomId : IComparable<UomId>
{
    public Guid Value { get; init; }

    public UomId(Guid value)
    {
        Value = value;
    }

    public static UomId New() => new(Guid.NewGuid());
    public static UomId Empty => new(Guid.Empty);
    public int CompareTo(UomId other) => Value.CompareTo(other.Value);
    public override string ToString() => Value.ToString("D");
    public static implicit operator Guid(UomId id) => id.Value;
    public static explicit operator UomId(Guid value) => new(value);
}

public readonly record struct InventoryItemId : IComparable<InventoryItemId>
{
    public Guid Value { get; init; }

    public InventoryItemId(Guid value)
    {
        Value = value;
    }

    public static InventoryItemId New() => new(Guid.NewGuid());
    public static InventoryItemId Empty => new(Guid.Empty);
    public int CompareTo(InventoryItemId other) => Value.CompareTo(other.Value);
    public override string ToString() => Value.ToString("D");
    public static implicit operator Guid(InventoryItemId id) => id.Value;
    public static explicit operator InventoryItemId(Guid value) => new(value);
}

public readonly record struct CostCenterId : IComparable<CostCenterId>
{
    public string Value { get; init; }

    public CostCenterId(string value)
    {
        Value = (value ?? string.Empty).Trim().ToUpperInvariant();
    }

    public int CompareTo(CostCenterId other) => string.Compare(Value, other.Value, StringComparison.OrdinalIgnoreCase);
    public override string ToString() => Value;
    public static implicit operator string(CostCenterId id) => id.Value;
    public static explicit operator CostCenterId(string value) => new(value);
}

public readonly record struct DepartmentId : IComparable<DepartmentId>
{
    public string Value { get; init; }

    public DepartmentId(string value)
    {
        Value = (value ?? string.Empty).Trim().ToUpperInvariant();
    }

    public int CompareTo(DepartmentId other) => string.Compare(Value, other.Value, StringComparison.OrdinalIgnoreCase);
    public override string ToString() => Value;
    public static implicit operator string(DepartmentId id) => id.Value;
    public static explicit operator DepartmentId(string value) => new(value);
}

public readonly record struct ExchangeRateId : IComparable<ExchangeRateId>
{
    public Guid Value { get; init; }

    public ExchangeRateId(Guid value)
    {
        Value = value;
    }

    public static ExchangeRateId New() => new(Guid.NewGuid());
    public static ExchangeRateId Empty => new(Guid.Empty);
    public int CompareTo(ExchangeRateId other) => Value.CompareTo(other.Value);
    public override string ToString() => Value.ToString("D");
    public static implicit operator Guid(ExchangeRateId id) => id.Value;
    public static explicit operator ExchangeRateId(Guid value) => new(value);
}

public readonly record struct ExpenseItemId : IComparable<ExpenseItemId>
{
    public string Value { get; init; }

    public ExpenseItemId(string value)
    {
        Value = (value ?? string.Empty).Trim().ToUpperInvariant();
    }

    public int CompareTo(ExpenseItemId other) => string.Compare(Value, other.Value, StringComparison.OrdinalIgnoreCase);
    public override string ToString() => Value;
    public static implicit operator string(ExpenseItemId id) => id.Value;
    public static explicit operator ExpenseItemId(string value) => new(value);
}
