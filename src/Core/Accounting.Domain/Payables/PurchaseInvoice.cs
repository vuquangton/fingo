using Accounting.Domain.Common;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.Payables;

public class PurchaseInvoice : AggregateRoot<PurchaseInvoiceId>, IAuditableEntity
{
    public string InvoiceNumber { get; private set; } = string.Empty;
    public string InvoiceSeries { get; private set; } = string.Empty;
    public DateOnly InvoiceDate { get; private set; }
    public PartnerId VendorId { get; private set; }
    public decimal SubTotalAmount { get; private set; }
    public decimal VatAmount { get; private set; }
    public decimal TotalAmount { get; private set; }
    public decimal PaidAmount { get; private set; }
    public decimal RemainingAmount => TotalAmount - PaidAmount;
    public DateOnly DueDate { get; private set; }
    public InvoiceStatus Status { get; private set; } = InvoiceStatus.Open;
    public VoucherId? LinkedVoucherId { get; private set; }

    // Audit fields
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    private readonly List<PurchaseInvoiceLine> _lines = [];
    public IReadOnlyCollection<PurchaseInvoiceLine> Lines => _lines.AsReadOnly();

    private PurchaseInvoice() { }

    public PurchaseInvoice(
        PurchaseInvoiceId id,
        string invoiceNumber,
        string invoiceSeries,
        DateOnly invoiceDate,
        DateOnly dueDate,
        PartnerId vendorId)
    {
        if (string.IsNullOrWhiteSpace(invoiceNumber))
            throw new ArgumentException("Invoice number cannot be empty.", nameof(invoiceNumber));
        if (dueDate < invoiceDate)
            throw new ArgumentException("Due date cannot precede invoice date.", nameof(dueDate));

        Id = id;
        InvoiceNumber = invoiceNumber.Trim();
        InvoiceSeries = invoiceSeries.Trim();
        InvoiceDate = invoiceDate;
        DueDate = dueDate;
        VendorId = vendorId;
        Status = InvoiceStatus.Open;
    }

    public void AddLine(
        AccountId accountId,
        decimal quantity,
        decimal unitPrice,
        decimal vatRate,
        string description,
        InventoryItemId? inventoryItemId = null)
    {
        if (Status == InvoiceStatus.Cancelled)
            throw new InvalidOperationException("Cannot modify cancelled invoice.");

        var line = new PurchaseInvoiceLine(
            PurchaseInvoiceLineId.New(),
            Id,
            accountId,
            quantity,
            unitPrice,
            vatRate,
            description,
            inventoryItemId);

        _lines.Add(line);
        RecalculateTotals();
    }

    private void RecalculateTotals()
    {
        SubTotalAmount = _lines.Sum(l => l.Amount);
        VatAmount = _lines.Sum(l => l.VatAmount);
        TotalAmount = SubTotalAmount + VatAmount;
    }

    public void ApplyPayment(decimal amount)
    {
        if (Status == InvoiceStatus.Cancelled)
            throw new InvalidOperationException($"Cannot apply payment to cancelled invoice '{InvoiceNumber}'.");
        if (amount <= 0)
            throw new ArgumentOutOfRangeException(nameof(amount), "Settlement amount must be strictly positive.");

        var remaining = RemainingAmount;
        if (amount > remaining)
        {
            throw new OverSettlementException(InvoiceNumber, amount, remaining);
        }

        PaidAmount += amount;
        Status = PaidAmount >= TotalAmount ? InvoiceStatus.FullyPaid : InvoiceStatus.PartiallyPaid;
        UpdatedAtUtc = DateTime.UtcNow;
    }

    public void ReversePayment(decimal amount)
    {
        if (amount <= 0)
            throw new ArgumentOutOfRangeException(nameof(amount), "Reverse payment amount must be strictly positive.");
        if (amount > PaidAmount)
            throw new InvalidOperationException($"Cannot reverse payment {amount:N2} exceeding paid amount {PaidAmount:N2}.");

        PaidAmount -= amount;
        Status = PaidAmount <= 0 ? InvoiceStatus.Open : InvoiceStatus.PartiallyPaid;
        UpdatedAtUtc = DateTime.UtcNow;
    }

    public void Cancel()
    {
        if (PaidAmount > 0)
            throw new InvalidOperationException($"Cannot cancel invoice '{InvoiceNumber}' with existing payments ({PaidAmount:N2}).");

        Status = InvoiceStatus.Cancelled;
        UpdatedAtUtc = DateTime.UtcNow;
    }

    public void LinkGeneralLedgerVoucher(VoucherId glVoucherId)
    {
        LinkedVoucherId = glVoucherId;
        UpdatedAtUtc = DateTime.UtcNow;
    }
}
