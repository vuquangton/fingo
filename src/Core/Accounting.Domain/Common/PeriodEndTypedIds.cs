namespace Accounting.Domain.Common;

public readonly record struct PeriodClosingRunId(Guid Value)
{
    public static PeriodClosingRunId New() => new(Guid.NewGuid());
    public static PeriodClosingRunId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();

    public static implicit operator Guid(PeriodClosingRunId id) => id.Value;
    public static explicit operator PeriodClosingRunId(Guid id) => new(id);
}

public readonly record struct ReportTemplateId(string Value)
{
    public override string ToString() => Value;

    public static implicit operator string(ReportTemplateId id) => id.Value;
    public static explicit operator ReportTemplateId(string id) => new(id);
}
