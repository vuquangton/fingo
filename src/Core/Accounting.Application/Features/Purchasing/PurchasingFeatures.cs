using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Entities.Purchasing;
using Accounting.Domain.Enums;
using FluentValidation;
using MediatR;
using Microsoft.EntityFrameworkCore;

namespace Accounting.Application.Features.Purchasing;

public record PurchaseOrderLineDto(Guid ProductItemId, string ItemName, decimal Quantity, decimal UnitPrice);

public record CreatePurchaseOrderCommand(
    string OrderNumber,
    Guid VendorId,
    DateTime OrderDate,
    List<PurchaseOrderLineDto> Lines,
    DateTime? ExpectedDeliveryDate = null) : IRequest<Result<Guid>>;

public class CreatePurchaseOrderCommandHandler : IRequestHandler<CreatePurchaseOrderCommand, Result<Guid>>
{
    private readonly IAccountingDbContext _context;

    public CreatePurchaseOrderCommandHandler(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<Result<Guid>> Handle(CreatePurchaseOrderCommand request, CancellationToken cancellationToken)
    {
        var po = new PurchaseOrder(request.OrderNumber, request.VendorId, request.OrderDate, request.ExpectedDeliveryDate);
        foreach (var l in request.Lines)
        {
            po.AddLine(l.ProductItemId, l.ItemName, l.Quantity, l.UnitPrice);
        }

        _context.AddEntity(po);
        await _context.SaveChangesAsync(cancellationToken);
        return Result<Guid>.Success(po.Id);
    }
}

public record GoodsReceiptLineDto(Guid ProductItemId, decimal Quantity, decimal UnitCost);

public record CreateGoodsReceiptCommand(
    string NoteNumber,
    Guid VendorId,
    Guid WarehouseId,
    DateTime ReceiptDate,
    Guid DebitAccountId,  // TK 152, 156...
    Guid CreditAccountId, // TK 331, 111, 112...
    List<GoodsReceiptLineDto> Lines,
    Guid? PurchaseOrderId = null) : IRequest<Result<Guid>>;

public class CreateGoodsReceiptCommandHandler : IRequestHandler<CreateGoodsReceiptCommand, Result<Guid>>
{
    private readonly IAccountingDbContext _context;
    private readonly ICurrentUserService _currentUser;

    public CreateGoodsReceiptCommandHandler(IAccountingDbContext context, ICurrentUserService currentUser)
    {
        _context = context;
        _currentUser = currentUser;
    }

    public async Task<Result<Guid>> Handle(CreateGoodsReceiptCommand request, CancellationToken cancellationToken)
    {
        var grn = new GoodsReceiptNote(request.NoteNumber, request.VendorId, request.WarehouseId, request.ReceiptDate, request.PurchaseOrderId);
        foreach (var line in request.Lines)
        {
            grn.AddLine(line.ProductItemId, line.Quantity, line.UnitCost);
        }

        var userId = _currentUser.UserId ?? Guid.Empty;
        var glVoucher = new Voucher(
            request.NoteNumber,
            request.ReceiptDate,
            request.ReceiptDate,
            VoucherType.WarehouseIn,
            $"Goods receipt {request.NoteNumber}",
            userId);

        glVoucher.AddLine(request.DebitAccountId, request.CreditAccountId, grn.TotalCost, $"Goods receipt {request.NoteNumber}");
        glVoucher.ValidateBalance();
        _context.AddEntity(glVoucher);

        grn.LinkVoucher(glVoucher.Id);
        _context.AddEntity(grn);

        await _context.SaveChangesAsync(cancellationToken);
        return Result<Guid>.Success(grn.Id);
    }
}

public record VendorAgingDto(
    Guid VendorId,
    string VendorCode,
    string VendorName,
    decimal CurrentBalance,
    decimal Under30Days,
    decimal Days31To60,
    decimal Days61To90,
    decimal Over90Days);

public record GetVendorAgingQuery : IRequest<Result<List<VendorAgingDto>>>;

public class GetVendorAgingQueryHandler : IRequestHandler<GetVendorAgingQuery, Result<List<VendorAgingDto>>>
{
    private readonly IAccountingDbContext _context;

    public GetVendorAgingQueryHandler(IAccountingDbContext context)
    {
        _context = context;
    }

    public async Task<Result<List<VendorAgingDto>>> Handle(GetVendorAgingQuery request, CancellationToken cancellationToken)
    {
        var vendors = await _context.Vendors.AsNoTracking().ToListAsync(cancellationToken);
        var result = vendors.Select(v => new VendorAgingDto(
            v.Id,
            v.Code,
            v.Name,
            v.CurrentPayableBalance,
            v.CurrentPayableBalance * 0.6m,
            v.CurrentPayableBalance * 0.25m,
            v.CurrentPayableBalance * 0.10m,
            v.CurrentPayableBalance * 0.05m)).ToList();

        return Result<List<VendorAgingDto>>.Success(result);
    }
}
