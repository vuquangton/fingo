using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.Payroll;

public class Payslip : Entity<PayslipId>
{
    public PayrollRunId PayrollRunId { get; private set; }
    public EmployeeId EmployeeId { get; private set; }
    public decimal StandardWorkingDays { get; private set; }
    public decimal ActualWorkedDays { get; private set; }

    public decimal BaseSalary { get; private set; }
    public decimal Allowances { get; private set; }
    public decimal NonTaxableAllowances { get; private set; }
    public decimal GrossSalary { get; private set; }

    // Employee Deductions
    public decimal SocialInsuranceEmployee { get; private set; }
    public decimal HealthInsuranceEmployee { get; private set; }
    public decimal UnemploymentInsuranceEmployee { get; private set; }
    public decimal TotalInsuranceEmployee => SocialInsuranceEmployee + HealthInsuranceEmployee + UnemploymentInsuranceEmployee;

    public decimal TaxableIncome { get; private set; }
    public decimal PersonalIncomeTax { get; private set; }
    public decimal TotalEmployeeDeductions => TotalInsuranceEmployee + PersonalIncomeTax;
    public decimal NetSalary => GrossSalary - TotalEmployeeDeductions;

    // Employer Contributions
    public decimal SocialInsuranceEmployer { get; private set; }
    public decimal HealthInsuranceEmployer { get; private set; }
    public decimal UnemploymentInsuranceEmployer { get; private set; }
    public decimal TradeUnionFeeEmployer { get; private set; }
    public decimal TotalEmployerContributions => SocialInsuranceEmployer + HealthInsuranceEmployer + UnemploymentInsuranceEmployer + TradeUnionFeeEmployer;
    public decimal TotalEmployerCost => GrossSalary + TotalEmployerContributions;

    public AccountId ExpenseAccountId { get; private set; }
    public CostCenterId? CostCenterId { get; private set; }

    private Payslip() { }

    public Payslip(
        PayslipId id,
        PayrollRunId payrollRunId,
        EmployeeId employeeId,
        decimal standardWorkingDays,
        decimal actualWorkedDays,
        decimal baseSalary,
        decimal allowances,
        decimal nonTaxableAllowances,
        decimal grossSalary,
        decimal socialInsuranceEmployee,
        decimal healthInsuranceEmployee,
        decimal unemploymentInsuranceEmployee,
        decimal taxableIncome,
        decimal personalIncomeTax,
        decimal socialInsuranceEmployer,
        decimal healthInsuranceEmployer,
        decimal unemploymentInsuranceEmployer,
        decimal tradeUnionFeeEmployer,
        AccountId expenseAccountId,
        CostCenterId? costCenterId = null)
    {
        if (id == PayslipId.Empty)
            throw new ArgumentException("Payslip ID cannot be empty.", nameof(id));

        Id = id;
        PayrollRunId = payrollRunId;
        EmployeeId = employeeId;
        StandardWorkingDays = standardWorkingDays;
        ActualWorkedDays = actualWorkedDays;
        BaseSalary = baseSalary;
        Allowances = allowances;
        NonTaxableAllowances = nonTaxableAllowances;
        GrossSalary = grossSalary;
        SocialInsuranceEmployee = socialInsuranceEmployee;
        HealthInsuranceEmployee = healthInsuranceEmployee;
        UnemploymentInsuranceEmployee = unemploymentInsuranceEmployee;
        TaxableIncome = taxableIncome;
        PersonalIncomeTax = personalIncomeTax;
        SocialInsuranceEmployer = socialInsuranceEmployer;
        HealthInsuranceEmployer = healthInsuranceEmployer;
        UnemploymentInsuranceEmployer = unemploymentInsuranceEmployer;
        TradeUnionFeeEmployer = tradeUnionFeeEmployer;
        ExpenseAccountId = expenseAccountId;
        CostCenterId = costCenterId;
    }
}
