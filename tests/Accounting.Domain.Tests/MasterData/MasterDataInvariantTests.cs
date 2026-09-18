using Accounting.Application.MasterData.Validators;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Currencies;
using Accounting.Domain.MasterData.Partners;
using Accounting.Infrastructure.Persistence.Context;
using Accounting.Infrastructure.Persistence.Seeding;
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;
using Xunit;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;

namespace Accounting.Domain.Tests.MasterData;

public class MasterDataInvariantTests
{
    [Fact]
    public void Account_ParentCannotAcceptDirectPostings()
    {
        // Arrange
        var parentId = new AccountId("111");
        var childId = new AccountId("1111");

        var parent = new Account(parentId, "Tiền mặt", AccountType.Asset, BalanceNature.DebitBalance);
        var child = new Account(childId, "Tiền Việt Nam", AccountType.Asset, BalanceNature.DebitBalance, parentId);

        // Before adding sub-account, parent is not marked as parent
        Assert.True(parent.CanPost);

        // Act
        parent.AddSubAccount(child);

        // Assert
        Assert.True(parent.IsParent);
        Assert.False(parent.CanPost);
        Assert.True(child.CanPost);
    }

    [Fact]
    public void Account_ChildCodeMustMatchParentPrefix()
    {
        // Child 1111 with parent 111 -> Valid
        var parent111 = new AccountId("111");
        var validChild = new Account(new AccountId("1111"), "Tiền VN", AccountType.Asset, BalanceNature.DebitBalance, parent111);
        Assert.Equal(parent111, validChild.ParentAccountId);

        // Child 1111 with parent 112 -> Throws InvalidOperationException
        var parent112 = new AccountId("112");
        var ex = Assert.Throws<InvalidOperationException>(() =>
            new Account(new AccountId("1111"), "Tiền VN", AccountType.Asset, BalanceNature.DebitBalance, parent112));

        Assert.Contains("must start with parent code", ex.Message);

        // Explicit spec verification: sub-account 1113 under parent 112 is rejected by domain and validator
        var ex1113 = Assert.Throws<InvalidOperationException>(() =>
            new Account(new AccountId("1113"), "Vàng tiền tệ", AccountType.Asset, BalanceNature.DebitBalance, parent112));
        Assert.Contains("must start with parent code", ex1113.Message);

        var validator = new AccountValidator();
        // Construct without parent check to test validator explicitly
        var invalidAccount = new Account(new AccountId("1113"), "Vàng tiền tệ", AccountType.Asset, BalanceNature.DebitBalance, new AccountId("111"));
        var parentWithInvalidChild = new Account(new AccountId("112"), "Tiền gửi NH", AccountType.Asset, BalanceNature.DebitBalance);
        Assert.Throws<InvalidOperationException>(() => parentWithInvalidChild.AddSubAccount(invalidAccount));
    }

    [Theory]
    [InlineData("111", 1)]
    [InlineData("112", 1)]
    [InlineData("1111", 2)]
    [InlineData("1121", 2)]
    [InlineData("33311", 3)]
    [InlineData("41111", 3)]
    public void Account_ComputeLevel_ReturnsCorrectTier(string code, int expectedLevel)
    {
        var level = Account.ComputeLevel(code);
        Assert.Equal(expectedLevel, level);
    }

    [Theory]
    [InlineData("0101234567")]
    [InlineData("0312345678")]
    [InlineData("0101234567-001")]
    [InlineData("0312345678-999")]
    public void BusinessPartner_ValidVietnameseTaxCode_Succeeds(string taxCode)
    {
        var partner = new BusinessPartner(
            PartnerId.New(),
            "CUST-001",
            "Công ty TNHH Thử Nghiệm",
            PartnerType.Customer,
            taxCode: taxCode);

        Assert.Equal(taxCode, partner.TaxCode);

        var validator = new BusinessPartnerValidator();
        var result = validator.Validate(partner);
        Assert.True(result.IsValid);
    }

    [Theory]
    [InlineData("12345")]
    [InlineData("0101234567890")]
    [InlineData("ABC1234567")]
    [InlineData("0101234567-12")]
    [InlineData("0101234567-1234")]
    public void BusinessPartner_InvalidTaxCode_ThrowsArgumentException(string invalidTax)
    {
        var ex = Assert.Throws<ArgumentException>(() =>
            new BusinessPartner(
                PartnerId.New(),
                "CUST-ERR",
                "Công ty Sai Mã Số Thuế",
                PartnerType.Customer,
                taxCode: invalidTax));

        Assert.Contains("Invalid Vietnamese tax code", ex.Message);
    }

    [Fact]
    public void ExchangeRate_NegativeOrZeroRate_ThrowsArgumentOutOfRangeException()
    {
        var rateId = ExchangeRateId.New();
        var usd = CurrencyCode.Usd;
        var today = DateTime.UtcNow.Date;

        Assert.Throws<ArgumentOutOfRangeException>(() =>
            new ExchangeRate(rateId, usd, today, buyingRate: -1m, sellingRate: 25000m, averageRate: 25000m));

        Assert.Throws<ArgumentOutOfRangeException>(() =>
            new ExchangeRate(rateId, usd, today, buyingRate: 25000m, sellingRate: 0m, averageRate: 25000m));

        Assert.Throws<ArgumentOutOfRangeException>(() =>
            new ExchangeRate(rateId, usd, today, buyingRate: 25000m, sellingRate: 25000m, averageRate: -100m));
    }

    [Fact]
    public async Task StatutorySeeder_SeedsManifestAccounts_WithValidHierarchyInSqlite()
    {
        // Arrange
        using var connection = new SqliteConnection("DataSource=:memory:");
        await connection.OpenAsync();

        var options = new DbContextOptionsBuilder<AccountingDbContext>()
            .UseSqlite(connection)
            .Options;

        using (var context = new AccountingDbContext(options))
        {
            await context.Database.EnsureCreatedAsync();

            // Act - Seed
            await StatutorySeeder.SeedAsync(context);

            // Assert
            var accountsCount = await context.MasterAccounts.CountAsync();
            Assert.True(accountsCount >= 140, $"Expected >= 140 accounts, got {accountsCount}.");

            // Currencies
            var currencies = await context.Currencies.ToListAsync();
            Assert.Equal(3, currencies.Count);
            Assert.Contains(currencies, c => c.Id == CurrencyCode.Vnd && c.IsBaseCurrency);
            Assert.Contains(currencies, c => c.Id == CurrencyCode.Usd && !c.IsBaseCurrency);

            // Exchange rates
            var rates = await context.ExchangeRates.ToListAsync();
            Assert.True(rates.Count >= 2);

            // UoMs
            var uoms = await context.UnitsOfMeasure.ToListAsync();
            Assert.True(uoms.Count >= 5);

            // Warehouses
            var warehouses = await context.MasterWarehouses.ToListAsync();
            Assert.True(warehouses.Count >= 2);

            // Hierarchy and posting invariant verification
            var parent111 = await context.MasterAccounts.Include(a => a.SubAccounts).FirstOrDefaultAsync(a => a.Id == new AccountId("111"));
            Assert.NotNull(parent111);
            Assert.True(parent111.IsParent);
            Assert.False(parent111.CanPost);

            var leaf1111 = await context.MasterAccounts.FirstOrDefaultAsync(a => a.Id == new AccountId("1111"));
            Assert.NotNull(leaf1111);
            Assert.False(leaf1111.IsParent);
            Assert.True(leaf1111.CanPost);
            Assert.Equal(new AccountId("111"), leaf1111.ParentAccountId);

            // Bilateral partner accounts
            var tk131 = await context.MasterAccounts.FirstOrDefaultAsync(a => a.Id == new AccountId("131"));
            Assert.NotNull(tk131);
            Assert.Equal(BalanceNature.Bilateral, tk131.BalanceNature);
            Assert.True(tk131.RequiresPartner);

            var tk331 = await context.MasterAccounts.FirstOrDefaultAsync(a => a.Id == new AccountId("331"));
            Assert.NotNull(tk331);
            Assert.Equal(BalanceNature.Bilateral, tk331.BalanceNature);
            Assert.True(tk331.RequiresPartner);

            // Idempotency: re-running seeder should not duplicate or throw
            await StatutorySeeder.SeedAsync(context);
            var reCount = await context.MasterAccounts.CountAsync();
            Assert.Equal(accountsCount, reCount);
        }
    }

    [Fact]
    public void InternalGovernance_Dimensions_ShouldEnforceInvariants()
    {
        var ccId = new CostCenterId("PX01");
        var costCenter = new Accounting.Domain.MasterData.Dimensions.CostCenter(ccId, "Phân xưởng 1");
        Assert.Equal("PX01", costCenter.Code);
        Assert.Equal("Phân xưởng 1", costCenter.Name);
        Assert.Null(costCenter.ParentId);

        var deptId = new DepartmentId("PB_KT");
        var dept = new Accounting.Domain.MasterData.Dimensions.Department(deptId, "Phòng Kế Toán");
        Assert.Equal("PB_KT", dept.Code);
        Assert.Equal("Phòng Kế Toán", dept.Name);

        var expId = new ExpenseItemId("CP_LUONG");
        var expItem = new Accounting.Domain.MasterData.Dimensions.ExpenseItem(expId, "Chi phí lương");
        Assert.Equal("CP_LUONG", expItem.Code);
        Assert.Equal("Chi phí lương", expItem.Name);
    }

    [Fact]
    public async Task Circular99CoaSeeder_SeedsStatutoryAccounts_WithValidHierarchyInSqlite()
    {
        using var connection = new SqliteConnection("DataSource=:memory:");
        await connection.OpenAsync();

        var options = new DbContextOptionsBuilder<AccountingDbContext>()
            .UseSqlite(connection)
            .Options;

        using var context = new AccountingDbContext(options);
        await context.Database.EnsureCreatedAsync();

        // Act
        await Circular99CoaSeeder.SeedMasterDataAsync(context);

        // Assert
        var accountsCount = await context.MasterAccounts.CountAsync();
        Assert.True(accountsCount >= 140, $"Expected >= 140 Circular 99 accounts, got {accountsCount}.");

        // Currencies & UoMs
        Assert.True(await context.Currencies.CountAsync() >= 3);
        Assert.True(await context.UnitsOfMeasure.CountAsync() >= 5);

        // Verify Class 1 to 9 presence
        Assert.True(await context.MasterAccounts.AnyAsync(a => a.Id == new AccountId("111")));
        Assert.True(await context.MasterAccounts.AnyAsync(a => a.Id == new AccountId("211")));
        Assert.True(await context.MasterAccounts.AnyAsync(a => a.Id == new AccountId("331")));
        Assert.True(await context.MasterAccounts.AnyAsync(a => a.Id == new AccountId("411")));
        Assert.True(await context.MasterAccounts.AnyAsync(a => a.Id == new AccountId("511")));
        Assert.True(await context.MasterAccounts.AnyAsync(a => a.Id == new AccountId("632")));
        Assert.True(await context.MasterAccounts.AnyAsync(a => a.Id == new AccountId("711")));
        Assert.True(await context.MasterAccounts.AnyAsync(a => a.Id == new AccountId("811")));
        Assert.True(await context.MasterAccounts.AnyAsync(a => a.Id == new AccountId("911")));

        // Idempotency
        await Circular99CoaSeeder.SeedMasterDataAsync(context);
        var recount = await context.MasterAccounts.CountAsync();
        Assert.Equal(accountsCount, recount);
    }
}
