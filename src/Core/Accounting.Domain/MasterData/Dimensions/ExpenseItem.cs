using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.MasterData.Dimensions;

public class ExpenseItem : Entity<ExpenseItemId>, IAuditableEntity, ISoftDeletable
{
    public string Code => Id.Value;
    public string Name { get; private set; } = string.Empty;
    public ExpenseItemId? ParentId { get; private set; }
    public ExpenseItem? Parent { get; private set; }
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

    private ExpenseItem() { }

    public ExpenseItem(ExpenseItemId id, string name, ExpenseItemId? parentId = null)
    {
        if (string.IsNullOrWhiteSpace(id.Value))
            throw new ArgumentException("Expense item ID cannot be empty.", nameof(id));

        var trimmedName = (name ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(trimmedName))
            throw new ArgumentException("Expense item name cannot be empty.", nameof(name));

        Id = id;
        Name = trimmedName;
        ParentId = parentId;
    }

    public void Update(string name, ExpenseItemId? parentId)
    {
        var trimmedName = (name ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(trimmedName))
            throw new ArgumentException("Expense item name cannot be empty.", nameof(name));

        Name = trimmedName;
        ParentId = parentId;
    }

    public void SetActive(bool isActive) => IsActive = isActive;
}
