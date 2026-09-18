using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Services;
using Accounting.Application.Features.Payroll;
using Accounting.Domain.Common;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Dimensions;
using Accounting.Domain.Payroll;
using Accounting.Domain.Payroll.Calculators;
using Accounting.Infrastructure.Persistence.Context;
using Microsoft.EntityFrameworkCore;
using Xunit;
using Account = Accounting.Domain.MasterData.Accounts.Account;

namespace Accounting.Domain.Tests.Payroll;

public class PayrollEngineTests : IDisposable
{
    private readonly string _testDbPath;
    private readonly AccountingDbContext _context;
    private readonly IGlVoucherBridgeService _glBridge;

    private readonly DepartmentId _adminDeptId = new("DEPT_ADMIN");
    private readonly DepartmentId _salesDeptId = new("DEPT_SALES");

    public PayrollEngineTests()
    {
        _testDbPath = $"payroll_test_{Guid.NewGuid():N}.db";

        var options = new DbContextOptionsBuilder<AccountingDbContext>()
            .UseSqlite($"Data Source={_testDbPath}")
            .Options;

        _context = new AccountingDbContext(options);
        _context.Database.EnsureCreated();

        _glBridge = new GlVoucherBridgeService(_context);

        SeedMasterData();
    }

    private void SeedMasterData()
    {
        // 1. Chart of Accounts (Circular 99/2025/TT-BTC)
        var acc334 = new Account(new AccountId("334"), "Phải trả người lao động", AccountType.Liability, BalanceNature.CreditBalance);
        var acc3341 = new Account(new AccountId("3341"), "Phải trả công nhân viên", AccountType.Liability, BalanceNature.CreditBalance);
        var acc3382 = new Account(new AccountId("3382"), "Kinh phí công đoàn", AccountType.Liability, BalanceNature.CreditBalance);
        var acc3383 = new Account(new AccountId("3383"), "Bảo hiểm xã hội", AccountType.Liability, BalanceNature.CreditBalance);
        var acc3384 = new Account(new AccountId("3384"), "Bảo hiểm y tế", AccountType.Liability, BalanceNature.CreditBalance);
        var acc3386 = new Account(new AccountId("3386"), "Bảo hiểm thất nghiệp", AccountType.Liability, BalanceNature.CreditBalance);
        var acc3335 = new Account(new AccountId("3335"), "Thuế thu nhập cá nhân", AccountType.Liability, BalanceNature.CreditBalance);
        var acc6422 = new Account(new AccountId("6422"), "Chi phí nhân viên quản lý", AccountType.Expense, BalanceNature.DebitBalance);
        var acc6412 = new Account(new AccountId("6412"), "Chi phí nhân viên bán hàng", AccountType.Expense, BalanceNature.DebitBalance);
        var acc622 = new Account(new AccountId("622"), "Chi phí nhân công trực tiếp", AccountType.Expense, BalanceNature.DebitBalance);

        _context.MasterAccounts.AddRange(acc334, acc3341, acc3382, acc3383, acc3384, acc3386, acc3335, acc6422, acc6412, acc622);

        // 2. Departments
        var deptAdmin = new Department(_adminDeptId, "Phòng Hành chính - Quản trị");
        var deptSales = new Department(_salesDeptId, "Phòng Kinh doanh");
        _context.Departments.AddRange(deptAdmin, deptSales);

        // 3. Fiscal Periods
        _context.FiscalPeriods.AddRange(
            new FiscalPeriod(2026, 1),
            new FiscalPeriod(2026, 2),
            new FiscalPeriod(2026, 3));

        _context.SaveChanges();
    }

    public void Dispose()
    {
        _context.Dispose();
        if (File.Exists(_testDbPath))
        {
            try { File.Delete(_testDbPath); } catch { }
        }
    }

    [Fact]
    public void PersonalIncomeTaxCalculator_ProgressiveBrackets_CalculatesExactStatutoryAmount()
    {
        // Case: Gross = 40,000,000 VND, 1 dependent, insurance deductions = 4,200,000 VND (10.5% of 40M)
        // Deductions: 4,200,000 + 11,000,000 (self) + 4,400,000 (1 dependent) = 19,600,000 VND
        // Taxable Income: 40,000,000 - 19,600,000 = 20,400,000 VND
        // Tier 1 (5% on 5M) = 250,000
        // Tier 2 (10% on 5M) = 500,000
        // Tier 3 (15% on 8M) = 1,200,000
        // Tier 4 (20% on 2.4M) = 480,000
        // Total PIT = 2,430,000 VND

        var result = PersonalIncomeTaxCalculator.Calculate(
            grossSalary: 40_000_000m,
            nonTaxableAllowances: 0m,
            insuranceDeductions: 4_200_000m,
            dependentCount: 1,
            contractType: ContractType.Indefinite);

        Assert.Equal(40_000_000m, result.AssessableIncome);
        Assert.Equal(19_600_000m, result.TotalDeductions);
        Assert.Equal(20_400_000m, result.TaxableIncome);
        Assert.Equal(2_430_000m, result.PersonalIncomeTax);
    }

    [Theory]
    [InlineData(5_000_000, 250_000)]        // Tier 1 max
    [InlineData(10_000_000, 750_000)]       // Tier 2 max
    [InlineData(18_000_000, 1_950_000)]     // Tier 3 max
    [InlineData(32_000_000, 4_750_000)]     // Tier 4 max
    [InlineData(52_000_000, 9_750_000)]     // Tier 5 max
    [InlineData(80_000_000, 18_150_000)]    // Tier 6 max
    [InlineData(100_000_000, 25_150_000)]   // Tier 7 (18.15M + 20M * 35%)
    public void PersonalIncomeTaxCalculator_AllProgressiveBrackets_MatchesExactFormula(decimal taxableIncome, decimal expectedTax)
    {
        // Set gross salary such that Assessable - 11,000,000 = taxableIncome (0 dependents, 0 insurance)
        var grossSalary = taxableIncome + 11_000_000m;

        var result = PersonalIncomeTaxCalculator.Calculate(
            grossSalary: grossSalary,
            nonTaxableAllowances: 0m,
            insuranceDeductions: 0m,
            dependentCount: 0,
            contractType: ContractType.Indefinite);

        Assert.Equal(taxableIncome, result.TaxableIncome);
        Assert.Equal(expectedTax, result.PersonalIncomeTax);
    }

    [Fact]
    public void StatutoryInsuranceCalculator_HighSalary_CapsAtStatutoryCeilings()
    {
        // Gross = 150,000,000 VND
        // BHXH & BHYT capped at 20 * 2,340,000 = 46,800,000 VND
        // BHTN capped at 20 * 4,960,000 = 99,200,000 VND

        var result = StatutoryInsuranceCalculator.Calculate(
            insuranceSalary: 150_000_000m,
            contractType: ContractType.Indefinite);

        // Employee
        Assert.Equal(3_744_000m, result.SocialInsuranceEmployee); // 46.8M * 8%
        Assert.Equal(702_000m, result.HealthInsuranceEmployee);   // 46.8M * 1.5%
        Assert.Equal(992_000m, result.UnemploymentInsuranceEmployee); // 99.2M * 1%
        Assert.Equal(5_438_000m, result.TotalInsuranceEmployee);

        // Employer
        Assert.Equal(8_190_000m, result.SocialInsuranceEmployer); // 46.8M * 17.5%
        Assert.Equal(1_404_000m, result.HealthInsuranceEmployer); // 46.8M * 3.0%
        Assert.Equal(992_000m, result.UnemploymentInsuranceEmployer); // 99.2M * 1.0%
        Assert.Equal(936_000m, result.TradeUnionFeeEmployer);     // 46.8M * 2.0%
        Assert.Equal(11_522_000m, result.TotalEmployerContributions);
    }

    [Fact]
    public void PersonalIncomeTaxCalculator_FreelanceContract_AppliesFlat10Percent()
    {
        // 1. Freelance >= 2,000,000 VND
        var res1 = PersonalIncomeTaxCalculator.Calculate(
            grossSalary: 15_000_000m,
            nonTaxableAllowances: 0m,
            insuranceDeductions: 0m,
            dependentCount: 0,
            contractType: ContractType.Freelance);

        Assert.Equal(1_500_000m, res1.PersonalIncomeTax); // 10% flat

        // 2. Freelance < 2,000,000 VND (No withholding)
        var res2 = PersonalIncomeTaxCalculator.Calculate(
            grossSalary: 1_800_000m,
            nonTaxableAllowances: 0m,
            insuranceDeductions: 0m,
            dependentCount: 0,
            contractType: ContractType.Freelance);

        Assert.Equal(0m, res2.PersonalIncomeTax);
    }

    [Fact]
    public async Task PayrollRun_FullLifecycleAndGlPosting_BalancesAndInsertsEntries()
    {
        var regHandler = new RegisterEmployeeCommandHandler(_context);
        var calcHandler = new CalculatePayrollCommandHandler(_context);
        var postHandler = new ApproveAndPostPayrollCommandHandler(_context, _glBridge);

        // 1. Register 2 Employees
        // Emp 1: Admin (TK 6422), Base: 30,000,000 VND, 0 dependents
        var emp1Res = await regHandler.Handle(new RegisterEmployeeCommand(
            EmployeeCode: "NV001",
            FullName: "Nguyen Van Admin",
            DepartmentId: _adminDeptId.Value,
            IdentityCard: "001090123456",
            ContractType: ContractType.Indefinite,
            BaseSalary: 30_000_000m,
            InsuranceSalary: 30_000_000m,
            ExpenseAccountCode: "6422",
            DependentCount: 0), CancellationToken.None);

        Assert.True(emp1Res.IsSuccess);

        // Emp 2: Sales (TK 6412), Base: 20,000,000 VND, 1 dependent
        var emp2Res = await regHandler.Handle(new RegisterEmployeeCommand(
            EmployeeCode: "NV002",
            FullName: "Tran Thi Sales",
            DepartmentId: _salesDeptId.Value,
            IdentityCard: "001090654321",
            ContractType: ContractType.FixedTerm,
            BaseSalary: 20_000_000m,
            InsuranceSalary: 20_000_000m,
            ExpenseAccountCode: "6412",
            DependentCount: 1), CancellationToken.None);

        Assert.True(emp2Res.IsSuccess);

        // 2. Calculate Payroll for Period 2026-02
        var calcResult = await calcHandler.Handle(new CalculatePayrollCommand(
            Year: 2026,
            Month: 2,
            Title: "Bảng lương Tháng 2/2026"), CancellationToken.None);

        Assert.True(calcResult.IsSuccess);
        var runId = calcResult.Value!;

        var run = await _context.PayrollRuns
            .Include(r => r.Payslips)
            .FirstOrDefaultAsync(r => r.Id == runId);

        Assert.NotNull(run);
        Assert.Equal(PayrollRunStatus.Calculated, run.Status);
        Assert.Equal(2, run.Payslips.Count);
        Assert.Equal(50_000_000m, run.TotalGrossPay); // 30M + 20M

        // Emp 1: InsEmp = 30M * 10.5% = 3,150,000 VND. Taxable = 30M - 3.15M - 11M = 15.85M.
        // PIT = 5M*5% + 5M*10% + 5.85M*15% = 250k + 500k + 877.5k = 1,627,500 VND. Net = 30M - 3.15M - 1,627,500 = 25,222,500 VND.
        var p1 = run.Payslips.First(p => p.EmployeeId == emp1Res.Value!);
        Assert.Equal(30_000_000m, p1.GrossSalary);
        Assert.Equal(3_150_000m, p1.TotalInsuranceEmployee);
        Assert.Equal(1_627_500m, p1.PersonalIncomeTax);
        Assert.Equal(25_222_500m, p1.NetSalary);

        // Emp 2: InsEmp = 20M * 10.5% = 2,100,000 VND. Taxable = 20M - 2.1M - 11M - 4.4M = 2.5M.
        // PIT = 2.5M * 5% = 125,000 VND. Net = 20M - 2.1M - 125,000 = 17,775,000 VND.
        var p2 = run.Payslips.First(p => p.EmployeeId == emp2Res.Value!);
        Assert.Equal(20_000_000m, p2.GrossSalary);
        Assert.Equal(2_100_000m, p2.TotalInsuranceEmployee);
        Assert.Equal(125_000m, p2.PersonalIncomeTax);
        Assert.Equal(17_775_000m, p2.NetSalary);

        // 3. Approve and Post to GL
        var postResult = await postHandler.Handle(new ApproveAndPostPayrollCommand(runId), CancellationToken.None);
        Assert.True(postResult.IsSuccess);

        var refreshedRun = await _context.PayrollRuns.FindAsync(runId);
        Assert.NotNull(refreshedRun);
        Assert.Equal(PayrollRunStatus.PostedToGl, refreshedRun.Status);
        Assert.NotNull(refreshedRun.LinkedVoucherId);

        // Verify underlying Phase 3 GL Voucher is Posted and balanced
        var glVoucher = await _context.GlVouchers
            .Include(v => v.Lines)
            .FirstOrDefaultAsync(v => v.Id == refreshedRun.LinkedVoucherId);

        Assert.NotNull(glVoucher);
        Assert.Equal(VoucherStatus.Posted, glVoucher.Status);
        Assert.Equal(glVoucher.TotalDebitBase, glVoucher.TotalCreditBase);

        // 4. Verify Idempotency Guard (Cannot re-post)
        await Assert.ThrowsAsync<PayrollRunAlreadyPostedException>(() =>
            postHandler.Handle(new ApproveAndPostPayrollCommand(runId), CancellationToken.None));
    }

    [Fact]
    public async Task Live_MariaDb_PayrollPosting_Succeeds()
    {
        var serverConnStr = "Server=localhost;Port=3306;User=dev;Password=123456;TreatTinyAsBoolean=true;CharSet=utf8mb4;";
        var connStr = "Server=localhost;Port=3306;Database=accounting_payroll_test;User=dev;Password=123456;TreatTinyAsBoolean=true;CharSet=utf8mb4;";
        var serverVersion = new MariaDbServerVersion(new Version(12, 3, 0));

        // Test connectivity first & create isolated database
        try
        {
            using var pingConn = new MySqlConnector.MySqlConnection(serverConnStr);
            await pingConn.OpenAsync();
            using var cmd = pingConn.CreateCommand();
            cmd.CommandText = "CREATE DATABASE IF NOT EXISTS accounting_payroll_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;";
            await cmd.ExecuteNonQueryAsync();
        }
        catch
        {
            return;
        }

        var options = new DbContextOptionsBuilder<AccountingDbContext>()
            .UseMySql(connStr, serverVersion)
            .Options;

        using var efContext = new AccountingDbContext(options);
        await efContext.Database.EnsureCreatedAsync();

        if (!await efContext.MasterAccounts.AnyAsync())
        {
            await Accounting.Infrastructure.Persistence.Seeding.DbInitializer.SeedAsync(efContext);
        }

        var glBridge = new GlVoucherBridgeService(efContext);

        // Register employee on Live MariaDB
        var regHandler = new RegisterEmployeeCommandHandler(efContext);
        var empCode = $"NV_{Guid.NewGuid():N}"[..10].ToUpperInvariant();
        var empResult = await regHandler.Handle(new RegisterEmployeeCommand(
            EmployeeCode: empCode,
            FullName: "MariaDB Test Employee",
            DepartmentId: "DEPT_ADMIN",
            IdentityCard: "001099888777",
            ContractType: ContractType.Indefinite,
            BaseSalary: 25_000_000m,
            InsuranceSalary: 25_000_000m,
            ExpenseAccountCode: "6422"), CancellationToken.None);

        Assert.True(empResult.IsSuccess);

        // Calculate and Post on Live MariaDB
        var calcHandler = new CalculatePayrollCommandHandler(efContext);
        var calcResult = await calcHandler.Handle(new CalculatePayrollCommand(
            Year: 2026,
            Month: 3,
            Title: $"Bảng lương MariaDB {Guid.NewGuid():N}"[..25]), CancellationToken.None);

        Assert.True(calcResult.IsSuccess);

        var postHandler = new ApproveAndPostPayrollCommandHandler(efContext, glBridge);
        var postResult = await postHandler.Handle(new ApproveAndPostPayrollCommand(calcResult.Value!), CancellationToken.None);

        Assert.True(postResult.IsSuccess);
        Assert.True(postResult.Value!.TotalGrossPay >= 25_000_000m);
    }
}
