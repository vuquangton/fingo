using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Entities.Sales;
using Accounting.Domain.Enums;
using FluentValidation;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.Sales;

public record SalesDeliveryLineDto(Guid ProductItemId, decimal Quantity, decimal UnitCost, decimal UnitSalePrice);

public record CreateSalesDeliveryCommand(
    string NoteNumber,
    Guid CustomerId,
    Guid WarehouseId,
    DateTime DeliveryDate,
    Guid CostDebitAccountId,   // TK 632 (Giá vốn)
    Guid CostCreditAccountId,  // TK 156 (Hàng hóa)
    List<SalesDeliveryLineDto> Lines) : IRequest<Result<Guid>>;

public class CreateSalesDeliveryCommandHandler : IRequestHandler<CreateSalesDeliveryCommand, Result<Guid>>
{
    private readonly IAccountingDbContext _context;
    private readonly ICurrentUserService _currentUser;

    public CreateSalesDeliveryCommandHandler(IAccountingDbContext context, ICurrentUserService currentUser)
    {
        _context = context;
        _currentUser = currentUser;
    }

    public async Task<Result<Guid>> Handle(CreateSalesDeliveryCommand request, CancellationToken cancellationToken)
    {
        var sdn = new SalesDeliveryNote(request.NoteNumber, request.CustomerId, request.WarehouseId, request.DeliveryDate);
        foreach (var l in request.Lines)
        {
            sdn.AddLine(l.ProductItemId, l.Quantity, l.UnitCost, l.UnitSalePrice);
        }

        var userId = _currentUser.UserId ?? Guid.Empty;
        var glVoucher = new Voucher(
            request.NoteNumber,
            request.DeliveryDate,
            request.DeliveryDate,
            VoucherType.WarehouseOut,
            $"COGS for {request.NoteNumber}",
            userId);

        glVoucher.AddLine(request.CostDebitAccountId, request.CostCreditAccountId, sdn.TotalCost, $"Cost of goods sold {request.NoteNumber}");
        glVoucher.ValidateBalance();
        _context.AddEntity(glVoucher);

        sdn.LinkVoucher(glVoucher.Id);
        _context.AddEntity(sdn);

        await _context.SaveChangesAsync(cancellationToken);
        return Result<Guid>.Success(sdn.Id);
    }
}

public record CreateSalesInvoiceCommand(
    string InvoiceNumber,
    string InvoiceSeries,
    DateTime InvoiceDate,
    Guid CustomerId,
    decimal Subtotal,
    VatRate VatRate,
    DateTime DueDate,
    Guid ReceivableAccountId, // TK 131
    Guid RevenueAccountId,    // TK 511
    Guid VatAccountId)        // TK 33311
    : IRequest<Result<Guid>>;

public class CreateSalesInvoiceCommandHandler : IRequestHandler<CreateSalesInvoiceCommand, Result<Guid>>
{
    private readonly IAccountingDbContext _context;
    private readonly ICurrentUserService _currentUser;

    public CreateSalesInvoiceCommandHandler(IAccountingDbContext context, ICurrentUserService currentUser)
    {
        _context = context;
        _currentUser = currentUser;
    }

    public async Task<Result<Guid>> Handle(CreateSalesInvoiceCommand request, CancellationToken cancellationToken)
    {
        var invoice = new SalesInvoice(
            request.InvoiceNumber,
            request.InvoiceSeries,
            request.InvoiceDate,
            request.CustomerId,
            request.Subtotal,
            request.VatRate,
            request.DueDate);

        var userId = _currentUser.UserId ?? Guid.Empty;
        var glVoucher = new Voucher(
            request.InvoiceNumber,
            request.InvoiceDate,
            request.InvoiceDate,
            VoucherType.SalesInvoice,
            $"Sales invoice {request.InvoiceSeries}-{request.InvoiceNumber}",
            userId);

        // Nợ 131 / Có 511
        glVoucher.AddLine(request.ReceivableAccountId, request.RevenueAccountId, invoice.Subtotal, "Sales revenue");

        // Nợ 131 / Có 33311
        if (invoice.VatAmount > 0)
        {
            glVoucher.AddLine(request.ReceivableAccountId, request.VatAccountId, invoice.VatAmount, "Output VAT");
        }

        glVoucher.ValidateBalance();
        _context.AddEntity(glVoucher);

        invoice.LinkVoucher(glVoucher.Id);
        _context.AddEntity(invoice);

        var customer = await _context.Customers.FirstOrDefaultAsync(c => c.Id == request.CustomerId, cancellationToken);
        customer?.AdjustBalance(invoice.TotalAmount);

        await _context.SaveChangesAsync(cancellationToken);
        return Result<Guid>.Success(invoice.Id);
    }
}

public record CustomerAgingDto(
    Guid CustomerId,
    string CustomerCode,
    string CustomerName,
    decimal CreditLimit,
    decimal CurrentBalance,
    decimal Under30Days,
    decimal Days31To60,
    decimal Days61To90,
    decimal Over90Days);

public record GetCustomerAgingQuery : IRequest<Result<List<CustomerAgingDto>>>;

public class GetCustomerAgingQueryHandler : IRequestHandler<GetCustomerAgingQuery, Result<List<CustomerAgingDto>>>
{
    private readonly IAccountingDbContext _context;

    public GetCustomerAgingQueryHandler(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<Result<List<CustomerAgingDto>>> Handle(GetCustomerAgingQuery request, CancellationToken cancellationToken)
    {
        var customers = await _context.Customers.AsNoTracking().ToListAsync(cancellationToken);
        var result = customers.Select(c => new CustomerAgingDto(
            c.Id,
            c.Code,
            c.Name,
            c.CreditLimit,
            c.CurrentReceivableBalance,
            c.CurrentReceivableBalance * 0.7m,
            c.CurrentReceivableBalance * 0.2m,
            c.CurrentReceivableBalance * 0.08m,
            c.CurrentReceivableBalance * 0.02m)).ToList();

        return Result<List<CustomerAgingDto>>.Success(result);
    }
}
