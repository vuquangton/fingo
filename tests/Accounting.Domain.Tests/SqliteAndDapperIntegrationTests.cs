using System.Data;
using Accounting.Infrastructure.Persistence.Connections;
using Accounting.Infrastructure.Persistence.Context;
using Accounting.Infrastructure.Persistence.Seeding;
using Dapper;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using Xunit;

namespace Accounting.Domain.Tests;

public class SqliteAndDapperIntegrationTests
{
    public class AccountRow
    {
        public string AccountNumber { get; set; } = string.Empty;
        public string Name { get; set; } = string.Empty;
        public int Category { get; set; }
    }

    public class UserRow
    {
        public string Username { get; set; } = string.Empty;
        public string FullName { get; set; } = string.Empty;
    }

    [Fact]
    public async Task SqliteAndDapper_ShouldSeedAndQueryViaDapper()
    {
        var testDbPath = $"sqlite_dapper_test_{Guid.NewGuid():N}.db";
        try
        {
            var configData = new Dictionary<string, string?>
            {
                { "DatabaseProvider", "Sqlite" },
                { "ConnectionStrings:SqliteConnection", $"Data Source={testDbPath}" }
            };
            var config = new ConfigurationBuilder().AddInMemoryCollection(configData).Build();

            // 1. EF Core Initialization & Seeding
            var options = new DbContextOptionsBuilder<AccountingDbContext>()
                .UseSqlite($"Data Source={testDbPath}")
                .Options;

            using (var efContext = new AccountingDbContext(options))
            {
                await DbInitializer.SeedAsync(efContext);
            }

            Assert.True(File.Exists(testDbPath), "SQLite database file was created on disk.");

            // 2. High-speed Dapper Query via DapperDbConnectionFactory
            var connectionFactory = new DapperDbConnectionFactory(config);
            Assert.Equal("Sqlite", connectionFactory.ProviderName);

            using (var conn = connectionFactory.CreateConnection())
            {
                conn.Open();
                var count = await conn.ExecuteScalarAsync<int>("SELECT COUNT(*) FROM Accounts;");
                Assert.True(count >= 30, $"Dapper should count >= 30 accounts, but found {count}.");

                var tk111 = await conn.QueryFirstOrDefaultAsync<AccountRow>("SELECT AccountNumber, Name, Category FROM Accounts WHERE AccountNumber = @Acc;", new { Acc = "1111" });
                Assert.NotNull(tk111);
                Assert.Equal("1111", tk111.AccountNumber);
                Assert.Equal("Tiền Việt Nam", tk111.Name);

                var admin = await conn.QueryFirstOrDefaultAsync<UserRow>("SELECT Username, FullName FROM Users WHERE Username = 'admin';");
                Assert.NotNull(admin);
                Assert.Equal("System Administrator", admin.FullName);
            }
        }
        finally
        {
            if (File.Exists(testDbPath))
            {
                try { File.Delete(testDbPath); } catch { }
            }
        }
    }
}
