using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Common;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.Payroll;
using Accounting.Domain.Payroll.Calculators;
using MediatR;
using Microsoft.EntityFrameworkCore;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;
using Voucher = Accounting.Domain.Ledger.Voucher;
using VoucherType = Accounting.Domain.Ledger.VoucherType;

namespace Accounting.Application.Features.Payroll;

public record RegisterEmployeeCommand(
    string EmployeeCode,
    string FullName,
    string DepartmentId,
    string IdentityCard,
    ContractType ContractType,
    decimal BaseSalary,
    decimal InsuranceSalary,
    string ExpenseAccountCode = "6422",
    decimal Allowances = 0m,
    decimal NonTaxableAllowances = 0m,
    int DependentCount = 0,
    string? TaxCode = null,
    string? CostCenterId = null) : IRequest<Result<EmployeeId>>;

public class RegisterEmployeeCommandHandler(
    IAccountingDbContext context) : IRequestHandler<RegisterEmployeeCommand, Result<EmployeeId>>
{
    public async Task<Result<EmployeeId>> Handle(RegisterEmployeeCommand request, CancellationToken cancellationToken)
    {
        var existing = await context.PayrollEmployees
            .AnyAsync(e => e.EmployeeCode == request.EmployeeCode.Trim().ToUpperInvariant(), cancellationToken);

        if (existing)
            return Result<EmployeeId>.Failure($"Employee code '{request.EmployeeCode}' already exists.");

        var employeeId = EmployeeId.New();
        var employee = new PayrollEmployee(
            employeeId,
            request.EmployeeCode,
            request.FullName,
            new DepartmentId(request.DepartmentId),
            request.IdentityCard,
            request.ContractType,
            request.BaseSalary,
            request.InsuranceSalary,
            new AccountId(request.ExpenseAccountCode),
            request.Allowances,
            request.NonTaxableAllowances,
            request.DependentCount,
            request.TaxCode,
            !string.IsNullOrWhiteSpace(request.CostCenterId) ? new CostCenterId(request.CostCenterId) : null);

        context.AddEntity(employee);
        await context.SaveChangesAsync(cancellationToken);

        return Result<EmployeeId>.Success(employee.Id);
    }
}

public record EmployeeWorkedDaysDto(
    Guid EmployeeId,
    decimal ActualWorkedDays);

public record CalculatePayrollCommand(
    int Year,
    int Month,
    string Title,
    decimal StandardWorkingDays = 22.0m,
    IReadOnlyList<EmployeeWorkedDaysDto>? EmployeeWorkedDays = null) : IRequest<Result<PayrollRunId>>;

public class CalculatePayrollCommandHandler(
    IAccountingDbContext context) : IRequestHandler<CalculatePayrollCommand, Result<PayrollRunId>>
{
    public async Task<Result<PayrollRunId>> Handle(CalculatePayrollCommand request, CancellationToken cancellationToken)
    {
        var activeEmployees = await context.PayrollEmployees
            .Where(e => e.IsActive)
            .ToListAsync(cancellationToken);

        if (activeEmployees.Count == 0)
            return Result<PayrollRunId>.Failure("No active employees found to calculate payroll.");

        var workedDaysMap = request.EmployeeWorkedDays?
            .ToDictionary(x => new EmployeeId(x.EmployeeId), x => x.ActualWorkedDays)
            ?? new Dictionary<EmployeeId, decimal>();

        var fiscalPeriodId = FiscalPeriodId.FromYearMonth(request.Year, request.Month);
        var runId = PayrollRunId.New();
        var payrollRun = new PayrollRun(runId, fiscalPeriodId, request.Title);

        foreach (var emp in activeEmployees)
        {
            var actualWorked = workedDaysMap.TryGetValue(emp.Id, out var days)
                ? days
                : request.StandardWorkingDays;

            var workRatio = request.StandardWorkingDays > 0m
                ? Math.Min(1.0m, Math.Max(0m, actualWorked / request.StandardWorkingDays))
                : 1.0m;

            var proratedBase = decimal.Round(emp.BaseSalary * workRatio, 2, MidpointRounding.AwayFromZero);
            var grossSalary = proratedBase + emp.Allowances + emp.NonTaxableAllowances;

            // 1. Calculate Statutory Insurance
            var insResult = StatutoryInsuranceCalculator.Calculate(emp.InsuranceSalary, emp.ContractType);

            // 2. Calculate PIT
            var pitResult = PersonalIncomeTaxCalculator.Calculate(
                grossSalary,
                emp.NonTaxableAllowances,
                insResult.TotalInsuranceEmployee,
                emp.DependentCount,
                emp.ContractType);

            var payslip = new Payslip(
                PayslipId.New(),
                runId,
                emp.Id,
                request.StandardWorkingDays,
                actualWorked,
                proratedBase,
                emp.Allowances,
                emp.NonTaxableAllowances,
                grossSalary,
                insResult.SocialInsuranceEmployee,
                insResult.HealthInsuranceEmployee,
                insResult.UnemploymentInsuranceEmployee,
                pitResult.TaxableIncome,
                pitResult.PersonalIncomeTax,
                insResult.SocialInsuranceEmployer,
                insResult.HealthInsuranceEmployer,
                insResult.UnemploymentInsuranceEmployer,
                insResult.TradeUnionFeeEmployer,
                emp.ExpenseAccountId,
                emp.CostCenterId);

            payrollRun.AddPayslip(payslip);
        }

        context.AddEntity(payrollRun);
        await context.SaveChangesAsync(cancellationToken);

        return Result<PayrollRunId>.Success(payrollRun.Id);
    }
}

public record PayrollPostResultDto(
    Guid PayrollRunId,
    decimal TotalGrossPay,
    decimal TotalNetPay,
    decimal TotalEmployerCost,
    Guid GlVoucherId);

public record ApproveAndPostPayrollCommand(
    PayrollRunId PayrollRunId,
    string SalaryPayableAccountCode = "3341",
    string SocialInsuranceAccountCode = "3383",
    string HealthInsuranceAccountCode = "3384",
    string UnemploymentInsuranceAccountCode = "3386",
    string TradeUnionAccountCode = "3382",
    string PersonalIncomeTaxAccountCode = "3335",
    string PostedBy = "payroll_accountant") : IRequest<Result<PayrollPostResultDto>>;

public class ApproveAndPostPayrollCommandHandler(
    IAccountingDbContext context,
    IGlVoucherBridgeService glBridge) : IRequestHandler<ApproveAndPostPayrollCommand, Result<PayrollPostResultDto>>
{
    public async Task<Result<PayrollPostResultDto>> Handle(ApproveAndPostPayrollCommand request, CancellationToken cancellationToken)
    {
        var payrollRun = await context.PayrollRuns
            .Include(r => r.Payslips)
            .FirstOrDefaultAsync(r => r.Id == request.PayrollRunId, cancellationToken);

        if (payrollRun == null)
            return Result<PayrollPostResultDto>.Failure($"Payroll run '{request.PayrollRunId.Value}' not found.");

        if (payrollRun.Status == PayrollRunStatus.PostedToGl)
            throw new PayrollRunAlreadyPostedException(payrollRun.Title, "payroll run is already posted to general ledger");

        payrollRun.Approve();

        var periodYear = payrollRun.FiscalPeriodId.Year;
        var periodMonth = payrollRun.FiscalPeriodId.PeriodNumber;
        var periodEndDate = new DateOnly(periodYear, periodMonth, 1).AddMonths(1).AddDays(-1);

        var voucherNum = $"BL-{periodYear}{periodMonth:D2}-{Guid.NewGuid():N}"[..18].ToUpperInvariant();
        var glVoucher = new Voucher(
            VoucherId.New(),
            voucherNum,
            VoucherType.GeneralJournal,
            periodEndDate,
            periodEndDate,
            $"Payroll and statutory contributions for {payrollRun.Title}",
            new CurrencyCode("VND"),
            1.0m);

        // 1. Gross Salary Expense (Debit 6422/6412/622 / Credit 334)
        var grossByAccount = payrollRun.Payslips
            .GroupBy(p => new { p.ExpenseAccountId, p.CostCenterId })
            .ToList();

        foreach (var g in grossByAccount)
        {
            var sumGross = g.Sum(x => x.GrossSalary);
            if (sumGross > 0m)
            {
                glVoucher.AddLine(
                    g.Key.ExpenseAccountId,
                    LedgerEntryType.Debit,
                    sumGross,
                    $"Gross salary expense for period {periodMonth:D2}/{periodYear}",
                    costCenterId: g.Key.CostCenterId);
            }
        }

        glVoucher.AddLine(
            new AccountId(request.SalaryPayableAccountCode),
            LedgerEntryType.Credit,
            payrollRun.TotalGrossPay,
            $"Total gross salary payable for {payrollRun.Title}");

        // 2. Employee Deductions (Debit 334 / Credit 3383, 3384, 3386, 3335)
        if (payrollRun.TotalEmployeeDeductions > 0m)
        {
            glVoucher.AddLine(
                new AccountId(request.SalaryPayableAccountCode),
                LedgerEntryType.Debit,
                payrollRun.TotalEmployeeDeductions,
                "Employee insurance and PIT deductions from salary");

            var sumBhxhEmp = payrollRun.Payslips.Sum(p => p.SocialInsuranceEmployee);
            if (sumBhxhEmp > 0m)
            {
                glVoucher.AddLine(
                    new AccountId(request.SocialInsuranceAccountCode),
                    LedgerEntryType.Credit,
                    sumBhxhEmp,
                    "Employee BHXH deduction (8%)");
            }

            var sumBhytEmp = payrollRun.Payslips.Sum(p => p.HealthInsuranceEmployee);
            if (sumBhytEmp > 0m)
            {
                glVoucher.AddLine(
                    new AccountId(request.HealthInsuranceAccountCode),
                    LedgerEntryType.Credit,
                    sumBhytEmp,
                    "Employee BHYT deduction (1.5%)");
            }

            var sumBhtnEmp = payrollRun.Payslips.Sum(p => p.UnemploymentInsuranceEmployee);
            if (sumBhtnEmp > 0m)
            {
                glVoucher.AddLine(
                    new AccountId(request.UnemploymentInsuranceAccountCode),
                    LedgerEntryType.Credit,
                    sumBhtnEmp,
                    "Employee BHTN deduction (1.0%)");
            }

            var sumPit = payrollRun.Payslips.Sum(p => p.PersonalIncomeTax);
            if (sumPit > 0m)
            {
                glVoucher.AddLine(
                    new AccountId(request.PersonalIncomeTaxAccountCode),
                    LedgerEntryType.Credit,
                    sumPit,
                    "Employee PIT withholding");
            }
        }

        // 3. Employer Contributions (Debit 6422/6412/622 / Credit 3383, 3384, 3386, 3382)
        if (payrollRun.TotalEmployerContributions > 0m)
        {
            var employerByAccount = payrollRun.Payslips
                .GroupBy(p => new { p.ExpenseAccountId, p.CostCenterId })
                .ToList();

            foreach (var g in employerByAccount)
            {
                var sumEmployer = g.Sum(x => x.TotalEmployerContributions);
                if (sumEmployer > 0m)
                {
                    glVoucher.AddLine(
                        g.Key.ExpenseAccountId,
                        LedgerEntryType.Debit,
                        sumEmployer,
                        $"Employer statutory insurance and union contribution for {periodMonth:D2}/{periodYear}",
                        costCenterId: g.Key.CostCenterId);
                }
            }

            var sumBhxhCo = payrollRun.Payslips.Sum(p => p.SocialInsuranceEmployer);
            if (sumBhxhCo > 0m)
            {
                glVoucher.AddLine(
                    new AccountId(request.SocialInsuranceAccountCode),
                    LedgerEntryType.Credit,
                    sumBhxhCo,
                    "Employer BHXH contribution (17.5%)");
            }

            var sumBhytCo = payrollRun.Payslips.Sum(p => p.HealthInsuranceEmployer);
            if (sumBhytCo > 0m)
            {
                glVoucher.AddLine(
                    new AccountId(request.HealthInsuranceAccountCode),
                    LedgerEntryType.Credit,
                    sumBhytCo,
                    "Employer BHYT contribution (3.0%)");
            }

            var sumBhtnCo = payrollRun.Payslips.Sum(p => p.UnemploymentInsuranceEmployer);
            if (sumBhtnCo > 0m)
            {
                glVoucher.AddLine(
                    new AccountId(request.UnemploymentInsuranceAccountCode),
                    LedgerEntryType.Credit,
                    sumBhtnCo,
                    "Employer BHTN contribution (1.0%)");
            }

            var sumUnionCo = payrollRun.Payslips.Sum(p => p.TradeUnionFeeEmployer);
            if (sumUnionCo > 0m)
            {
                glVoucher.AddLine(
                    new AccountId(request.TradeUnionAccountCode),
                    LedgerEntryType.Credit,
                    sumUnionCo,
                    "Employer Trade Union fee (2.0%)");
            }
        }

        var glResult = await glBridge.PostOperationalVoucherAsync(glVoucher, request.PostedBy, cancellationToken);
        if (!glResult.IsSuccess)
            return Result<PayrollPostResultDto>.Failure(glResult.ErrorMessage ?? "Failed to post payroll GL voucher.");

        payrollRun.LinkGeneralLedgerVoucher(glVoucher.Id);
        await context.SaveChangesAsync(cancellationToken);

        return Result<PayrollPostResultDto>.Success(new PayrollPostResultDto(
            payrollRun.Id.Value,
            payrollRun.TotalGrossPay,
            payrollRun.TotalNetPay,
            payrollRun.TotalEmployerCost,
            glVoucher.Id.Value));
    }
}
