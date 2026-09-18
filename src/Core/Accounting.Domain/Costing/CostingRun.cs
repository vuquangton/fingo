using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.Costing;

public class CostingRun : AggregateRoot<CostAllocationRunId>, IAuditableEntity
{
    public FiscalPeriodId FiscalPeriodId { get; private set; }
    public WarehouseId? WarehouseId { get; private set; }
    public InventoryItemId? InventoryItemId { get; private set; }
    public InventoryCostingMethod CostingMethod { get; private set; }
    public CostingRunStatus Status { get; private set; }
    public DateTime ExecutionDate { get; private set; }
    public decimal TotalAdjustmentAmount { get; private set; }
    public VoucherId? LinkedVoucherId { get; private set; }

    // IAuditableEntity
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    private CostingRun() { }

    public CostingRun(
        CostAllocationRunId id,
        FiscalPeriodId fiscalPeriodId,
        InventoryCostingMethod costingMethod,
        WarehouseId? warehouseId = null,
        InventoryItemId? inventoryItemId = null)
    {
        if (id == CostAllocationRunId.Empty)
            throw new ArgumentException("Cost allocation run ID cannot be empty.", nameof(id));

        Id = id;
        FiscalPeriodId = fiscalPeriodId;
        CostingMethod = costingMethod;
        WarehouseId = warehouseId;
        InventoryItemId = inventoryItemId;
        Status = CostingRunStatus.Pending;
        ExecutionDate = DateTime.UtcNow;
        TotalAdjustmentAmount = 0m;
    }

    public void MarkCalculated(decimal totalAdjustmentAmount)
    {
        TotalAdjustmentAmount = totalAdjustmentAmount;
        Status = CostingRunStatus.Calculated;
    }

    public void LinkGeneralLedgerVoucher(VoucherId voucherId)
    {
        LinkedVoucherId = voucherId;
        Status = CostingRunStatus.PostedToGl;
    }
}
