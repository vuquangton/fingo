using Accounting.Domain.Common;
using Accounting.Domain.Enums;
using Accounting.Domain.Exceptions;

namespace Accounting.Domain.Entities.Sales;

public class Customer : AggregateRoot<Guid>, IAuditableEntity, ISoftDeletable
{
    public string Code { get; private set; } = string.Empty;
    public string Name { get; private set; } = string.Empty;
    public string TaxCode { get; private set; } = string.Empty;
    public string? Address { get; private set; }
    public string? Phone { get; private set; }
    public string? Email { get; private set; }
    public decimal CreditLimit { get; private set; }
    public int CreditTermDays { get; private set; } = 30;
    public decimal CurrentReceivableBalance { get; private set; }
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

    private Customer() { }

    public Customer(string code, string name, string taxCode, decimal creditLimit, int creditTermDays = 30, string? address = null, string? phone = null, string? email = null)
    {
        Id = Guid.NewGuid();
        Code = code.Trim().ToUpperInvariant();
        Name = name.Trim();
        TaxCode = taxCode.Trim();
        CreditLimit = creditLimit;
        CreditTermDays = creditTermDays;
        Address = address?.Trim();
        Phone = phone?.Trim();
        Email = email?.Trim();
    }

    public void CheckCreditLimit(decimal additionalAmount)
    {
        if (CreditLimit > 0 && (CurrentReceivableBalance + additionalAmount) > CreditLimit)
            throw new CreditLimitExceededException(Code, CreditLimit, CurrentReceivableBalance + additionalAmount);
    }

    public void AdjustBalance(decimal delta) => CurrentReceivableBalance += delta;
}

public class SalesDeliveryNote : AggregateRoot<Guid>
{
    public string NoteNumber { get; private set; } = string.Empty; // PXK-...
    public Guid CustomerId { get; private set; }
    public Customer? Customer { get; private set; }
    public Guid WarehouseId { get; private set; }
    public DateTime DeliveryDate { get; private set; }
    public decimal TotalCost { get; private set; }
    public decimal TotalSaleValue { get; private set; }
    public Guid? CorrespondingVoucherId { get; private set; }

    private readonly List<SalesDeliveryNoteLine> _lines = [];
    public IReadOnlyCollection<SalesDeliveryNoteLine> Lines => _lines.AsReadOnly();

    private SalesDeliveryNote() { }

    public SalesDeliveryNote(string noteNumber, Guid customerId, Guid warehouseId, DateTime deliveryDate)
    {
        Id = Guid.NewGuid();
        NoteNumber = noteNumber.Trim();
        CustomerId = customerId;
        WarehouseId = warehouseId;
        DeliveryDate = deliveryDate;
    }

    public void AddLine(Guid itemId, decimal quantity, decimal unitCost, decimal unitSalePrice)
    {
        var line = new SalesDeliveryNoteLine(Id, itemId, quantity, unitCost, unitSalePrice);
        _lines.Add(line);
        TotalCost = _lines.Sum(l => l.LineCost);
        TotalSaleValue = _lines.Sum(l => l.LineSaleValue);
    }

    public void LinkVoucher(Guid voucherId) => CorrespondingVoucherId = voucherId;
}

public class SalesDeliveryNoteLine : Entity<Guid>
{
    public Guid SalesDeliveryNoteId { get; private set; }
    public Guid ProductItemId { get; private set; }
    public decimal Quantity { get; private set; }
    public decimal UnitCost { get; private set; }
    public decimal UnitSalePrice { get; private set; }
    public decimal LineCost => Quantity * UnitCost;
    public decimal LineSaleValue => Quantity * UnitSalePrice;

    private SalesDeliveryNoteLine() { }

    public SalesDeliveryNoteLine(Guid sdnId, Guid itemId, decimal quantity, decimal unitCost, decimal unitSalePrice)
    {
        Id = Guid.NewGuid();
        SalesDeliveryNoteId = sdnId;
        ProductItemId = itemId;
        Quantity = quantity;
        UnitCost = unitCost;
        UnitSalePrice = unitSalePrice;
    }
}

public class SalesInvoice : AggregateRoot<Guid>
{
    public string InvoiceNumber { get; private set; } = string.Empty;
    public string InvoiceSeries { get; private set; } = string.Empty;
    public DateTime InvoiceDate { get; private set; }
    public Guid CustomerId { get; private set; }
    public Customer? Customer { get; private set; }
    public decimal Subtotal { get; private set; }
    public VatRate VatRate { get; private set; }
    public decimal VatAmount { get; private set; }
    public decimal TotalAmount { get; private set; }
    public DateTime DueDate { get; private set; }
    public decimal PaidAmount { get; private set; }
    public bool IsFullyPaid => PaidAmount >= TotalAmount;
    public Guid? CorrespondingVoucherId { get; private set; }

    private SalesInvoice() { }

    public SalesInvoice(
        string invoiceNumber,
        string invoiceSeries,
        DateTime invoiceDate,
        Guid customerId,
        decimal subtotal,
        VatRate vatRate,
        DateTime dueDate)
    {
        Id = Guid.NewGuid();
        InvoiceNumber = invoiceNumber.Trim();
        InvoiceSeries = invoiceSeries.Trim();
        InvoiceDate = invoiceDate;
        CustomerId = customerId;
        Subtotal = subtotal;
        VatRate = vatRate;
        VatAmount = vatRate == VatRate.Exempt ? 0m : decimal.Round(subtotal * ((int)vatRate / 100m), 2);
        TotalAmount = Subtotal + VatAmount;
        DueDate = dueDate;
    }

    public void RecordPayment(decimal amount) => PaidAmount += amount;
    public void LinkVoucher(Guid voucherId) => CorrespondingVoucherId = voucherId;
}
