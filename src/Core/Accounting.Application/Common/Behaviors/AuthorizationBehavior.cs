using System.Reflection;
using Accounting.Application.Common.Interfaces;
using Accounting.Domain.Security;
using MediatR;

namespace Accounting.Application.Common.Behaviors;

public class AuthorizationBehavior<TRequest, TResponse>(
    ICurrentUserContext currentUserContext) : IPipelineBehavior<TRequest, TResponse>
    where TRequest : notnull
{
    public async Task<TResponse> Handle(
        TRequest request,
        RequestHandlerDelegate<TResponse> next,
        CancellationToken cancellationToken)
    {
        var requirePermissionAttrs = request.GetType()
            .GetCustomAttributes<RequirePermissionAttribute>(inherit: true)
            .ToList();

        if (requirePermissionAttrs.Count == 0)
        {
            return await next();
        }

        if (!currentUserContext.IsAuthenticated)
        {
            throw new UnauthorizedAccessException("User is not authenticated.");
        }

        foreach (var attr in requirePermissionAttrs)
        {
            if (!currentUserContext.HasPermission(attr.Permission))
            {
                throw new UnauthorizedAccessException(
                    $"Access denied. User '{currentUserContext.Username}' lacks required permission '{attr.Permission}'.");
            }
        }

        return await next();
    }
}
