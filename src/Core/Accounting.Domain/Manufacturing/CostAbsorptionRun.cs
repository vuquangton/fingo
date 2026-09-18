using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.Manufacturing;

public class CostAbsorptionRun : AggregateRoot<CostAbsorptionRunId>, IAuditableEntity
{
    public FiscalPeriodId FiscalPeriodId { get; private set; }
    public InventoryItemId FinishedGoodItemId { get; private set; }
    public decimal ProducedQuantity { get; private set; }
    public decimal DirectMaterialCost { get; private set; }
    public decimal DirectLaborCost { get; private set; }
    public decimal OverheadCost { get; private set; }
    public decimal WipBeginning { get; private set; }
    public decimal WipEnding { get; private set; }
    public decimal TotalManufacturingCost { get; private set; }
    public decimal UnitCostPerItem { get; private set; }
    public CostAbsorptionStatus Status { get; private set; }
    public VoucherId? LinkedClearanceVoucherId { get; private set; }
    public VoucherId? LinkedReceiptVoucherId { get; private set; }

    // IAuditableEntity
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    private CostAbsorptionRun() { }

    public CostAbsorptionRun(
        CostAbsorptionRunId id,
        FiscalPeriodId fiscalPeriodId,
        InventoryItemId finishedGoodItemId,
        decimal producedQuantity,
        decimal directMaterialCost,
        decimal directLaborCost,
        decimal overheadCost,
        decimal wipBeginning = 0m,
        decimal wipEnding = 0m)
    {
        if (id == CostAbsorptionRunId.Empty)
            throw new ArgumentException("Run ID cannot be empty.", nameof(id));

        if (finishedGoodItemId == InventoryItemId.Empty)
            throw new ArgumentException("Finished good item ID cannot be empty.", nameof(finishedGoodItemId));

        if (producedQuantity <= 0m)
            throw new ArgumentException("Produced quantity must be strictly positive.", nameof(producedQuantity));

        if (directMaterialCost < 0m || directLaborCost < 0m || overheadCost < 0m || wipBeginning < 0m || wipEnding < 0m)
            throw new ArgumentException("Cost components and WIP amounts cannot be negative.");

        Id = id;
        FiscalPeriodId = fiscalPeriodId;
        FinishedGoodItemId = finishedGoodItemId;
        ProducedQuantity = producedQuantity;
        DirectMaterialCost = directMaterialCost;
        DirectLaborCost = directLaborCost;
        OverheadCost = overheadCost;
        WipBeginning = wipBeginning;
        WipEnding = wipEnding;

        var total = wipBeginning + directMaterialCost + directLaborCost + overheadCost - wipEnding;
        if (total < 0m)
            throw new InvalidOperationException($"Total manufacturing cost ({total:N2}) cannot be negative.");

        TotalManufacturingCost = total;
        UnitCostPerItem = decimal.Round(total / producedQuantity, 4, MidpointRounding.AwayFromZero);
        Status = CostAbsorptionStatus.Calculated;
    }

    public void LinkVouchers(VoucherId clearanceVoucherId, VoucherId receiptVoucherId)
    {
        LinkedClearanceVoucherId = clearanceVoucherId;
        LinkedReceiptVoucherId = receiptVoucherId;
        Status = CostAbsorptionStatus.PostedToGl;
    }
}
