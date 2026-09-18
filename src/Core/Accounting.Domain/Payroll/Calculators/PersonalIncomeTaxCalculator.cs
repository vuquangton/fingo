namespace Accounting.Domain.Payroll.Calculators;

public record PitCalculationResult(
    decimal AssessableIncome,
    decimal TotalDeductions,
    decimal TaxableIncome,
    decimal PersonalIncomeTax);

public static class PersonalIncomeTaxCalculator
{
    public const decimal PersonalDeduction = 11_000_000m;
    public const decimal DependentDeductionPerPerson = 4_400_000m;
    public const decimal FreelanceThreshold = 2_000_000m;
    public const decimal FreelanceTaxRate = 0.10m;

    public static PitCalculationResult Calculate(
        decimal grossSalary,
        decimal nonTaxableAllowances,
        decimal insuranceDeductions,
        int dependentCount,
        ContractType contractType)
    {
        if (grossSalary <= 0m)
            return new PitCalculationResult(0m, 0m, 0m, 0m);

        // 1. Non-contracted / Freelance: 10% flat tax on gross >= 2,000,000 VND
        if (contractType == ContractType.Freelance)
        {
            if (grossSalary >= FreelanceThreshold)
            {
                var flatTax = decimal.Round(grossSalary * FreelanceTaxRate, 0, MidpointRounding.AwayFromZero);
                return new PitCalculationResult(grossSalary, 0m, grossSalary, flatTax);
            }

            return new PitCalculationResult(grossSalary, 0m, 0m, 0m);
        }

        // 2. Standard labor contract (Progressive 7 brackets)
        var assessableIncome = Math.Max(0m, grossSalary - nonTaxableAllowances);
        var totalDeductions = insuranceDeductions + PersonalDeduction + (Math.Max(0, dependentCount) * DependentDeductionPerPerson);
        var taxableIncome = Math.Max(0m, assessableIncome - totalDeductions);

        if (taxableIncome <= 0m)
            return new PitCalculationResult(assessableIncome, totalDeductions, 0m, 0m);

        decimal tax = taxableIncome switch
        {
            <= 5_000_000m => taxableIncome * 0.05m,
            <= 10_000_000m => (taxableIncome * 0.10m) - 250_000m,
            <= 18_000_000m => (taxableIncome * 0.15m) - 750_000m,
            <= 32_000_000m => (taxableIncome * 0.20m) - 1_650_000m,
            <= 52_000_000m => (taxableIncome * 0.25m) - 3_250_000m,
            <= 80_000_000m => (taxableIncome * 0.30m) - 5_850_000m,
            _ => (taxableIncome * 0.35m) - 9_850_000m
        };

        var finalTax = decimal.Round(Math.Max(0m, tax), 0, MidpointRounding.AwayFromZero);
        return new PitCalculationResult(assessableIncome, totalDeductions, taxableIncome, finalTax);
    }
}
