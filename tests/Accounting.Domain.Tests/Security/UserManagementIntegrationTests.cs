using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Services;
using Accounting.Application.Features.Security;
using Accounting.Domain.Entities.Security;
using Accounting.Domain.Security;
using Accounting.Infrastructure.Persistence.Context;
using Accounting.Infrastructure.Persistence.Seeding;
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;
using Xunit;

namespace Accounting.Domain.Tests.Security;

public class UserManagementIntegrationTests : IDisposable
{
    private readonly SqliteConnection _connection;
    private readonly AccountingDbContext _context;
    private readonly IPasswordHasher _passwordHasher;

    public UserManagementIntegrationTests()
    {
        _connection = new SqliteConnection("DataSource=:memory:");
        _connection.Open();

        var options = new DbContextOptionsBuilder<AccountingDbContext>()
            .UseSqlite(_connection)
            .Options;

        _context = new AccountingDbContext(options);
        _context.Database.EnsureCreated();

        _passwordHasher = new PasswordHasher();
    }

    public void Dispose()
    {
        _context.Dispose();
        _connection.Dispose();
    }

    [Fact]
    public async Task SecuritySeeder_ShouldSeed5StandardRolesAndAdminUser()
    {
        await SecuritySeeder.SeedSecurityAsync(_context, _passwordHasher);

        var roles = await _context.Roles.ToListAsync();
        Assert.True(roles.Count >= 5);
        Assert.Contains(roles, r => r.Name == "SystemAdministrator");
        Assert.Contains(roles, r => r.Name == "ChiefAccountant");
        Assert.Contains(roles, r => r.Name == "GeneralAccountant");
        Assert.Contains(roles, r => r.Name == "Cashier");
        Assert.Contains(roles, r => r.Name == "StockKeeper");

        var admin = await _context.Users.Include(u => u.Role).FirstOrDefaultAsync(u => u.Username == "admin");
        Assert.NotNull(admin);
        Assert.True(admin.IsActive);
        Assert.True(_passwordHasher.VerifyPassword("Admin@123456", admin.PasswordHash));
    }

    [Fact]
    public async Task UserManagementHandlers_CreateUpdateToggleReset_ShouldWorkProperly()
    {
        await SecuritySeeder.SeedSecurityAsync(_context, _passwordHasher);
        var handler = new UserManagementHandlers(_context, _passwordHasher);

        var chiefRole = await _context.Roles.FirstAsync(r => r.Name == "ChiefAccountant");

        // 1. Create User
        var createRes = await handler.Handle(new CreateUserCommand(
            "accountant01",
            "Pass@123456",
            "Nguyễn Thị Kế Toán",
            "ketoan@company.vn",
            chiefRole.Id), CancellationToken.None);

        Assert.True(createRes.IsSuccess);
        var user = createRes.Value!;
        Assert.Equal("accountant01", user.Username);
        Assert.Equal("ChiefAccountant", user.RoleName);

        // 2. Duplicate Username should fail
        var dupRes = await handler.Handle(new CreateUserCommand(
            "accountant01",
            "Pass@123456",
            "Duplicate User",
            "dup@company.vn",
            chiefRole.Id), CancellationToken.None);

        Assert.False(dupRes.IsSuccess);

        // 3. Update User
        var updateRes = await handler.Handle(new UpdateUserCommand(
            user.Id,
            "Nguyễn Thị Kế Toán Trưởng",
            "ktt@company.vn",
            chiefRole.Id), CancellationToken.None);

        Assert.True(updateRes.IsSuccess);
        Assert.Equal("Nguyễn Thị Kế Toán Trưởng", updateRes.Value!.FullName);

        // 4. Toggle Status
        var deactivateRes = await handler.Handle(new SetUserStatusCommand(user.Id, false), CancellationToken.None);
        Assert.True(deactivateRes.IsSuccess);

        var dbUser = await _context.Users.FindAsync(user.Id);
        Assert.False(dbUser!.IsActive);

        // 5. Reset Password
        var resetRes = await handler.Handle(new ResetUserPasswordCommand(user.Id, "NewSecret@2026"), CancellationToken.None);
        Assert.True(resetRes.IsSuccess);

        var refreshedUser = await _context.Users.FindAsync(user.Id);
        Assert.True(_passwordHasher.VerifyPassword("NewSecret@2026", refreshedUser!.PasswordHash));
    }
}
