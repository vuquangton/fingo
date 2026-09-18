using Accounting.Application.Features.EInvoice;
using Accounting.Domain.Receivables;

namespace Accounting.Infrastructure.Compliance.EInvoice;

public interface IEInvoiceProviderAdapter
{
    string ProviderKey { get; }
    Task<EInvoiceSubmissionResult> IssueInvoiceAsync(SalesInvoice invoice, CancellationToken cancellationToken = default);
    Task<NormalizedIncomingInvoiceDto> ParseIncomingXmlAsync(string xmlContent, CancellationToken cancellationToken = default);
}
