using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.Manufacturing;

public class BomLine : Entity<BomLineId>
{
    public BomId BomId { get; private set; }
    public InventoryItemId MaterialItemId { get; private set; }
    public decimal StandardQuantity { get; private set; }
    public decimal ScrapPercentage { get; private set; }

    private BomLine() { }

    public BomLine(
        BomLineId id,
        BomId bomId,
        InventoryItemId materialItemId,
        decimal standardQuantity,
        decimal scrapPercentage = 0m)
    {
        if (id == BomLineId.Empty)
            throw new ArgumentException("BOM line ID cannot be empty.", nameof(id));

        if (standardQuantity <= 0m)
            throw new ArgumentException("Standard quantity must be strictly positive.", nameof(standardQuantity));

        if (scrapPercentage < 0m || scrapPercentage > 100m)
            throw new ArgumentException("Scrap percentage must be between 0 and 100.", nameof(scrapPercentage));

        Id = id;
        BomId = bomId;
        MaterialItemId = materialItemId;
        StandardQuantity = standardQuantity;
        ScrapPercentage = scrapPercentage;
    }
}
