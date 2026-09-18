using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.Settlement;

public enum AllocationInvoiceType
{
    SalesInvoice = 1,
    PurchaseInvoice = 2
}

public class InvoicePaymentAllocation : Entity<InvoicePaymentAllocationId>, IAuditableEntity
{
    public Guid InvoiceId { get; private set; }
    public AllocationInvoiceType InvoiceType { get; private set; }
    public VoucherId PaymentVoucherId { get; private set; }
    public PartnerId PartnerId { get; private set; }
    public DateOnly AllocationDate { get; private set; }
    public decimal AllocatedAmount { get; private set; }
    public string Description { get; private set; } = string.Empty;
    public bool IsReversed { get; private set; }
    public DateTime? ReversedAtUtc { get; private set; }
    public string? ReversedBy { get; private set; }

    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    private InvoicePaymentAllocation() { }

    public InvoicePaymentAllocation(
        InvoicePaymentAllocationId id,
        Guid invoiceId,
        AllocationInvoiceType invoiceType,
        VoucherId paymentVoucherId,
        PartnerId partnerId,
        DateOnly allocationDate,
        decimal allocatedAmount,
        string description)
    {
        if (allocatedAmount <= 0)
            throw new ArgumentOutOfRangeException(nameof(allocatedAmount), "Allocated amount must be strictly positive.");

        Id = id;
        InvoiceId = invoiceId;
        InvoiceType = invoiceType;
        PaymentVoucherId = paymentVoucherId;
        PartnerId = partnerId;
        AllocationDate = allocationDate;
        AllocatedAmount = allocatedAmount;
        Description = description.Trim();
        IsReversed = false;
    }

    public void Reverse(string reversedBy)
    {
        if (IsReversed)
            throw new InvalidOperationException("Allocation is already reversed.");

        IsReversed = true;
        ReversedBy = reversedBy.Trim();
        ReversedAtUtc = DateTime.UtcNow;
        UpdatedAtUtc = DateTime.UtcNow;
    }
}
