namespace Accounting.Application.Costing.Engines;

public record MonthlyWeightedAverageResult(
    decimal OpeningQuantity,
    decimal OpeningValue,
    decimal InwardQuantity,
    decimal InwardValue,
    decimal TotalAvailableQuantity,
    decimal TotalAvailableValue,
    decimal WeightedAverageUnitCost,
    decimal TotalOutwardQuantity,
    decimal ActualOutwardCost,
    decimal ProvisionedOutwardCost,
    decimal AdjustmentVariance);

public static class MonthlyWeightedAverageEngine
{
    public static MonthlyWeightedAverageResult Calculate(
        decimal openingQuantity,
        decimal openingValue,
        decimal inwardQuantity,
        decimal inwardValue,
        decimal totalOutwardQuantity,
        decimal provisionedOutwardCost)
    {
        if (openingQuantity < 0m || openingValue < 0m || inwardQuantity < 0m || inwardValue < 0m || totalOutwardQuantity < 0m)
            throw new ArgumentException("Quantities and values must be non-negative.");

        var totalQty = openingQuantity + inwardQuantity;
        var totalVal = openingValue + inwardValue;

        var unitCost = totalQty > 0m
            ? decimal.Round(totalVal / totalQty, 4, MidpointRounding.AwayFromZero)
            : 0m;

        var actualOutwardCost = decimal.Round(totalOutwardQuantity * unitCost, 2, MidpointRounding.AwayFromZero);
        var adjustmentVariance = actualOutwardCost - provisionedOutwardCost;

        return new MonthlyWeightedAverageResult(
            openingQuantity,
            openingValue,
            inwardQuantity,
            inwardValue,
            totalQty,
            totalVal,
            unitCost,
            totalOutwardQuantity,
            actualOutwardCost,
            provisionedOutwardCost,
            adjustmentVariance);
    }
}
