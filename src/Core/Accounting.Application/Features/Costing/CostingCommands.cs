using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Application.Costing.Engines;
using Accounting.Domain.Common;
using Accounting.Domain.Costing;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.WarehouseOperations;
using MediatR;
using Microsoft.EntityFrameworkCore;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;
using Voucher = Accounting.Domain.Ledger.Voucher;
using VoucherType = Accounting.Domain.Ledger.VoucherType;

namespace Accounting.Application.Features.Costing;

public record RecordInwardInventoryLayerCommand(
    string WarehouseId,
    Guid InventoryItemId,
    DateOnly ReceiptDate,
    decimal Quantity,
    decimal UnitCost,
    Guid? SourceVoucherId = null) : IRequest<Result<InventoryLayerId>>;

public class RecordInwardInventoryLayerCommandHandler(
    IAccountingDbContext context) : IRequestHandler<RecordInwardInventoryLayerCommand, Result<InventoryLayerId>>
{
    public async Task<Result<InventoryLayerId>> Handle(RecordInwardInventoryLayerCommand request, CancellationToken cancellationToken)
    {
        var layerId = InventoryLayerId.New();
        var layer = new InventoryLayer(
            layerId,
            new WarehouseId(request.WarehouseId),
            new InventoryItemId(request.InventoryItemId),
            request.ReceiptDate,
            request.Quantity,
            request.UnitCost,
            request.SourceVoucherId.HasValue ? new VoucherId(request.SourceVoucherId.Value) : null);

        context.AddEntity(layer);
        await context.SaveChangesAsync(cancellationToken);

        return Result<InventoryLayerId>.Success(layer.Id);
    }
}

public record RunMonthlyWeightedAverageCostingCommand(
    int Year,
    int Month,
    string WarehouseId,
    Guid InventoryItemId,
    string InventoryAccountCode = "1561",
    string CogsAccountCode = "632",
    string PostedBy = "cost_accountant") : IRequest<Result<CostAllocationRunId>>;

public class RunMonthlyWeightedAverageCostingCommandHandler(
    IAccountingDbContext context,
    IGlVoucherBridgeService glBridge) : IRequestHandler<RunMonthlyWeightedAverageCostingCommand, Result<CostAllocationRunId>>
{
    public async Task<Result<CostAllocationRunId>> Handle(RunMonthlyWeightedAverageCostingCommand request, CancellationToken cancellationToken)
    {
        var warehouseId = new WarehouseId(request.WarehouseId);
        var itemId = new InventoryItemId(request.InventoryItemId);
        var periodStartDate = new DateOnly(request.Year, request.Month, 1);
        var periodEndDate = periodStartDate.AddMonths(1).AddDays(-1);

        // 1. Calculate Inward and Outward movements for the period from Warehouse Vouchers
        // Inward voucher types: InwardPurchase = 1, InwardProduction = 2, InwardTransfer = 3
        var inwardLines = await (
            from v in context.SubWarehouseVouchers
            join l in context.SubWarehouseVoucherLines on v.Id equals l.VoucherId
            where v.WarehouseId == warehouseId && l.InventoryItemId == itemId
                  && v.PostingDate >= periodStartDate && v.PostingDate <= periodEndDate
                  && ((int)v.VoucherType == 1 || (int)v.VoucherType == 2 || (int)v.VoucherType == 3)
            select new { l.Quantity, l.TotalAmount }).ToListAsync(cancellationToken);

        var inwardQty = inwardLines.Sum(x => x.Quantity);
        var inwardVal = inwardLines.Sum(x => x.TotalAmount);

        // Outward voucher types: OutwardSales = 4, OutwardProduction = 5, OutwardInternal = 6
        var outwardLines = await (
            from v in context.SubWarehouseVouchers
            join l in context.SubWarehouseVoucherLines on v.Id equals l.VoucherId
            where v.WarehouseId == warehouseId && l.InventoryItemId == itemId
                  && v.PostingDate >= periodStartDate && v.PostingDate <= periodEndDate
                  && ((int)v.VoucherType == 4 || (int)v.VoucherType == 5 || (int)v.VoucherType == 6)
            select new { l.Quantity, l.TotalAmount }).ToListAsync(cancellationToken);

        var outwardQty = outwardLines.Sum(x => x.Quantity);
        var provisionedOutwardCost = outwardLines.Sum(x => x.TotalAmount);

        // Calculate opening stock prior to this period
        var priorInward = await (
            from v in context.SubWarehouseVouchers
            join l in context.SubWarehouseVoucherLines on v.Id equals l.VoucherId
            where v.WarehouseId == warehouseId && l.InventoryItemId == itemId
                  && v.PostingDate < periodStartDate
                  && ((int)v.VoucherType == 1 || (int)v.VoucherType == 2 || (int)v.VoucherType == 3)
            select new { l.Quantity, l.TotalAmount }).ToListAsync(cancellationToken);

        var priorOutward = await (
            from v in context.SubWarehouseVouchers
            join l in context.SubWarehouseVoucherLines on v.Id equals l.VoucherId
            where v.WarehouseId == warehouseId && l.InventoryItemId == itemId
                  && v.PostingDate < periodStartDate
                  && ((int)v.VoucherType == 4 || (int)v.VoucherType == 5 || (int)v.VoucherType == 6)
            select new { l.Quantity, l.TotalAmount }).ToListAsync(cancellationToken);

        var openingQty = Math.Max(0m, priorInward.Sum(x => x.Quantity) - priorOutward.Sum(x => x.Quantity));
        var openingVal = Math.Max(0m, priorInward.Sum(x => x.TotalAmount) - priorOutward.Sum(x => x.TotalAmount));

        // 2. Pure Weighted Average Calculation
        var calcResult = MonthlyWeightedAverageEngine.Calculate(
            openingQty,
            openingVal,
            inwardQty,
            inwardVal,
            outwardQty,
            provisionedOutwardCost);

        var fiscalPeriodId = FiscalPeriodId.FromYearMonth(request.Year, request.Month);
        var runId = CostAllocationRunId.New();
        var costingRun = new CostingRun(
            runId,
            fiscalPeriodId,
            InventoryCostingMethod.MonthlyWeightedAverage,
            warehouseId,
            itemId);

        costingRun.MarkCalculated(calcResult.AdjustmentVariance);

        // 3. Emit GL Adjustment Voucher if there is cost variance
        if (calcResult.AdjustmentVariance != 0m)
        {
            var voucherDate = periodEndDate;
            var voucherNum = $"ADJ-COST-{request.Year}{request.Month:D2}-{warehouseId.Value}";
            var glVoucher = new Voucher(
                VoucherId.New(),
                voucherNum,
                VoucherType.GeneralJournal,
                voucherDate,
                voucherDate,
                $"Monthly weighted average cost adjustment {request.Month:D2}/{request.Year} for {warehouseId.Value}",
                new CurrencyCode("VND"),
                1.0m);

            if (calcResult.AdjustmentVariance > 0m)
            {
                // Actual cost > Provisioned cost: Debit COGS (632), Credit Inventory (1561)
                glVoucher.AddLine(
                    new AccountId(request.CogsAccountCode),
                    LedgerEntryType.Debit,
                    calcResult.AdjustmentVariance,
                    "Cost of sales upward adjustment",
                    warehouseId: warehouseId);

                glVoucher.AddLine(
                    new AccountId(request.InventoryAccountCode),
                    LedgerEntryType.Credit,
                    calcResult.AdjustmentVariance,
                    "Inventory valuation adjustment",
                    warehouseId: warehouseId);
            }
            else
            {
                // Actual cost < Provisioned cost: Debit Inventory (1561), Credit COGS (632)
                var positiveVariance = Math.Abs(calcResult.AdjustmentVariance);
                glVoucher.AddLine(
                    new AccountId(request.InventoryAccountCode),
                    LedgerEntryType.Debit,
                    positiveVariance,
                    "Inventory valuation reduction recovery",
                    warehouseId: warehouseId);

                glVoucher.AddLine(
                    new AccountId(request.CogsAccountCode),
                    LedgerEntryType.Credit,
                    positiveVariance,
                    "Cost of sales downward adjustment",
                    warehouseId: warehouseId);
            }

            var glResult = await glBridge.PostOperationalVoucherAsync(glVoucher, request.PostedBy, cancellationToken);
            if (!glResult.IsSuccess)
                return Result<CostAllocationRunId>.Failure(glResult.ErrorMessage ?? "Failed to post cost adjustment voucher.");

            costingRun.LinkGeneralLedgerVoucher(glVoucher.Id);
        }

        context.AddEntity(costingRun);
        await context.SaveChangesAsync(cancellationToken);

        return Result<CostAllocationRunId>.Success(costingRun.Id);
    }
}
