using Accounting.Application.Common.Models;
using MediatR;

namespace Accounting.Application.Features.Settlement;

public record OutstandingInvoiceDto(
    Guid InvoiceId,
    string InvoiceNumber,
    DateOnly InvoiceDate,
    DateOnly DueDate,
    Guid PartnerId,
    string PartnerName,
    decimal TotalAmount,
    decimal SettledAmount,
    decimal RemainingAmount,
    int OverdueDays);

public record GetArOutstandingInvoicesQuery(Guid? CustomerId = null) : IRequest<Result<IReadOnlyList<OutstandingInvoiceDto>>>;
public record GetApOutstandingInvoicesQuery(Guid? VendorId = null) : IRequest<Result<IReadOnlyList<OutstandingInvoiceDto>>>;

public record AgingBucketDto(
    string BucketLabel,
    decimal Amount,
    int InvoiceCount);

public record PartnerAgingSummaryDto(
    Guid PartnerId,
    string PartnerCode,
    string PartnerName,
    decimal TotalOutstanding,
    decimal CurrentNotDue,
    decimal Overdue1To30,
    decimal Overdue31To60,
    decimal Overdue61To90,
    decimal OverdueAbove90);

public record AgingReportDto(
    DateOnly AsOfDate,
    IReadOnlyList<PartnerAgingSummaryDto> PartnerSummaries,
    decimal GrandTotalOutstanding,
    IReadOnlyList<AgingBucketDto> OverallBuckets);

public record GetArAgingReportQuery(DateOnly AsOfDate) : IRequest<Result<AgingReportDto>>;
public record GetApAgingReportQuery(DateOnly AsOfDate) : IRequest<Result<AgingReportDto>>;
