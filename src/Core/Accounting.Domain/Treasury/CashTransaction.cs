using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Common;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;

namespace Accounting.Domain.Treasury;

public class CashTransaction : AggregateRoot<CashVoucherId>, IAuditableEntity
{
    public string VoucherNumber { get; private set; } = string.Empty;
    public TreasuryTransactionType TransactionType { get; private set; }
    public DateOnly TransactionDate { get; private set; }
    public string? BankAccountId { get; private set; }
    public PartnerId? PartnerId { get; private set; }
    public string ReceiverPayerName { get; private set; } = string.Empty;
    public string Description { get; private set; } = string.Empty;
    public decimal TotalAmount { get; private set; }
    public CurrencyCode CurrencyId { get; private set; } = new("VND");
    public decimal ExchangeRate { get; private set; } = 1.0m;
    public VoucherId? LinkedVoucherId { get; private set; }

    // Audit metadata
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    private CashTransaction() { }

    public CashTransaction(
        CashVoucherId id,
        string voucherNumber,
        TreasuryTransactionType transactionType,
        DateOnly transactionDate,
        string receiverPayerName,
        string description,
        decimal totalAmount,
        CurrencyCode currencyId,
        decimal exchangeRate = 1.0m,
        PartnerId? partnerId = null,
        string? bankAccountId = null)
    {
        if (string.IsNullOrWhiteSpace(voucherNumber))
            throw new ArgumentException("Voucher number cannot be empty.", nameof(voucherNumber));
        if (totalAmount <= 0)
            throw new ArgumentOutOfRangeException(nameof(totalAmount), "Total amount must be strictly positive.");
        if (exchangeRate <= 0)
            throw new ArgumentOutOfRangeException(nameof(exchangeRate), "Exchange rate must be strictly positive.");

        if ((transactionType == TreasuryTransactionType.BankDeposit || transactionType == TreasuryTransactionType.BankPaymentOrder)
            && string.IsNullOrWhiteSpace(bankAccountId))
        {
            throw new ArgumentException("Bank account is required for bank transactions.", nameof(bankAccountId));
        }

        Id = id;
        VoucherNumber = voucherNumber.Trim().ToUpperInvariant();
        TransactionType = transactionType;
        TransactionDate = transactionDate;
        ReceiverPayerName = receiverPayerName.Trim();
        Description = description.Trim();
        TotalAmount = totalAmount;
        CurrencyId = currencyId;
        ExchangeRate = exchangeRate;
        PartnerId = partnerId;
        BankAccountId = bankAccountId?.Trim();
    }

    public void LinkGeneralLedgerVoucher(VoucherId glVoucherId)
    {
        LinkedVoucherId = glVoucherId;
        UpdatedAtUtc = DateTime.UtcNow;
    }
}
