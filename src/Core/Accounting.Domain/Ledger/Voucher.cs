using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;

namespace Accounting.Domain.Ledger;

public class Voucher : AggregateRoot<VoucherId>, IAuditableEntity
{
    public string VoucherNumber { get; private set; } = string.Empty;
    public VoucherType VoucherType { get; private set; }
    public DateOnly VoucherDate { get; private set; }
    public DateOnly PostingDate { get; private set; }
    public VoucherStatus Status { get; private set; }
    public string Description { get; private set; } = string.Empty;
    public CurrencyCode CurrencyId { get; private set; } = CurrencyCode.Vnd;
    public decimal ExchangeRate { get; private set; } = 1.0m;

    public decimal TotalDebitBase { get; private set; }
    public decimal TotalCreditBase { get; private set; }
    public decimal TotalDebitOriginal { get; private set; }
    public decimal TotalCreditOriginal { get; private set; }

    public DateTime? PostedAtUtc { get; private set; }
    public string? PostedBy { get; private set; }
    public VoucherId? ReversalOfVoucherId { get; private set; }

    // IAuditableEntity
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    private readonly List<VoucherLine> _lines = [];
    public IReadOnlyCollection<VoucherLine> Lines => _lines.AsReadOnly();

    private Voucher() { } // EF Core

    public Voucher(
        VoucherId id,
        string voucherNumber,
        VoucherType voucherType,
        DateOnly voucherDate,
        DateOnly postingDate,
        string description,
        CurrencyCode currencyId = default,
        decimal exchangeRate = 1.0m,
        VoucherId? reversalOfVoucherId = null)
    {
        var number = voucherNumber.Trim().ToUpperInvariant();
        if (string.IsNullOrWhiteSpace(number))
            throw new ArgumentException("Voucher number cannot be empty.", nameof(voucherNumber));

        if (exchangeRate <= 0)
            throw new ArgumentOutOfRangeException(nameof(exchangeRate), "Exchange rate must be strictly positive.");

        Id = id;
        VoucherNumber = number;
        VoucherType = voucherType;
        VoucherDate = voucherDate;
        PostingDate = postingDate;
        Status = VoucherStatus.Draft;
        Description = description.Trim();
        CurrencyId = currencyId == default ? CurrencyCode.Vnd : currencyId;
        ExchangeRate = exchangeRate;
        ReversalOfVoucherId = reversalOfVoucherId;
    }

    public VoucherLine AddLine(
        AccountId accountId,
        LedgerEntryType entryType,
        decimal amountOriginal,
        string description,
        PartnerId? partnerId = null,
        WarehouseId? warehouseId = null,
        CostCenterId? costCenterId = null,
        string? projectId = null)
    {
        EnsureNotPosted();

        if (amountOriginal <= 0)
            throw new ArgumentOutOfRangeException(nameof(amountOriginal), "Line amount must be strictly positive.");

        var amountBase = CurrencyId == CurrencyCode.Vnd
            ? amountOriginal
            : decimal.Round(amountOriginal * ExchangeRate, 2, MidpointRounding.AwayFromZero);

        var line = new VoucherLine(
            VoucherLineId.New(),
            Id,
            _lines.Count + 1,
            accountId,
            entryType,
            amountOriginal,
            amountBase,
            description,
            partnerId,
            warehouseId,
            costCenterId,
            projectId);

        _lines.Add(line);
        RecalculateTotals();
        return line;
    }

    public void RemoveLine(VoucherLineId lineId)
    {
        EnsureNotPosted();
        var line = _lines.FirstOrDefault(l => l.Id == lineId);
        if (line != null)
        {
            _lines.Remove(line);
            RenumberLines();
            RecalculateTotals();
        }
    }

    public void ClearLines()
    {
        EnsureNotPosted();
        _lines.Clear();
        RecalculateTotals();
    }

    public void Approve()
    {
        EnsureNotPosted();
        if (Status != VoucherStatus.Draft)
            throw new InvalidOperationException($"Cannot approve voucher in '{Status}' status.");

        ValidateBalance();
        Status = VoucherStatus.Approved;
    }

    public void Post(string postedBy)
    {
        EnsureNotPosted();

        if (string.IsNullOrWhiteSpace(postedBy))
            throw new ArgumentException("Posted by username is required.", nameof(postedBy));

        ValidateBalance();

        Status = VoucherStatus.Posted;
        PostedBy = postedBy.Trim();
        PostedAtUtc = DateTime.UtcNow;
    }

    public Voucher CreateReversal(string newVoucherNumber, string reversedBy, DateOnly reversalDate, string reason)
    {
        if (Status != VoucherStatus.Posted)
            throw new InvalidOperationException($"Only posted vouchers can be reversed. Current status: '{Status}'.");

        var reversal = new Voucher(
            VoucherId.New(),
            newVoucherNumber,
            VoucherType,
            reversalDate,
            reversalDate,
            $"Reversal of {VoucherNumber}: {reason.Trim()}",
            CurrencyId,
            ExchangeRate,
            reversalOfVoucherId: Id);

        foreach (var line in _lines)
        {
            // Swap entry direction: Debit -> Credit, Credit -> Debit
            var reversedEntry = line.EntryType == LedgerEntryType.Debit
                ? LedgerEntryType.Credit
                : LedgerEntryType.Debit;

            reversal.AddLine(
                line.AccountId,
                reversedEntry,
                line.AmountOriginal,
                $"Reversal: {line.Description}",
                line.PartnerId,
                line.WarehouseId,
                line.CostCenterId,
                line.ProjectId);
        }

        // Transition this voucher to Reversed
        Status = VoucherStatus.Reversed;
        UpdatedAtUtc = DateTime.UtcNow;
        UpdatedBy = reversedBy;

        return reversal;
    }

    public void ValidateBalance()
    {
        if (_lines.Count < 2)
            throw new UnbalancedJournalException(TotalDebitBase, TotalCreditBase);

        RecalculateTotals();

        // 1. Strict Base Currency Balance Invariant: Sum(DebitBase) == Sum(CreditBase)
        if (TotalDebitBase != TotalCreditBase)
            throw new UnbalancedJournalException(TotalDebitBase, TotalCreditBase);

        // 2. Strict Original Currency Balance Invariant: Sum(DebitOriginal) == Sum(CreditOriginal)
        if (TotalDebitOriginal != TotalCreditOriginal)
            throw new UnbalancedJournalException(TotalDebitOriginal, TotalCreditOriginal);
    }

    private void RecalculateTotals()
    {
        TotalDebitBase = _lines.Where(l => l.EntryType == LedgerEntryType.Debit).Sum(l => l.AmountBase);
        TotalCreditBase = _lines.Where(l => l.EntryType == LedgerEntryType.Credit).Sum(l => l.AmountBase);

        TotalDebitOriginal = _lines.Where(l => l.EntryType == LedgerEntryType.Debit).Sum(l => l.AmountOriginal);
        TotalCreditOriginal = _lines.Where(l => l.EntryType == LedgerEntryType.Credit).Sum(l => l.AmountOriginal);
    }

    private void RenumberLines()
    {
        for (int i = 0; i < _lines.Count; i++)
        {
            _lines[i].SetLineNumber(i + 1);
        }
    }

    private void EnsureNotPosted()
    {
        if (Status == VoucherStatus.Posted || Status == VoucherStatus.Reversed)
            throw new ImmutablePostedVoucherException(VoucherNumber);
    }
}
