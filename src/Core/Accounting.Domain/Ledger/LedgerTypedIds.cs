namespace Accounting.Domain.Ledger;

public readonly record struct VoucherId(Guid Value) : IComparable<VoucherId>
{
    public static VoucherId New() => new(Guid.NewGuid());
    public static VoucherId Empty => new(Guid.Empty);

    public int CompareTo(VoucherId other) => Value.CompareTo(other.Value);
    public override string ToString() => Value.ToString("D");

    public static implicit operator Guid(VoucherId id) => id.Value;
    public static explicit operator VoucherId(Guid value) => new(value);
}

public readonly record struct VoucherLineId(Guid Value) : IComparable<VoucherLineId>
{
    public static VoucherLineId New() => new(Guid.NewGuid());
    public static VoucherLineId Empty => new(Guid.Empty);

    public int CompareTo(VoucherLineId other) => Value.CompareTo(other.Value);
    public override string ToString() => Value.ToString("D");

    public static implicit operator Guid(VoucherLineId id) => id.Value;
    public static explicit operator VoucherLineId(Guid value) => new(value);
}
