namespace Accounting.Domain.Common;

public readonly record struct CashVoucherId(Guid Value)
{
    public static CashVoucherId New() => new(Guid.NewGuid());
    public static CashVoucherId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct PurchaseInvoiceId(Guid Value)
{
    public static PurchaseInvoiceId New() => new(Guid.NewGuid());
    public static PurchaseInvoiceId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct PurchaseInvoiceLineId(Guid Value)
{
    public static PurchaseInvoiceLineId New() => new(Guid.NewGuid());
    public static PurchaseInvoiceLineId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct SalesInvoiceId(Guid Value)
{
    public static SalesInvoiceId New() => new(Guid.NewGuid());
    public static SalesInvoiceId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct SalesInvoiceLineId(Guid Value)
{
    public static SalesInvoiceLineId New() => new(Guid.NewGuid());
    public static SalesInvoiceLineId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct WarehouseVoucherId(Guid Value)
{
    public static WarehouseVoucherId New() => new(Guid.NewGuid());
    public static WarehouseVoucherId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct WarehouseVoucherLineId(Guid Value)
{
    public static WarehouseVoucherLineId New() => new(Guid.NewGuid());
    public static WarehouseVoucherLineId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}

public readonly record struct InvoicePaymentAllocationId(Guid Value)
{
    public static InvoicePaymentAllocationId New() => new(Guid.NewGuid());
    public static InvoicePaymentAllocationId Empty => new(Guid.Empty);
    public override string ToString() => Value.ToString();
}
