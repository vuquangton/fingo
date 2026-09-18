namespace Accounting.Domain.Exceptions;

public class PeriodAlreadyClosedException : Exception
{
    public int Year { get; }
    public int Month { get; }
    public string Step { get; }

    public PeriodAlreadyClosedException(int year, int month, string step)
        : base($"Fiscal period {year}/{month:D2} has already completed period closing step '{step}'. Duplicate closing execution is rejected.")
    {
        Year = year;
        Month = month;
        Step = step;
    }
}

public class UnbalancedTrialBalanceException : Exception
{
    public decimal TotalDebit { get; }
    public decimal TotalCredit { get; }

    public UnbalancedTrialBalanceException(decimal totalDebit, decimal totalCredit)
        : base($"Trial balance is out of balance. Total Debit ({totalDebit:N2}) != Total Credit ({totalCredit:N2}). Period-end closing cannot proceed.")
    {
        TotalDebit = totalDebit;
        TotalCredit = totalCredit;
    }
}

public class InvalidTaxDeclarationException : Exception
{
    public InvalidTaxDeclarationException(string message) : base(message) { }
}
