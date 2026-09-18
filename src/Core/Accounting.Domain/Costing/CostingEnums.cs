namespace Accounting.Domain.Costing;

public enum InventoryCostingMethod
{
    Fifo = 1,
    MovingAverage = 2,
    MonthlyWeightedAverage = 3
}

public enum CostingRunStatus
{
    Pending = 1,
    Calculated = 2,
    PostedToGl = 3,
    Cancelled = 4
}
