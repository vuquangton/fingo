using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Compliance;
using Accounting.Domain.MasterData.Partners;
using Accounting.Infrastructure.Persistence.Context;
using Accounting.Infrastructure.Persistence.Seeding;
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;
using Xunit;

namespace Accounting.Domain.Tests.Compliance;

public class StatutoryComplianceGuardrailTests
{
    [Theory]
    [InlineData("142")]
    [InlineData("311")]
    [InlineData("315")]
    [InlineData("512")]
    [InlineData("001")]
    [InlineData("002")]
    [InlineData("003")]
    [InlineData("004")]
    [InlineData("007")]
    public void ValidateManifest_WithBannedCode_ThrowsInvalidOperationException(string bannedCode)
    {
        var manifest = new List<AccountSeedModel>
        {
            new("111", "Tiền mặt", AccountType.Asset, BalanceNature.DebitBalance),
            new(bannedCode, "Abolished Account", AccountType.Asset, BalanceNature.DebitBalance)
        };

        var ex = Assert.Throws<InvalidOperationException>(() => StatutorySeeder.ValidateManifest(manifest));
        Assert.Contains(bannedCode, ex.Message);
        Assert.Contains("abolished legacy code banned", ex.Message);
    }

    [Theory]
    [InlineData("142")]
    [InlineData("311")]
    [InlineData("315")]
    [InlineData("512")]
    [InlineData("001")]
    [InlineData("002")]
    [InlineData("003")]
    [InlineData("004")]
    [InlineData("007")]
    public void AccountConstructor_WithBannedCode_ThrowsInvalidOperationException(string bannedCode)
    {
        var ex = Assert.Throws<InvalidOperationException>(() =>
            new Account(new AccountId(bannedCode), "Tài khoản bãi bỏ", AccountType.Asset, BalanceNature.DebitBalance));

        Assert.Contains(bannedCode, ex.Message);
        Assert.Contains("abolished legacy code banned", ex.Message);
    }

    [Fact]
    public async Task Database_ActiveAccounts_ContainsZeroBannedCodes()
    {
        using var connection = new SqliteConnection("DataSource=:memory:");
        await connection.OpenAsync();

        var options = new DbContextOptionsBuilder<AccountingDbContext>()
            .UseSqlite(connection)
            .Options;

        using var context = new AccountingDbContext(options);
        await context.Database.EnsureCreatedAsync();

        await StatutorySeeder.SeedAsync(context);

        var seededAccountIds = await context.MasterAccounts
            .Select(a => a.Id.Value)
            .ToListAsync();

        Assert.NotEmpty(seededAccountIds);

        foreach (var banned in BannedLegacyAccountCodes.AbolishedCodes)
        {
            Assert.DoesNotContain(banned, seededAccountIds);
        }
    }

    [Fact]
    public void Account_CanPostOn_EnforcesTemporalValidity()
    {
        var account = new Account(
            new AccountId("1111"),
            "Tiền Việt Nam",
            AccountType.Asset,
            BalanceNature.DebitBalance,
            new AccountId("111"),
            isParent: false,
            effectiveFrom: new DateOnly(2026, 1, 1),
            effectiveTo: new DateOnly(2026, 12, 31));

        // Outside effective window before start
        Assert.False(account.CanPostOn(new DateOnly(2025, 12, 31)));

        // Within effective window
        Assert.True(account.CanPostOn(new DateOnly(2026, 1, 1)));
        Assert.True(account.CanPostOn(new DateOnly(2026, 6, 15)));
        Assert.True(account.CanPostOn(new DateOnly(2026, 12, 31)));

        // Outside effective window after end
        Assert.False(account.CanPostOn(new DateOnly(2027, 1, 1)));
    }

    [Fact]
    public void Account_Parent_CannotPostDirectly()
    {
        var parent = new Account(
            new AccountId("111"),
            "Tiền mặt",
            AccountType.Asset,
            BalanceNature.DebitBalance,
            isParent: true,
            effectiveFrom: new DateOnly(2026, 1, 1));

        Assert.False(parent.CanPost);
        Assert.False(parent.CanPostOn(new DateOnly(2026, 1, 1)));
        Assert.False(parent.CanPostOn(new DateOnly(2026, 6, 1)));
    }

    [Theory]
    [InlineData("0101234567")]
    [InlineData("0312345678-001")]
    [InlineData("0109999999-123")]
    public void TaxCode_Validation_EnforcesVietnameseStatutoryRegex_Valid(string validTaxCode)
    {
        var partner = new BusinessPartner(
            PartnerId.New(),
            "SUPP-01",
            "Công ty Hợp Lệ",
            PartnerType.Vendor,
            taxCode: validTaxCode);

        Assert.Equal(validTaxCode, partner.TaxCode);
    }

    [Theory]
    [InlineData("123")]
    [InlineData("ABC1234567")]
    [InlineData("0101234567-1")]
    [InlineData("0101234567-12")]
    [InlineData("0101234567-1234")]
    [InlineData("0101234567890")]
    public void TaxCode_Validation_EnforcesVietnameseStatutoryRegex_Invalid(string invalidTaxCode)
    {
        var ex = Assert.Throws<ArgumentException>(() =>
            new BusinessPartner(
                PartnerId.New(),
                "SUPP-BAD",
                "Công ty Sai Thuế",
                PartnerType.Vendor,
                taxCode: invalidTaxCode));

        Assert.Contains("Invalid Vietnamese tax code", ex.Message);
    }
}
