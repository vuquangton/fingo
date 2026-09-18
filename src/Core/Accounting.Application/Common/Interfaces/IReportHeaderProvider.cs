namespace Accounting.Application.Common.Interfaces;

public record ReportHeaderInfo(
    string CompanyName,
    string TaxCode,
    string Address,
    string LegalRepresentative,
    string ChiefAccountant,
    string CurrencyCode);

public interface IReportHeaderProvider
{
    Task<ReportHeaderInfo> GetReportHeaderAsync(CancellationToken cancellationToken = default);
}
