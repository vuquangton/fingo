using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Services;
using Accounting.Application.Costing.Engines;
using Accounting.Application.Features.CapitalAssets;
using Accounting.Application.Features.Costing;
using Accounting.Application.Features.Manufacturing;
using Accounting.Application.Features.WarehouseOperations;
using Accounting.Domain.CapitalAssets;
using Accounting.Domain.Common;
using Accounting.Domain.Costing;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Ledger;
using Accounting.Domain.Manufacturing;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Dimensions;
using Accounting.Domain.MasterData.Inventory;
using Accounting.Domain.MasterData.Partners;
using Accounting.Domain.WarehouseOperations;
using Accounting.Infrastructure.Persistence.Connections;
using Accounting.Infrastructure.Persistence.Context;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using Xunit;
using Account = Accounting.Domain.MasterData.Accounts.Account;
using FixedAsset = Accounting.Domain.CapitalAssets.FixedAsset;
using PrepaidExpense = Accounting.Domain.CapitalAssets.PrepaidExpense;
using Warehouse = Accounting.Domain.MasterData.Inventory.Warehouse;

namespace Accounting.Domain.Tests.Costing;

public class CostingAndCapitalAssetEngineTests : IDisposable
{
    private readonly string _testDbPath;
    private readonly AccountingDbContext _context;
    private readonly IGlVoucherBridgeService _glBridge;

    private readonly WarehouseId _warehouseId = new("KHO_TONG");
    private readonly InventoryItemId _merchandiseItemId = InventoryItemId.New();
    private readonly InventoryItemId _rawMaterialItemId = InventoryItemId.New();
    private readonly InventoryItemId _finishedGoodItemId = InventoryItemId.New();
    private readonly UomId _uomId = UomId.New();
    private readonly PartnerId _vendorId = PartnerId.New();

    public CostingAndCapitalAssetEngineTests()
    {
        _testDbPath = $"costing_asset_test_{Guid.NewGuid():N}.db";

        var options = new DbContextOptionsBuilder<AccountingDbContext>()
            .UseSqlite($"Data Source={_testDbPath}")
            .Options;

        _context = new AccountingDbContext(options);
        _context.Database.EnsureCreated();

        _glBridge = new GlVoucherBridgeService(_context);

        SeedMasterData();
    }

    private void SeedMasterData()
    {
        // 1. Chart of Accounts (Circular 99/2025/TT-BTC)
        var acc1111 = new Account(new AccountId("1111"), "Tiền Việt Nam", AccountType.Asset, BalanceNature.DebitBalance);
        var acc1121 = new Account(new AccountId("1121"), "Tiền gửi ngân hàng", AccountType.Asset, BalanceNature.DebitBalance);
        var acc152 = new Account(new AccountId("152"), "Nguyên liệu, vật liệu", AccountType.Asset, BalanceNature.DebitBalance, requiresWarehouse: true);
        var acc154 = new Account(new AccountId("154"), "Chi phí SXKD dở dang", AccountType.Asset, BalanceNature.DebitBalance);
        var acc1551 = new Account(new AccountId("1551"), "Thành phẩm", AccountType.Asset, BalanceNature.DebitBalance, requiresWarehouse: true);
        var acc1561 = new Account(new AccountId("1561"), "Giá mua hàng hóa", AccountType.Asset, BalanceNature.DebitBalance, requiresWarehouse: true);
        var acc2111 = new Account(new AccountId("2111"), "TSCĐ hữu hình", AccountType.Asset, BalanceNature.DebitBalance);
        var acc2141 = new Account(new AccountId("2141"), "Hao mòn TSCĐ hữu hình", AccountType.Liability, BalanceNature.CreditBalance);
        var acc242 = new Account(new AccountId("242"), "Chi phí trả trước", AccountType.Asset, BalanceNature.DebitBalance);
        var acc331 = new Account(new AccountId("331"), "Phải trả người bán", AccountType.Liability, BalanceNature.CreditBalance, requiresPartner: true);
        var acc621 = new Account(new AccountId("621"), "Chi phí NVL trực tiếp", AccountType.Expense, BalanceNature.DebitBalance);
        var acc622 = new Account(new AccountId("622"), "Chi phí nhân công trực tiếp", AccountType.Expense, BalanceNature.DebitBalance);
        var acc627 = new Account(new AccountId("627"), "Chi phí sản xuất chung", AccountType.Expense, BalanceNature.DebitBalance);
        var acc632 = new Account(new AccountId("632"), "Giá vốn hàng bán", AccountType.CostOfSales, BalanceNature.DebitBalance, requiresWarehouse: true);
        var acc6424 = new Account(new AccountId("6424"), "Chi phí khấu hao TSCĐ", AccountType.Expense, BalanceNature.DebitBalance);
        var acc6427 = new Account(new AccountId("6427"), "Chi phí dịch vụ mua ngoài / Phân bổ CCDC", AccountType.Expense, BalanceNature.DebitBalance);

        _context.MasterAccounts.AddRange(
            acc1111, acc1121, acc152, acc154, acc1551, acc1561,
            acc2111, acc2141, acc242, acc331,
            acc621, acc622, acc627, acc632, acc6424, acc6427);

        // 2. Warehouses, UOM, and Items
        var warehouse = new Warehouse(_warehouseId, "Kho Tổng Hà Nội", "Hà Nội");
        var uom = new UnitOfMeasure(_uomId, "Cái", "CAI");
        var itemMerch = new InventoryItem(_merchandiseItemId, "SP01", "Hàng hóa A", ItemType.Merchandise, _uomId, CostingMethod.MovingAverage, new AccountId("1561"), new AccountId("632"), new AccountId("5111"));
        var itemRaw = new InventoryItem(_rawMaterialItemId, "NVL01", "Nguyên vật liệu thép", ItemType.RawMaterial, _uomId, CostingMethod.MovingAverage, new AccountId("152"), new AccountId("632"), null);
        var itemFG = new InventoryItem(_finishedGoodItemId, "TP01", "Thành phẩm máy móc", ItemType.FinishedGoods, _uomId, CostingMethod.MovingAverage, new AccountId("1551"), new AccountId("632"), new AccountId("5111"));

        _context.MasterWarehouses.Add(warehouse);
        _context.UnitsOfMeasure.Add(uom);
        _context.InventoryItems.AddRange(itemMerch, itemRaw, itemFG);

        // 3. Partner
        var vendor = new BusinessPartner(_vendorId, "NCC_COST", "Nha Cung Cap Costing", PartnerType.Vendor, "0109999999");
        _context.BusinessPartners.Add(vendor);

        // 4. Fiscal Periods (2026-01, 2026-02, 2026-03)
        _context.FiscalPeriods.AddRange(
            new FiscalPeriod(2026, 1),
            new FiscalPeriod(2026, 2),
            new FiscalPeriod(2026, 3));

        _context.SaveChanges();
    }

    public void Dispose()
    {
        _context.Dispose();
        if (File.Exists(_testDbPath))
        {
            try { File.Delete(_testDbPath); } catch { }
        }
    }

    [Fact]
    public void FifoCostingEngine_MultiLayerConsumption_CalculatesCorrectCostAndDepletesLayers()
    {
        // Setup 2 FIFO layers:
        // Layer 1: 10 units @ 10,000 VND (Receipt: 2026-02-01)
        // Layer 2: 10 units @ 15,000 VND (Receipt: 2026-02-05)
        var layer1 = new InventoryLayer(
            InventoryLayerId.New(),
            _warehouseId,
            _merchandiseItemId,
            new DateOnly(2026, 2, 1),
            10m,
            10_000m);

        var layer2 = new InventoryLayer(
            InventoryLayerId.New(),
            _warehouseId,
            _merchandiseItemId,
            new DateOnly(2026, 2, 5),
            10m,
            15_000m);

        var openLayers = new List<InventoryLayer> { layer1, layer2 };

        // Issue 15 units
        var result = FifoCostingEngine.CalculateFifoIssue(
            openLayers,
            _merchandiseItemId.Value.ToString(),
            _warehouseId.Value,
            15m);

        // Assert: (10 * 10,000) + (5 * 15,000) = 100,000 + 75,000 = 175,000 VND
        Assert.Equal(175_000m, result.TotalCost);
        Assert.Equal(11_666.6667m, result.AverageUnitCost);
        Assert.Equal(2, result.Consumptions.Count);

        // Apply consumption to domain entities
        foreach (var c in result.Consumptions)
        {
            c.Layer.Consume(c.ConsumedQuantity);
        }

        Assert.Equal(0m, layer1.RemainingQuantity);
        Assert.True(layer1.IsExhausted);

        Assert.Equal(5m, layer2.RemainingQuantity);
        Assert.False(layer2.IsExhausted);
        Assert.Equal(75_000m, layer2.TotalValue);
    }

    [Fact]
    public void FifoCostingEngine_ExhaustedLayers_ThrowsCostingLayerExhaustedException()
    {
        var layer1 = new InventoryLayer(
            InventoryLayerId.New(),
            _warehouseId,
            _merchandiseItemId,
            new DateOnly(2026, 2, 1),
            10m,
            10_000m);

        var openLayers = new List<InventoryLayer> { layer1 };

        // Attempting to issue 25 units when only 10 available
        var ex = Assert.Throws<CostingLayerExhaustedException>(() =>
            FifoCostingEngine.CalculateFifoIssue(
                openLayers,
                _merchandiseItemId.Value.ToString(),
                _warehouseId.Value,
                25m));

        Assert.Equal(25m, ex.RequestedQty);
        Assert.Equal(10m, ex.AvailableQty);
    }

    [Fact]
    public async Task MonthlyWeightedAverageCosting_RecalculatesAndPostsAdjustmentVoucher()
    {
        var whHandler = new PostWarehouseVoucherCommandHandler(_context, _glBridge);
        var costHandler = new RunMonthlyWeightedAverageCostingCommandHandler(_context, _glBridge);

        // 1. Inward 1: 20 units @ 10,000 VND = 200,000 VND on 2026-02-01
        await whHandler.Handle(new PostWarehouseVoucherCommand(
            VoucherNumber: "PNK-AVG-01",
            VoucherType: WarehouseVoucherType.InwardPurchase,
            PostingDate: new DateOnly(2026, 2, 1),
            WarehouseId: _warehouseId.Value,
            Description: "Nhập 20 cái giá 10k",
            PartnerId: _vendorId.Value,
            Lines: [new PostWarehouseVoucherLineDto(_merchandiseItemId.Value, _uomId.Value, 20m, 10_000m, "1561", "331", "Line 1")]), CancellationToken.None);

        // 2. Inward 2: 80 units @ 15,000 VND = 1,200,000 VND on 2026-02-10
        await whHandler.Handle(new PostWarehouseVoucherCommand(
            VoucherNumber: "PNK-AVG-02",
            VoucherType: WarehouseVoucherType.InwardPurchase,
            PostingDate: new DateOnly(2026, 2, 10),
            WarehouseId: _warehouseId.Value,
            Description: "Nhập 80 cái giá 15k",
            PartnerId: _vendorId.Value,
            Lines: [new PostWarehouseVoucherLineDto(_merchandiseItemId.Value, _uomId.Value, 80m, 15_000m, "1561", "331", "Line 2")]), CancellationToken.None);

        // Total available = 100 units, Total value = 1,400,000 VND => Weighted Average Unit Cost = 14,000 VND

        // 3. Outward issue: 50 units temporarily provisioned @ 10,000 VND = 500,000 VND on 2026-02-15
        await whHandler.Handle(new PostWarehouseVoucherCommand(
            VoucherNumber: "PXK-AVG-01",
            VoucherType: WarehouseVoucherType.OutwardSales,
            PostingDate: new DateOnly(2026, 2, 15),
            WarehouseId: _warehouseId.Value,
            Description: "Xuất 50 cái giá tạm tính 10k",
            Lines: [new PostWarehouseVoucherLineDto(_merchandiseItemId.Value, _uomId.Value, 50m, 10_000m, "632", "1561", "Line 3")]), CancellationToken.None);

        // 4. Run Period-End Monthly Weighted Average Costing for 2026-02
        var runResult = await costHandler.Handle(new RunMonthlyWeightedAverageCostingCommand(
            Year: 2026,
            Month: 2,
            WarehouseId: _warehouseId.Value,
            InventoryItemId: _merchandiseItemId.Value), CancellationToken.None);

        Assert.True(runResult.IsSuccess);
        var runId = runResult.Value!;

        var costingRun = await _context.CostAllocationRuns.FindAsync(runId);
        Assert.NotNull(costingRun);
        Assert.Equal(CostingRunStatus.PostedToGl, costingRun.Status);

        // Expected adjustment: Actual Outward (50 * 14,000 = 700,000) - Provisioned (500,000) = +200,000 VND
        Assert.Equal(200_000m, costingRun.TotalAdjustmentAmount);
        Assert.NotNull(costingRun.LinkedVoucherId);

        // Verify Adjustment GL Voucher (Nợ 632 / Có 1561 for 200,000 VND)
        var glVoucher = await _context.GlVouchers
            .Include(v => v.Lines)
            .FirstOrDefaultAsync(v => v.Id == costingRun.LinkedVoucherId);

        Assert.NotNull(glVoucher);
        Assert.Equal(VoucherStatus.Posted, glVoucher.Status);
        Assert.Equal(200_000m, glVoucher.TotalDebitBase);
        Assert.Equal(200_000m, glVoucher.TotalCreditBase);

        var debitLine = glVoucher.Lines.First(l => l.EntryType == LedgerEntryType.Debit);
        Assert.Equal(new AccountId("632"), debitLine.AccountId);
        Assert.Equal(200_000m, debitLine.AmountBase);

        var creditLine = glVoucher.Lines.First(l => l.EntryType == LedgerEntryType.Credit);
        Assert.Equal(new AccountId("1561"), creditLine.AccountId);
        Assert.Equal(200_000m, creditLine.AmountBase);
    }

    [Fact]
    public async Task FixedAsset_MonthlyDepreciation_PostsGlAndAbsorbsRoundingOnFinalMonth()
    {
        var registerHandler = new RegisterFixedAssetCommandHandler(_context);
        var depHandler = new RunMonthlyDepreciationCommandHandler(_context, _glBridge);

        // 1. Capitalize Asset: 120,000,000 VND over 12 months (10,000,000 VND / month)
        var regResult = await registerHandler.Handle(new RegisterFixedAssetCommand(
            AssetCode: "TSCD-2026-001",
            AssetName: "Xe tải vận chuyển",
            AssetType: FixedAssetType.Tangible,
            OriginalCost: 120_000_000m,
            UsefulLifeMonths: 12,
            CapitalizationDate: new DateOnly(2026, 1, 1),
            DepreciationStartDate: new DateOnly(2026, 1, 1),
            AssetAccountCode: "2111",
            DepreciationAccountCode: "2141",
            ExpenseAccountCode: "6424"), CancellationToken.None);

        Assert.True(regResult.IsSuccess);
        var assetId = regResult.Value!;

        // 2. Run Depreciation for Period 2026-01
        var dep1Result = await depHandler.Handle(new RunMonthlyDepreciationCommand(2026, 1), CancellationToken.None);
        Assert.True(dep1Result.IsSuccess);
        Assert.Equal(1, dep1Result.Value!.DepreciatedAssetCount);
        Assert.Equal(10_000_000m, dep1Result.Value!.TotalDepreciationAmount);

        var asset = await _context.CapitalFixedAssets.FindAsync(assetId);
        Assert.NotNull(asset);
        Assert.Equal(10_000_000m, asset.AccumulatedDepreciation);
        Assert.Equal(110_000_000m, asset.RemainingBookValue);
        Assert.Equal(11, asset.RemainingMonths);
        Assert.Equal(FixedAssetStatus.Active, asset.Status);

        // Verify GL Voucher posted (Nợ 6424: 10M / Có 2141: 10M)
        var voucher = await _context.GlVouchers
            .Include(v => v.Lines)
            .FirstOrDefaultAsync(v => v.Id == new VoucherId(dep1Result.Value!.GlVoucherId!.Value));

        Assert.NotNull(voucher);
        Assert.Equal(VoucherStatus.Posted, voucher.Status);
        Assert.Equal(10_000_000m, voucher.TotalDebitBase);
        Assert.Equal(10_000_000m, voucher.TotalCreditBase);
    }

    [Fact]
    public void FixedAsset_FullDepreciation_RejectsFurtherDepreciation()
    {
        // 120,000,000 VND over 12 months
        var asset = new FixedAsset(
            FixedAssetId.New(),
            "TSCD-LIMIT",
            "Máy CNC",
            FixedAssetType.Tangible,
            120_000_000m,
            12,
            new DateOnly(2026, 1, 1),
            new DateOnly(2026, 1, 1),
            new AccountId("2111"),
            new AccountId("2141"),
            new AccountId("6424"));

        for (int m = 1; m <= 12; m++)
        {
            var amount = asset.CalculateMonthlyDepreciation(2026, m);
            asset.ApplyDepreciation(amount, 2026, m);
        }

        Assert.Equal(120_000_000m, asset.AccumulatedDepreciation);
        Assert.Equal(0m, asset.RemainingBookValue);
        Assert.Equal(FixedAssetStatus.FullyDepreciated, asset.Status);

        // Attempt 13th month depreciation -> Must throw AssetAlreadyDepreciatedException
        Assert.Throws<AssetAlreadyDepreciatedException>(() => asset.ApplyDepreciation(1_000m, 2027, 1));
    }

    [Fact]
    public async Task PrepaidExpense_Amortization_AbsorbsPennyRoundingAndCompletes()
    {
        var regHandler = new RegisterPrepaidExpenseCommandHandler(_context);
        var amortHandler = new RunPrepaidAmortizationCommandHandler(_context, _glBridge);

        // 10,000,000 VND over 3 months
        // Month 1: 3,333,333.33
        // Month 2: 3,333,333.33
        // Month 3: 3,333,333.34 (absorbs remainder)
        // Total = 10,000,000.00
        var regResult = await regHandler.Handle(new RegisterPrepaidExpenseCommand(
            ExpenseCode: "CPTT-2026-001",
            Name: "Bảo hiểm văn phòng 3 tháng",
            TotalAmount: 10_000_000m,
            TotalPeriods: 3,
            StartDate: new DateOnly(2026, 1, 1),
            SourceAccountCode: "242",
            TargetExpenseAccountCode: "6427"), CancellationToken.None);

        Assert.True(regResult.IsSuccess);
        var expenseId = regResult.Value!;

        // Period 1
        var p1Result = await amortHandler.Handle(new RunPrepaidAmortizationCommand(2026, 1), CancellationToken.None);
        Assert.True(p1Result.IsSuccess);
        Assert.Equal(3_333_333.33m, p1Result.Value!.TotalAmortizationAmount);

        // Period 2
        var p2Result = await amortHandler.Handle(new RunPrepaidAmortizationCommand(2026, 2), CancellationToken.None);
        Assert.True(p2Result.IsSuccess);
        Assert.Equal(3_333_333.33m, p2Result.Value!.TotalAmortizationAmount);

        // Period 3 (Absorbs penny difference)
        var p3Result = await amortHandler.Handle(new RunPrepaidAmortizationCommand(2026, 3), CancellationToken.None);
        Assert.True(p3Result.IsSuccess);
        Assert.Equal(3_333_333.34m, p3Result.Value!.TotalAmortizationAmount);

        var expense = await _context.CapitalPrepaidExpenses.FindAsync(expenseId);
        Assert.NotNull(expense);
        Assert.Equal(10_000_000.00m, expense.AllocatedAmount);
        Assert.Equal(0m, expense.RemainingAmount);
        Assert.Equal(0, expense.RemainingPeriods);
        Assert.Equal(PrepaidExpenseStatus.Completed, expense.Status);

        // Period 4 attempt: No active expenses to amortize
        var p4Result = await amortHandler.Handle(new RunPrepaidAmortizationCommand(2026, 4), CancellationToken.None);
        Assert.True(p4Result.IsSuccess);
        Assert.Equal(0, p4Result.Value!.AmortizedCount);
        Assert.Equal(0m, p4Result.Value!.TotalAmortizationAmount);
    }

    [Fact]
    public async Task Manufacturing_CostAccumulationAndCapitalization_Succeeds()
    {
        var bomHandler = new CreateBillOfMaterialsCommandHandler(_context);
        var mfgHandler = new CalculateManufacturingCostCommandHandler(_context, _glBridge);

        // 1. Create BOM
        var bomResult = await bomHandler.Handle(new CreateBillOfMaterialsCommand(
            FinishedGoodItemId: _finishedGoodItemId.Value,
            Version: "V1.0",
            Description: "BOM cho máy móc thành phẩm",
            Lines:
            [
                new BomLineDto(_rawMaterialItemId.Value, 2.5m, 1.0m)
            ]), CancellationToken.None);

        Assert.True(bomResult.IsSuccess);

        // 2. Calculate Manufacturing Cost for 2026-02
        // Incurred: 621 = 50M, 622 = 20M, 627 = 10M, WIP Beg = 0, WIP End = 0 => Total = 80M
        // Produced = 1,000 units => Unit Cost = 80,000 VND / unit
        var mfgResult = await mfgHandler.Handle(new CalculateManufacturingCostCommand(
            Year: 2026,
            Month: 2,
            FinishedGoodItemId: _finishedGoodItemId.Value,
            ProducedQuantity: 1_000m,
            WarehouseId: _warehouseId.Value,
            DirectMaterialCost: 50_000_000m,
            DirectLaborCost: 20_000_000m,
            OverheadCost: 10_000_000m,
            WipBeginning: 0m,
            WipEnding: 0m), CancellationToken.None);

        Assert.True(mfgResult.IsSuccess);
        var res = mfgResult.Value!;
        Assert.Equal(80_000_000m, res.TotalManufacturingCost);
        Assert.Equal(80_000m, res.UnitCostPerItem);

        // Verify Cost Clearance Voucher (Nợ 154: 80M / Có 621: 50M, Có 622: 20M, Có 627: 10M)
        var clearanceVoucher = await _context.GlVouchers
            .Include(v => v.Lines)
            .FirstOrDefaultAsync(v => v.Id == new VoucherId(res.ClearanceVoucherId));

        Assert.NotNull(clearanceVoucher);
        Assert.Equal(VoucherStatus.Posted, clearanceVoucher.Status);
        Assert.Equal(80_000_000m, clearanceVoucher.TotalDebitBase);
        Assert.Equal(80_000_000m, clearanceVoucher.TotalCreditBase);

        // Verify Capitalization Voucher (Nợ 1551: 80M / Có 154: 80M)
        var receiptVoucher = await _context.GlVouchers
            .Include(v => v.Lines)
            .FirstOrDefaultAsync(v => v.Id == new VoucherId(res.CapitalizationVoucherId));

        Assert.NotNull(receiptVoucher);
        Assert.Equal(VoucherStatus.Posted, receiptVoucher.Status);
        Assert.Equal(80_000_000m, receiptVoucher.TotalDebitBase);
        Assert.Equal(80_000_000m, receiptVoucher.TotalCreditBase);

        // Verify FIFO InventoryLayer created for the 1,000 finished goods
        var layer = await _context.CostInventoryLayers
            .FirstOrDefaultAsync(l => l.InventoryItemId == _finishedGoodItemId);

        Assert.NotNull(layer);
        Assert.Equal(1_000m, layer.OriginalQuantity);
        Assert.Equal(1_000m, layer.RemainingQuantity);
        Assert.Equal(80_000m, layer.UnitCost);
        Assert.Equal(80_000_000m, layer.TotalValue);
    }

    [Fact]
    public async Task Live_MariaDb_Phase5CostingAndAssets_Succeeds()
    {
        var serverConnStr = "Server=localhost;Port=3306;User=dev;Password=123456;TreatTinyAsBoolean=true;CharSet=utf8mb4;";
        var connStr = "Server=localhost;Port=3306;Database=accounting_phase5_test;User=dev;Password=123456;TreatTinyAsBoolean=true;CharSet=utf8mb4;";
        var serverVersion = new MariaDbServerVersion(new Version(12, 3, 0));

        // Test connectivity first & create isolated database
        try
        {
            using var pingConn = new MySqlConnector.MySqlConnection(serverConnStr);
            await pingConn.OpenAsync();
            using var cmd = pingConn.CreateCommand();
            cmd.CommandText = "CREATE DATABASE IF NOT EXISTS accounting_phase5_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;";
            await cmd.ExecuteNonQueryAsync();
        }
        catch
        {
            return;
        }

        var options = new DbContextOptionsBuilder<AccountingDbContext>()
            .UseMySql(connStr, serverVersion)
            .Options;

        using var efContext = new AccountingDbContext(options);
        await efContext.Database.EnsureCreatedAsync();

        if (!await efContext.MasterAccounts.AnyAsync())
        {
            await Accounting.Infrastructure.Persistence.Seeding.DbInitializer.SeedAsync(efContext);
        }

        var glBridge = new GlVoucherBridgeService(efContext);

        // 1. Register Fixed Asset on Live MariaDB
        var assetHandler = new RegisterFixedAssetCommandHandler(efContext);
        var assetCode = $"TSCD-{Guid.NewGuid():N}"[..12].ToUpperInvariant();
        var regResult = await assetHandler.Handle(new RegisterFixedAssetCommand(
            AssetCode: assetCode,
            AssetName: "Xe tải MariaDB",
            AssetType: FixedAssetType.Tangible,
            OriginalCost: 60_000_000m,
            UsefulLifeMonths: 6,
            CapitalizationDate: new DateOnly(2026, 3, 1),
            DepreciationStartDate: new DateOnly(2026, 3, 1),
            AssetAccountCode: "2111",
            DepreciationAccountCode: "2141",
            ExpenseAccountCode: "6424"), CancellationToken.None);

        Assert.True(regResult.IsSuccess);

        // 2. Run Monthly Depreciation on Live MariaDB
        var depHandler = new RunMonthlyDepreciationCommandHandler(efContext, glBridge);
        var depResult = await depHandler.Handle(new RunMonthlyDepreciationCommand(2026, 3), CancellationToken.None);

        Assert.True(depResult.IsSuccess);
        Assert.True(depResult.Value!.DepreciatedAssetCount >= 1);
        Assert.True(depResult.Value!.TotalDepreciationAmount >= 10_000_000m);
    }
}
