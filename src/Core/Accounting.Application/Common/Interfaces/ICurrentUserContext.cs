using Accounting.Domain.Security;

namespace Accounting.Application.Common.Interfaces;

public interface ICurrentUserContext
{
    string? UserId { get; }
    string Username { get; }
    string MachineName { get; }
    string? IpAddress { get; }
    bool IsAuthenticated { get; }
    IReadOnlyCollection<AppPermission> Permissions { get; }
    bool HasPermission(AppPermission permission);
}
