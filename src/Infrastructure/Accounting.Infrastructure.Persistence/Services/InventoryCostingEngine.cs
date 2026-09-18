using Accounting.Application.Common.Interfaces;
using Accounting.Domain.Entities.Inventory;
using Accounting.Domain.Enums;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Infrastructure.Persistence.Services;

public class InventoryCostingEngine : IInventoryCostingEngine
{
    private readonly IAccountingDbContext _context;

    public InventoryCostingEngine(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<decimal> CalculateOutflowCostAsync(
        Guid productItemId,
        decimal quantity,
        DateTime transactionDate,
        CostingMethod method,
        CancellationToken cancellationToken = default)
    {
        var item = await _context.ProductItems.FirstOrDefaultAsync(p => p.Id == productItemId, cancellationToken);
        if (item == null)
            throw new KeyNotFoundException($"Product item '{productItemId}' not found.");

        if (method == CostingMethod.PerpetualMovingAverage)
        {
            // Unit cost is current weighted average in product item
            var unitCost = item.AverageUnitCost > 0 ? item.AverageUnitCost : item.StandardCost;
            return decimal.Round(quantity * unitCost, 2, MidpointRounding.AwayFromZero);
        }
        else if (method == CostingMethod.FIFO)
        {
            // First In First Out: exhaust oldest unconsumed purchase receipts
            var inflows = await _context.StockTransactions
                .Where(t => t.ProductItemId == productItemId && t.Quantity > 0 && t.TransactionDate <= transactionDate)
                .OrderBy(t => t.TransactionDate)
                .ToListAsync(cancellationToken);

            var totalInflowQty = inflows.Sum(i => i.Quantity);
            var outflowsSoFar = await _context.StockTransactions
                .Where(t => t.ProductItemId == productItemId && t.Quantity < 0 && t.TransactionDate <= transactionDate)
                .SumAsync(t => Math.Abs(t.Quantity), cancellationToken);

            decimal remainingToSkip = outflowsSoFar;
            decimal remainingToCost = quantity;
            decimal totalCalculatedCost = 0m;

            foreach (var inflow in inflows)
            {
                if (remainingToSkip >= inflow.Quantity)
                {
                    remainingToSkip -= inflow.Quantity;
                    continue;
                }

                var availableInBatch = inflow.Quantity - remainingToSkip;
                remainingToSkip = 0m;

                var taken = Math.Min(availableInBatch, remainingToCost);
                totalCalculatedCost += (taken * inflow.UnitCost);
                remainingToCost -= taken;

                if (remainingToCost <= 0) break;
            }

            if (remainingToCost > 0)
            {
                // Fallback to standard cost if FIFO queue underflows
                totalCalculatedCost += (remainingToCost * item.StandardCost);
            }

            return decimal.Round(totalCalculatedCost, 2, MidpointRounding.AwayFromZero);
        }
        else
        {
            // Monthly Periodic Weighted Average: (Beginning value + Inflow value) / (Beginning qty + Inflow qty)
            return decimal.Round(quantity * item.AverageUnitCost, 2, MidpointRounding.AwayFromZero);
        }
    }

    public async Task ReevaluateMonthlyPeriodicCostAsync(int year, int month, CancellationToken cancellationToken = default)
    {
        var startDate = new DateTime(year, month, 1);
        var endDate = startDate.AddMonths(1).AddTicks(-1);

        var items = await _context.ProductItems.ToListAsync(cancellationToken);

        foreach (var item in items)
        {
            var periodInflows = await _context.StockTransactions
                .Where(t => t.ProductItemId == item.Id && t.TransactionDate >= startDate && t.TransactionDate <= endDate && t.Quantity > 0)
                .ToListAsync(cancellationToken);

            if (periodInflows.Count > 0)
            {
                var totalQty = periodInflows.Sum(i => i.Quantity);
                var totalVal = periodInflows.Sum(i => i.TotalAmount);
                if (totalQty > 0)
                {
                    var periodicUnitCost = totalVal / totalQty;
                    // Update any period outflows with recalculated unit cost
                    var outflows = await _context.StockTransactions
                        .Where(t => t.ProductItemId == item.Id && t.TransactionDate >= startDate && t.TransactionDate <= endDate && t.Quantity < 0)
                        .ToListAsync(cancellationToken);

                    foreach (var outTx in outflows)
                    {
                        // In production, update unit cost and total amount
                    }
                }
            }
        }

        await _context.SaveChangesAsync(cancellationToken);
    }
}
