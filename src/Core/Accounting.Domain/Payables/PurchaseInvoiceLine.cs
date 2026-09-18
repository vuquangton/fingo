using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.Payables;

public class PurchaseInvoiceLine : Entity<PurchaseInvoiceLineId>
{
    public PurchaseInvoiceId InvoiceId { get; private set; }
    public AccountId AccountId { get; private set; }
    public InventoryItemId? InventoryItemId { get; private set; }
    public decimal Quantity { get; private set; }
    public decimal UnitPrice { get; private set; }
    public decimal VatRate { get; private set; } // 0, 5, 8, 10
    public decimal Amount { get; private set; }
    public decimal VatAmount { get; private set; }
    public string Description { get; private set; } = string.Empty;

    private PurchaseInvoiceLine() { }

    public PurchaseInvoiceLine(
        PurchaseInvoiceLineId id,
        PurchaseInvoiceId invoiceId,
        AccountId accountId,
        decimal quantity,
        decimal unitPrice,
        decimal vatRate,
        string description,
        InventoryItemId? inventoryItemId = null)
    {
        if (quantity <= 0)
            throw new ArgumentOutOfRangeException(nameof(quantity), "Quantity must be strictly positive.");
        if (unitPrice < 0)
            throw new ArgumentOutOfRangeException(nameof(unitPrice), "Unit price cannot be negative.");
        if (vatRate < 0)
            throw new ArgumentOutOfRangeException(nameof(vatRate), "VAT rate cannot be negative.");

        Id = id;
        InvoiceId = invoiceId;
        AccountId = accountId;
        Quantity = quantity;
        UnitPrice = unitPrice;
        VatRate = vatRate;
        Description = description.Trim();
        InventoryItemId = inventoryItemId;

        Amount = decimal.Round(quantity * unitPrice, 2, MidpointRounding.AwayFromZero);
        VatAmount = decimal.Round(Amount * (vatRate / 100m), 2, MidpointRounding.AwayFromZero);
    }
}
