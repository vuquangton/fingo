using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.MasterData.Inventory;

public class UnitOfMeasure : Entity<UomId>, IAuditableEntity, ISoftDeletable
{
    public string UomCode { get; private set; } = string.Empty;
    public string UomName { get; private set; } = string.Empty;
    public string? Description { get; private set; }
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

    private UnitOfMeasure() { }

    public UnitOfMeasure(UomId id, string uomCode, string uomName, string? description = null)
    {
        if (id == UomId.Empty)
            throw new ArgumentException("UoM ID cannot be empty.", nameof(id));

        var code = (uomCode ?? string.Empty).Trim().ToUpperInvariant();
        if (string.IsNullOrWhiteSpace(code))
            throw new ArgumentException("UoM code cannot be empty.", nameof(uomCode));

        var name = (uomName ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(name))
            throw new ArgumentException("UoM name cannot be empty.", nameof(uomName));

        Id = id;
        UomCode = code;
        UomName = name;
        Description = description?.Trim();
    }

    public void Update(string uomName, string? description)
    {
        var name = (uomName ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(name))
            throw new ArgumentException("UoM name cannot be empty.", nameof(uomName));

        UomName = name;
        Description = description?.Trim();
    }

    public void SetActive(bool isActive) => IsActive = isActive;
}
