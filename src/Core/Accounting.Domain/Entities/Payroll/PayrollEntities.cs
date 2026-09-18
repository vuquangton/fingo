using Accounting.Domain.Common;

namespace Accounting.Domain.Entities.Payroll;

public class Employee : AggregateRoot<Guid>
{
    public string Code { get; private set; } = string.Empty;
    public string FullName { get; private set; } = string.Empty;
    public string CitizenId { get; private set; } = string.Empty; // CCCD
    public string? TaxIdNumber { get; private set; }              // Mã số thuế cá nhân
    public string Department { get; private set; } = string.Empty;
    public string Position { get; private set; } = string.Empty;
    public decimal BaseSalary { get; private set; }
    public decimal InsuranceSalary { get; private set; }
    public decimal Allowances { get; private set; }
    public int DependentsCount { get; private set; }
    public bool IsActive { get; private set; } = true;

    private Employee() { }

    public Employee(
        string code,
        string fullName,
        string citizenId,
        string department,
        string position,
        decimal baseSalary,
        decimal insuranceSalary,
        decimal allowances = 0m,
        int dependentsCount = 0,
        string? taxIdNumber = null)
    {
        Id = Guid.NewGuid();
        Code = code.Trim().ToUpperInvariant();
        FullName = fullName.Trim();
        CitizenId = citizenId.Trim();
        Department = department.Trim();
        Position = position.Trim();
        BaseSalary = baseSalary;
        InsuranceSalary = insuranceSalary;
        Allowances = allowances;
        DependentsCount = dependentsCount;
        TaxIdNumber = taxIdNumber?.Trim();
    }
}

public class SalarySlip : AggregateRoot<Guid>
{
    public int Year { get; private set; }
    public int Month { get; private set; }
    public Guid EmployeeId { get; private set; }
    public Employee? Employee { get; private set; }

    public decimal BaseSalary { get; private set; }
    public decimal Allowances { get; private set; }
    public decimal GrossSalary => BaseSalary + Allowances;

    // Employee Deductions (Trích trừ lương người lao động)
    public decimal SocialInsuranceEmployee { get; private set; }       // BHXH 8%
    public decimal HealthInsuranceEmployee { get; private set; }       // BHYT 1.5%
    public decimal UnemploymentInsuranceEmployee { get; private set; } // BHTN 1%
    public decimal TotalInsuranceEmployee => SocialInsuranceEmployee + HealthInsuranceEmployee + UnemploymentInsuranceEmployee;

    public decimal PersonalIncomeTax { get; private set; }            // Thuế TNCN
    public decimal TotalEmployeeDeductions => TotalInsuranceEmployee + PersonalIncomeTax;
    public decimal NetSalary => GrossSalary - TotalEmployeeDeductions;// Lương thực lĩnh

    // Employer Contributions (Doanh nghiệp chịu tính vào chi phí 641/642/622)
    public decimal SocialInsuranceEmployer { get; private set; }       // BHXH 17.5%
    public decimal HealthInsuranceEmployer { get; private set; }       // BHYT 3.0%
    public decimal UnemploymentInsuranceEmployer { get; private set; } // BHTN 1.0%
    public decimal TradeUnionFeeEmployer { get; private set; }         // Kinh phí công đoàn 2.0%
    public decimal TotalEmployerCost => GrossSalary + SocialInsuranceEmployer + HealthInsuranceEmployer + UnemploymentInsuranceEmployer + TradeUnionFeeEmployer;

    private SalarySlip() { }

    public SalarySlip(int year, int month, Employee employee)
    {
        Id = Guid.NewGuid();
        Year = year;
        Month = month;
        EmployeeId = employee.Id;
        Employee = employee;

        BaseSalary = employee.BaseSalary;
        Allowances = employee.Allowances;
        var insSalary = employee.InsuranceSalary;

        // Statutory rates (Luật BHXH, BHYT hiện hành)
        SocialInsuranceEmployee = decimal.Round(insSalary * 0.08m, 0);
        HealthInsuranceEmployee = decimal.Round(insSalary * 0.015m, 0);
        UnemploymentInsuranceEmployee = decimal.Round(insSalary * 0.01m, 0);

        SocialInsuranceEmployer = decimal.Round(insSalary * 0.175m, 0);
        HealthInsuranceEmployer = decimal.Round(insSalary * 0.03m, 0);
        UnemploymentInsuranceEmployer = decimal.Round(insSalary * 0.01m, 0);
        TradeUnionFeeEmployer = decimal.Round(insSalary * 0.02m, 0);

        // PIT Calculation
        PersonalIncomeTax = CalculateVietnamesePit(GrossSalary, TotalInsuranceEmployee, employee.DependentsCount);
    }

    public static decimal CalculateVietnamesePit(decimal grossIncome, decimal insuranceDeductions, int dependentsCount)
    {
        const decimal personalDeduction = 11_000_000m; // 11M VND
        const decimal dependentDeductionRate = 4_400_000m; // 4.4M VND per dependent

        var totalDeductions = insuranceDeductions + personalDeduction + (dependentsCount * dependentDeductionRate);
        var taxableIncome = grossIncome - totalDeductions;

        if (taxableIncome <= 0) return 0m;

        // Progressive brackets (Biểu thuế lũy tiến từng phần)
        decimal tax = 0m;
        if (taxableIncome <= 5_000_000m)
        {
            tax = taxableIncome * 0.05m;
        }
        else if (taxableIncome <= 10_000_000m)
        {
            tax = 5_000_000m * 0.05m + (taxableIncome - 5_000_000m) * 0.10m;
        }
        else if (taxableIncome <= 18_000_000m)
        {
            tax = 5_000_000m * 0.05m + 5_000_000m * 0.10m + (taxableIncome - 10_000_000m) * 0.15m;
        }
        else if (taxableIncome <= 32_000_000m)
        {
            tax = 5_000_000m * 0.05m + 5_000_000m * 0.10m + 8_000_000m * 0.15m + (taxableIncome - 18_000_000m) * 0.20m;
        }
        else if (taxableIncome <= 52_000_000m)
        {
            tax = 5_000_000m * 0.05m + 5_000_000m * 0.10m + 8_000_000m * 0.15m + 14_000_000m * 0.20m + (taxableIncome - 32_000_000m) * 0.25m;
        }
        else if (taxableIncome <= 80_000_000m)
        {
            tax = 5_000_000m * 0.05m + 5_000_000m * 0.10m + 8_000_000m * 0.15m + 14_000_000m * 0.20m + 20_000_000m * 0.25m + (taxableIncome - 52_000_000m) * 0.30m;
        }
        else
        {
            tax = 5_000_000m * 0.05m + 5_000_000m * 0.10m + 8_000_000m * 0.15m + 14_000_000m * 0.20m + 20_000_000m * 0.25m + 28_000_000m * 0.30m + (taxableIncome - 80_000_000m) * 0.35m;
        }

        return decimal.Round(tax, 0);
    }
}
