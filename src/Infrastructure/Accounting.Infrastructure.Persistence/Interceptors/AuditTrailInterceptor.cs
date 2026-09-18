using System.Text.Json;
using Accounting.Application.Common.Interfaces;
using Accounting.Domain.Common;
using Accounting.Domain.Entities.Security;
using Accounting.Domain.Enums;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Diagnostics;

namespace Accounting.Infrastructure.Persistence.Interceptors;

public class AuditTrailInterceptor : SaveChangesInterceptor
{
    private readonly ICurrentUserService _currentUserService;
    private readonly IDateTimeService _dateTimeService;

    public AuditTrailInterceptor(ICurrentUserService currentUserService, IDateTimeService dateTimeService)
    {
        _currentUserService = currentUserService;
        _dateTimeService = dateTimeService;
    }

    public override InterceptionResult<int> SavingChanges(DbContextEventData eventData, InterceptionResult<int> result)
    {
        UpdateAuditEntitiesAndTrails(eventData.Context);
        return base.SavingChanges(eventData, result);
    }

    public override ValueTask<InterceptionResult<int>> SavingChangesAsync(
        DbContextEventData eventData,
        InterceptionResult<int> result,
        CancellationToken cancellationToken = default)
    {
        UpdateAuditEntitiesAndTrails(eventData.Context);
        return base.SavingChangesAsync(eventData, result, cancellationToken);
    }

    private void UpdateAuditEntitiesAndTrails(DbContext? context)
    {
        if (context == null) return;

        var now = _dateTimeService.UtcNow;
        var userId = _currentUserService.UserId;
        var username = string.IsNullOrWhiteSpace(_currentUserService.Username) ? "System" : _currentUserService.Username;
        var machineName = string.IsNullOrWhiteSpace(_currentUserService.MachineName) ? Environment.MachineName : _currentUserService.MachineName;
        var ipAddress = _currentUserService.IpAddress;

        var entries = context.ChangeTracker.Entries().ToList();
        var auditTrails = new List<AuditTrail>();

        foreach (var entry in entries)
        {
            // 1. Stamp IAuditableEntity
            if (entry.Entity is IAuditableEntity auditable)
            {
                if (entry.State == EntityState.Added)
                {
                    auditable.CreatedAtUtc = now;
                    auditable.CreatedBy = username;
                }
                else if (entry.State == EntityState.Modified)
                {
                    auditable.UpdatedAtUtc = now;
                    auditable.UpdatedBy = username;
                }
            }

            // 2. Capture AuditTrail entries (avoid auditing audit records themselves)
            if (entry.Entity is AuditTrail or AuditLog)
            {
                continue;
            }

            if (entry.State is not (EntityState.Added or EntityState.Modified or EntityState.Deleted))
            {
                continue;
            }

            var entityName = entry.Entity.GetType().Name;
            var primaryKey = entry.Properties.FirstOrDefault(p => p.Metadata.IsPrimaryKey())?.CurrentValue?.ToString() ?? string.Empty;

            string? oldValuesJson = null;
            string? newValuesJson = null;
            string? diffSummary = null;
            AuditAction action;

            switch (entry.State)
            {
                case EntityState.Added:
                    action = AuditAction.Create;
                    var addedValues = entry.Properties
                        .Where(p => !p.Metadata.IsShadowProperty())
                        .ToDictionary(p => p.Metadata.Name, p => p.CurrentValue);
                    newValuesJson = JsonSerializer.Serialize(addedValues);
                    diffSummary = $"Created {entityName}";
                    break;

                case EntityState.Deleted:
                    action = AuditAction.Delete;
                    var deletedValues = entry.Properties
                        .Where(p => !p.Metadata.IsShadowProperty())
                        .ToDictionary(p => p.Metadata.Name, p => p.OriginalValue);
                    oldValuesJson = JsonSerializer.Serialize(deletedValues);
                    diffSummary = $"Deleted {entityName}";
                    break;

                case EntityState.Modified:
                    action = AuditAction.Update;
                    var modifiedProperties = entry.Properties
                        .Where(p => p.IsModified && !p.Metadata.IsShadowProperty())
                        .ToList();

                    if (modifiedProperties.Count == 0)
                    {
                        continue;
                    }

                    var oldDict = modifiedProperties.ToDictionary(p => p.Metadata.Name, p => p.OriginalValue);
                    var newDict = modifiedProperties.ToDictionary(p => p.Metadata.Name, p => p.CurrentValue);

                    oldValuesJson = JsonSerializer.Serialize(oldDict);
                    newValuesJson = JsonSerializer.Serialize(newDict);
                    diffSummary = string.Join("; ", modifiedProperties.Select(p => $"{p.Metadata.Name}: {p.OriginalValue} -> {p.CurrentValue}"));
                    break;

                default:
                    continue;
            }

            var auditTrail = new AuditTrail(
                id: AuditTrailId.New(),
                userId: userId?.ToString(),
                username: username,
                machineName: machineName,
                action: action,
                entityName: entityName,
                entityId: primaryKey,
                oldValues: oldValuesJson,
                newValues: newValuesJson,
                diffSummary: diffSummary,
                ipAddress: ipAddress);

            auditTrails.Add(auditTrail);
        }

        if (auditTrails.Count > 0)
        {
            context.Set<AuditTrail>().AddRange(auditTrails);
        }
    }
}
