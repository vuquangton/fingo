namespace Accounting.Domain.Security;

[AttributeUsage(AttributeTargets.Class, AllowMultiple = true, Inherited = true)]
public class RequirePermissionAttribute : Attribute
{
    public AppPermission Permission { get; }

    public RequirePermissionAttribute(AppPermission permission)
    {
        Permission = permission;
    }
}
