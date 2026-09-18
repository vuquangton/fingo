using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.Manufacturing;

public class BillOfMaterials : AggregateRoot<BomId>, IAuditableEntity, ISoftDeletable
{
    private readonly List<BomLine> _lines = [];

    public InventoryItemId FinishedGoodItemId { get; private set; }
    public string Version { get; private set; } = "1.0";
    public string Description { get; private set; } = string.Empty;
    public bool IsActive { get; private set; } = true;
    public IReadOnlyCollection<BomLine> Lines => _lines.AsReadOnly();

    // IAuditableEntity
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    // ISoftDeletable
    public bool IsDeleted { get; set; }
    public DateTime? DeletedAtUtc { get; set; }
    public string? DeletedBy { get; set; }

    private BillOfMaterials() { }

    public BillOfMaterials(
        BomId id,
        InventoryItemId finishedGoodItemId,
        string version,
        string description)
    {
        if (id == BomId.Empty)
            throw new ArgumentException("BOM ID cannot be empty.", nameof(id));

        if (finishedGoodItemId == InventoryItemId.Empty)
            throw new ArgumentException("Finished good item ID cannot be empty.", nameof(finishedGoodItemId));

        var v = (version ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(v))
            throw new ArgumentException("Version cannot be empty.", nameof(version));

        Id = id;
        FinishedGoodItemId = finishedGoodItemId;
        Version = v;
        Description = description?.Trim() ?? string.Empty;
        IsActive = true;
    }

    public void AddLine(InventoryItemId materialItemId, decimal standardQuantity, decimal scrapPercentage = 0m)
    {
        var line = new BomLine(BomLineId.New(), Id, materialItemId, standardQuantity, scrapPercentage);
        _lines.Add(line);
    }
}
