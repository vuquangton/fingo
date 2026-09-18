using Accounting.Domain.Common;
using Accounting.Domain.Enums;

namespace Accounting.Domain.Entities.Purchasing;

public class Vendor : AggregateRoot<Guid>, IAuditableEntity, ISoftDeletable
{
    public string Code { get; private set; } = string.Empty;
    public string Name { get; private set; } = string.Empty;
    public string TaxCode { get; private set; } = string.Empty; // Mã số thuế
    public string? Address { get; private set; }
    public string? ContactPerson { get; private set; }
    public string? Phone { get; private set; }
    public string? Email { get; private set; }
    public int PaymentTermsDays { get; private set; } = 30;
    public decimal CurrentPayableBalance { get; private set; }
    public bool IsActive { get; private set; } = true;

    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    public bool IsDeleted { get; set; }
    public DateTime? DeletedAtUtc { get; set; }
    public string? DeletedBy { get; set; }

    private Vendor() { }

    public Vendor(string code, string name, string taxCode, string? address = null, string? phone = null, string? email = null, int paymentTermsDays = 30)
    {
        Id = Guid.NewGuid();
        Code = code.Trim().ToUpperInvariant();
        Name = name.Trim();
        TaxCode = taxCode.Trim();
        Address = address?.Trim();
        Phone = phone?.Trim();
        Email = email?.Trim();
        PaymentTermsDays = paymentTermsDays;
    }

    public void AdjustBalance(decimal delta) => CurrentPayableBalance += delta;
}

public class PurchaseOrder : AggregateRoot<Guid>
{
    public string OrderNumber { get; private set; } = string.Empty; // PO-...
    public Guid VendorId { get; private set; }
    public Vendor? Vendor { get; private set; }
    public DateTime OrderDate { get; private set; }
    public DateTime? ExpectedDeliveryDate { get; private set; }
    public decimal TotalAmount { get; private set; }
    public VoucherStatus Status { get; private set; }

    private readonly List<PurchaseOrderLine> _lines = [];
    public IReadOnlyCollection<PurchaseOrderLine> Lines => _lines.AsReadOnly();

    private PurchaseOrder() { }

    public PurchaseOrder(string orderNumber, Guid vendorId, DateTime orderDate, DateTime? expectedDeliveryDate = null)
    {
        Id = Guid.NewGuid();
        OrderNumber = orderNumber.Trim();
        VendorId = vendorId;
        OrderDate = orderDate;
        ExpectedDeliveryDate = expectedDeliveryDate;
        Status = VoucherStatus.Draft;
    }

    public void AddLine(Guid itemId, string itemName, decimal quantity, decimal unitPrice)
    {
        var line = new PurchaseOrderLine(Id, itemId, itemName, quantity, unitPrice);
        _lines.Add(line);
        TotalAmount = _lines.Sum(l => l.LineTotal);
    }
}

public class PurchaseOrderLine : Entity<Guid>
{
    public Guid PurchaseOrderId { get; private set; }
    public Guid ProductItemId { get; private set; }
    public string ItemName { get; private set; } = string.Empty;
    public decimal Quantity { get; private set; }
    public decimal UnitPrice { get; private set; }
    public decimal LineTotal => Quantity * UnitPrice;

    private PurchaseOrderLine() { }

    public PurchaseOrderLine(Guid poId, Guid itemId, string itemName, decimal quantity, decimal unitPrice)
    {
        Id = Guid.NewGuid();
        PurchaseOrderId = poId;
        ProductItemId = itemId;
        ItemName = itemName;
        Quantity = quantity;
        UnitPrice = unitPrice;
    }
}

public class GoodsReceiptNote : AggregateRoot<Guid>
{
    public string NoteNumber { get; private set; } = string.Empty; // PNK-...
    public Guid? PurchaseOrderId { get; private set; }
    public Guid VendorId { get; private set; }
    public Guid WarehouseId { get; private set; }
    public DateTime ReceiptDate { get; private set; }
    public decimal TotalCost { get; private set; }
    public Guid? CorrespondingVoucherId { get; private set; }

    private readonly List<GoodsReceiptNoteLine> _lines = [];
    public IReadOnlyCollection<GoodsReceiptNoteLine> Lines => _lines.AsReadOnly();

    private GoodsReceiptNote() { }

    public GoodsReceiptNote(string noteNumber, Guid vendorId, Guid warehouseId, DateTime receiptDate, Guid? poId = null)
    {
        Id = Guid.NewGuid();
        NoteNumber = noteNumber.Trim();
        VendorId = vendorId;
        WarehouseId = warehouseId;
        ReceiptDate = receiptDate;
        PurchaseOrderId = poId;
    }

    public void AddLine(Guid itemId, decimal quantity, decimal unitCost)
    {
        var line = new GoodsReceiptNoteLine(Id, itemId, quantity, unitCost);
        _lines.Add(line);
        TotalCost = _lines.Sum(l => l.LineCost);
    }

    public void LinkVoucher(Guid voucherId) => CorrespondingVoucherId = voucherId;
}

public class GoodsReceiptNoteLine : Entity<Guid>
{
    public Guid GoodsReceiptNoteId { get; private set; }
    public Guid ProductItemId { get; private set; }
    public decimal Quantity { get; private set; }
    public decimal UnitCost { get; private set; }
    public decimal LineCost => Quantity * UnitCost;

    private GoodsReceiptNoteLine() { }

    public GoodsReceiptNoteLine(Guid grnId, Guid itemId, decimal quantity, decimal unitCost)
    {
        Id = Guid.NewGuid();
        GoodsReceiptNoteId = grnId;
        ProductItemId = itemId;
        Quantity = quantity;
        UnitCost = unitCost;
    }
}

public class VendorInvoice : AggregateRoot<Guid>
{
    public string InvoiceNumber { get; private set; } = string.Empty;
    public string InvoiceSeries { get; private set; } = string.Empty;
    public DateTime InvoiceDate { get; private set; }
    public Guid VendorId { get; private set; }
    public Vendor? Vendor { get; private set; }
    public decimal Subtotal { get; private set; }
    public VatRate VatRate { get; private set; }
    public decimal VatAmount { get; private set; }
    public decimal TotalAmount { get; private set; }
    public DateTime DueDate { get; private set; }
    public decimal PaidAmount { get; private set; }
    public bool IsFullyPaid => PaidAmount >= TotalAmount;
    public ThreeWayMatchStatus MatchStatus { get; private set; } = ThreeWayMatchStatus.Unmatched;

    private VendorInvoice() { }

    public VendorInvoice(
        string invoiceNumber,
        string invoiceSeries,
        DateTime invoiceDate,
        Guid vendorId,
        decimal subtotal,
        VatRate vatRate,
        DateTime dueDate)
    {
        Id = Guid.NewGuid();
        InvoiceNumber = invoiceNumber.Trim();
        InvoiceSeries = invoiceSeries.Trim();
        InvoiceDate = invoiceDate;
        VendorId = vendorId;
        Subtotal = subtotal;
        VatRate = vatRate;
        VatAmount = vatRate == VatRate.Exempt ? 0m : decimal.Round(subtotal * ((int)vatRate / 100m), 2);
        TotalAmount = Subtotal + VatAmount;
        DueDate = dueDate;
    }

    public void RecordPayment(decimal amount) => PaidAmount += amount;
    public void SetMatchStatus(ThreeWayMatchStatus status) => MatchStatus = status;
}
