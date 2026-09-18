using Accounting.Domain.Common;
using Accounting.Domain.Exceptions;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.CapitalAssets;

public class PrepaidExpense : AggregateRoot<PrepaidExpenseId>, IAuditableEntity, ISoftDeletable
{
    public string ExpenseCode { get; private set; } = string.Empty;
    public string Name { get; private set; } = string.Empty;
    public decimal TotalAmount { get; private set; }
    public decimal AllocatedAmount { get; private set; }
    public decimal RemainingAmount => TotalAmount - AllocatedAmount;
    public int TotalPeriods { get; private set; }
    public int RemainingPeriods { get; private set; }
    public DateOnly StartDate { get; private set; }
    public AccountId SourceAccountId { get; private set; }
    public AccountId TargetExpenseAccountId { get; private set; }
    public CostCenterId? CostCenterId { get; private set; }
    public PrepaidExpenseStatus Status { get; private set; }

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

    private PrepaidExpense() { }

    public PrepaidExpense(
        PrepaidExpenseId id,
        string expenseCode,
        string name,
        decimal totalAmount,
        int totalPeriods,
        DateOnly startDate,
        AccountId sourceAccountId,
        AccountId targetExpenseAccountId,
        CostCenterId? costCenterId = null)
    {
        if (id == PrepaidExpenseId.Empty)
            throw new ArgumentException("Prepaid expense ID cannot be empty.", nameof(id));

        var code = (expenseCode ?? string.Empty).Trim().ToUpperInvariant();
        if (string.IsNullOrWhiteSpace(code))
            throw new ArgumentException("Expense code cannot be empty.", nameof(expenseCode));

        var n = (name ?? string.Empty).Trim();
        if (string.IsNullOrWhiteSpace(n))
            throw new ArgumentException("Expense name cannot be empty.", nameof(name));

        if (totalAmount <= 0m)
            throw new ArgumentException("Total amount must be strictly positive.", nameof(totalAmount));

        if (totalPeriods <= 0)
            throw new ArgumentException("Total periods must be strictly positive.", nameof(totalPeriods));

        Id = id;
        ExpenseCode = code;
        Name = n;
        TotalAmount = totalAmount;
        AllocatedAmount = 0m;
        TotalPeriods = totalPeriods;
        RemainingPeriods = totalPeriods;
        StartDate = startDate;
        SourceAccountId = sourceAccountId;
        TargetExpenseAccountId = targetExpenseAccountId;
        CostCenterId = costCenterId;
        Status = PrepaidExpenseStatus.Active;
    }

    public decimal CalculateMonthlyAmortization()
    {
        if (Status != PrepaidExpenseStatus.Active)
            return 0m;

        if (RemainingAmount <= 0m)
            return 0m;

        if (RemainingPeriods <= 1)
            return RemainingAmount;

        var standard = decimal.Round(TotalAmount / TotalPeriods, 2, MidpointRounding.AwayFromZero);
        return Math.Min(standard, RemainingAmount);
    }

    public decimal ApplyAmortization(decimal amount)
    {
        if (Status == PrepaidExpenseStatus.Completed)
            throw new PrepaidExpenseFullyAmortizedException(ExpenseCode, "prepaid expense is already fully amortized");

        if (amount <= 0m)
            throw new ArgumentException("Amortization amount must be strictly positive.", nameof(amount));

        var actual = Math.Min(amount, RemainingAmount);
        AllocatedAmount += actual;
        RemainingPeriods = Math.Max(0, RemainingPeriods - 1);

        if (AllocatedAmount >= TotalAmount || RemainingPeriods == 0)
        {
            Status = PrepaidExpenseStatus.Completed;
        }

        return actual;
    }
}
