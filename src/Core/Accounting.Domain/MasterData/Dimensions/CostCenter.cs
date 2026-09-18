using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.MasterData.Dimensions;

public class CostCenter : Entity<CostCenterId>, IAuditableEntity, ISoftDeletable
{
    public string Code => Id.Value;
    public string Name => CostCenterName;
    public CostCenterId? ParentId => ParentCostCenterId;

    public string CostCenterName { get; private set; } = string.Empty;
    public CostCenterId? ParentCostCenterId { get; private set; }
    public CostCenter? ParentCostCenter { get; private set; }
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

    private CostCenter() { }

    public CostCenter(CostCenterId id, string costCenterName, CostCenterId? parentCostCenterId = null)
    {
        if (string.IsNullOrWhiteSpace(id.Value))
            throw new ArgumentException("Cost center ID cannot be empty.", nameof(id));

        var name = (costCenterName ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(name))
            throw new ArgumentException("Cost center name cannot be empty.", nameof(costCenterName));

        Id = id;
        CostCenterName = name;
        ParentCostCenterId = parentCostCenterId;
    }

    public void Update(string costCenterName, CostCenterId? parentCostCenterId)
    {
        var name = (costCenterName ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(name))
            throw new ArgumentException("Cost center name cannot be empty.", nameof(costCenterName));

        CostCenterName = name;
        ParentCostCenterId = parentCostCenterId;
    }

    public void SetActive(bool isActive) => IsActive = isActive;
}
