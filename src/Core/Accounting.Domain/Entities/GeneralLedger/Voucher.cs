using Accounting.Domain.Common;
using Accounting.Domain.Enums;
using Accounting.Domain.Exceptions;

namespace Accounting.Domain.Entities.GeneralLedger;

public record VoucherPostedDomainEvent(Guid VoucherId, string VoucherNumber, DateTime PostingDate, Guid PostedBy) : IDomainEvent
{
    public Guid EventId { get; } = Guid.NewGuid();
    public DateTime OccurredOnUtc { get; } = DateTime.UtcNow;
}

public record VoucherUnpostedDomainEvent(Guid VoucherId, string VoucherNumber, Guid UnpostedBy) : IDomainEvent
{
    public Guid EventId { get; } = Guid.NewGuid();
    public DateTime OccurredOnUtc { get; } = DateTime.UtcNow;
}

public class Voucher : AggregateRoot<Guid>
{
    public string VoucherNumber { get; private set; } = string.Empty;
    public DateTime VoucherDate { get; private set; }
    public DateTime PostingDate { get; private set; }
    public VoucherType VoucherType { get; private set; }
    public VoucherStatus Status { get; private set; }
    public string Description { get; private set; } = string.Empty;
    public string? ReferenceNumber { get; private set; }
    public string Currency { get; private set; } = CurrencyCode.VND;
    public decimal ExchangeRate { get; private set; } = 1.0m;
    public decimal TotalDebitAmount { get; private set; }
    public decimal TotalCreditAmount { get; private set; }

    public Guid? OriginalVoucherId { get; private set; }
    public Guid? ReversingVoucherId { get; private set; }
    public bool IsReversal { get; private set; }

    public Guid CreatedBy { get; private set; }
    public DateTime CreatedAtUtc { get; private set; }
    public Guid? PostedBy { get; private set; }
    public DateTime? PostedAtUtc { get; private set; }

    private readonly List<VoucherLine> _lines = [];
    public IReadOnlyCollection<VoucherLine> Lines => _lines.AsReadOnly();

    private Voucher() { } // EF Core

    public Voucher(
        string voucherNumber,
        DateTime voucherDate,
        DateTime postingDate,
        VoucherType voucherType,
        string description,
        Guid createdBy,
        string? referenceNumber = null,
        string currency = CurrencyCode.VND,
        decimal exchangeRate = 1.0m)
    {
        Id = Guid.NewGuid();
        VoucherNumber = voucherNumber.Trim();
        VoucherDate = voucherDate;
        PostingDate = postingDate;
        VoucherType = voucherType;
        Status = VoucherStatus.Draft;
        Description = description.Trim();
        CreatedBy = createdBy;
        CreatedAtUtc = DateTime.UtcNow;
        ReferenceNumber = referenceNumber?.Trim();
        Currency = currency.ToUpperInvariant();
        ExchangeRate = exchangeRate > 0 ? exchangeRate : 1.0m;
    }

    public VoucherLine AddLine(
        Guid debitAccountId,
        Guid creditAccountId,
        decimal amount,
        string description,
        decimal? foreignAmount = null,
        Guid? partnerId = null,
        PartnerType? partnerType = null,
        Guid? costCenterId = null,
        Guid? projectId = null)
    {
        EnsureNotPosted();

        if (amount <= 0)
            throw new ArgumentOutOfRangeException(nameof(amount), "Transaction line amount must be greater than zero.");

        if (debitAccountId == creditAccountId)
            throw new ArgumentException("Debit account and Credit account cannot be identical in double-entry bookkeeping.");

        var line = new VoucherLine(
            Id,
            _lines.Count + 1,
            debitAccountId,
            creditAccountId,
            amount,
            description,
            foreignAmount,
            Currency,
            ExchangeRate,
            partnerId,
            partnerType,
            costCenterId,
            projectId);

        _lines.Add(line);
        RecalculateTotals();
        return line;
    }

    public void RemoveLine(Guid lineId)
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

    public void Post(Guid userId)
    {
        EnsureNotPosted();
        ValidateBalance();

        Status = VoucherStatus.Posted;
        PostedBy = userId;
        PostedAtUtc = DateTime.UtcNow;

        AddDomainEvent(new VoucherPostedDomainEvent(Id, VoucherNumber, PostingDate, userId));
    }

    public void Unpost(Guid userId)
    {
        if (Status != VoucherStatus.Posted)
            throw new InvalidOperationException($"Cannot unpost voucher '{VoucherNumber}' because it is not currently posted.");

        Status = VoucherStatus.Draft;
        PostedBy = null;
        PostedAtUtc = null;

        AddDomainEvent(new VoucherUnpostedDomainEvent(Id, VoucherNumber, userId));
    }

    public Voucher CreateReversal(string newVoucherNumber, Guid userId, DateTime reversalDate, string reason)
    {
        if (Status != VoucherStatus.Posted)
            throw new InvalidOperationException("Only posted vouchers can be reversed.");

        var reversal = new Voucher(
            newVoucherNumber,
            reversalDate,
            reversalDate,
            VoucherType,
            $"Reversal for {VoucherNumber}: {reason}",
            userId,
            VoucherNumber,
            Currency,
            ExchangeRate)
        {
            OriginalVoucherId = Id,
            IsReversal = true
        };

        foreach (var line in _lines)
        {
            reversal.AddLine(
                line.CreditAccountId,
                line.DebitAccountId,
                line.Amount,
                $"Reversal: {line.Description}",
                line.ForeignAmount,
                line.PartnerId,
                line.PartnerType,
                line.CostCenterId,
                line.ProjectId);
        }

        ReversingVoucherId = reversal.Id;
        return reversal;
    }

    public void ValidateBalance()
    {
        if (_lines.Count == 0)
            throw new AccountingDomainException($"Voucher '{VoucherNumber}' contains no journal entries.");

        RecalculateTotals();

        if (TotalDebitAmount != TotalCreditAmount)
            throw new UnbalancedVoucherException(TotalDebitAmount, TotalCreditAmount);
    }

    private void RecalculateTotals()
    {
        TotalDebitAmount = _lines.Sum(l => l.Amount);
        TotalCreditAmount = _lines.Sum(l => l.Amount);
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
        if (Status == VoucherStatus.Posted)
            throw new ImmutablePostedVoucherException(VoucherNumber);
    }
}

public class VoucherLine : Entity<Guid>
{
    public Guid VoucherId { get; private set; }
    public int LineNumber { get; private set; }
    public Guid DebitAccountId { get; private set; }
    public Account? DebitAccount { get; private set; }
    public Guid CreditAccountId { get; private set; }
    public Account? CreditAccount { get; private set; }
    public decimal Amount { get; private set; }
    public decimal? ForeignAmount { get; private set; }
    public string Currency { get; private set; } = CurrencyCode.VND;
    public decimal ExchangeRate { get; private set; } = 1.0m;
    public string Description { get; private set; } = string.Empty;

    public Guid? PartnerId { get; private set; }
    public PartnerType? PartnerType { get; private set; }
    public Guid? CostCenterId { get; private set; }
    public Guid? ProjectId { get; private set; }

    private VoucherLine() { } // EF Core

    internal VoucherLine(
        Guid voucherId,
        int lineNumber,
        Guid debitAccountId,
        Guid creditAccountId,
        decimal amount,
        string description,
        decimal? foreignAmount,
        string currency,
        decimal exchangeRate,
        Guid? partnerId,
        PartnerType? partnerType,
        Guid? costCenterId,
        Guid? projectId)
    {
        Id = Guid.NewGuid();
        VoucherId = voucherId;
        LineNumber = lineNumber;
        DebitAccountId = debitAccountId;
        CreditAccountId = creditAccountId;
        Amount = decimal.Round(amount, 2, MidpointRounding.AwayFromZero);
        Description = description.Trim();
        ForeignAmount = foreignAmount.HasValue ? decimal.Round(foreignAmount.Value, 4, MidpointRounding.AwayFromZero) : null;
        Currency = currency.ToUpperInvariant();
        ExchangeRate = exchangeRate;
        PartnerId = partnerId;
        PartnerType = partnerType;
        CostCenterId = costCenterId;
        ProjectId = projectId;
    }

    internal void SetLineNumber(int number) => LineNumber = number;
}
