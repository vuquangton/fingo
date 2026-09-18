using Accounting.Domain.Common;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.Receivables;

public class SalesInvoice : AggregateRoot<SalesInvoiceId>, IAuditableEntity
{
    public string InvoiceNumber { get; private set; } = string.Empty;
    public DateOnly InvoiceDate { get; private set; }
    public PartnerId CustomerId { get; private set; }
    public decimal SubTotalAmount { get; private set; }
    public decimal VatAmount { get; private set; }
    public decimal TotalAmount { get; private set; }
    public decimal ReceivedAmount { get; private set; }
    public decimal RemainingAmount => TotalAmount - ReceivedAmount;
    public DateOnly DueDate { get; private set; }
    public SalesInvoiceStatus Status { get; private set; } = SalesInvoiceStatus.Open;
    public VoucherId? LinkedVoucherId { get; private set; }

    // Audit fields
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    private readonly List<SalesInvoiceLine> _lines = [];
    public IReadOnlyCollection<SalesInvoiceLine> Lines => _lines.AsReadOnly();

    private SalesInvoice() { }

    public SalesInvoice(
        SalesInvoiceId id,
        string invoiceNumber,
        DateOnly invoiceDate,
        DateOnly dueDate,
        PartnerId customerId)
    {
        if (string.IsNullOrWhiteSpace(invoiceNumber))
            throw new ArgumentException("Invoice number cannot be empty.", nameof(invoiceNumber));
        if (dueDate < invoiceDate)
            throw new ArgumentException("Due date cannot precede invoice date.", nameof(dueDate));

        Id = id;
        InvoiceNumber = invoiceNumber.Trim();
        InvoiceDate = invoiceDate;
        DueDate = dueDate;
        CustomerId = customerId;
        Status = SalesInvoiceStatus.Open;
    }

    public void AddLine(
        AccountId accountId,
        decimal quantity,
        decimal unitPrice,
        decimal vatRate,
        string description,
        InventoryItemId? inventoryItemId = null)
    {
        if (Status == SalesInvoiceStatus.Cancelled)
            throw new InvalidOperationException("Cannot modify cancelled invoice.");

        var line = new SalesInvoiceLine(
            SalesInvoiceLineId.New(),
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
        SubTotalAmount = _lines.Sum(l => l.RevenueAmount);
        VatAmount = _lines.Sum(l => l.VatAmount);
        TotalAmount = SubTotalAmount + VatAmount;
    }

    public void ApplyReceipt(decimal amount)
    {
        if (Status == SalesInvoiceStatus.Cancelled)
            throw new InvalidOperationException($"Cannot apply receipt to cancelled invoice '{InvoiceNumber}'.");
        if (amount <= 0)
            throw new ArgumentOutOfRangeException(nameof(amount), "Settlement receipt amount must be strictly positive.");

        var remaining = RemainingAmount;
        if (amount > remaining)
        {
            throw new OverSettlementException(InvoiceNumber, amount, remaining);
        }

        ReceivedAmount += amount;
        Status = ReceivedAmount >= TotalAmount ? SalesInvoiceStatus.FullyPaid : SalesInvoiceStatus.PartiallyPaid;
        UpdatedAtUtc = DateTime.UtcNow;
    }

    public void ReverseReceipt(decimal amount)
    {
        if (amount <= 0)
            throw new ArgumentOutOfRangeException(nameof(amount), "Reverse receipt amount must be strictly positive.");
        if (amount > ReceivedAmount)
            throw new InvalidOperationException($"Cannot reverse receipt {amount:N2} exceeding received amount {ReceivedAmount:N2}.");

        ReceivedAmount -= amount;
        Status = ReceivedAmount <= 0 ? SalesInvoiceStatus.Open : SalesInvoiceStatus.PartiallyPaid;
        UpdatedAtUtc = DateTime.UtcNow;
    }

    public void Cancel()
    {
        if (ReceivedAmount > 0)
            throw new InvalidOperationException($"Cannot cancel invoice '{InvoiceNumber}' with existing receipts ({ReceivedAmount:N2}).");

        Status = SalesInvoiceStatus.Cancelled;
        UpdatedAtUtc = DateTime.UtcNow;
    }

    public void LinkGeneralLedgerVoucher(VoucherId glVoucherId)
    {
        LinkedVoucherId = glVoucherId;
        UpdatedAtUtc = DateTime.UtcNow;
    }
}
