using System.Security.Cryptography;
using Accounting.Application.Common.Behaviors;
using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Services;
using Accounting.Application.Features.Ops;
using Accounting.Application.Features.Security;
using Accounting.Domain.Common;
using Accounting.Domain.Entities.Security;
using Accounting.Domain.Enums;
using Accounting.Domain.Security;
using Accounting.Infrastructure.Persistence.Connections;
using Accounting.Infrastructure.Persistence.Context;
using Accounting.Infrastructure.Persistence.Seeding;
using MediatR;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using Xunit;

namespace Accounting.Domain.Tests.Security;

public class HardeningAndSecurityTests : IDisposable
{
    private readonly string _testDbPath;
    private readonly AccountingDbContext _context;
    private readonly ISqlConnectionFactory _connectionFactory;
    private readonly IPasswordHasher _passwordHasher;
    private readonly string _tempBackupDir;

    public HardeningAndSecurityTests()
    {
        _testDbPath = $"hardening_test_{Guid.NewGuid():N}.db";
        _tempBackupDir = Path.Combine(Path.GetTempPath(), $"acc_backup_{Guid.NewGuid():N}");
        Directory.CreateDirectory(_tempBackupDir);

        var configData = new Dictionary<string, string?>
        {
            { "DatabaseProvider", "Sqlite" },
            { "ConnectionStrings:SqliteConnection", $"Data Source={_testDbPath}" }
        };
        var config = new ConfigurationBuilder().AddInMemoryCollection(configData).Build();
        _connectionFactory = new DapperDbConnectionFactory(config);

        var options = new DbContextOptionsBuilder<AccountingDbContext>()
            .UseSqlite($"Data Source={_testDbPath}")
            .Options;

        _context = new AccountingDbContext(options);
        _context.Database.EnsureCreated();

        _passwordHasher = new PasswordHasher();
    }

    public void Dispose()
    {
        _context.Dispose();
        if (File.Exists(_testDbPath))
        {
            try { File.Delete(_testDbPath); } catch { }
        }
        if (Directory.Exists(_tempBackupDir))
        {
            try { Directory.Delete(_tempBackupDir, recursive: true); } catch { }
        }
    }

    private class MockUserContext : ICurrentUserContext
    {
        public string? UserId { get; set; } = Guid.NewGuid().ToString();
        public string Username { get; set; } = "limited_user";
        public string MachineName => "TEST-PC";
        public string? IpAddress => "127.0.0.1";
        public bool IsAuthenticated => !string.IsNullOrEmpty(UserId);
        public List<AppPermission> Permissions { get; set; } = [];
        IReadOnlyCollection<AppPermission> ICurrentUserContext.Permissions => Permissions;
        public bool HasPermission(AppPermission permission) =>
            Permissions.Contains(AppPermission.SystemAdmin) || Permissions.Contains(permission);
    }

    [RequirePermission(AppPermission.PostVoucher)]
    public record SecuredCommand(string Data) : IRequest<string>;

    [Fact]
    public async Task Pipeline_AuthorizationBehavior_UnauthorizedUser_ThrowsUnauthorizedAccessException()
    {
        // User with only ViewLedger permission (lacks PostVoucher)
        var userContext = new MockUserContext
        {
            Permissions = [AppPermission.ViewLedger]
        };

        var behavior = new AuthorizationBehavior<SecuredCommand, string>(userContext);
        var command = new SecuredCommand("Test payload");

        bool handlerInvoked = false;
        RequestHandlerDelegate<string> next = (ct) =>
        {
            handlerInvoked = true;
            return Task.FromResult("OK");
        };

        // Assert: Throws UnauthorizedAccessException and handler is NEVER called
        var ex = await Assert.ThrowsAsync<UnauthorizedAccessException>(() =>
            behavior.Handle(command, next, CancellationToken.None));

        Assert.Contains("PostVoucher", ex.Message);
        Assert.False(handlerInvoked);
    }

    [Fact]
    public async Task Pipeline_AuthorizationBehavior_AuthorizedUser_ExecutesHandlerSuccessfully()
    {
        // User with PostVoucher permission
        var userContext = new MockUserContext
        {
            Permissions = [AppPermission.PostVoucher]
        };

        var behavior = new AuthorizationBehavior<SecuredCommand, string>(userContext);
        var command = new SecuredCommand("Test payload");

        bool handlerInvoked = false;
        RequestHandlerDelegate<string> next = (ct) =>
        {
            handlerInvoked = true;
            return Task.FromResult("OK");
        };

        var result = await behavior.Handle(command, next, CancellationToken.None);

        Assert.Equal("OK", result);
        Assert.True(handlerInvoked);
    }

    [Fact]
    public async Task CryptographicBackup_Aes256_EncryptsAtRest_AndRestoresMatchingChecksum()
    {
        // 1. Prepare sample database file with plain text secret
        var sampleDbFile = Path.Combine(_tempBackupDir, "original.db");
        var secretText = "ACCOUNTING_PLAINTEXT_SECRET_TK111_TK112_DATA_2026";
        await File.WriteAllTextAsync(sampleDbFile, secretText);

        var secretPass = new SecretString("P@ssw0rd2026CryptographicSafety!");
        var backupFile = Path.Combine(_tempBackupDir, "encrypted.enc.bak");

        // 2. Encrypt
        await CryptoBackupEngine.EncryptFileAsync(sampleDbFile, backupFile, secretPass.Reveal(), CancellationToken.None);

        // Assert: Encrypted file exists and does NOT contain raw plaintext
        Assert.True(File.Exists(backupFile));
        var encryptedBytes = await File.ReadAllBytesAsync(backupFile);
        var encryptedString = System.Text.Encoding.UTF8.GetString(encryptedBytes);
        Assert.DoesNotContain(secretText, encryptedString);

        // 3. Compute SHA-256 of backup
        var checksum = await CryptoBackupEngine.ComputeSha256Async(backupFile, CancellationToken.None);
        Assert.False(string.IsNullOrWhiteSpace(checksum));
        Assert.Equal(64, checksum.Length); // 64 hex characters for SHA-256

        // 4. Restore / Decrypt
        var restoredFile = Path.Combine(_tempBackupDir, "restored.db");
        await CryptoBackupEngine.DecryptFileAsync(backupFile, restoredFile, secretPass.Reveal(), CancellationToken.None);

        // Assert: Restored file matches original plaintext
        Assert.True(File.Exists(restoredFile));
        var restoredText = await File.ReadAllTextAsync(restoredFile);
        Assert.Equal(secretText, restoredText);
    }

    [Fact]
    public void SecretString_ToString_ObfuscatesContent()
    {
        var secret = new SecretString("SuperSecretMasterKey12345");

        Assert.Equal("***", secret.ToString());
        Assert.Equal("SuperSecretMasterKey12345", secret.Reveal());
        Assert.False(secret.IsEmpty);
    }

    [Fact]
    public void PasswordHasher_PBKDF2_HashesAndVerifiesCorrectly()
    {
        var rawPassword = "ComplexP@ssword2026!";
        var hash = _passwordHasher.HashPassword(rawPassword);

        Assert.NotNull(hash);
        Assert.Contains(":", hash); // Salt:Hash separator
        Assert.True(_passwordHasher.VerifyPassword(rawPassword, hash));
        Assert.False(_passwordHasher.VerifyPassword("WrongPassword!", hash));
    }

    [Fact]
    public async Task DatabaseOps_OptimizeCommand_ExecutesVacuumSuccessfully()
    {
        var handler = new OptimizeDatabaseCommandHandler(_connectionFactory);
        var result = await handler.Handle(new OptimizeDatabaseCommand(), CancellationToken.None);

        Assert.True(result.IsSuccess);
        Assert.Contains("VACUUM", result.Value);
    }

    [Fact]
    public async Task AuditTrail_DapperQuery_ReturnsPaginatedHistory()
    {
        // 1. Seed audit trail records
        var audit1 = new AuditTrail(
            AuditTrailId.New(),
            "00000000-0000-0000-0000-000000000001",
            "accountant_a",
            "WORKSTATION-01",
            AuditAction.Post,
            "GlVoucher",
            "V-2026-001",
            null,
            "{\"amount\": 1000000}",
            "Posted journal voucher V-2026-001");

        var audit2 = new AuditTrail(
            AuditTrailId.New(),
            "00000000-0000-0000-0000-000000000002",
            "accountant_b",
            "WORKSTATION-02",
            AuditAction.Unpost,
            "GlVoucher",
            "V-2026-002",
            "{\"amount\": 500000}",
            null,
            "Unposted journal voucher V-2026-002");

        _context.AuditTrails.AddRange(audit1, audit2);
        await _context.SaveChangesAsync();

        // 2. Query via Dapper handler
        var queryHandler = new GetAuditTrailRecordsQueryHandler(_connectionFactory);
        var result = await queryHandler.Handle(new GetAuditTrailRecordsQuery(PageNumber: 1, PageSize: 10), CancellationToken.None);

        Assert.True(result.IsSuccess);
        var paged = result.Value!;
        Assert.True(paged.TotalCount >= 2);
        Assert.Contains(paged.Items, x => x.Username == "accountant_a");
        Assert.Contains(paged.Items, x => x.Username == "accountant_b");
    }

    [Fact]
    public async Task SecuritySeeder_ProvisionsAdminUserAndSystemAdministratorRole()
    {
        await SecuritySeeder.SeedSecurityAsync(_context, _passwordHasher);

        var adminUser = await _context.Users.FirstOrDefaultAsync(u => u.Username == "admin");
        Assert.NotNull(adminUser);
        Assert.True(adminUser.IsActive);
        Assert.True(_passwordHasher.VerifyPassword("Admin@123456", adminUser.PasswordHash));

        var adminRole = await _context.Roles.FirstOrDefaultAsync(r => r.Name == "SystemAdministrator");
        Assert.NotNull(adminRole);
        Assert.True(adminRole.HasPermission(nameof(AppPermission.SystemAdmin)));
        Assert.True(adminRole.HasPermission(nameof(AppPermission.PostVoucher)));
    }
}
