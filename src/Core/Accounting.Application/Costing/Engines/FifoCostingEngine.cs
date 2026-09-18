using Accounting.Domain.Costing;
using Accounting.Domain.Exceptions;

namespace Accounting.Application.Costing.Engines;

public record FifoLayerConsumption(
    InventoryLayer Layer,
    decimal ConsumedQuantity,
    decimal UnitCost,
    decimal ConsumedAmount);

public record FifoCostingResult(
    decimal TotalCost,
    decimal AverageUnitCost,
    IReadOnlyList<FifoLayerConsumption> Consumptions);

public static class FifoCostingEngine
{
    public static FifoCostingResult CalculateFifoIssue(
        IEnumerable<InventoryLayer> openLayers,
        string itemId,
        string warehouseId,
        decimal requestedQuantity)
    {
        if (requestedQuantity <= 0m)
            throw new ArgumentException("Requested quantity must be strictly positive.", nameof(requestedQuantity));

        var availableLayers = openLayers
            .Where(l => !l.IsExhausted)
            .OrderBy(l => l.ReceiptDate)
            .ThenBy(l => l.CreatedAtUtc)
            .ToList();

        var totalAvailable = availableLayers.Sum(l => l.RemainingQuantity);
        if (requestedQuantity > totalAvailable)
        {
            throw new CostingLayerExhaustedException(itemId, warehouseId, requestedQuantity, totalAvailable);
        }

        var consumptions = new List<FifoLayerConsumption>();
        var remainingToFulfill = requestedQuantity;
        var totalCost = 0m;

        foreach (var layer in availableLayers)
        {
            if (remainingToFulfill <= 0m)
                break;

            var qtyToConsume = Math.Min(remainingToFulfill, layer.RemainingQuantity);
            var amount = decimal.Round(qtyToConsume * layer.UnitCost, 2, MidpointRounding.AwayFromZero);

            consumptions.Add(new FifoLayerConsumption(layer, qtyToConsume, layer.UnitCost, amount));
            totalCost += amount;
            remainingToFulfill -= qtyToConsume;
        }

        var avgUnitCost = decimal.Round(totalCost / requestedQuantity, 4, MidpointRounding.AwayFromZero);
        return new FifoCostingResult(totalCost, avgUnitCost, consumptions);
    }
}
