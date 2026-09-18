using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.Payroll;

public class PayrollEmployee : AggregateRoot<EmployeeId>, IAuditableEntity, ISoftDeletable
{
    public string EmployeeCode { get; private set; } = string.Empty;
    public string FullName { get; private set; } = string.Empty;
    public DepartmentId DepartmentId { get; private set; }
    public string IdentityCard { get; private set; } = string.Empty; // CCCD
    public string? TaxCode { get; private set; }                     // MST Cá nhân
    public ContractType ContractType { get; private set; }
    public decimal BaseSalary { get; private set; }
    public decimal InsuranceSalary { get; private set; }
    public decimal Allowances { get; private set; }
    public decimal NonTaxableAllowances { get; private set; }
    public int DependentCount { get; private set; }
    public AccountId ExpenseAccountId { get; private set; }
    public CostCenterId? CostCenterId { get; private set; }
    public bool IsActive { get; private set; } = true;

    // IAuditableEntity
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    // ISoftDeletable
    public bool IsDeleted { get; set; }
    public DateTime? DeletedAtUtc { get; set; }
    public string? DeletedBy { get; set; }

    private PayrollEmployee() { }

    public PayrollEmployee(
        EmployeeId id,
        string employeeCode,
        string fullName,
        DepartmentId departmentId,
        string identityCard,
        ContractType contractType,
        decimal baseSalary,
        decimal insuranceSalary,
        AccountId expenseAccountId,
        decimal allowances = 0m,
        decimal nonTaxableAllowances = 0m,
        int dependentCount = 0,
        string? taxCode = null,
        CostCenterId? costCenterId = null)
    {
        if (id == EmployeeId.Empty)
            throw new ArgumentException("Employee ID cannot be empty.", nameof(id));

        var code = (employeeCode ?? string.Empty).Trim().ToUpperInvariant();
        if (string.IsNullOrWhiteSpace(code))
            throw new ArgumentException("Employee code cannot be empty.", nameof(employeeCode));

        var name = (fullName ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(name))
            throw new ArgumentException("Full name cannot be empty.", nameof(fullName));

        var idCard = (identityCard ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(idCard))
            throw new ArgumentException("Identity card (CCCD) cannot be empty.", nameof(identityCard));

        if (baseSalary < 0m)
            throw new ArgumentException("Base salary cannot be negative.", nameof(baseSalary));

        if (insuranceSalary < 0m)
            throw new ArgumentException("Insurance salary cannot be negative.", nameof(insuranceSalary));

        if (dependentCount < 0)
            throw new ArgumentException("Dependent count cannot be negative.", nameof(dependentCount));

        Id = id;
        EmployeeCode = code;
        FullName = name;
        DepartmentId = departmentId;
        IdentityCard = idCard;
        TaxCode = taxCode?.Trim();
        ContractType = contractType;
        BaseSalary = baseSalary;
        InsuranceSalary = insuranceSalary;
        Allowances = allowances;
        NonTaxableAllowances = nonTaxableAllowances;
        DependentCount = dependentCount;
        ExpenseAccountId = expenseAccountId;
        CostCenterId = costCenterId;
        IsActive = true;
    }

    public void UpdateSalaries(decimal baseSalary, decimal insuranceSalary, decimal allowances, decimal nonTaxableAllowances, int dependentCount)
    {
        if (baseSalary < 0m || insuranceSalary < 0m || allowances < 0m || nonTaxableAllowances < 0m || dependentCount < 0)
            throw new ArgumentException("Salary components and dependent count must be non-negative.");

        BaseSalary = baseSalary;
        InsuranceSalary = insuranceSalary;
        Allowances = allowances;
        NonTaxableAllowances = nonTaxableAllowances;
        DependentCount = dependentCount;
    }

    public void SetActive(bool isActive) => IsActive = isActive;
}
