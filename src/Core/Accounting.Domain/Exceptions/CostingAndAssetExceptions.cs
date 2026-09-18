namespace Accounting.Domain.Exceptions;

public class AssetAlreadyDepreciatedException : AccountingDomainException
{
    public AssetAlreadyDepreciatedException(string assetCode, string reason)
        : base($"Fixed asset '{assetCode}' cannot accept further depreciation: {reason}.")
    {
        AssetCode = assetCode;
        Reason = reason;
    }

    public string AssetCode { get; }
    public string Reason { get; }
}

public class PrepaidExpenseFullyAmortizedException : AccountingDomainException
{
    public PrepaidExpenseFullyAmortizedException(string expenseCode, string reason)
        : base($"Prepaid expense '{expenseCode}' cannot accept further amortization: {reason}.")
    {
        ExpenseCode = expenseCode;
        Reason = reason;
    }

    public string ExpenseCode { get; }
    public string Reason { get; }
}

public class InvalidCostAllocationException : AccountingDomainException
{
    public InvalidCostAllocationException(string message)
        : base(message)
    {
    }
}

public class CostingLayerExhaustedException : AccountingDomainException
{
    public CostingLayerExhaustedException(string itemId, string warehouseId, decimal requestedQty, decimal availableQty)
        : base($"Insufficient FIFO layers for item '{itemId}' in warehouse '{warehouseId}'. Requested {requestedQty:N4}, but only {availableQty:N4} available across active layers.")
    {
        ItemId = itemId;
        WarehouseId = warehouseId;
        RequestedQty = requestedQty;
        AvailableQty = availableQty;
    }

    public string ItemId { get; }
    public string WarehouseId { get; }
    public decimal RequestedQty { get; }
    public decimal AvailableQty { get; }
}
