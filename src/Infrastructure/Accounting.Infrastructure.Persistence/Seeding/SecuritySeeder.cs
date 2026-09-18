using Accounting.Application.Common.Interfaces;
using Accounting.Domain.Entities.Security;
using Accounting.Domain.Security;
using Accounting.Infrastructure.Persistence.Context;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Infrastructure.Persistence.Seeding;

public static class SecuritySeeder
{
    public static async Task SeedSecurityAsync(
        AccountingDbContext context,
        IPasswordHasher passwordHasher,
        CancellationToken cancellationToken = default)
    {
        var adminRoleId = Guid.Parse("00000000-0000-0000-0000-000000000001");
        var chiefRoleId = Guid.Parse("00000000-0000-0000-0000-000000000002");
        var generalRoleId = Guid.Parse("00000000-0000-0000-0000-000000000003");
        var cashierRoleId = Guid.Parse("00000000-0000-0000-0000-000000000004");
        var stockKeeperRoleId = Guid.Parse("00000000-0000-0000-0000-000000000005");

        var adminUserId = Guid.Parse("00000000-0000-0000-0000-000000000001");

        var standardRoles = new[]
        {
            (Id: adminRoleId, Name: "SystemAdministrator", Description: "Quản trị hệ thống toàn quyền", Permissions: Enum.GetNames<AppPermission>()),
            (Id: chiefRoleId, Name: "ChiefAccountant", Description: "Kế toán trưởng - Phê duyệt, khóa sổ, báo cáo thuế & tài chính", Permissions: new[] {
                nameof(AppPermission.ViewLedger), nameof(AppPermission.CreateVoucher), nameof(AppPermission.EditVoucher),
                nameof(AppPermission.PostVoucher), nameof(AppPermission.ApproveVoucher), nameof(AppPermission.DeleteVoucher),
                nameof(AppPermission.ExportReport)
            }),
            (Id: generalRoleId, Name: "GeneralAccountant", Description: "Kế toán tổng hợp - Lập chứng từ, đối soát, xem báo cáo", Permissions: new[] {
                nameof(AppPermission.ViewLedger), nameof(AppPermission.CreateVoucher), nameof(AppPermission.EditVoucher),
                nameof(AppPermission.PostVoucher), nameof(AppPermission.ExportReport)
            }),
            (Id: cashierRoleId, Name: "Cashier", Description: "Thủ quỹ - Thu chi tiền mặt, tiền gửi ngân hàng", Permissions: new[] {
                nameof(AppPermission.ViewLedger), nameof(AppPermission.CreateVoucher), nameof(AppPermission.ExportReport)
            }),
            (Id: stockKeeperRoleId, Name: "StockKeeper", Description: "Thủ kho - Nhập xuất tồn kho vật tư hàng hóa", Permissions: new[] {
                nameof(AppPermission.ViewLedger), nameof(AppPermission.CreateVoucher), nameof(AppPermission.ExportReport)
            })
        };

        foreach (var r in standardRoles)
        {
            var existing = await context.Roles.FirstOrDefaultAsync(role => role.Id == r.Id || role.Name == r.Name, cancellationToken);
            if (existing == null)
            {
                var newRole = new AppRole(r.Name, r.Description, r.Permissions);
                typeof(AppRole).GetProperty(nameof(AppRole.Id))?.SetValue(newRole, r.Id);
                context.Roles.Add(newRole);
            }
        }
        await context.SaveChangesAsync(cancellationToken);

        var adminUser = await context.Users.FirstOrDefaultAsync(u => u.Id == adminUserId || u.Username == "admin", cancellationToken);
        if (adminUser == null)
        {
            var hashedPassword = passwordHasher.HashPassword("Admin@123456");
            adminUser = new AppUser("admin", hashedPassword, "System Administrator", "admin@accounting.local", adminRoleId);
            typeof(AppUser).GetProperty(nameof(AppUser.Id))?.SetValue(adminUser, adminUserId);
            context.Users.Add(adminUser);
            await context.SaveChangesAsync(cancellationToken);
        }
    }
}
