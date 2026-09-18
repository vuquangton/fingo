using Accounting.Application.Common.Models;
using Accounting.Domain.Settlement;
using MediatR;

namespace Accounting.Application.Features.Settlement;

public record AllocatePaymentCommand(
    Guid InvoiceId,
    AllocationInvoiceType InvoiceType,
    Guid PaymentVoucherId,
    Guid PartnerId,
    DateOnly AllocationDate,
    decimal Amount,
    string Description,
    string AllocatedBy) : IRequest<Result<Guid>>;

public record ReverseAllocationCommand(
    Guid AllocationId,
    string ReversedBy,
    string Reason) : IRequest<Result<Unit>>;
