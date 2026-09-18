namespace Accounting.Domain.Manufacturing;

public enum WipEvaluationMethod
{
    EquivalentUnits = 1,
    RawMaterialCostOnly = 2,
    FiftyPercentCompletion = 3
}

public enum CostAbsorptionStatus
{
    Draft = 1,
    Calculated = 2,
    PostedToGl = 3
}
