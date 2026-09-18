using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.MasterData.Dimensions;

public class Department : Entity<DepartmentId>, IAuditableEntity, ISoftDeletable
{
    public string Code => Id.Value;
    public string Name => DepartmentName;
    public DepartmentId? ParentId => ParentDepartmentId;

    public string DepartmentName { get; private set; } = string.Empty;
    public DepartmentId? ParentDepartmentId { get; private set; }
    public Department? ParentDepartment { get; private set; }
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

    private Department() { }

    public Department(DepartmentId id, string departmentName, DepartmentId? parentDepartmentId = null)
    {
        if (string.IsNullOrWhiteSpace(id.Value))
            throw new ArgumentException("Department ID cannot be empty.", nameof(id));

        var name = (departmentName ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(name))
            throw new ArgumentException("Department name cannot be empty.", nameof(departmentName));

        Id = id;
        DepartmentName = name;
        ParentDepartmentId = parentDepartmentId;
    }

    public void Update(string departmentName, DepartmentId? parentDepartmentId)
    {
        var name = (departmentName ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(name))
            throw new ArgumentException("Department name cannot be empty.", nameof(departmentName));

        DepartmentName = name;
        ParentDepartmentId = parentDepartmentId;
    }

    public void SetActive(bool isActive) => IsActive = isActive;
}
