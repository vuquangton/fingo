using Accounting.Domain.Common;
using Accounting.Domain.Enums;

namespace Accounting.Domain.Entities.Treasury;

public class CashVoucher : AggregateRoot<Guid>
{
    public string VoucherNumber { get; private set; } = string.Empty; // PT-... or PC-...
    public VoucherType Type { get; private set; } // CashReceipt or CashPayment
    public DateTime Date { get; private set; }
    public string PersonName { get; private set; } = string.Empty; // Người nộp / Người nhận
    public string? Address { get; private set; }
    public string Reason { get; private set; } = string.Empty; // Lý do nộp / chi
    public decimal Amount { get; private set; }
    public string Currency { get; private set; } = CurrencyCode.VND;
    public decimal ExchangeRate { get; private set; } = 1.0m;
    public int AttachedDocCount { get; private set; }
    public Guid CorrespondingVoucherId { get; private set; } // Linked General Ledger Voucher
    public Guid CreatedBy { get; private set; }
    public DateTime CreatedAtUtc { get; private set; }

    private CashVoucher() { }

    public CashVoucher(
        string voucherNumber,
        VoucherType type,
        DateTime date,
        string personName,
        string reason,
        decimal amount,
        Guid correspondingVoucherId,
        Guid createdBy,
        string? address = null,
        int attachedDocCount = 0,
        string currency = CurrencyCode.VND,
        decimal exchangeRate = 1.0m)
    {
        Id = Guid.NewGuid();
        VoucherNumber = voucherNumber.Trim();
        Type = type;
        Date = date;
        PersonName = personName.Trim();
        Reason = reason.Trim();
        Amount = amount;
        CorrespondingVoucherId = correspondingVoucherId;
        CreatedBy = createdBy;
        CreatedAtUtc = DateTime.UtcNow;
        Address = address?.Trim();
        AttachedDocCount = attachedDocCount;
        Currency = currency.ToUpperInvariant();
        ExchangeRate = exchangeRate;
    }
}

public class CashCountRecord : AggregateRoot<Guid>
{
    public string RecordNumber { get; private set; } = string.Empty;
    public DateTime AuditDate { get; private set; }
    public string VaultLocation { get; private set; } = string.Empty;
    public string BoardMembers { get; private set; } = string.Empty;
    public decimal BookBalance { get; private set; }
    public decimal PhysicalBalance { get; private set; }
    public decimal VarianceAmount => PhysicalBalance - BookBalance;
    public string? VarianceReason { get; private set; }
    public string? ResolutionNotes { get; private set; }

    private readonly List<CashCountLine> _denominations = [];
    public IReadOnlyCollection<CashCountLine> Denominations => _denominations.AsReadOnly();

    private CashCountRecord() { }

    public CashCountRecord(string recordNumber, DateTime auditDate, string vaultLocation, string boardMembers, decimal bookBalance)
    {
        Id = Guid.NewGuid();
        RecordNumber = recordNumber;
        AuditDate = auditDate;
        VaultLocation = vaultLocation;
        BoardMembers = boardMembers;
        BookBalance = bookBalance;
    }

    public void AddDenomination(decimal denominationValue, int count)
    {
        _denominations.Add(new CashCountLine(Id, denominationValue, count));
        PhysicalBalance = _denominations.Sum(d => d.Total);
    }

    public void SetResolution(string varianceReason, string resolutionNotes)
    {
        VarianceReason = varianceReason;
        ResolutionNotes = resolutionNotes;
    }
}

public class CashCountLine : Entity<Guid>
{
    public Guid CashCountRecordId { get; private set; }
    public decimal DenominationValue { get; private set; } // 500,000; 200,000; 100,000; etc.
    public int SheetCount { get; private set; }
    public decimal Total => DenominationValue * SheetCount;

    private CashCountLine() { }

    public CashCountLine(Guid recordId, decimal denominationValue, int count)
    {
        Id = Guid.NewGuid();
        CashCountRecordId = recordId;
        DenominationValue = denominationValue;
        SheetCount = count;
    }
}
