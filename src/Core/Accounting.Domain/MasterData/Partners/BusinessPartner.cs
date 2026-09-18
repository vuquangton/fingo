using System.Text.RegularExpressions;
using Accounting.Domain.Common;
using Accounting.Domain.Exceptions;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.MasterData.Partners;

public class BusinessPartner : AggregateRoot<PartnerId>, IAuditableEntity, ISoftDeletable
{
    private static readonly Regex TaxCodeRegex = new(@"^\d{10}(-\d{3})?$", RegexOptions.Compiled);

    public string PartnerCode { get; private set; } = string.Empty;
    public string Name { get; private set; } = string.Empty;
    public PartnerType PartnerType { get; private set; }
    public LegalEntityType LegalEntityType { get; private set; } = LegalEntityType.Corporate;
    public string? TaxCode { get; private set; }
    public string? TaxAuthorityCode { get; private set; }
    public string? Address { get; private set; }
    public string? ContactPhone { get; private set; }
    public string? ContactEmail { get; private set; }
    public string? InvoiceReceivingEmail { get; private set; }
    public decimal CreditLimit { get; private set; }
    public int PaymentTermDays { get; private set; }
    public decimal DiscountRate { get; private set; }
    public decimal CurrentReceivableBalance { get; private set; }
    public decimal CurrentPayableBalance { get; private set; }
    public PartnerRiskTier RiskTier { get; private set; } = PartnerRiskTier.Low;
    public string? PartnerGroupId { get; private set; }
    public Guid? AssignedSalesRepId { get; private set; }
    public bool IsActive { get; private set; } = true;

    // Sub-collections
    private readonly List<PartnerBankAccount> _bankAccounts = [];
    public IReadOnlyCollection<PartnerBankAccount> BankAccounts => _bankAccounts.AsReadOnly();

    private readonly List<PartnerDeliveryAddress> _deliveryAddresses = [];
    public IReadOnlyCollection<PartnerDeliveryAddress> DeliveryAddresses => _deliveryAddresses.AsReadOnly();

    private readonly List<PartnerContact> _contacts = [];
    public IReadOnlyCollection<PartnerContact> Contacts => _contacts.AsReadOnly();

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
        int paymentTermDays = 30,
        LegalEntityType legalEntityType = LegalEntityType.Corporate,
        string? invoiceReceivingEmail = null,
        string? taxAuthorityCode = null,
        decimal discountRate = 0m,
        string? partnerGroupId = null,
        Guid? assignedSalesRepId = null)
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
        LegalEntityType = legalEntityType;
        TaxAuthorityCode = taxAuthorityCode?.Trim();
        Address = address?.Trim();
        ContactPhone = contactPhone?.Trim();
        ContactEmail = contactEmail?.Trim();
        InvoiceReceivingEmail = invoiceReceivingEmail?.Trim();
        CreditLimit = creditLimit >= 0 ? creditLimit : throw new ArgumentOutOfRangeException(nameof(creditLimit), "Credit limit must be non-negative.");
        PaymentTermDays = paymentTermDays >= 0 ? paymentTermDays : 0;
        DiscountRate = discountRate >= 0 ? discountRate : 0m;
        PartnerGroupId = partnerGroupId?.Trim();
        AssignedSalesRepId = assignedSalesRepId;
    }

    public void UpdateProfile(
        string name,
        PartnerType partnerType,
        string? taxCode,
        string? address,
        string? contactPhone,
        string? contactEmail,
        decimal creditLimit,
        int paymentTermDays,
        LegalEntityType? legalEntityType = null,
        string? invoiceReceivingEmail = null,
        string? taxAuthorityCode = null,
        decimal? discountRate = null,
        string? partnerGroupId = null,
        Guid? assignedSalesRepId = null)
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

        if (legalEntityType.HasValue) LegalEntityType = legalEntityType.Value;
        if (invoiceReceivingEmail != null) InvoiceReceivingEmail = invoiceReceivingEmail.Trim();
        if (taxAuthorityCode != null) TaxAuthorityCode = taxAuthorityCode.Trim();
        if (discountRate.HasValue) DiscountRate = discountRate.Value >= 0 ? discountRate.Value : 0m;
        if (partnerGroupId != null) PartnerGroupId = partnerGroupId.Trim();
        if (assignedSalesRepId.HasValue) AssignedSalesRepId = assignedSalesRepId.Value;
    }

    // Collections Management
    public void AddBankAccount(string bankName, string bankCode, string accountNumber, string accountHolder, string? branch = null, bool isDefault = false)
    {
        if (isDefault)
        {
            foreach (var b in _bankAccounts) b.SetDefault(false);
        }
        else if (_bankAccounts.Count == 0)
        {
            isDefault = true;
        }

        _bankAccounts.Add(new PartnerBankAccount(bankName, bankCode, accountNumber, accountHolder, branch, isDefault));
    }

    public void AddDeliveryAddress(string addressLine, string? ward = null, string? district = null, string? city = null, string? receiverName = null, string? receiverPhone = null, bool isDefault = false)
    {
        if (isDefault)
        {
            foreach (var a in _deliveryAddresses) a.SetDefault(false);
        }
        else if (_deliveryAddresses.Count == 0)
        {
            isDefault = true;
        }

        _deliveryAddresses.Add(new PartnerDeliveryAddress(addressLine, ward, district, city, receiverName, receiverPhone, isDefault));
    }

    public void AddContact(string fullName, string? position = null, string? mobile = null, string? email = null, bool isPrimary = false)
    {
        if (isPrimary)
        {
            foreach (var c in _contacts) c.SetPrimary(false);
        }
        else if (_contacts.Count == 0)
        {
            isPrimary = true;
        }

        _contacts.Add(new PartnerContact(fullName, position, mobile, email, isPrimary));
    }

    // Financial & Credit Controls
    public void CheckCreditLimit(decimal additionalAmount)
    {
        if (CreditLimit > 0 && (CurrentReceivableBalance + additionalAmount) > CreditLimit)
            throw new CreditLimitExceededException(PartnerCode, CreditLimit, CurrentReceivableBalance + additionalAmount);
    }

    public void AdjustReceivableBalance(decimal delta)
    {
        CurrentReceivableBalance += delta;
        UpdateRiskTier();
    }

    public void AdjustPayableBalance(decimal delta)
    {
        CurrentPayableBalance += delta;
    }

    public void SetRiskTier(PartnerRiskTier tier) => RiskTier = tier;

    public void UpdateRiskTier()
    {
        if (CreditLimit <= 0)
        {
            RiskTier = PartnerRiskTier.Low;
            return;
        }

        var ratio = CurrentReceivableBalance / CreditLimit;
        if (ratio > 1.0m)
            RiskTier = PartnerRiskTier.Critical;
        else if (ratio >= 0.85m)
            RiskTier = PartnerRiskTier.High;
        else if (ratio >= 0.50m)
            RiskTier = PartnerRiskTier.Medium;
        else
            RiskTier = PartnerRiskTier.Low;
    }

    public void SetActive(bool isActive) => IsActive = isActive;
}
