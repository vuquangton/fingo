using Accounting.Domain.Common;

namespace Accounting.Domain.Tax;

public class TaxDeclarationSnapshot : Entity<Guid>
{
    public TaxDeclarationType TaxType { get; private set; }
    public string Period { get; private set; } = string.Empty;
    public string CompanyTaxCode { get; private set; } = string.Empty;
    public string CompanyName { get; private set; } = string.Empty;
    public string DirectorName { get; private set; } = string.Empty;
    public string XmlPayload { get; private set; } = string.Empty;
    public DateTime CreatedAtUtc { get; private set; } = DateTime.UtcNow;

    private TaxDeclarationSnapshot() { } // EF Core

    public TaxDeclarationSnapshot(
        Guid id,
        TaxDeclarationType taxType,
        string period,
        string companyTaxCode,
        string companyName,
        string directorName,
        string xmlPayload)
    {
        if (id == Guid.Empty)
            throw new ArgumentException("Snapshot ID cannot be empty.", nameof(id));

        if (string.IsNullOrWhiteSpace(companyTaxCode))
            throw new ArgumentException("Company tax code is mandatory.", nameof(companyTaxCode));

        if (string.IsNullOrWhiteSpace(companyName))
            throw new ArgumentException("Company name is mandatory.", nameof(companyName));

        if (string.IsNullOrWhiteSpace(directorName))
            throw new ArgumentException("Director name is mandatory for statutory tax declarations.", nameof(directorName));

        if (string.IsNullOrWhiteSpace(xmlPayload))
            throw new ArgumentException("XML payload cannot be empty.", nameof(xmlPayload));

        Id = id;
        TaxType = taxType;
        Period = period;
        CompanyTaxCode = companyTaxCode.Trim();
        CompanyName = companyName.Trim();
        DirectorName = directorName.Trim();
        XmlPayload = xmlPayload;
        CreatedAtUtc = DateTime.UtcNow;
    }
}
