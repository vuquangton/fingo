using Accounting.Domain.Common;

namespace Accounting.Domain.Entities.GeneralLedger;

public class FiscalPeriod : Entity<FiscalPeriodId>, IAuditableEntity
{
    public int Year { get; private set; }
    public int PeriodNumber { get; private set; }
    public int Month => PeriodNumber;
    public DateTime StartDate { get; private set; }
    public DateTime EndDate { get; private set; }
    public bool IsSoftLocked { get; private set; }
    public bool IsHardLocked { get; private set; }
    public Guid? LockedBy { get; private set; }
    public DateTime? LockedAtUtc { get; private set; }

    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    private FiscalPeriod() { }

    public FiscalPeriod(int year, int periodNumber)
    {
        Id = FiscalPeriodId.FromYearMonth(year, periodNumber);
        Year = year;
        PeriodNumber = periodNumber;
        StartDate = new DateTime(year, periodNumber, 1, 0, 0, 0, DateTimeKind.Utc);
        EndDate = StartDate.AddMonths(1).AddTicks(-1);
    }

    public FiscalPeriod(FiscalPeriodId id, int year, int periodNumber)
    {
        Id = id;
        Year = year;
        PeriodNumber = periodNumber;
        StartDate = new DateTime(year, periodNumber, 1, 0, 0, 0, DateTimeKind.Utc);
        EndDate = StartDate.AddMonths(1).AddTicks(-1);
    }

    public void SoftLock(Guid userId)
    {
        IsSoftLocked = true;
        LockedBy = userId;
        LockedAtUtc = DateTime.UtcNow;
    }

    public void HardLock(Guid userId)
    {
        IsSoftLocked = true;
        IsHardLocked = true;
        LockedBy = userId;
        LockedAtUtc = DateTime.UtcNow;
    }

    public void Unlock()
    {
        IsSoftLocked = false;
        IsHardLocked = false;
        LockedBy = null;
        LockedAtUtc = null;
    }
}

public class PeriodClosingRule : Entity<Guid>
{
    public string RuleName { get; private set; } = string.Empty;
    public string SourceAccountPrefix { get; private set; } = string.Empty; // e.g. "511", "632", "641", "642"
    public string TargetAccountNumber { get; private set; } = "911";      // TK 911 (Xác định KQKD)
    public bool IsDebitToTarget { get; private set; }                       // true if debiting 911 (expenses), false if crediting 911 (revenues)
    public int Sequence { get; private set; }

    private PeriodClosingRule() { }

    public PeriodClosingRule(string ruleName, string sourceAccountPrefix, string targetAccountNumber, bool isDebitToTarget, int sequence)
    {
        Id = Guid.NewGuid();
        RuleName = ruleName;
        SourceAccountPrefix = sourceAccountPrefix;
        TargetAccountNumber = targetAccountNumber;
        IsDebitToTarget = isDebitToTarget;
        Sequence = sequence;
    }
}
