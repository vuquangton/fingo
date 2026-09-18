using Accounting.Domain.Common;
using Accounting.Domain.Enums;

namespace Accounting.Domain.Entities.Tax;

public class VatTransactionRecord : AggregateRoot<Guid>
{
    public VatInvoiceType InvoiceType { get; private set; } // Input (TK 133) or Output (TK 3331)
    public string InvoiceNumber { get; private set; } = string.Empty;
    public string InvoiceSeries { get; private set; } = string.Empty; // Ký hiệu hóa đơn: 1C26TAA...
    public DateTime InvoiceDate { get; private set; }
    public string PartnerTaxCode { get; private set; } = string.Empty; // Mã số thuế đối tác
    public string PartnerName { get; private set; } = string.Empty;
    public string Description { get; private set; } = string.Empty;
    public decimal TaxableAmount { get; private set; }
    public VatRate VatRate { get; private set; }
    public decimal VatAmount { get; private set; }
    public Guid? CorrespondingVoucherId { get; private set; }

    private VatTransactionRecord() { }

    public VatTransactionRecord(
        VatInvoiceType invoiceType,
        string invoiceNumber,
        string invoiceSeries,
        DateTime invoiceDate,
        string partnerTaxCode,
        string partnerName,
        string description,
        decimal taxableAmount,
        VatRate vatRate,
        Guid? correspondingVoucherId = null)
    {
        Id = Guid.NewGuid();
        InvoiceType = invoiceType;
        InvoiceNumber = invoiceNumber.Trim();
        InvoiceSeries = invoiceSeries.Trim();
        InvoiceDate = invoiceDate;
        PartnerTaxCode = partnerTaxCode.Trim();
        PartnerName = partnerName.Trim();
        Description = description.Trim();
        TaxableAmount = taxableAmount;
        VatRate = vatRate;
        VatAmount = vatRate == VatRate.Exempt ? 0m : decimal.Round(taxableAmount * ((int)vatRate / 100m), 2);
        CorrespondingVoucherId = correspondingVoucherId;
    }
}
