using Accounting.Domain.Common;
using Accounting.Domain.Enums;
using Accounting.Domain.Exceptions;

namespace Accounting.Domain.Entities.Inventory;

public class Warehouse : Entity<Guid>
{
    public string Code { get; private set; } = string.Empty;
    public string Name { get; private set; } = string.Empty;
    public string? Address { get; private set; }
    public bool IsActive { get; private set; } = true;

    private Warehouse() { }

    public Warehouse(string code, string name, string? address = null)
    {
        Id = Guid.NewGuid();
        Code = code.Trim().ToUpperInvariant();
        Name = name.Trim();
        Address = address?.Trim();
    }
}

public class ProductItem : AggregateRoot<Guid>
{
    public string Code { get; private set; } = string.Empty;
    public string Name { get; private set; } = string.Empty;
    public string Unit { get; private set; } = "Cái"; // Đơn vị tính: Cái, Kg, Hộp, Thùng...
    public Guid? CategoryId { get; private set; }
    public CostingMethod CostingMethod { get; private set; } = CostingMethod.PerpetualMovingAverage;
    public decimal StandardCost { get; private set; }
    public decimal CurrentStockQuantity { get; private set; }
    public decimal CurrentStockValue { get; private set; }
    public decimal AverageUnitCost => CurrentStockQuantity > 0 ? decimal.Round(CurrentStockValue / CurrentStockQuantity, 4, MidpointRounding.AwayFromZero) : 0m;
    public bool IsActive { get; private set; } = true;

    private ProductItem() { }

    public ProductItem(string code, string name, string unit, CostingMethod costingMethod = CostingMethod.PerpetualMovingAverage, decimal standardCost = 0m)
    {
        Id = Guid.NewGuid();
        Code = code.Trim().ToUpperInvariant();
        Name = name.Trim();
        Unit = unit.Trim();
        CostingMethod = costingMethod;
        StandardCost = standardCost;
    }

    public void RecordInflow(decimal quantity, decimal unitCost)
    {
        if (quantity <= 0) throw new ArgumentOutOfRangeException(nameof(quantity), "Quantity must be positive.");
        CurrentStockQuantity += quantity;
        CurrentStockValue += (quantity * unitCost);
    }

    public decimal RecordOutflow(decimal quantity, string warehouseCode = "KHO")
    {
        if (quantity <= 0) throw new ArgumentOutOfRangeException(nameof(quantity), "Quantity must be positive.");
        if (quantity > CurrentStockQuantity)
            throw new NegativeStockException(Code, warehouseCode, CurrentStockQuantity, quantity);

        var unitCost = AverageUnitCost;
        var totalCost = decimal.Round(quantity * unitCost, 2, MidpointRounding.AwayFromZero);

        CurrentStockQuantity -= quantity;
        CurrentStockValue -= totalCost;

        if (CurrentStockQuantity == 0) CurrentStockValue = 0m;

        return totalCost;
    }
}

public enum StockTransactionType
{
    PurchaseReceipt = 1,
    SalesDelivery = 2,
    TransferIn = 3,
    TransferOut = 4,
    PhysicalAdjustment = 5
}

public class StockTransaction : Entity<Guid>
{
    public DateTime TransactionDate { get; private set; }
    public Guid WarehouseId { get; private set; }
    public Guid ProductItemId { get; private set; }
    public StockTransactionType TransactionType { get; private set; }
    public decimal Quantity { get; private set; }
    public decimal UnitCost { get; private set; }
    public decimal TotalAmount { get; private set; }
    public Guid? VoucherId { get; private set; }
    public string? ReferenceNumber { get; private set; }

    private StockTransaction() { }

    public StockTransaction(
        DateTime transactionDate,
        Guid warehouseId,
        Guid productItemId,
        StockTransactionType transactionType,
        decimal quantity,
        decimal unitCost,
        Guid? voucherId = null,
        string? referenceNumber = null)
    {
        Id = Guid.NewGuid();
        TransactionDate = transactionDate;
        WarehouseId = warehouseId;
        ProductItemId = productItemId;
        TransactionType = transactionType;
        Quantity = quantity;
        UnitCost = unitCost;
        TotalAmount = decimal.Round(quantity * unitCost, 2, MidpointRounding.AwayFromZero);
        VoucherId = voucherId;
        ReferenceNumber = referenceNumber;
    }
}

public class WarehouseTransfer : AggregateRoot<Guid>
{
    public string TransferNumber { get; private set; } = string.Empty;
    public DateTime TransferDate { get; private set; }
    public Guid SourceWarehouseId { get; private set; }
    public Guid TargetWarehouseId { get; private set; }
    public string Reason { get; private set; } = string.Empty;
    public VoucherStatus Status { get; private set; }

    private readonly List<WarehouseTransferLine> _lines = [];
    public IReadOnlyCollection<WarehouseTransferLine> Lines => _lines.AsReadOnly();

    private WarehouseTransfer() { }

    public WarehouseTransfer(string transferNumber, DateTime transferDate, Guid sourceWarehouseId, Guid targetWarehouseId, string reason)
    {
        if (sourceWarehouseId == targetWarehouseId)
            throw new ArgumentException("Source and Target warehouse cannot be the same.");

        Id = Guid.NewGuid();
        TransferNumber = transferNumber.Trim();
        TransferDate = transferDate;
        SourceWarehouseId = sourceWarehouseId;
        TargetWarehouseId = targetWarehouseId;
        Reason = reason.Trim();
        Status = VoucherStatus.Draft;
    }

    public void AddLine(Guid itemId, decimal quantity, decimal unitCost)
    {
        _lines.Add(new WarehouseTransferLine(Id, itemId, quantity, unitCost));
    }
}

public class WarehouseTransferLine : Entity<Guid>
{
    public Guid WarehouseTransferId { get; private set; }
    public Guid ProductItemId { get; private set; }
    public decimal Quantity { get; private set; }
    public decimal UnitCost { get; private set; }
    public decimal LineCost => Quantity * UnitCost;

    private WarehouseTransferLine() { }

    public WarehouseTransferLine(Guid transferId, Guid itemId, decimal quantity, decimal unitCost)
    {
        Id = Guid.NewGuid();
        WarehouseTransferId = transferId;
        ProductItemId = itemId;
        Quantity = quantity;
        UnitCost = unitCost;
    }
}

public class StocktakeRecord : AggregateRoot<Guid>
{
    public string RecordNumber { get; private set; } = string.Empty;
    public DateTime AuditDate { get; private set; }
    public Guid WarehouseId { get; private set; }
    public string CommitteeMembers { get; private set; } = string.Empty;

    private readonly List<StocktakeLine> _lines = [];
    public IReadOnlyCollection<StocktakeLine> Lines => _lines.AsReadOnly();

    private StocktakeRecord() { }

    public StocktakeRecord(string recordNumber, DateTime auditDate, Guid warehouseId, string committeeMembers)
    {
        Id = Guid.NewGuid();
        RecordNumber = recordNumber.Trim();
        AuditDate = auditDate;
        WarehouseId = warehouseId;
        CommitteeMembers = committeeMembers.Trim();
    }

    public void AddLine(Guid itemId, decimal bookQuantity, decimal physicalQuantity, decimal unitCost, string? notes = null)
    {
        _lines.Add(new StocktakeLine(Id, itemId, bookQuantity, physicalQuantity, unitCost, notes));
    }
}

public class StocktakeLine : Entity<Guid>
{
    public Guid StocktakeRecordId { get; private set; }
    public Guid ProductItemId { get; private set; }
    public decimal BookQuantity { get; private set; }
    public decimal PhysicalQuantity { get; private set; }
    public decimal VarianceQuantity => PhysicalQuantity - BookQuantity;
    public decimal UnitCost { get; private set; }
    public decimal VarianceAmount => VarianceQuantity * UnitCost;
    public string? Notes { get; private set; }

    private StocktakeLine() { }

    public StocktakeLine(Guid recordId, Guid itemId, decimal bookQty, decimal physicalQty, decimal unitCost, string? notes)
    {
        Id = Guid.NewGuid();
        StocktakeRecordId = recordId;
        ProductItemId = itemId;
        BookQuantity = bookQty;
        PhysicalQuantity = physicalQty;
        UnitCost = unitCost;
        Notes = notes;
    }
}
