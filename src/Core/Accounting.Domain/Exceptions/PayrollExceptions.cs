namespace Accounting.Domain.Exceptions;

public class PayrollRunAlreadyPostedException : AccountingDomainException
{
    public PayrollRunAlreadyPostedException(string runTitle, string reason)
        : base($"Payroll run '{runTitle}' cannot be modified or re-posted: {reason}.")
    {
        RunTitle = runTitle;
        Reason = reason;
    }

    public string RunTitle { get; }
    public string Reason { get; }
}

public class InvalidPayrollCalculationException : AccountingDomainException
{
    public InvalidPayrollCalculationException(string message) : base(message) { }
}
