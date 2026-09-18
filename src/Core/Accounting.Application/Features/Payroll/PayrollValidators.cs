using FluentValidation;

namespace Accounting.Application.Features.Payroll;

public class RegisterEmployeeCommandValidator : AbstractValidator<RegisterEmployeeCommand>
{
    public RegisterEmployeeCommandValidator()
    {
        RuleFor(x => x.EmployeeCode)
            .NotEmpty().WithMessage("Employee code is required.")
            .MaximumLength(50).WithMessage("Employee code cannot exceed 50 characters.");

        RuleFor(x => x.FullName)
            .NotEmpty().WithMessage("Full name is required.")
            .MaximumLength(250).WithMessage("Full name cannot exceed 250 characters.");

        RuleFor(x => x.DepartmentId)
            .NotEmpty().WithMessage("Department ID is required.");

        RuleFor(x => x.IdentityCard)
            .NotEmpty().WithMessage("Identity card (CCCD) is required.")
            .MaximumLength(50).WithMessage("Identity card cannot exceed 50 characters.");

        RuleFor(x => x.BaseSalary)
            .GreaterThanOrEqualTo(0).WithMessage("Base salary cannot be negative.");

        RuleFor(x => x.InsuranceSalary)
            .GreaterThanOrEqualTo(0).WithMessage("Insurance salary cannot be negative.");

        RuleFor(x => x.DependentCount)
            .GreaterThanOrEqualTo(0).WithMessage("Dependent count cannot be negative.");

        RuleFor(x => x.ExpenseAccountCode)
            .NotEmpty().WithMessage("Expense account code is required.");
    }
}

public class CalculatePayrollCommandValidator : AbstractValidator<CalculatePayrollCommand>
{
    public CalculatePayrollCommandValidator()
    {
        RuleFor(x => x.Year).InclusiveBetween(2000, 2100).WithMessage("Invalid fiscal year.");
        RuleFor(x => x.Month).InclusiveBetween(1, 12).WithMessage("Month must be between 1 and 12.");
        RuleFor(x => x.Title).NotEmpty().WithMessage("Payroll title is required.").MaximumLength(250);
        RuleFor(x => x.StandardWorkingDays).GreaterThan(0).WithMessage("Standard working days must be strictly positive.");
    }
}

public class ApproveAndPostPayrollCommandValidator : AbstractValidator<ApproveAndPostPayrollCommand>
{
    public ApproveAndPostPayrollCommandValidator()
    {
        RuleFor(x => x.SalaryPayableAccountCode).NotEmpty().WithMessage("Salary payable account code is required.");
        RuleFor(x => x.SocialInsuranceAccountCode).NotEmpty().WithMessage("Social insurance account code is required.");
        RuleFor(x => x.HealthInsuranceAccountCode).NotEmpty().WithMessage("Health insurance account code is required.");
        RuleFor(x => x.UnemploymentInsuranceAccountCode).NotEmpty().WithMessage("Unemployment insurance account code is required.");
        RuleFor(x => x.PersonalIncomeTaxAccountCode).NotEmpty().WithMessage("Personal income tax account code is required.");
    }
}
