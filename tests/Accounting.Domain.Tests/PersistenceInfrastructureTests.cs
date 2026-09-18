using System.Data;
using Accounting.Application.Common.Behaviors;
using Accounting.Application.Common.Interfaces;
using Accounting.Domain.Common;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Entities.Purchasing;
using Accounting.Domain.Entities.Sales;
using Accounting.Domain.Entities.Security;
using Accounting.Domain.Enums;
using Accounting.Infrastructure.Persistence.Connections;
using Accounting.Infrastructure.Persistence.Context;
using Accounting.Infrastructure.Persistence.Interceptors;
using Accounting.Infrastructure.Persistence.Seeding;
using FluentValidation;
using MediatR;
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using MySqlConnector;
using Xunit;

namespace Accounting.Domain.Tests;

public class PersistenceInfrastructureTests
{
    private class TestCurrentUserService : ICurrentUserService
    {
        public Guid? UserId { get; set; } = Guid.Parse("11111111-2222-3333-4444-555555555555");
        public string Username { get; set; } = "auditor_user";
        public string MachineName { get; set; } = "TEST-DESKTOP-01";
        public string? IpAddress { get; set; } = "127.0.0.1";
    }

    private class TestDateTimeService : IDateTimeService
    {
        public DateTime UtcNow => new(2026, 9, 17, 10, 0, 0, DateTimeKind.Utc);
        public DateTime Now => UtcNow.ToLocalTime();
    }

    private static AccountingDbContext CreateInMemoryContext(ICurrentUserService? userService = null, IDateTimeService? timeService = null)
    {
        userService ??= new TestCurrentUserService();
        timeService ??= new TestDateTimeService();

        var auditInterceptor = new AuditTrailInterceptor(userService, timeService);
        var softDeleteInterceptor = new SoftDeleteInterceptor(userService, timeService);

        var connection = new SqliteConnection("Data Source=:memory:;Mode=Memory;Cache=Shared");
        connection.Open();

        var options = new DbContextOptionsBuilder<AccountingDbContext>()
            .UseSqlite(connection)
            .AddInterceptors(auditInterceptor, softDeleteInterceptor)
            .Options;

        var context = new AccountingDbContext(options);
        context.Database.EnsureCreated();
        return context;
    }

    [Fact]
    public void FiscalPeriodId_And_AuditTrailId_ShouldBehaveAsValueObjects()
    {
        // 1. FiscalPeriodId
        var id1 = new FiscalPeriodId(202609);
        var id2 = FiscalPeriodId.FromYearMonth(2026, 9);
        var id3 = new FiscalPeriodId(202610);

        Assert.Equal(id1, id2);
        Assert.NotEqual(id1, id3);
        Assert.True(id1 < id3);
        Assert.Equal("202609", id1.ToString());
        int intVal = id1;
        Assert.Equal(202609, intVal);
        Assert.Equal(id1, (FiscalPeriodId)202609);

        // 2. AuditTrailId
        var guid = Guid.NewGuid();
        var auditId1 = new AuditTrailId(guid);
        var auditId2 = new AuditTrailId(guid);
        var auditId3 = AuditTrailId.New();

        Assert.Equal(auditId1, auditId2);
        Assert.NotEqual(auditId1, auditId3);
        Assert.Equal(guid.ToString("D"), auditId1.ToString());
        Guid guidVal = auditId1;
        Assert.Equal(guid, guidVal);
        Assert.Equal(auditId1, (AuditTrailId)guid);
    }

    [Fact]
    public async Task AuditTrailInterceptor_ShouldStampAuditableEntity_AndGenerateAuditTrailOnAddAndModify()
    {
        var userService = new TestCurrentUserService();
        var timeService = new TestDateTimeService();
        using var context = CreateInMemoryContext(userService, timeService);

        // 1. Add Customer (IAuditableEntity)
        var customer = new Customer("KH-TEST-001", "Cong ty TNHH Song Hong", "0101234567", 500_000_000m);
        context.Customers.Add(customer);
        await context.SaveChangesAsync();

        // Verify entity stamped
        Assert.Equal(timeService.UtcNow, customer.CreatedAtUtc);
        Assert.Equal(userService.Username, customer.CreatedBy);
        Assert.Null(customer.LastModifiedAtUtc);

        // Verify AuditTrail row created
        var auditRecords = await context.AuditTrails.Where(a => a.EntityName == nameof(Customer)).ToListAsync();
        Assert.Single(auditRecords);
        var createAudit = auditRecords.First();
        Assert.Equal(AuditAction.Create, createAudit.Action);
        Assert.Equal(customer.Id.ToString(), createAudit.EntityId);
        Assert.Equal(userService.Username, createAudit.Username);
        Assert.NotNull(createAudit.NewValuesJson);
        Assert.Contains("KH-TEST-001", createAudit.NewValuesJson);

        // 2. Modify Customer
        customer.AdjustBalance(15_000_000m);
        await context.SaveChangesAsync();

        // Verify entity modification stamped
        Assert.Equal(timeService.UtcNow, customer.LastModifiedAtUtc);
        Assert.Equal(userService.Username, customer.LastModifiedBy);

        // Verify second AuditTrail row
        var updatedAudits = await context.AuditTrails.Where(a => a.EntityName == nameof(Customer)).OrderBy(a => a.TimestampUtc).ToListAsync();
        Assert.Equal(2, updatedAudits.Count);
        var updateAudit = updatedAudits.Last();
        Assert.Equal(AuditAction.Update, updateAudit.Action);
        Assert.NotNull(updateAudit.OldValuesJson);
        Assert.NotNull(updateAudit.NewValuesJson);
        Assert.Contains("CurrentReceivableBalance", updateAudit.DiffSummary);
    }

    [Fact]
    public async Task SoftDeleteInterceptor_ShouldSoftDelete_AndQueryFilterShouldExcludeByDefault()
    {
        var userService = new TestCurrentUserService();
        var timeService = new TestDateTimeService();
        using var context = CreateInMemoryContext(userService, timeService);

        // 1. Create Vendor
        var vendor = new Vendor("NCC-001", "Nha Cung Cap An Phat", "0309998888");
        context.Vendors.Add(vendor);
        await context.SaveChangesAsync();

        var vendorId = vendor.Id;
        Assert.False(vendor.IsDeleted);

        // 2. Delete Vendor
        context.Vendors.Remove(vendor);
        await context.SaveChangesAsync();

        // Interceptor converted Delete to Modified with IsDeleted = true
        Assert.True(vendor.IsDeleted);
        Assert.Equal(timeService.UtcNow, vendor.DeletedAtUtc);
        Assert.Equal(userService.Username, vendor.DeletedBy);

        // 3. Query should filter it out by default
        var activeVendor = await context.Vendors.FirstOrDefaultAsync(v => v.Id == vendorId);
        Assert.Null(activeVendor);

        // 4. IgnoreQueryFilters should still find the soft-deleted record
        var softDeletedVendor = await context.Vendors.IgnoreQueryFilters().FirstOrDefaultAsync(v => v.Id == vendorId);
        Assert.NotNull(softDeletedVendor);
        Assert.True(softDeletedVendor.IsDeleted);

        // 5. Audit trail should record Delete action
        var deleteAudit = await context.AuditTrails
            .FirstOrDefaultAsync(a => a.EntityName == nameof(Vendor) && a.Action == AuditAction.Delete);
        Assert.NotNull(deleteAudit);
        Assert.Equal(vendorId.ToString(), deleteAudit.EntityId);
    }

    [Fact]
    public void DapperSqlConnectionFactory_ShouldReturnCorrectConnection_BasedOnProviderConfig()
    {
        // 1. SQLite Provider
        var sqliteConfigData = new Dictionary<string, string?>
        {
            { "Database:Provider", "Sqlite" },
            { "Database:SqliteConnection", "Data Source=accounting_unit_test.db" }
        };
        var sqliteConfig = new ConfigurationBuilder().AddInMemoryCollection(sqliteConfigData).Build();
        ISqlConnectionFactory sqliteFactory = new DapperSqlConnectionFactory(sqliteConfig);

        Assert.Equal("Sqlite", sqliteFactory.ProviderName);
        using var sqliteConn = sqliteFactory.CreateConnection();
        Assert.IsType<SqliteConnection>(sqliteConn);
        Assert.Equal("Data Source=accounting_unit_test.db", sqliteConn.ConnectionString);

        // 2. MariaDB Provider using ConnectionStrings:MariaDB
        var mariaDbConfigData = new Dictionary<string, string?>
        {
            { "Database:Provider", "MariaDB" },
            { "ConnectionStrings:MariaDB", "Server=127.0.0.1;Port=3306;Database=test_accounting;User=root;Password=secret;" }
        };
        var mariaDbConfig = new ConfigurationBuilder().AddInMemoryCollection(mariaDbConfigData).Build();
        ISqlConnectionFactory mariaDbFactory = new DapperSqlConnectionFactory(mariaDbConfig);

        Assert.Equal("MariaDB", mariaDbFactory.ProviderName);
        using var mariaConn = mariaDbFactory.CreateConnection();
        Assert.IsType<MySqlConnection>(mariaConn);
        Assert.Contains("127.0.0.1", mariaConn.ConnectionString);
    }

    [Fact]
    public void MariaDbProvider_ShouldConfigureCorrectDialectVersion()
    {
        var builder = new DbContextOptionsBuilder<AccountingDbContext>();
        var serverVersion = new MariaDbServerVersion(new Version(12, 3, 0));
        builder.UseMySql("Server=localhost;Port=3306;Database=accounting;User=root;Password=root;", serverVersion);

        var options = builder.Options;
        var mySqlExtension = options.Extensions.FirstOrDefault(e => e.GetType().Name == "MySqlOptionsExtension");
        Assert.NotNull(mySqlExtension);
    }

    [Fact]
    public void DomainErrorsAndResult_ShouldSupportExplicitTypedFailureAndSuccess()
    {
        // Success case
        Result<FiscalPeriodId, DomainError> successResult = Result<FiscalPeriodId, DomainError>.Success(new FiscalPeriodId(202609));
        Assert.True(successResult.IsSuccess);
        Assert.False(successResult.IsFailure);
        Assert.Equal(202609, successResult.Value.Value);

        var matchedSuccess = successResult.Match(
            onSuccess: id => $"Period: {id}",
            onFailure: err => $"Error: {err.Message}");
        Assert.Equal("Period: 202609", matchedSuccess);

        // Failure case
        var error = DomainError.Validation("Fiscal period is locked");
        Result<FiscalPeriodId, DomainError> failureResult = Result<FiscalPeriodId, DomainError>.Failure(error);
        Assert.False(failureResult.IsSuccess);
        Assert.True(failureResult.IsFailure);
        Assert.Equal("VALIDATION_FAILED", failureResult.Error.Code);
        Assert.Equal("Fiscal period is locked", failureResult.Error.Message);

        var matchedFailure = failureResult.Match(
            onSuccess: id => $"Period: {id}",
            onFailure: err => $"Error: {err.Message}");
        Assert.Equal("Error: Fiscal period is locked", matchedFailure);
    }

    [Fact]
    public void DependencyInjection_ShouldResolve_AllCoreContracts()
    {
        var configData = new Dictionary<string, string?>
        {
            { "Database:Provider", "Sqlite" },
            { "ConnectionStrings:Sqlite", "Data Source=accounting_di_test.db;" }
        };
        var config = new ConfigurationBuilder().AddInMemoryCollection(configData).Build();

        var services = new Microsoft.Extensions.DependencyInjection.ServiceCollection();
        Accounting.Infrastructure.Persistence.DependencyInjection.AddPersistenceInfrastructure(services, config);

        using var provider = services.BuildServiceProvider();
        using var scope = provider.CreateScope();

        // 1. DbContext contracts
        var accountingContext = scope.ServiceProvider.GetService<IAccountingDbContext>();
        Assert.NotNull(accountingContext);

        var appContext = scope.ServiceProvider.GetService<IApplicationDbContext>();
        Assert.NotNull(appContext);

        // 2. Connection factories
        var sqlFactory = scope.ServiceProvider.GetService<ISqlConnectionFactory>();
        Assert.NotNull(sqlFactory);
        Assert.Equal("Sqlite", sqlFactory.ProviderName);

        var dbFactory = scope.ServiceProvider.GetService<IDbConnectionFactory>();
        Assert.NotNull(dbFactory);

        // 3. User contexts
        var userService = scope.ServiceProvider.GetService<ICurrentUserService>();
        Assert.NotNull(userService);

        var userContext = scope.ServiceProvider.GetService<ICurrentUserContext>();
        Assert.NotNull(userContext);
        Assert.Equal("admin", userContext.Username);
    }

    [Fact]
    public async Task ValidationBehavior_ShouldThrowValidationException_WhenValidationFails()
    {
        var validator = new TestCommandValidator();
        var behavior = new ValidationBehavior<TestCommand, string>(new[] { validator });

        var invalidCommand = new TestCommand(string.Empty);
        await Assert.ThrowsAsync<ValidationException>(() =>
            behavior.Handle(invalidCommand, _ => Task.FromResult("OK"), CancellationToken.None));

        var validCommand = new TestCommand("Valid Name");
        var result = await behavior.Handle(validCommand, _ => Task.FromResult("Success"), CancellationToken.None);
        Assert.Equal("Success", result);
    }

    [Fact]
    public async Task MariaDb_LiveConnection_ShouldConnectWithDevCredentials()
    {
        var connStr = "Server=localhost;Port=3306;User=dev;Password=123456;TreatTinyAsBoolean=true;CharSet=utf8mb4;";
        using var conn = new MySqlConnection(connStr);
        await conn.OpenAsync();
        Assert.Equal(ConnectionState.Open, conn.State);

        using var cmd = conn.CreateCommand();
        cmd.CommandText = "SELECT VERSION();";
        var version = await cmd.ExecuteScalarAsync();
        Assert.NotNull(version);

        cmd.CommandText = "CREATE DATABASE IF NOT EXISTS accounting_core CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;";
        await cmd.ExecuteNonQueryAsync();

        var appConnStr = "Server=localhost;Port=3306;Database=accounting_core;User=dev;Password=123456;TreatTinyAsBoolean=true;CharSet=utf8mb4;";
        using var appConn = new MySqlConnection(appConnStr);
        await appConn.OpenAsync();
        Assert.Equal(ConnectionState.Open, appConn.State);
    }

    [Fact]
    public async Task MariaDb_LiveSeedingAndQuerying_ShouldSucceedWithDevUser()
    {
        var connStr = "Server=localhost;Port=3306;Database=accounting_core;User=dev;Password=123456;TreatTinyAsBoolean=true;CharSet=utf8mb4;";
        var serverVersion = new MariaDbServerVersion(new Version(12, 3, 0));

        var options = new DbContextOptionsBuilder<AccountingDbContext>()
            .UseMySql(connStr, serverVersion)
            .Options;

        using var context = new AccountingDbContext(options);
        await context.Database.EnsureDeletedAsync();
        await context.Database.EnsureCreatedAsync();
        await DbInitializer.SeedAsync(context);

        var accountCount = await context.Accounts.CountAsync();
        Assert.True(accountCount >= 30, $"Expected >= 30 accounts on live MariaDB, got {accountCount}");

        var masterAccountCount = await context.MasterAccounts.CountAsync();
        Assert.True(masterAccountCount >= 150, $"Expected >= 150 master accounts on live MariaDB, got {masterAccountCount}");

        var adminUser = await context.Users.FirstOrDefaultAsync(u => u.Username == "admin");
        Assert.NotNull(adminUser);

        var config = new ConfigurationBuilder().AddInMemoryCollection(new Dictionary<string, string?>
        {
            { "Database:Provider", "MariaDB" },
            { "ConnectionStrings:MariaDB", connStr }
        }).Build();

        var factory = new DapperSqlConnectionFactory(config);
        using var dapperConn = factory.CreateConnection();
        dapperConn.Open();
        using var cmd = dapperConn.CreateCommand();
        cmd.CommandText = "SELECT COUNT(*) FROM Accounts;";
        var scalarCount = Convert.ToInt32(cmd.ExecuteScalar());
        Assert.True(scalarCount >= 30);
    }

    public record TestCommand(string Name) : IRequest<string>;

    public class TestCommandValidator : AbstractValidator<TestCommand>
    {
        public TestCommandValidator()
        {
            RuleFor(x => x.Name).NotEmpty().WithMessage("Name is required");
        }
    }
}
