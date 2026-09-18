using Accounting.Application.Common.Models;
using MediatR;

namespace Accounting.Application.Features.EInvoice;

public record NormalizedInvoiceLineDto(
    int LineNumber,
    string ItemName,
    string UnitName,
    decimal Quantity,
    decimal UnitPrice,
    decimal Amount,
    decimal VatRate,
    decimal VatAmount);

public record NormalizedIncomingInvoiceDto(
    string InvoiceTemplate,
    string InvoiceSeries,
    string InvoiceNumber,
    DateOnly InvoiceDate,
    string SellerTaxCode,
    string SellerName,
    string SellerAddress,
    string BuyerTaxCode,
    string BuyerName,
    decimal SubTotalAmount,
    decimal VatAmount,
    decimal TotalAmount,
    IReadOnlyList<NormalizedInvoiceLineDto> Lines);

public record EInvoiceSubmissionResult(
    bool Success,
    string? ProviderTransactionId,
    string? InvoiceCode,
    string? ReservationCode,
    string? ErrorMessage);

public record ParseIncomingXmlInvoiceCommand(string XmlContent) : IRequest<Result<NormalizedIncomingInvoiceDto>>;

public record SubmitOutgoingInvoiceCommand(
    Guid SalesInvoiceId,
    string ProviderKey = "MOCK") : IRequest<Result<EInvoiceSubmissionResult>>;
