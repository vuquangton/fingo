using Accounting.Domain.Common;
using Accounting.Domain.Enums;

namespace Accounting.Domain.Entities.Security;

public class AuditTrail : Entity<AuditTrailId>
{
    public DateTime TimestampUtc { get; private set; }
    public string? UserId { get; private set; }
    public string Username { get; private set; } = string.Empty;
    public string MachineName { get; private set; } = string.Empty;
    public string? IpAddress { get; private set; }
    public AuditAction Action { get; private set; }
    public string EntityName { get; private set; } = string.Empty;
    public string EntityId { get; private set; } = string.Empty;
    public string? OldValues { get; private set; }
    public string? NewValues { get; private set; }
    public string? DiffSummary { get; private set; }

    // Compatibility aliases
    public string? OldValuesJson => OldValues;
    public string? NewValuesJson => NewValues;

    private AuditTrail() { }

    public AuditTrail(
        AuditTrailId id,
        string? userId,
        string username,
        string machineName,
        AuditAction action,
        string entityName,
        string entityId,
        string? oldValues = null,
        string? newValues = null,
        string? diffSummary = null,
        string? ipAddress = null)
    {
        Id = id;
        TimestampUtc = DateTime.UtcNow;
        UserId = userId;
        Username = username;
        MachineName = machineName;
        Action = action;
        EntityName = entityName;
        EntityId = entityId;
        OldValues = oldValues;
        NewValues = newValues;
        DiffSummary = diffSummary;
        IpAddress = ipAddress;
    }
}
