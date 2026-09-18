namespace Accounting.Domain.Common;

public readonly record struct FiscalPeriodId(int Value) : IComparable<FiscalPeriodId>
{
    public static FiscalPeriodId Empty => new(0);
    public static FiscalPeriodId FromYearMonth(int year, int month) => new(year * 100 + month);

    public int Year => Value / 100;
    public int PeriodNumber => Value % 100;
    public int Month => PeriodNumber;

    public int CompareTo(FiscalPeriodId other) => Value.CompareTo(other.Value);
    public override string ToString() => Value.ToString();

    public static implicit operator int(FiscalPeriodId id) => id.Value;
    public static explicit operator FiscalPeriodId(int value) => new(value);
}

public readonly record struct AuditTrailId(Guid Value) : IComparable<AuditTrailId>
{
    public static AuditTrailId New() => new(Guid.NewGuid());
    public static AuditTrailId Empty => new(Guid.Empty);

    public int CompareTo(AuditTrailId other) => Value.CompareTo(other.Value);
    public override string ToString() => Value.ToString("D");

    public static implicit operator Guid(AuditTrailId id) => id.Value;
    public static explicit operator AuditTrailId(Guid value) => new(value);
}
