using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.WarehouseOperations;

public class WarehouseVoucherLine : Entity<WarehouseVoucherLineId>
{
    public WarehouseVoucherId VoucherId { get; private set; }
    public InventoryItemId InventoryItemId { get; private set; }
    public UomId UnitOfMeasureId { get; private set; }
    public decimal Quantity { get; private set; }
    public decimal UnitPrice { get; private set; }
    public decimal TotalAmount { get; private set; }
    public AccountId DebitAccountId { get; private set; }
    public AccountId CreditAccountId { get; private set; }
    public string Description { get; private set; } = string.Empty;

    private WarehouseVoucherLine() { }

    public WarehouseVoucherLine(
        WarehouseVoucherLineId id,
        WarehouseVoucherId voucherId,
        InventoryItemId inventoryItemId,
        UomId unitOfMeasureId,
        decimal quantity,
        decimal unitPrice,
        AccountId debitAccountId,
        AccountId creditAccountId,
        string description)
    {
        if (quantity <= 0)
            throw new ArgumentOutOfRangeException(nameof(quantity), "Quantity must be strictly positive.");
        if (unitPrice < 0)
            throw new ArgumentOutOfRangeException(nameof(unitPrice), "Unit price cannot be negative.");

        Id = id;
        VoucherId = voucherId;
        InventoryItemId = inventoryItemId;
        UnitOfMeasureId = unitOfMeasureId;
        Quantity = quantity;
        UnitPrice = unitPrice;
        TotalAmount = decimal.Round(quantity * unitPrice, 2, MidpointRounding.AwayFromZero);
        DebitAccountId = debitAccountId;
        CreditAccountId = creditAccountId;
        Description = description.Trim();
    }
}
