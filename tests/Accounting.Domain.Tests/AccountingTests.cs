using Accounting.Domain.Common;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Entities.Inventory;
using Accounting.Domain.Entities.Payroll;
using Accounting.Domain.Enums;
using Accounting.Domain.Exceptions;
using Accounting.Infrastructure.Persistence.Context;
using Accounting.Infrastructure.Persistence.Seeding;
using Microsoft.EntityFrameworkCore;
using Xunit;

namespace Accounting.Domain.Tests;

public class DoubleEntryBalanceTests
{
    [Fact]
    public void BalancedVoucher_ShouldSucceedValidation()
    {
        // Arrange
        var userId = Guid.NewGuid();
        var debitAccId = Guid.NewGuid();
        var creditAccId = Guid.NewGuid();

        var voucher = new Voucher("PKT-2026-001", DateTime.Today, DateTime.Today, VoucherType.GeneralJournal, "Sample balanced journal entry", userId);

        // Act
        voucher.AddLine(debitAccId, creditAccId, 100_000_000m, "Line 1: Debit 1111 / Credit 5111");
        voucher.ValidateBalance();

        // Assert
        Assert.Equal(100_000_000m, voucher.TotalDebitAmount);
        Assert.Equal(100_000_000m, voucher.TotalCreditAmount);
        Assert.Equal(VoucherStatus.Draft, voucher.Status);
    }

    [Fact]
    public void IdenticalDebitAndCreditAccount_ShouldThrowArgumentException()
    {
        // Arrange
        var userId = Guid.NewGuid();
        var sameAccId = Guid.NewGuid();
        var voucher = new Voucher("PKT-2026-002", DateTime.Today, DateTime.Today, VoucherType.GeneralJournal, "Invalid entry", userId);

        // Act & Assert
        Assert.Throws<ArgumentException>(() => voucher.AddLine(sameAccId, sameAccId, 50_000_000m, "Invalid line"));
    }

    [Fact]
    public void EmptyVoucher_ShouldThrowDomainExceptionOnValidation()
    {
        // Arrange
        var userId = Guid.NewGuid();
        var voucher = new Voucher("PKT-2026-003", DateTime.Today, DateTime.Today, VoucherType.GeneralJournal, "Empty voucher", userId);

        // Act & Assert
        Assert.Throws<AccountingDomainException>(() => voucher.ValidateBalance());
    }
}

public class VoucherLifecycleTests
{
    [Fact]
    public void PostVoucher_ShouldTransitionToPostedStatusAndSetMetadata()
    {
        // Arrange
        var userId = Guid.NewGuid();
        var debitAccId = Guid.NewGuid();
        var creditAccId = Guid.NewGuid();
        var voucher = new Voucher("PKT-2026-004", DateTime.Today, DateTime.Today, VoucherType.GeneralJournal, "Voucher lifecycle test", userId);
        voucher.AddLine(debitAccId, creditAccId, 25_000_000m, "Cash deposit");

        // Act
        voucher.Post(userId);

        // Assert
        Assert.Equal(VoucherStatus.Posted, voucher.Status);
        Assert.Equal(userId, voucher.PostedBy);
        Assert.NotNull(voucher.PostedAtUtc);
    }

    [Fact]
    public void ModifyingPostedVoucher_ShouldThrowImmutablePostedVoucherException()
    {
        // Arrange
        var userId = Guid.NewGuid();
        var debitAccId = Guid.NewGuid();
        var creditAccId = Guid.NewGuid();
        var voucher = new Voucher("PKT-2026-005", DateTime.Today, DateTime.Today, VoucherType.GeneralJournal, "Immutable check", userId);
        voucher.AddLine(debitAccId, creditAccId, 10_000_000m, "Test line");
        voucher.Post(userId);

        // Act & Assert
        Assert.Throws<ImmutablePostedVoucherException>(() => voucher.AddLine(debitAccId, creditAccId, 5_000_000m, "Attempt edit"));
        Assert.Throws<ImmutablePostedVoucherException>(() => voucher.ClearLines());
    }

    [Fact]
    public void ReversingPostedVoucher_ShouldInvertDebitAndCreditAccounts()
    {
        // Arrange
        var userId = Guid.NewGuid();
        var debitAccId = Guid.NewGuid();
        var creditAccId = Guid.NewGuid();
        var voucher = new Voucher("PKT-2026-006", DateTime.Today, DateTime.Today, VoucherType.GeneralJournal, "Original voucher", userId);
        voucher.AddLine(debitAccId, creditAccId, 15_000_000m, "Original transaction");
        voucher.Post(userId);

        // Act
        var reversal = voucher.CreateReversal("PKT-REV-001", userId, DateTime.Today, "Correction of accounting error");

        // Assert
        Assert.True(reversal.IsReversal);
        Assert.Equal(voucher.Id, reversal.OriginalVoucherId);
        Assert.Single(reversal.Lines);

        var revLine = reversal.Lines.First();
        Assert.Equal(creditAccId, revLine.DebitAccountId);  // Swapped
        Assert.Equal(debitAccId, revLine.CreditAccountId);  // Swapped
        Assert.Equal(15_000_000m, revLine.Amount);
    }
}

public class InventoryCostingTests
{
    [Fact]
    public void MovingAverageCosting_ShouldCalculateCorrectAverageAndOutflowCost()
    {
        // Arrange
        var item = new ProductItem("VT-001", "Thép cuộn D10", "Kg", CostingMethod.PerpetualMovingAverage);

        // Batch 1: Inflow 1,000 kg @ 10,000 VND/kg
        item.RecordInflow(1000m, 10_000m);
        Assert.Equal(1000m, item.CurrentStockQuantity);
        Assert.Equal(10_000_000m, item.CurrentStockValue);
        Assert.Equal(10_000m, item.AverageUnitCost);

        // Batch 2: Inflow 500 kg @ 13,000 VND/kg
        // Total Qty = 1,500 kg, Total Value = 10,000,000 + 6,500,000 = 16,500,000 VND
        // Average unit cost = 16,500,000 / 1,500 = 11,000 VND/kg
        item.RecordInflow(500m, 13_000m);
        Assert.Equal(1500m, item.CurrentStockQuantity);
        Assert.Equal(16_500_000m, item.CurrentStockValue);
        Assert.Equal(11_000m, item.AverageUnitCost);

        // Act: Outflow 600 kg
        // Expected cost = 600 * 11,000 = 6,600,000 VND
        var outflowCost = item.RecordOutflow(600m);

        // Assert
        Assert.Equal(6_600_000m, outflowCost);
        Assert.Equal(900m, item.CurrentStockQuantity);
        Assert.Equal(9_900_000m, item.CurrentStockValue);
        Assert.Equal(11_000m, item.AverageUnitCost);
    }

    [Fact]
    public void OutflowExceedingStock_ShouldThrowNegativeStockException()
    {
        // Arrange
        var item = new ProductItem("VT-002", "Xi măng PCB40", "Bao", CostingMethod.PerpetualMovingAverage);
        item.RecordInflow(50m, 90_000m);

        // Act & Assert
        Assert.Throws<NegativeStockException>(() => item.RecordOutflow(60m, "KHO-TONG"));
    }
}

public class PayrollAndStatutoryTests
{
    [Fact]
    public void SalaryCalculation_ShouldApplyStatutoryVietnameseInsuranceAndProgressivePit()
    {
        // Arrange: Employee with Gross = 30M VND, InsSalary = 20M VND, 1 dependent
        var emp = new Employee(
            "NV-001",
            "Nguyen Van An",
            "079090001234",
            "Ke Toan",
            "Ke Toan Truong",
            baseSalary: 25_000_000m,
            insuranceSalary: 20_000_000m,
            allowances: 5_000_000m,
            dependentsCount: 1);

        // Act
        var slip = new SalarySlip(2026, 9, emp);

        // Assert Employee Deductions:
        // BHXH 8% of 20M = 1,600,000 VND
        // BHYT 1.5% of 20M = 300,000 VND
        // BHTN 1.0% of 20M = 200,000 VND
        // Total Insurance = 2,100,000 VND
        Assert.Equal(1_600_000m, slip.SocialInsuranceEmployee);
        Assert.Equal(300_000m, slip.HealthInsuranceEmployee);
        Assert.Equal(200_000m, slip.UnemploymentInsuranceEmployee);
        Assert.Equal(2_100_000m, slip.TotalInsuranceEmployee);

        // PIT Deductions:
        // Gross = 30,000,000 VND
        // Deductions = 2,100,000 (BH) + 11,000,000 (Personal) + 4,400,000 (1 dependent) = 17,500,000 VND
        // Taxable Income = 30,000,000 - 17,500,000 = 12,500,000 VND
        // Bracket 1 (0 - 5M @ 5%) = 250,000 VND
        // Bracket 2 (5M - 10M @ 10%) = 500,000 VND
        // Bracket 3 (10M - 12.5M @ 15%) = 2,500,000 * 0.15 = 375,000 VND
        // Total PIT = 250,000 + 500,000 + 375,000 = 1,125,000 VND
        Assert.Equal(1_125_000m, slip.PersonalIncomeTax);

        // Net Salary = 30,000,000 - 2,100,000 - 1,125,000 = 26,775,000 VND
        Assert.Equal(26_775_000m, slip.NetSalary);

        // Employer contributions:
        // BHXH 17.5% of 20M = 3,500,000 VND
        // BHYT 3.0% of 20M = 600,000 VND
        // BHTN 1.0% of 20M = 200,000 VND
        // Trade Union 2.0% of 20M = 400,000 VND
        Assert.Equal(3_500_000m, slip.SocialInsuranceEmployer);
        Assert.Equal(600_000m, slip.HealthInsuranceEmployer);
        Assert.Equal(200_000m, slip.UnemploymentInsuranceEmployer);
        Assert.Equal(400_000m, slip.TradeUnionFeeEmployer);
    }
}

public class DatabaseSeederTests
{
    [Fact]
    public async Task SeedAsync_ShouldPopulateCircular200ChartOfAccountsAndAdminUser()
    {
        // Arrange: In-memory SQLite database
        var options = new DbContextOptionsBuilder<AccountingDbContext>()
            .UseSqlite("Data Source=InMemorySample;Mode=Memory;Cache=Shared")
            .Options;

        using var context = new AccountingDbContext(options);
        await context.Database.OpenConnectionAsync();

        // Act
        await DbInitializer.SeedAsync(context);

        // Assert
        var accountsCount = await context.Accounts.CountAsync();
        Assert.True(accountsCount >= 30, $"Chart of accounts should have seeded standard circular accounts, but found {accountsCount}.");

        // Assert crucial VAS Circular 200 accounts exist
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "1111"));
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "1121"));
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "131"));
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "1561"));
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "211"));
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "214"));
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "331"));
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "33311"));
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "334"));
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "411"));
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "4212"));
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "5111"));
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "632"));
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "641"));
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "642"));
        Assert.NotNull(await context.Accounts.FirstOrDefaultAsync(a => a.AccountNumber == "911"));

        // Assert Admin user & Warehouse
        var adminUser = await context.Users.FirstOrDefaultAsync(u => u.Username == "admin");
        Assert.NotNull(adminUser);

        var warehouse = await context.Warehouses.FirstOrDefaultAsync(w => w.Code == "KHO-TONG");
        Assert.NotNull(warehouse);
    }
}
