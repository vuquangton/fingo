namespace Accounting.Domain.PeriodEnd;

public enum ClosingStep
{
    FxRevaluation = 1,
    PnLClearance = 2
}

public enum ClosingStatus
{
    Success = 1,
    Failed = 2
}
