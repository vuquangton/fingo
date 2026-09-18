namespace Accounting.Domain.Exceptions;

public class OverSettlementException : AccountingDomainException
{
    public string DocumentNumber { get; }
    public decimal RequestedAmount { get; }
    public decimal RemainingBalance { get; }

    public OverSettlementException(string documentNumber, decimal requestedAmount, decimal remainingBalance)
        : base($"Settlement amount {requestedAmount:N2} exceeds the remaining unpaid balance {remainingBalance:N2} for document '{documentNumber}'.")
    {
        DocumentNumber = documentNumber;
        RequestedAmount = requestedAmount;
        RemainingBalance = remainingBalance;
    }
}

public class InsufficientStockException : AccountingDomainException
{
    public string ItemCode { get; }
    public string WarehouseCode { get; }
    public decimal RequestedQuantity { get; }
    public decimal AvailableQuantity { get; }

    public InsufficientStockException(string itemCode, string warehouseCode, decimal requestedQuantity, decimal availableQuantity)
        : base($"Insufficient stock for item '{itemCode}' in warehouse '{warehouseCode}'. Requested: {requestedQuantity:N4}, Available: {availableQuantity:N4}.")
    {
        ItemCode = itemCode;
        WarehouseCode = warehouseCode;
        RequestedQuantity = requestedQuantity;
        AvailableQuantity = availableQuantity;
    }
}

public class SubLedgerValidationException : AccountingDomainException
{
    public SubLedgerValidationException(string message) : base(message) { }
}
