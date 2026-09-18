using Accounting.Domain.Common;
using Accounting.Domain.Enums;

namespace Accounting.Domain.Entities.Security;

public class AppUser : AggregateRoot<Guid>
{
    public string Username { get; private set; } = string.Empty;
    public string PasswordHash { get; private set; } = string.Empty;
    public string FullName { get; private set; } = string.Empty;
    public string Email { get; private set; } = string.Empty;
    public Guid RoleId { get; private set; }
    public AppRole? Role { get; private set; }
    public bool IsActive { get; private set; } = true;
    public DateTime CreatedAtUtc { get; private set; }

    private AppUser() { }

    public AppUser(string username, string passwordHash, string fullName, string email, Guid roleId)
    {
        Id = Guid.NewGuid();
        Username = username.Trim().ToLowerInvariant();
        PasswordHash = passwordHash;
        FullName = fullName.Trim();
        Email = email.Trim();
        RoleId = roleId;
        CreatedAtUtc = DateTime.UtcNow;
    }

    public void UpdatePassword(string passwordHash) => PasswordHash = passwordHash;
    public void SetStatus(bool isActive) => IsActive = isActive;

    public void UpdateProfile(string fullName, string email, Guid roleId)
    {
        FullName = fullName.Trim();
        Email = email.Trim();
        RoleId = roleId;
    }
}

public class AppRole : Entity<Guid>
{
    public string Name { get; private set; } = string.Empty;
    public string Description { get; private set; } = string.Empty;
    public string PermissionsCsv { get; private set; } = string.Empty;

    private AppRole() { }

    public AppRole(string name, string description, IEnumerable<string> permissions)
    {
        Id = Guid.NewGuid();
        Name = name.Trim();
        Description = description.Trim();
        PermissionsCsv = string.Join(",", permissions.Select(p => p.Trim()));
    }

    public bool HasPermission(string permission) =>
        PermissionsCsv.Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries)
            .Contains(permission, StringComparer.OrdinalIgnoreCase);
}

public class AuditLog : Entity<long>
{
    public DateTime TimestampUtc { get; private set; }
    public Guid? UserId { get; private set; }
    public string Username { get; private set; } = string.Empty;
    public string MachineName { get; private set; } = string.Empty;
    public string? IpAddress { get; private set; }
    public AuditAction Action { get; private set; }
    public string EntityName { get; private set; } = string.Empty;
    public string EntityId { get; private set; } = string.Empty;
    public string? OldValuesJson { get; private set; }
    public string? NewValuesJson { get; private set; }
    public string? DiffSummary { get; private set; }

    private AuditLog() { }

    public AuditLog(
        Guid? userId,
        string username,
        string machineName,
        AuditAction action,
        string entityName,
        string entityId,
        string? oldValuesJson = null,
        string? newValuesJson = null,
        string? diffSummary = null,
        string? ipAddress = null)
    {
        TimestampUtc = DateTime.UtcNow;
        UserId = userId;
        Username = username;
        MachineName = machineName;
        Action = action;
        EntityName = entityName;
        EntityId = entityId;
        OldValuesJson = oldValuesJson;
        NewValuesJson = newValuesJson;
        DiffSummary = diffSummary;
        IpAddress = ipAddress;
    }
}
