using Accounting.Domain.Exceptions;

namespace Accounting.Domain.Ledger;

public class UnbalancedJournalException : AccountingDomainException
{
    public decimal TotalDebit { get; }
    public decimal TotalCredit { get; }
    public decimal Difference => Math.Abs(TotalDebit - TotalCredit);

    public UnbalancedJournalException(decimal totalDebit, decimal totalCredit)
        : base($"Journal voucher is unbalanced: Total Debit ({totalDebit:N2}) != Total Credit ({totalCredit:N2}). Drift: {Math.Abs(totalDebit - totalCredit):N2}")
    {
        TotalDebit = totalDebit;
        TotalCredit = totalCredit;
    }
}

public class FiscalPeriodClosedException : AccountingDomainException
{
    public int Year { get; }
    public int Month { get; }
    public string LockType { get; }

    public FiscalPeriodClosedException(int year, int month, string lockType = "hard-locked (permanently closed)")
        : base($"Fiscal period {month:D2}/{year} is {lockType}. Postings and modifications are strictly rejected.")
    {
        Year = year;
        Month = month;
        LockType = lockType;
    }
}

public class StatutoryComplianceException : AccountingDomainException
{
    public StatutoryComplianceException(string message) : base(message) { }
    public StatutoryComplianceException(string message, Exception innerException) : base(message, innerException) { }
}

public class ImmutablePostedVoucherException : AccountingDomainException
{
    public string VoucherNumber { get; }

    public ImmutablePostedVoucherException(string voucherNumber)
        : base($"Voucher '{voucherNumber}' is already posted and immutable under Thông tư 99/2025/TT-BTC. Modifications require a reversing entry (bút toán đảo).")
    {
        VoucherNumber = voucherNumber;
    }
}
