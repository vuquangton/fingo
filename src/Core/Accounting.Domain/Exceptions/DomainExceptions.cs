namespace Accounting.Domain.Exceptions;

public class AccountingDomainException : Exception
{
    public AccountingDomainException(string message) : base(message) { }
    public AccountingDomainException(string message, Exception innerException) : base(message, innerException) { }
}

public sealed class UnbalancedVoucherException : AccountingDomainException
{
    public decimal TotalDebit { get; }
    public decimal TotalCredit { get; }

    public UnbalancedVoucherException(decimal totalDebit, decimal totalCredit)
        : base($"Voucher is unbalanced! Total Debit ({totalDebit:N2}) must equal Total Credit ({totalCredit:N2}). Difference: {Math.Abs(totalDebit - totalCredit):N2}")
    {
        TotalDebit = totalDebit;
        TotalCredit = totalCredit;
    }
}

public sealed class FiscalPeriodClosedException : AccountingDomainException
{
    public int Year { get; }
    public int Month { get; }

    public FiscalPeriodClosedException(int year, int month, string reason = "locked")
        : base($"Cannot post or edit transactions in fiscal period {month:D2}/{year}. The period is {reason}.")
    {
        Year = year;
        Month = month;
    }
}

public sealed class ImmutablePostedVoucherException : AccountingDomainException
{
    public string VoucherNumber { get; }

    public ImmutablePostedVoucherException(string voucherNumber)
        : base($"Voucher '{voucherNumber}' has already been posted and is immutable. To adjust, unpost or create a reversing entry.")
    {
        VoucherNumber = voucherNumber;
    }
}

public sealed class CreditLimitExceededException : AccountingDomainException
{
    public string CustomerCode { get; }
    public decimal CreditLimit { get; }
    public decimal ProposedBalance { get; }

    public CreditLimitExceededException(string customerCode, decimal creditLimit, decimal proposedBalance)
        : base($"Customer '{customerCode}' credit limit exceeded. Limit: {creditLimit:N2}, Proposed Balance: {proposedBalance:N2}.")
    {
        CustomerCode = customerCode;
        CreditLimit = creditLimit;
        ProposedBalance = proposedBalance;
    }
}

public sealed class NegativeStockException : AccountingDomainException
{
    public string ItemCode { get; }
    public string WarehouseCode { get; }
    public decimal AvailableQuantity { get; }
    public decimal RequestedQuantity { get; }

    public NegativeStockException(string itemCode, string warehouseCode, decimal available, decimal requested)
        : base($"Insufficient stock for item '{itemCode}' in warehouse '{warehouseCode}'. Available: {available:N2}, Requested: {requested:N2}.")
    {
        ItemCode = itemCode;
        WarehouseCode = warehouseCode;
        AvailableQuantity = available;
        RequestedQuantity = requested;
    }
}
