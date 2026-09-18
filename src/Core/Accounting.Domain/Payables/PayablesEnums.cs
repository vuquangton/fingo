namespace Accounting.Domain.Payables;

public enum InvoiceStatus
{
    Open = 1,
    PartiallyPaid = 2,
    FullyPaid = 3,
    Cancelled = 4
}
