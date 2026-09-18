using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Common;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.WarehouseOperations;
using MediatR;
using Microsoft.EntityFrameworkCore;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;
using Voucher = Accounting.Domain.Ledger.Voucher;
using VoucherType = Accounting.Domain.Ledger.VoucherType;

namespace Accounting.Application.Features.WarehouseOperations;

public record PostWarehouseVoucherLineDto(
    Guid InventoryItemId,
    Guid UnitOfMeasureId,
    decimal Quantity,
    decimal UnitPrice,
    string DebitAccountCode,
    string CreditAccountCode,
    string Description);

public record PostWarehouseVoucherCommand(
    string VoucherNumber,
    WarehouseVoucherType VoucherType,
    DateOnly PostingDate,
    string WarehouseId,
    string Description,
    Guid? PartnerId = null,
    string? DestinationWarehouseId = null,
    IReadOnlyList<PostWarehouseVoucherLineDto>? Lines = null,
    string PostedBy = "storekeeper") : IRequest<Result<WarehouseVoucherId>>;

public class PostWarehouseVoucherCommandHandler(
    IAccountingDbContext context,
    IGlVoucherBridgeService glBridge) : IRequestHandler<PostWarehouseVoucherCommand, Result<WarehouseVoucherId>>
{
    private static readonly HashSet<WarehouseVoucherType> OutwardTypes =
    [
        WarehouseVoucherType.OutwardSales,
        WarehouseVoucherType.OutwardProduction,
        WarehouseVoucherType.OutwardInternal
    ];

    public async Task<Result<WarehouseVoucherId>> Handle(PostWarehouseVoucherCommand request, CancellationToken cancellationToken)
    {
        if (request.Lines == null || request.Lines.Count == 0)
            return Result<WarehouseVoucherId>.Failure("Warehouse voucher must contain at least one line item.");

        var warehouseId = new WarehouseId(request.WarehouseId);
        var destWarehouseId = !string.IsNullOrWhiteSpace(request.DestinationWarehouseId)
            ? new WarehouseId(request.DestinationWarehouseId)
            : (WarehouseId?)null;

        var isOutward = OutwardTypes.Contains(request.VoucherType);

        // 1. Stock Availability Check for Outward Transactions (Negative Stock Guardrail)
        if (isOutward)
        {
            foreach (var line in request.Lines)
            {
                var itemId = new InventoryItemId(line.InventoryItemId);

                // Compute historical stock on hand: Sum(Inward) - Sum(Outward)
                var inwardQty = await (
                    from v in context.SubWarehouseVouchers
                    join l in context.SubWarehouseVoucherLines on v.Id equals l.VoucherId
                    where v.WarehouseId == warehouseId && l.InventoryItemId == itemId
                          && !OutwardTypes.Contains(v.VoucherType)
                    select (decimal?)l.Quantity).SumAsync(cancellationToken) ?? 0m;

                var outwardQty = await (
                    from v in context.SubWarehouseVouchers
                    join l in context.SubWarehouseVoucherLines on v.Id equals l.VoucherId
                    where v.WarehouseId == warehouseId && l.InventoryItemId == itemId
                          && OutwardTypes.Contains(v.VoucherType)
                    select (decimal?)l.Quantity).SumAsync(cancellationToken) ?? 0m;

                var availableQty = inwardQty - outwardQty;
                if (line.Quantity > availableQty)
                {
                    throw new InsufficientStockException(
                        itemId.Value.ToString(),
                        warehouseId.Value,
                        line.Quantity,
                        availableQty);
                }
            }
        }

        // 2. Build Warehouse Voucher Aggregate
        var voucher = new WarehouseVoucher(
            WarehouseVoucherId.New(),
            request.VoucherNumber,
            request.VoucherType,
            request.PostingDate,
            warehouseId,
            request.Description,
            destWarehouseId);

        foreach (var l in request.Lines)
        {
            voucher.AddLine(
                new InventoryItemId(l.InventoryItemId),
                new UomId(l.UnitOfMeasureId),
                l.Quantity,
                l.UnitPrice,
                new AccountId(l.DebitAccountCode),
                new AccountId(l.CreditAccountCode),
                l.Description);
        }

        // 3. Project Phase 3 GL Voucher
        var glVoucherType = isOutward ? VoucherType.InventoryIssue : VoucherType.InventoryReceipt;
        var glVoucher = new Voucher(
            VoucherId.New(),
            request.VoucherNumber,
            glVoucherType,
            request.PostingDate,
            request.PostingDate,
            request.Description,
            new CurrencyCode("VND"),
            1.0m);

        var partnerId = request.PartnerId.HasValue ? new PartnerId(request.PartnerId.Value) : (PartnerId?)null;

        foreach (var l in voucher.Lines)
        {
            // Debit inventory or COGS account (with Warehouse and optional Partner dimension)
            glVoucher.AddLine(
                l.DebitAccountId,
                LedgerEntryType.Debit,
                l.TotalAmount,
                l.Description,
                partnerId: partnerId,
                warehouseId: warehouseId);

            // Credit inventory or counter account (with Warehouse and optional Partner dimension)
            glVoucher.AddLine(
                l.CreditAccountId,
                LedgerEntryType.Credit,
                l.TotalAmount,
                l.Description,
                partnerId: partnerId,
                warehouseId: warehouseId);
        }

        context.AddEntity(voucher);

        var glResult = await glBridge.PostOperationalVoucherAsync(glVoucher, request.PostedBy, cancellationToken);
        if (!glResult.IsSuccess)
            return Result<WarehouseVoucherId>.Failure(glResult.ErrorMessage ?? "Failed to post GL voucher for warehouse operations.");

        voucher.LinkGeneralLedgerVoucher(glVoucher.Id);
        await context.SaveChangesAsync(cancellationToken);

        return Result<WarehouseVoucherId>.Success(voucher.Id);
    }
}
