namespace Accounting.Domain.Ledger;

public enum LedgerEntryType
{
    Debit = 1,
    Credit = 2
}

public enum VoucherType
{
    GeneralJournal = 1,
    CashReceipt = 2,
    CashDisbursement = 3,
    BankPayment = 4,
    InventoryReceipt = 5,
    InventoryIssue = 6,
    SalesInvoice = 7,
    PurchaseInvoice = 8
}

public enum VoucherStatus
{
    Draft = 1,
    Approved = 2,
    Posted = 3,
    Reversed = 4
}
