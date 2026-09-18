using System.Text.RegularExpressions;
using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.MasterData.Partners;

public class BusinessPartner : AggregateRoot<PartnerId>, IAuditableEntity, ISoftDeletable
{
    private static readonly Regex TaxCodeRegex = new(@"^\d{10}(-\d{3})?$", RegexOptions.Compiled);

    public string PartnerCode { get; private set; } = string.Empty;
    public string Name { get; private set; } = string.Empty;
    public PartnerType PartnerType { get; private set; }
    public string? TaxCode { get; private set; }
    public string? Address { get; private set; }
    public string? ContactPhone { get; private set; }
    public string? ContactEmail { get; private set; }
    public decimal CreditLimit { get; private set; }
    public int PaymentTermDays { get; private set; }
    public bool IsActive { get; private set; } = true;

    // IAuditableEntity
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    // ISoftDeletable
    public bool IsDeleted { get; set; }
    public DateTime? DeletedAtUtc { get; set; }
    public string? DeletedBy { get; set; }

    private BusinessPartner() { }

    public BusinessPartner(
        PartnerId id,
        string partnerCode,
        string name,
        PartnerType partnerType,
        string? taxCode = null,
        string? address = null,
        string? contactPhone = null,
        string? contactEmail = null,
        decimal creditLimit = 0m,
        int paymentTermDays = 30)
    {
        var code = partnerCode.Trim().ToUpperInvariant();
        if (string.IsNullOrWhiteSpace(code))
            throw new ArgumentException("Partner code cannot be empty.", nameof(partnerCode));

        if (!string.IsNullOrWhiteSpace(taxCode))
        {
            var trimmedTax = taxCode.Trim();
            if (!TaxCodeRegex.IsMatch(trimmedTax))
                throw new ArgumentException($"Invalid Vietnamese tax code '{taxCode}'. Must be 10 digits or 10-3 digits (e.g. 0101234567 or 0101234567-001).", nameof(taxCode));
            TaxCode = trimmedTax;
        }

        Id = id;
        PartnerCode = code;
        Name = name.Trim();
        PartnerType = partnerType;
        Address = address?.Trim();
        ContactPhone = contactPhone?.Trim();
        ContactEmail = contactEmail?.Trim();
        CreditLimit = creditLimit >= 0 ? creditLimit : throw new ArgumentOutOfRangeException(nameof(creditLimit), "Credit limit must be non-negative.");
        PaymentTermDays = paymentTermDays >= 0 ? paymentTermDays : 0;
    }

    public void UpdateProfile(
        string name,
        PartnerType partnerType,
        string? taxCode,
        string? address,
        string? contactPhone,
        string? contactEmail,
        decimal creditLimit,
        int paymentTermDays)
    {
        if (!string.IsNullOrWhiteSpace(taxCode))
        {
            var trimmedTax = taxCode.Trim();
            if (!TaxCodeRegex.IsMatch(trimmedTax))
                throw new ArgumentException($"Invalid Vietnamese tax code '{taxCode}'.", nameof(taxCode));
            TaxCode = trimmedTax;
        }
        else
        {
            TaxCode = null;
        }

        Name = name.Trim();
        PartnerType = partnerType;
        Address = address?.Trim();
        ContactPhone = contactPhone?.Trim();
        ContactEmail = contactEmail?.Trim();
        CreditLimit = creditLimit >= 0 ? creditLimit : throw new ArgumentOutOfRangeException(nameof(creditLimit));
        PaymentTermDays = paymentTermDays >= 0 ? paymentTermDays : 0;
    }

    public void SetActive(bool isActive) => IsActive = isActive;
}
