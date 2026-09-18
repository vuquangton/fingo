using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Common;
using Accounting.Domain.Costing;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Ledger;
using Accounting.Domain.Manufacturing;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.WarehouseOperations;
using MediatR;
using Microsoft.EntityFrameworkCore;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;
using Voucher = Accounting.Domain.Ledger.Voucher;
using VoucherType = Accounting.Domain.Ledger.VoucherType;

namespace Accounting.Application.Features.Manufacturing;

public record BomLineDto(
    Guid MaterialItemId,
    decimal StandardQuantity,
    decimal ScrapPercentage = 0m);

public record CreateBillOfMaterialsCommand(
    Guid FinishedGoodItemId,
    string Version,
    string Description,
    IReadOnlyList<BomLineDto> Lines) : IRequest<Result<BomId>>;

public class CreateBillOfMaterialsCommandHandler(
    IAccountingDbContext context) : IRequestHandler<CreateBillOfMaterialsCommand, Result<BomId>>
{
    public async Task<Result<BomId>> Handle(CreateBillOfMaterialsCommand request, CancellationToken cancellationToken)
    {
        var existing = await context.ManufacturingBoms
            .AnyAsync(b => b.FinishedGoodItemId == new InventoryItemId(request.FinishedGoodItemId) && b.Version == request.Version.Trim(), cancellationToken);

        if (existing)
            return Result<BomId>.Failure($"BOM for item '{request.FinishedGoodItemId}' version '{request.Version}' already exists.");

        var bomId = BomId.New();
        var bom = new BillOfMaterials(
            bomId,
            new InventoryItemId(request.FinishedGoodItemId),
            request.Version,
            request.Description);

        foreach (var l in request.Lines)
        {
            bom.AddLine(new InventoryItemId(l.MaterialItemId), l.StandardQuantity, l.ScrapPercentage);
        }

        context.AddEntity(bom);
        await context.SaveChangesAsync(cancellationToken);

        return Result<BomId>.Success(bom.Id);
    }
}

public record ManufacturingCostResultDto(
    Guid AbsorptionRunId,
    decimal TotalManufacturingCost,
    decimal UnitCostPerItem,
    Guid ClearanceVoucherId,
    Guid CapitalizationVoucherId);

public record CalculateManufacturingCostCommand(
    int Year,
    int Month,
    Guid FinishedGoodItemId,
    decimal ProducedQuantity,
    string WarehouseId,
    decimal DirectMaterialCost,
    decimal DirectLaborCost,
    decimal OverheadCost,
    decimal WipBeginning = 0m,
    decimal WipEnding = 0m,
    string WipAccountCode = "154",
    string DirectMaterialAccountCode = "621",
    string DirectLaborAccountCode = "622",
    string OverheadAccountCode = "627",
    string FinishedGoodAccountCode = "1551",
    string PostedBy = "cost_accountant") : IRequest<Result<ManufacturingCostResultDto>>;

public class CalculateManufacturingCostCommandHandler(
    IAccountingDbContext context,
    IGlVoucherBridgeService glBridge) : IRequestHandler<CalculateManufacturingCostCommand, Result<ManufacturingCostResultDto>>
{
    public async Task<Result<ManufacturingCostResultDto>> Handle(CalculateManufacturingCostCommand request, CancellationToken cancellationToken)
    {
        var periodStartDate = new DateOnly(request.Year, request.Month, 1);
        var periodEndDate = periodStartDate.AddMonths(1).AddDays(-1);
        var fiscalPeriodId = FiscalPeriodId.FromYearMonth(request.Year, request.Month);
        var finishedGoodItemId = new InventoryItemId(request.FinishedGoodItemId);
        var warehouseId = new WarehouseId(request.WarehouseId);

        var runId = CostAbsorptionRunId.New();
        var run = new CostAbsorptionRun(
            runId,
            fiscalPeriodId,
            finishedGoodItemId,
            request.ProducedQuantity,
            request.DirectMaterialCost,
            request.DirectLaborCost,
            request.OverheadCost,
            request.WipBeginning,
            request.WipEnding);

        var incurredCost = request.DirectMaterialCost + request.DirectLaborCost + request.OverheadCost;

        // 1. Voucher 1: Cost Clearance to WIP (Nợ 154 / Có 621, 622, 627)
        var clearanceVoucher = new Voucher(
            VoucherId.New(),
            $"KC-CP-{request.Year}{request.Month:D2}",
            VoucherType.GeneralJournal,
            periodEndDate,
            periodEndDate,
            $"Period cost clearance to WIP for {request.Month:D2}/{request.Year}",
            new CurrencyCode("VND"),
            1.0m);

        if (incurredCost > 0m)
        {
            clearanceVoucher.AddLine(
                new AccountId(request.WipAccountCode),
                LedgerEntryType.Debit,
                incurredCost,
                "Transfer direct and overhead manufacturing costs to WIP");

            if (request.DirectMaterialCost > 0m)
            {
                clearanceVoucher.AddLine(
                    new AccountId(request.DirectMaterialAccountCode),
                    LedgerEntryType.Credit,
                    request.DirectMaterialCost,
                    "Clear direct material expenses");
            }

            if (request.DirectLaborCost > 0m)
            {
                clearanceVoucher.AddLine(
                    new AccountId(request.DirectLaborAccountCode),
                    LedgerEntryType.Credit,
                    request.DirectLaborCost,
                    "Clear direct labor expenses");
            }

            if (request.OverheadCost > 0m)
            {
                clearanceVoucher.AddLine(
                    new AccountId(request.OverheadAccountCode),
                    LedgerEntryType.Credit,
                    request.OverheadCost,
                    "Clear manufacturing overhead expenses");
            }

            var clearResult = await glBridge.PostOperationalVoucherAsync(clearanceVoucher, request.PostedBy, cancellationToken);
            if (!clearResult.IsSuccess)
                return Result<ManufacturingCostResultDto>.Failure(clearResult.ErrorMessage ?? "Failed to post cost clearance voucher.");
        }

        // 2. Voucher 2: Finished Goods Capitalization (Nợ 1551 / Có 154)
        var receiptVoucher = new Voucher(
            VoucherId.New(),
            $"NK-TP-{request.Year}{request.Month:D2}",
            VoucherType.InventoryReceipt,
            periodEndDate,
            periodEndDate,
            $"Capitalize finished goods {finishedGoodItemId.Value} for period {request.Month:D2}/{request.Year}",
            new CurrencyCode("VND"),
            1.0m);

        receiptVoucher.AddLine(
            new AccountId(request.FinishedGoodAccountCode),
            LedgerEntryType.Debit,
            run.TotalManufacturingCost,
            $"Finished goods stock receipt ({request.ProducedQuantity:N2} units)",
            warehouseId: warehouseId);

        receiptVoucher.AddLine(
            new AccountId(request.WipAccountCode),
            LedgerEntryType.Credit,
            run.TotalManufacturingCost,
            "Transfer WIP to finished goods inventory");

        var receiptResult = await glBridge.PostOperationalVoucherAsync(receiptVoucher, request.PostedBy, cancellationToken);
        if (!receiptResult.IsSuccess)
            return Result<ManufacturingCostResultDto>.Failure(receiptResult.ErrorMessage ?? "Failed to post finished goods receipt voucher.");

        run.LinkVouchers(clearanceVoucher.Id, receiptVoucher.Id);
        context.AddEntity(run);

        // Also add InventoryLayer for FIFO tracking of finished goods
        var layer = new InventoryLayer(
            InventoryLayerId.New(),
            warehouseId,
            finishedGoodItemId,
            periodEndDate,
            request.ProducedQuantity,
            run.UnitCostPerItem,
            receiptVoucher.Id);

        context.AddEntity(layer);

        await context.SaveChangesAsync(cancellationToken);

        return Result<ManufacturingCostResultDto>.Success(new ManufacturingCostResultDto(
            run.Id.Value,
            run.TotalManufacturingCost,
            run.UnitCostPerItem,
            clearanceVoucher.Id.Value,
            receiptVoucher.Id.Value));
    }
}
