using System.Text.RegularExpressions;
using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;

namespace Accounting.Domain.Organization;

public class CompanySetting : AggregateRoot<Guid>, IAuditableEntity
{
    private static readonly Regex TaxCodeRegex = new(@"^\d{10}(-\d{3})?$", RegexOptions.Compiled);

    public string TaxCode { get; private set; } = string.Empty;
    public string CompanyName { get; private set; } = string.Empty;
    public string Address { get; private set; } = string.Empty;
    public string LegalRepresentative { get; private set; } = string.Empty;
    public string ChiefAccountant { get; private set; } = string.Empty;
    public string CurrencyCode { get; private set; } = "VND";
    public GoverningCircular GoverningCircular { get; private set; } = GoverningCircular.TT99_2025_BTC;
    public int FiscalYearStartMonth { get; private set; } = 1;
    public string? ContactPhone { get; private set; }
    public string? ContactEmail { get; private set; }
    public string? TaxOffice { get; private set; }

    // IAuditableEntity
    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public string? CreatedBy { get; set; }
    public DateTime? UpdatedAtUtc { get; set; }
    public string? UpdatedBy { get; set; }
    public DateTime? LastModifiedAtUtc { get => UpdatedAtUtc; set => UpdatedAtUtc = value; }
    public string? LastModifiedBy { get => UpdatedBy; set => UpdatedBy = value; }

    private CompanySetting() { }

    public CompanySetting(
        Guid id,
        string taxCode,
        string companyName,
        string address,
        string legalRepresentative,
        string chiefAccountant,
        string currencyCode = "VND",
        GoverningCircular governingCircular = GoverningCircular.TT99_2025_BTC,
        int fiscalYearStartMonth = 1,
        string? contactPhone = null,
        string? contactEmail = null,
        string? taxOffice = null)
    {
        Validate(taxCode, companyName, address, legalRepresentative, chiefAccountant);

        Id = id;
        TaxCode = taxCode.Trim();
        CompanyName = companyName.Trim();
        Address = address.Trim();
        LegalRepresentative = legalRepresentative.Trim();
        ChiefAccountant = chiefAccountant.Trim();
        CurrencyCode = string.IsNullOrWhiteSpace(currencyCode) ? "VND" : currencyCode.Trim().ToUpperInvariant();
        GoverningCircular = governingCircular;
        FiscalYearStartMonth = fiscalYearStartMonth is >= 1 and <= 12 ? fiscalYearStartMonth : 1;
        ContactPhone = contactPhone?.Trim();
        ContactEmail = contactEmail?.Trim();
        TaxOffice = taxOffice?.Trim();
    }

    public void UpdateProfile(
        string taxCode,
        string companyName,
        string address,
        string legalRepresentative,
        string chiefAccountant,
        string? contactPhone = null,
        string? contactEmail = null,
        string? taxOffice = null)
    {
        Validate(taxCode, companyName, address, legalRepresentative, chiefAccountant);

        TaxCode = taxCode.Trim();
        CompanyName = companyName.Trim();
        Address = address.Trim();
        LegalRepresentative = legalRepresentative.Trim();
        ChiefAccountant = chiefAccountant.Trim();
        ContactPhone = contactPhone?.Trim();
        ContactEmail = contactEmail?.Trim();
        TaxOffice = taxOffice?.Trim();
        UpdatedAtUtc = DateTime.UtcNow;
    }

    private static void Validate(string taxCode, string companyName, string address, string legalRepresentative, string chiefAccountant)
    {
        if (string.IsNullOrWhiteSpace(taxCode))
            throw new ArgumentException("Mã số thuế doanh nghiệp không được để trống.", nameof(taxCode));

        var cleanTax = taxCode.Trim();
        if (!TaxCodeRegex.IsMatch(cleanTax))
            throw new ArgumentException($"Mã số thuế '{taxCode}' không đúng định dạng chuẩn (10 chữ số hoặc 10 chữ số kèm 3 số chi nhánh).", nameof(taxCode));

        if (string.IsNullOrWhiteSpace(companyName))
            throw new ArgumentException("Tên doanh nghiệp không được để trống.", nameof(companyName));

        if (string.IsNullOrWhiteSpace(address))
            throw new ArgumentException("Địa chỉ trụ sở doanh nghiệp không được để trống.", nameof(address));

        if (string.IsNullOrWhiteSpace(legalRepresentative))
            throw new ArgumentException("Tên người đại diện pháp luật (Giám đốc) không được để trống.", nameof(legalRepresentative));

        if (string.IsNullOrWhiteSpace(chiefAccountant))
            throw new ArgumentException("Tên Kế toán trưởng / Người phụ trách kế toán không được để trống.", nameof(chiefAccountant));
    }
}
