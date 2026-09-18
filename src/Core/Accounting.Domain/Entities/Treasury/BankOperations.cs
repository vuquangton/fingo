using Accounting.Domain.Common;

namespace Accounting.Domain.Entities.Treasury;

public class BankAccount : Entity<Guid>
{
    public string AccountNumber { get; private set; } = string.Empty;
    public string BankName { get; private set; } = string.Empty;
    public string? Branch { get; private set; }
    public string? SwiftCode { get; private set; }
    public string Currency { get; private set; } = CurrencyCode.VND;
    public Guid GlAccountId { get; private set; } // TK 1121, 1122...
    public decimal CurrentBalance { get; private set; }
    public bool IsActive { get; private set; } = true;

    private BankAccount() { }

    public BankAccount(string accountNumber, string bankName, Guid glAccountId, string? branch = null, string currency = CurrencyCode.VND)
    {
        Id = Guid.NewGuid();
        AccountNumber = accountNumber.Trim();
        BankName = bankName.Trim();
        GlAccountId = glAccountId;
        Branch = branch?.Trim();
        Currency = currency.ToUpperInvariant();
    }

    public void AdjustBalance(decimal amount) => CurrentBalance += amount;
}

public class PaymentOrder : AggregateRoot<Guid>
{
    public string OrderNumber { get; private set; } = string.Empty; // UNC-...
    public DateTime OrderDate { get; private set; }
    public Guid BankAccountId { get; private set; }
    public BankAccount? BankAccount { get; private set; }
    public string BeneficiaryName { get; private set; } = string.Empty;
    public string BeneficiaryAccountNumber { get; private set; } = string.Empty;
    public string BeneficiaryBankName { get; private set; } = string.Empty;
    public string? BeneficiaryBranch { get; private set; }
    public decimal Amount { get; private set; }
    public string Currency { get; private set; } = CurrencyCode.VND;
    public string PaymentPurpose { get; private set; } = string.Empty;
    public bool IsFeeBorneBySender { get; private set; } = true;
    public Guid? CorrespondingVoucherId { get; private set; }

    private PaymentOrder() { }

    public PaymentOrder(
        string orderNumber,
        DateTime orderDate,
        Guid bankAccountId,
        string beneficiaryName,
        string beneficiaryAccountNumber,
        string beneficiaryBankName,
        decimal amount,
        string paymentPurpose,
        bool isFeeBorneBySender = true,
        string currency = CurrencyCode.VND)
    {
        Id = Guid.NewGuid();
        OrderNumber = orderNumber.Trim();
        OrderDate = orderDate;
        BankAccountId = bankAccountId;
        BeneficiaryName = beneficiaryName.Trim();
        BeneficiaryAccountNumber = beneficiaryAccountNumber.Trim();
        BeneficiaryBankName = beneficiaryBankName.Trim();
        Amount = amount;
        PaymentPurpose = paymentPurpose.Trim();
        IsFeeBorneBySender = isFeeBorneBySender;
        Currency = currency.ToUpperInvariant();
    }

    public void LinkVoucher(Guid voucherId) => CorrespondingVoucherId = voucherId;
}

public class BankStatementReconciliation : AggregateRoot<Guid>
{
    public Guid BankAccountId { get; private set; }
    public DateTime StatementDate { get; private set; }
    public decimal StatementClosingBalance { get; private set; }
    public decimal BookClosingBalance { get; private set; }
    public decimal Difference => StatementClosingBalance - BookClosingBalance;
    public bool IsReconciled => Math.Abs(Difference) < 0.01m;
    public string? Notes { get; private set; }

    private BankStatementReconciliation() { }

    public BankStatementReconciliation(Guid bankAccountId, DateTime statementDate, decimal statementBalance, decimal bookBalance, string? notes = null)
    {
        Id = Guid.NewGuid();
        BankAccountId = bankAccountId;
        StatementDate = statementDate;
        StatementClosingBalance = statementBalance;
        BookClosingBalance = bookBalance;
        Notes = notes;
    }
}
