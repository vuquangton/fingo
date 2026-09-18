using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.MasterData.Inventory;

public class Warehouse : Entity<WarehouseId>, IAuditableEntity, ISoftDeletable
{
    public string WarehouseName { get; private set; } = string.Empty;
    public string? Address { get; private set; }
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

    private Warehouse() { }

    public Warehouse(WarehouseId id, string warehouseName, string? address = null)
    {
        if (string.IsNullOrWhiteSpace(id.Value))
            throw new ArgumentException("Warehouse ID cannot be empty.", nameof(id));

        Id = id;
        WarehouseName = warehouseName.Trim();
        Address = address?.Trim();
    }

    public void Update(string warehouseName, string? address)
    {
        WarehouseName = warehouseName.Trim();
        Address = address?.Trim();
    }

    public void SetActive(bool isActive) => IsActive = isActive;
}
