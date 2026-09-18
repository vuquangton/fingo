using Accounting.Domain.Common;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.Costing;

public class InventoryLayer : Entity<InventoryLayerId>, IAuditableEntity
{
    public WarehouseId WarehouseId { get; private set; }
    public InventoryItemId InventoryItemId { get; private set; }
    public DateOnly ReceiptDate { get; private set; }
    public decimal OriginalQuantity { get; private set; }
    public decimal RemainingQuantity { get; private set; }
    public decimal UnitCost { get; private set; }
    public decimal TotalValue => decimal.Round(RemainingQuantity * UnitCost, 2, MidpointRounding.AwayFromZero);
    public VoucherId? SourceVoucherId { get; private set; }
    public bool IsExhausted => RemainingQuantity <= 0m;

    // IAuditableEntity
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    private InventoryLayer() { }

    public InventoryLayer(
        InventoryLayerId id,
        WarehouseId warehouseId,
        InventoryItemId inventoryItemId,
        DateOnly receiptDate,
        decimal quantity,
        decimal unitCost,
        VoucherId? sourceVoucherId = null)
    {
        if (id == InventoryLayerId.Empty)
            throw new ArgumentException("Inventory layer ID cannot be empty.", nameof(id));

        if (quantity <= 0m)
            throw new ArgumentException("Layer quantity must be strictly positive.", nameof(quantity));

        if (unitCost < 0m)
            throw new ArgumentException("Unit cost cannot be negative.", nameof(unitCost));

        Id = id;
        WarehouseId = warehouseId;
        InventoryItemId = inventoryItemId;
        ReceiptDate = receiptDate;
        OriginalQuantity = quantity;
        RemainingQuantity = quantity;
        UnitCost = unitCost;
        SourceVoucherId = sourceVoucherId;
    }

    public decimal Consume(decimal requestedQuantity)
    {
        if (requestedQuantity <= 0m)
            throw new ArgumentException("Requested quantity to consume must be strictly positive.", nameof(requestedQuantity));

        if (RemainingQuantity <= 0m)
            throw new InvalidOperationException($"Inventory layer '{Id.Value}' is already fully exhausted.");

        var consumedQuantity = Math.Min(requestedQuantity, RemainingQuantity);
        RemainingQuantity -= consumedQuantity;
        return consumedQuantity;
    }
}
