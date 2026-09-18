using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.MasterData.Inventory;

public class InventoryItem : AggregateRoot<InventoryItemId>, IAuditableEntity, ISoftDeletable
{
    public string ItemCode { get; private set; } = string.Empty;
    public string ItemName { get; private set; } = string.Empty;
    public ItemType ItemType { get; private set; }
    public UomId BaseUomId { get; private set; }
    public UnitOfMeasure? BaseUom { get; private set; }
    public CostingMethod DefaultCostingMethod { get; private set; }
    public AccountId? DefaultInventoryAccountId { get; private set; }
    public AccountId? DefaultCogsAccountId { get; private set; }
    public AccountId? DefaultRevenueAccountId { get; private set; }
    public decimal TaxRate { get; private set; }
    public bool IsActive { get; private set; } = true;

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

    private InventoryItem() { }

    public InventoryItem(
        InventoryItemId id,
        string itemCode,
        string itemName,
        ItemType itemType,
        UomId baseUomId,
        CostingMethod defaultCostingMethod = CostingMethod.MovingAverage,
        AccountId? defaultInventoryAccountId = null,
        AccountId? defaultCogsAccountId = null,
        AccountId? defaultRevenueAccountId = null,
        decimal taxRate = 0m)
    {
        if (id == InventoryItemId.Empty)
            throw new ArgumentException("Item ID cannot be empty.", nameof(id));

        var code = (itemCode ?? string.Empty).Trim().ToUpperInvariant();
        if (string.IsNullOrWhiteSpace(code))
            throw new ArgumentException("Item code cannot be empty.", nameof(itemCode));

        var name = (itemName ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(name))
            throw new ArgumentException("Item name cannot be empty.", nameof(itemName));

        if (baseUomId == UomId.Empty)
            throw new ArgumentException("Base UoM ID cannot be empty.", nameof(baseUomId));

        if (taxRate < 0)
            throw new ArgumentException("Tax rate cannot be negative.", nameof(taxRate));

        Id = id;
        ItemCode = code;
        ItemName = name;
        ItemType = itemType;
        BaseUomId = baseUomId;
        DefaultCostingMethod = defaultCostingMethod;
        DefaultInventoryAccountId = defaultInventoryAccountId;
        DefaultCogsAccountId = defaultCogsAccountId;
        DefaultRevenueAccountId = defaultRevenueAccountId;
        TaxRate = taxRate;
    }

    public void Update(
        string itemName,
        ItemType itemType,
        UomId baseUomId,
        CostingMethod defaultCostingMethod,
        decimal taxRate)
    {
        var name = (itemName ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(name))
            throw new ArgumentException("Item name cannot be empty.", nameof(itemName));

        if (baseUomId == UomId.Empty)
            throw new ArgumentException("Base UoM ID cannot be empty.", nameof(baseUomId));

        if (taxRate < 0)
            throw new ArgumentException("Tax rate cannot be negative.", nameof(taxRate));

        ItemName = name;
        ItemType = itemType;
        BaseUomId = baseUomId;
        DefaultCostingMethod = defaultCostingMethod;
        TaxRate = taxRate;
    }

    public void SetAccounts(
        AccountId? inventoryAccountId,
        AccountId? cogsAccountId,
        AccountId? revenueAccountId)
    {
        DefaultInventoryAccountId = inventoryAccountId;
        DefaultCogsAccountId = cogsAccountId;
        DefaultRevenueAccountId = revenueAccountId;
    }

    public void SetActive(bool isActive) => IsActive = isActive;
}
