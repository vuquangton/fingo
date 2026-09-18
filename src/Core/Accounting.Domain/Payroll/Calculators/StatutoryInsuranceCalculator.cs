namespace Accounting.Domain.Payroll.Calculators;

public record InsuranceCalculationResult(
    decimal SocialInsuranceEmployee,
    decimal HealthInsuranceEmployee,
    decimal UnemploymentInsuranceEmployee,
    decimal TotalInsuranceEmployee,
    decimal SocialInsuranceEmployer,
    decimal HealthInsuranceEmployer,
    decimal UnemploymentInsuranceEmployer,
    decimal TradeUnionFeeEmployer,
    decimal TotalEmployerContributions);

public static class StatutoryInsuranceCalculator
{
    // Statutory Constants (Decree 73/2024/ND-CP & Decree 74/2024/ND-CP)
    public const decimal StatutoryBaseSalary = 2_340_000m;
    public const decimal RegionalMinimumWageRegion1 = 4_960_000m;

    public const decimal BhxhBhytCap = 20m * StatutoryBaseSalary; // 46,800,000 VND
    public const decimal BhtnCap = 20m * RegionalMinimumWageRegion1; // 99,200,000 VND

    // Rates
    public const decimal RateBhxhEmployee = 0.08m;
    public const decimal RateBhytEmployee = 0.015m;
    public const decimal RateBhtnEmployee = 0.01m;

    public const decimal RateBhxhEmployer = 0.175m;
    public const decimal RateBhytEmployer = 0.03m;
    public const decimal RateBhtnEmployer = 0.01m;
    public const decimal RateTradeUnionEmployer = 0.02m;

    public static InsuranceCalculationResult Calculate(
        decimal insuranceSalary,
        ContractType contractType,
        decimal? customBhxhCap = null,
        decimal? customBhtnCap = null)
    {
        // Insurance is only mandatory for Indefinite and FixedTerm contracts (> 1 month)
        if (contractType == ContractType.Freelance || contractType == ContractType.Probation || insuranceSalary <= 0m)
        {
            return new InsuranceCalculationResult(0m, 0m, 0m, 0m, 0m, 0m, 0m, 0m, 0m);
        }

        var bhxhCap = customBhxhCap ?? BhxhBhytCap;
        var bhtnCap = customBhtnCap ?? BhtnCap;

        var cappedBhxhSalary = Math.Min(insuranceSalary, bhxhCap);
        var cappedBhtnSalary = Math.Min(insuranceSalary, bhtnCap);

        // Employee
        var bhxhEmp = decimal.Round(cappedBhxhSalary * RateBhxhEmployee, 0, MidpointRounding.AwayFromZero);
        var bhytEmp = decimal.Round(cappedBhxhSalary * RateBhytEmployee, 0, MidpointRounding.AwayFromZero);
        var bhtnEmp = decimal.Round(cappedBhtnSalary * RateBhtnEmployee, 0, MidpointRounding.AwayFromZero);
        var totalEmp = bhxhEmp + bhytEmp + bhtnEmp;

        // Employer
        var bhxhCo = decimal.Round(cappedBhxhSalary * RateBhxhEmployer, 0, MidpointRounding.AwayFromZero);
        var bhytCo = decimal.Round(cappedBhxhSalary * RateBhytEmployer, 0, MidpointRounding.AwayFromZero);
        var bhtnCo = decimal.Round(cappedBhtnSalary * RateBhtnEmployer, 0, MidpointRounding.AwayFromZero);
        var tradeUnionCo = decimal.Round(cappedBhxhSalary * RateTradeUnionEmployer, 0, MidpointRounding.AwayFromZero);
        var totalCo = bhxhCo + bhytCo + bhtnCo + tradeUnionCo;

        return new InsuranceCalculationResult(
            bhxhEmp,
            bhytEmp,
            bhtnEmp,
            totalEmp,
            bhxhCo,
            bhytCo,
            bhtnCo,
            tradeUnionCo,
            totalCo);
    }
}
