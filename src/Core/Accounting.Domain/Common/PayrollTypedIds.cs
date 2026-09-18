namespace Accounting.Domain.Common;

public readonly record struct EmployeeId(Guid Value)
{
    public static EmployeeId New() => new(Guid.NewGuid());
    public static EmployeeId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct PayrollRunId(Guid Value)
{
    public static PayrollRunId New() => new(Guid.NewGuid());
    public static PayrollRunId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct PayslipId(Guid Value)
{
    public static PayslipId New() => new(Guid.NewGuid());
    public static PayslipId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct TimesheetId(Guid Value)
{
    public static TimesheetId New() => new(Guid.NewGuid());
    public static TimesheetId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}
