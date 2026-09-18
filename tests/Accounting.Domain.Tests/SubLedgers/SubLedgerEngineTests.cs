using System.Data;
using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Services;
using Accounting.Application.Features.Payables;
using Accounting.Application.Features.Receivables;
using Accounting.Application.Features.Reports;
using Accounting.Application.Features.Treasury;
using Accounting.Application.Features.WarehouseOperations;
using Accounting.Domain.Common;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Dimensions;
using Accounting.Domain.MasterData.Inventory;
using Accounting.Domain.MasterData.Partners;
using Accounting.Domain.Payables;
using Accounting.Domain.Receivables;
using Accounting.Domain.WarehouseOperations;
using Accounting.Infrastructure.Persistence.Connections;
using Accounting.Infrastructure.Persistence.Context;
using Dapper;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using Xunit;
using Account = Accounting.Domain.MasterData.Accounts.Account;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;
using Warehouse = Accounting.Domain.MasterData.Inventory.Warehouse;

namespace Accounting.Domain.Tests.SubLedgers;

public class SubLedgerEngineTests : IDisposable
{
    private readonly string _testDbPath;
    private readonly AccountingDbContext _context;
    private readonly ISqlConnectionFactory _connectionFactory;
    private readonly IGlVoucherBridgeService _glBridge;

    private readonly PartnerId _vendorId = PartnerId.New();
    private readonly PartnerId _customerId = PartnerId.New();
    private readonly WarehouseId _warehouseId = new("KHO_TONG");
    private readonly InventoryItemId _itemId = InventoryItemId.New();
    private readonly UomId _uomId = UomId.New();

    public SubLedgerEngineTests()
    {
        _testDbPath = $"subledger_test_{Guid.NewGuid():N}.db";

        var configData = new Dictionary<string, string?>
        {
            { "DatabaseProvider", "Sqlite" },
            { "ConnectionStrings:SqliteConnection", $"Data Source={_testDbPath}" }
        };
        var config = new ConfigurationBuilder().AddInMemoryCollection(configData).Build();
        _connectionFactory = new DapperDbConnectionFactory(config);

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
        var acc1121 = new Account(new AccountId("1121"), "Tiền gửi ngân hàng VNĐ", AccountType.Asset, BalanceNature.DebitBalance);
        var acc131 = new Account(new AccountId("131"), "Phải thu khách hàng", AccountType.Asset, BalanceNature.DebitBalance, requiresPartner: true);
        var acc331 = new Account(new AccountId("331"), "Phải trả người bán", AccountType.Liability, BalanceNature.CreditBalance, requiresPartner: true);
        var acc1331 = new Account(new AccountId("1331"), "Thuế GTGT được khấu trừ của HHDV", AccountType.Asset, BalanceNature.DebitBalance, requiresPartner: true);
        var acc33311 = new Account(new AccountId("33311"), "Thuế GTGT đầu ra", AccountType.Liability, BalanceNature.CreditBalance);
        var acc1561 = new Account(new AccountId("1561"), "Giá mua hàng hóa", AccountType.Asset, BalanceNature.DebitBalance, requiresWarehouse: true);
        var acc5111 = new Account(new AccountId("5111"), "Doanh thu bán hàng hóa", AccountType.Revenue, BalanceNature.CreditBalance);
        var acc632 = new Account(new AccountId("632"), "Giá vốn hàng bán", AccountType.CostOfSales, BalanceNature.DebitBalance, requiresWarehouse: true);
        var acc6421 = new Account(new AccountId("6421"), "Chi phí bán hàng", AccountType.Expense, BalanceNature.DebitBalance, requiresCostCenter: true);

        _context.MasterAccounts.AddRange(acc1111, acc1121, acc131, acc331, acc1331, acc33311, acc1561, acc5111, acc632, acc6421);

        // 2. Business Partners
        var vendor = new BusinessPartner(_vendorId, "NCC_A", "CONG TY NHA CUNG CAP A", PartnerType.Vendor, "0101234567");
        var customer = new BusinessPartner(_customerId, "KH_B", "CONG TY KHACH HANG B", PartnerType.Customer, "0109876543");
        _context.BusinessPartners.AddRange(vendor, customer);

        // 3. Warehouse, UOM & Item
        var warehouse = new Warehouse(_warehouseId, "Kho Tổng Hà Nội", "Hà Nội");
        var uom = new UnitOfMeasure(_uomId, "Cái", "CAI");
        var item = new InventoryItem(_itemId, "SP01", "Sản phẩm A", ItemType.Merchandise, _uomId, CostingMethod.MovingAverage, new AccountId("1561"), new AccountId("632"), new AccountId("5111"));
        _context.MasterWarehouses.Add(warehouse);
        _context.UnitsOfMeasure.Add(uom);
        _context.InventoryItems.Add(item);

        // 4. Cost Center
        var costCenter = new CostCenter(new CostCenterId("CC_MKT"), "Phòng Marketing");
        _context.CostCenters.Add(costCenter);

        // 5. Open Fiscal Periods
        _context.FiscalPeriods.AddRange(new FiscalPeriod(2026, 2), new FiscalPeriod(2026, 3));

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
    public async Task RegisterPurchaseInvoice_SplitsVatAndPostsToGeneralLedger()
    {
        var handler = new RegisterPurchaseInvoiceCommandHandler(_context, _glBridge);

        // Purchase of merchandise: 50,000,000 VND + 10% VAT (5,000,000 VND) = 55,000,000 VND
        var command = new RegisterPurchaseInvoiceCommand(
            InvoiceNumber: "INV-2026-001",
            InvoiceSeries: "AA/26E",
            InvoiceDate: new DateOnly(2026, 2, 10),
            DueDate: new DateOnly(2026, 3, 10),
            VendorId: _vendorId.Value,
            DefaultWarehouseId: _warehouseId.Value,
            Lines:
            [
                new RegisterPurchaseInvoiceLineDto("1561", 100m, 500_000m, 10m, "Nhập mua 100 cái SP01", _itemId.Value)
            ]);

        var result = await handler.Handle(command, CancellationToken.None);
        Assert.True(result.IsSuccess);

        var invoiceId = result.Value!;
        var invoice = await _context.SubPurchaseInvoices
            .Include(i => i.Lines)
            .FirstOrDefaultAsync(i => i.Id == invoiceId);

        Assert.NotNull(invoice);
        Assert.Equal(InvoiceStatus.Open, invoice.Status);
        Assert.Equal(50_000_000m, invoice.SubTotalAmount);
        Assert.Equal(5_000_000m, invoice.VatAmount);
        Assert.Equal(55_000_000m, invoice.TotalAmount);
        Assert.Equal(0m, invoice.PaidAmount);
        Assert.Equal(55_000_000m, invoice.RemainingAmount);
        Assert.NotNull(invoice.LinkedVoucherId);

        // Verify underlying GL Voucher is Posted and balanced
        var glVoucher = await _context.GlVouchers
            .Include(v => v.Lines)
            .FirstOrDefaultAsync(v => v.Id == invoice.LinkedVoucherId);

        Assert.NotNull(glVoucher);
        Assert.Equal(VoucherStatus.Posted, glVoucher.Status);
        Assert.Equal(55_000_000m, glVoucher.TotalDebitBase);
        Assert.Equal(55_000_000m, glVoucher.TotalCreditBase);

        // Verify GeneralLedgerEntries generated
        var glEntries = await _context.GeneralLedgerEntries
            .Where(e => e.VoucherId == glVoucher.Id)
            .ToListAsync();

        Assert.Equal(3, glEntries.Count);

        // 1. Debit 1561: 50,000,000
        var debit1561 = glEntries.First(e => e.AccountId == new AccountId("1561"));
        Assert.Equal(50_000_000m, debit1561.DebitAmount);
        Assert.Equal(0m, debit1561.CreditAmount);

        // 2. Debit 1331: 5,000,000 (Input VAT)
        var debit1331 = glEntries.First(e => e.AccountId == new AccountId("1331"));
        Assert.Equal(5_000_000m, debit1331.DebitAmount);
        Assert.Equal(0m, debit1331.CreditAmount);

        // 3. Credit 331: 55,000,000 (Vendor Payable)
        var credit331 = glEntries.First(e => e.AccountId == new AccountId("331"));
        Assert.Equal(0m, credit331.DebitAmount);
        Assert.Equal(55_000_000m, credit331.CreditAmount);
        Assert.Equal(_vendorId, credit331.PartnerId);
    }

    [Fact]
    public async Task SalesInvoice_PartialAndFullSettlement_TracksRemainingBalance()
    {
        var invoiceHandler = new CreateSalesInvoiceCommandHandler(_context, _glBridge);
        var settleHandler = new SettleCustomerReceiptCommandHandler(_context, _glBridge);

        // 1. Create Sales Invoice: 10,000,000 VND total (Revenue 5111: 10,000,000 VND, 0% VAT)
        var createCommand = new CreateSalesInvoiceCommand(
            InvoiceNumber: "INV-SALE-001",
            InvoiceDate: new DateOnly(2026, 2, 1),
            DueDate: new DateOnly(2026, 2, 28),
            CustomerId: _customerId.Value,
            Lines:
            [
                new CreateSalesInvoiceLineDto("5111", 10m, 1_000_000m, 0m, "Bán 10 cái hàng hóa", _itemId.Value)
            ]);

        var createResult = await invoiceHandler.Handle(createCommand, CancellationToken.None);
        Assert.True(createResult.IsSuccess);
        var invoiceId = createResult.Value!;

        // 2. Partial Settlement: Settle 6,000,000 VND
        var settle1Result = await settleHandler.Handle(new SettleCustomerReceiptCommand(
            InvoiceId: invoiceId,
            ReceiptVoucherNumber: "PT-2026-001",
            ReceiptDate: new DateOnly(2026, 2, 15),
            Amount: 6_000_000m), CancellationToken.None);

        Assert.True(settle1Result.IsSuccess);

        var refreshedInvoice = await _context.SubSalesInvoices.FindAsync(invoiceId);
        Assert.NotNull(refreshedInvoice);
        Assert.Equal(SalesInvoiceStatus.PartiallyPaid, refreshedInvoice.Status);
        Assert.Equal(6_000_000m, refreshedInvoice.ReceivedAmount);
        Assert.Equal(4_000_000m, refreshedInvoice.RemainingAmount);

        // 3. Full Settlement: Settle remaining 4,000,000 VND
        var settle2Result = await settleHandler.Handle(new SettleCustomerReceiptCommand(
            InvoiceId: invoiceId,
            ReceiptVoucherNumber: "PT-2026-002",
            ReceiptDate: new DateOnly(2026, 2, 20),
            Amount: 4_000_000m), CancellationToken.None);

        Assert.True(settle2Result.IsSuccess);

        var fullyPaidInvoice = await _context.SubSalesInvoices.FindAsync(invoiceId);
        Assert.NotNull(fullyPaidInvoice);
        Assert.Equal(SalesInvoiceStatus.FullyPaid, fullyPaidInvoice.Status);
        Assert.Equal(10_000_000m, fullyPaidInvoice.ReceivedAmount);
        Assert.Equal(0m, fullyPaidInvoice.RemainingAmount);
    }

    [Fact]
    public async Task SalesInvoice_OverSettlement_ThrowsOverSettlementException()
    {
        var invoiceHandler = new CreateSalesInvoiceCommandHandler(_context, _glBridge);
        var settleHandler = new SettleCustomerReceiptCommandHandler(_context, _glBridge);

        var createResult = await invoiceHandler.Handle(new CreateSalesInvoiceCommand(
            InvoiceNumber: "INV-SALE-OVER",
            InvoiceDate: new DateOnly(2026, 2, 1),
            DueDate: new DateOnly(2026, 2, 28),
            CustomerId: _customerId.Value,
            Lines:
            [
                new CreateSalesInvoiceLineDto("5111", 4m, 1_000_000m, 0m, "Hàng hóa", _itemId.Value)
            ]), CancellationToken.None);

        var invoiceId = createResult.Value!;

        // Attempting to settle 5,000,000 against a 4,000,000 balance
        var ex = await Assert.ThrowsAsync<OverSettlementException>(() =>
            settleHandler.Handle(new SettleCustomerReceiptCommand(
                InvoiceId: invoiceId,
                ReceiptVoucherNumber: "PT-ERR-001",
                ReceiptDate: new DateOnly(2026, 2, 10),
                Amount: 5_000_000m), CancellationToken.None));

        Assert.Equal(5_000_000m, ex.RequestedAmount);
        Assert.Equal(4_000_000m, ex.RemainingBalance);
    }

    [Fact]
    public void PurchaseInvoice_VatSplitCalculation_VerifiesExactRounding()
    {
        var invoice = new PurchaseInvoice(
            PurchaseInvoiceId.New(),
            "INV-VAT-TEST",
            "AB/26E",
            new DateOnly(2026, 2, 1),
            new DateOnly(2026, 2, 28),
            _vendorId);

        // 3 items @ 333,333.33 each with 8% VAT
        invoice.AddLine(new AccountId("1561"), 3m, 333_333.3333m, 8m, "Item 1");

        // Amount = 3 * 333333.3333 = 1,000,000.00
        // VatAmount = 1,000,000.00 * 8% = 80,000.00
        // Total = 1,080,000.00
        Assert.Equal(1_000_000.00m, invoice.SubTotalAmount);
        Assert.Equal(80_000.00m, invoice.VatAmount);
        Assert.Equal(1_080_000.00m, invoice.TotalAmount);
    }

    [Fact]
    public async Task WarehouseVoucher_InsufficientStock_ThrowsInsufficientStockException()
    {
        var handler = new PostWarehouseVoucherCommandHandler(_context, _glBridge);

        // 1. Post Inward Receipt of 50 units
        await handler.Handle(new PostWarehouseVoucherCommand(
            VoucherNumber: "PNK-2026-001",
            VoucherType: WarehouseVoucherType.InwardPurchase,
            PostingDate: new DateOnly(2026, 2, 1),
            WarehouseId: _warehouseId.Value,
            Description: "Nhập mua 50 cái",
            PartnerId: _vendorId.Value,
            Lines:
            [
                new PostWarehouseVoucherLineDto(_itemId.Value, _uomId.Value, 50m, 10_000m, "1561", "331", "Line 1")
            ]), CancellationToken.None);

        // 2. Attempt Outward Issue of 60 units (> 50 available) -> Must Throw InsufficientStockException
        var ex = await Assert.ThrowsAsync<InsufficientStockException>(() =>
            handler.Handle(new PostWarehouseVoucherCommand(
                VoucherNumber: "PXK-2026-FAIL",
                VoucherType: WarehouseVoucherType.OutwardSales,
                PostingDate: new DateOnly(2026, 2, 5),
                WarehouseId: _warehouseId.Value,
                Description: "Xuất bán 60 cái",
                Lines:
                [
                    new PostWarehouseVoucherLineDto(_itemId.Value, _uomId.Value, 60m, 12_000m, "632", "1561", "Line 1")
                ]), CancellationToken.None));

        Assert.Equal(60m, ex.RequestedQuantity);
        Assert.Equal(50m, ex.AvailableQuantity);
    }

    [Fact]
    public async Task WarehouseVoucher_ValidOutward_DeductsStockAndPostsCostOfGoodsSold()
    {
        var handler = new PostWarehouseVoucherCommandHandler(_context, _glBridge);

        // 1. Inward receipt of 100 units @ 50,000 VND = 5,000,000 VND
        await handler.Handle(new PostWarehouseVoucherCommand(
            VoucherNumber: "PNK-2026-002",
            VoucherType: WarehouseVoucherType.InwardPurchase,
            PostingDate: new DateOnly(2026, 2, 1),
            WarehouseId: _warehouseId.Value,
            Description: "Nhập mua hàng",
            PartnerId: _vendorId.Value,
            Lines:
            [
                new PostWarehouseVoucherLineDto(_itemId.Value, _uomId.Value, 100m, 50_000m, "1561", "331", "Nhập 100 cái")
            ]), CancellationToken.None);

        // 2. Outward issue of 40 units @ 50,000 VND = 2,000,000 VND (Debit 632 / Credit 1561)
        var outResult = await handler.Handle(new PostWarehouseVoucherCommand(
            VoucherNumber: "PXK-2026-002",
            VoucherType: WarehouseVoucherType.OutwardSales,
            PostingDate: new DateOnly(2026, 2, 10),
            WarehouseId: _warehouseId.Value,
            Description: "Xuất bán hàng",
            Lines:
            [
                new PostWarehouseVoucherLineDto(_itemId.Value, _uomId.Value, 40m, 50_000m, "632", "1561", "Xuất 40 cái giá vốn")
            ]), CancellationToken.None);

        Assert.True(outResult.IsSuccess);
        var outwardVoucherId = outResult.Value!;

        var outwardVoucher = await _context.SubWarehouseVouchers.FindAsync(outwardVoucherId);
        Assert.NotNull(outwardVoucher);
        Assert.NotNull(outwardVoucher.LinkedVoucherId);

        // Verify GL Voucher posted
        var glVoucher = await _context.GlVouchers.FindAsync(outwardVoucher.LinkedVoucherId);
        Assert.NotNull(glVoucher);
        Assert.Equal(VoucherStatus.Posted, glVoucher.Status);
        Assert.Equal(2_000_000m, glVoucher.TotalDebitBase);
        Assert.Equal(2_000_000m, glVoucher.TotalCreditBase);

        // Verify GL entries generated for COGS (632) and Inventory (1561)
        var glEntries = await _context.GeneralLedgerEntries
            .Where(e => e.VoucherId == glVoucher.Id)
            .ToListAsync();

        Assert.Equal(2, glEntries.Count);
        var cogsEntry = glEntries.First(e => e.AccountId == new AccountId("632"));
        Assert.Equal(2_000_000m, cogsEntry.DebitAmount);
        Assert.Equal(_warehouseId, cogsEntry.WarehouseId);

        var inventoryEntry = glEntries.First(e => e.AccountId == new AccountId("1561"));
        Assert.Equal(2_000_000m, inventoryEntry.CreditAmount);
        Assert.Equal(_warehouseId, inventoryEntry.WarehouseId);
    }

    [Fact]
    public async Task Dapper_CustomerAgingSchedule_GroupsOverdueCorrectly()
    {
        var invoiceHandler = new CreateSalesInvoiceCommandHandler(_context, _glBridge);

        // Invoice 1: Due in future (Current) -> 5,000,000
        await invoiceHandler.Handle(new CreateSalesInvoiceCommand(
            InvoiceNumber: "INV-AGE-01",
            InvoiceDate: new DateOnly(2026, 2, 1),
            DueDate: new DateOnly(2026, 2, 28),
            CustomerId: _customerId.Value,
            Lines: [new CreateSalesInvoiceLineDto("5111", 5m, 1_000_000m, 0m, "Line 1", _itemId.Value)]), CancellationToken.None);

        // Invoice 2: Overdue 15 days -> 3,000,000
        await invoiceHandler.Handle(new CreateSalesInvoiceCommand(
            InvoiceNumber: "INV-AGE-02",
            InvoiceDate: new DateOnly(2026, 1, 1),
            DueDate: new DateOnly(2026, 1, 20),
            CustomerId: _customerId.Value,
            Lines: [new CreateSalesInvoiceLineDto("5111", 3m, 1_000_000m, 0m, "Line 2", _itemId.Value)]), CancellationToken.None);

        // Query Aging As Of 2026-02-05
        var queryHandler = new GetCustomerAgingScheduleQueryHandler(_connectionFactory);
        var result = await queryHandler.Handle(
            new GetCustomerAgingScheduleQuery(new DateOnly(2026, 2, 5)),
            CancellationToken.None);

        Assert.True(result.IsSuccess);
        var rows = result.Value!;
        Assert.NotEmpty(rows);

        var customerRow = rows.First(r => r.CustomerId == _customerId.Value);
        Assert.Equal(8_000_000m, customerRow.TotalBalance);
        Assert.Equal(5_000_000m, customerRow.CurrentBalance); // Due 2026-02-28 >= 2026-02-05
        Assert.Equal(3_000_000m, customerRow.Overdue1To30);     // Due 2026-01-20 is 16 days overdue
    }

    [Fact]
    public async Task Dapper_StockBalanceReport_CalculatesInwardOutwardAndClosing()
    {
        var whHandler = new PostWarehouseVoucherCommandHandler(_context, _glBridge);

        // Inward 70 units
        await whHandler.Handle(new PostWarehouseVoucherCommand(
            VoucherNumber: "PNK-REP-01",
            VoucherType: WarehouseVoucherType.InwardPurchase,
            PostingDate: new DateOnly(2026, 2, 1),
            WarehouseId: _warehouseId.Value,
            Description: "Inward 70",
            PartnerId: _vendorId.Value,
            Lines: [new PostWarehouseVoucherLineDto(_itemId.Value, _uomId.Value, 70m, 20_000m, "1561", "331", "70 pcs")]), CancellationToken.None);

        // Outward 25 units
        await whHandler.Handle(new PostWarehouseVoucherCommand(
            VoucherNumber: "PXK-REP-01",
            VoucherType: WarehouseVoucherType.OutwardSales,
            PostingDate: new DateOnly(2026, 2, 10),
            WarehouseId: _warehouseId.Value,
            Description: "Outward 25",
            Lines: [new PostWarehouseVoucherLineDto(_itemId.Value, _uomId.Value, 25m, 20_000m, "632", "1561", "25 pcs")]), CancellationToken.None);

        // Query Stock Report
        var reportHandler = new GetStockBalanceReportQueryHandler(_connectionFactory);
        var result = await reportHandler.Handle(
            new GetStockBalanceReportQuery(_warehouseId.Value),
            CancellationToken.None);

        Assert.True(result.IsSuccess);
        var rows = result.Value!;
        Assert.NotEmpty(rows);

        var itemRow = rows.First(r => r.InventoryItemId == _itemId.Value);
        Assert.Equal(70m, itemRow.InwardQuantity);
        Assert.Equal(25m, itemRow.OutwardQuantity);
        Assert.Equal(45m, itemRow.ClosingQuantity);
        Assert.Equal(900_000m, itemRow.ClosingValue); // 45 * 20,000
    }

    [Fact]
    public async Task Live_MariaDb_SubLedgerPosting_Succeeds()
    {
        var serverConnStr = "Server=localhost;Port=3306;User=dev;Password=123456;TreatTinyAsBoolean=true;CharSet=utf8mb4;";
        var connStr = "Server=localhost;Port=3306;Database=accounting_subledger_test;User=dev;Password=123456;TreatTinyAsBoolean=true;CharSet=utf8mb4;";
        var serverVersion = new MariaDbServerVersion(new Version(12, 3, 0));

        // Test connectivity first & create isolated database
        try
        {
            using var pingConn = new MySqlConnector.MySqlConnection(serverConnStr);
            await pingConn.OpenAsync();
            using var cmd = pingConn.CreateCommand();
            cmd.CommandText = "CREATE DATABASE IF NOT EXISTS accounting_subledger_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;";
            await cmd.ExecuteNonQueryAsync();
        }
        catch
        {
            return;
        }

        var config = new ConfigurationBuilder().AddInMemoryCollection(new Dictionary<string, string?>
        {
            { "DatabaseProvider", "MariaDb" },
            { "ConnectionStrings:MariaDbConnection", connStr }
        }).Build();

        var dapperFactory = new DapperDbConnectionFactory(config);

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

        // Ensure partner and item exist on MariaDB
        var vendorId = PartnerId.New();
        var vendorCode = $"NCC_{Guid.NewGuid():N}"[..12].ToUpperInvariant();
        var vendor = new BusinessPartner(vendorId, vendorCode, "CONG TY TNHH TEST MARIADB", PartnerType.Vendor, "0109998887");
        efContext.BusinessPartners.Add(vendor);
        await efContext.SaveChangesAsync();

        // Register Purchase Invoice on Live MariaDB
        var apHandler = new RegisterPurchaseInvoiceCommandHandler(efContext, glBridge);
        var invoiceNum = $"MDB-AP-{Guid.NewGuid():N}"[..15].ToUpperInvariant();
        var result = await apHandler.Handle(new RegisterPurchaseInvoiceCommand(
            InvoiceNumber: invoiceNum,
            InvoiceSeries: "MB/26E",
            InvoiceDate: new DateOnly(2026, 3, 5),
            DueDate: new DateOnly(2026, 3, 25),
            VendorId: vendorId.Value,
            DefaultWarehouseId: "KHO-TONG",
            Lines:
            [
                new RegisterPurchaseInvoiceLineDto("1561", 10m, 100_000m, 10m, "Test MariaDB AP Line")
            ]), CancellationToken.None);

        Assert.True(result.IsSuccess);

        // Query Vendor Aging via Dapper on Live MariaDB
        var agingHandler = new GetVendorAgingScheduleQueryHandler(dapperFactory);
        var agingResult = await agingHandler.Handle(
            new GetVendorAgingScheduleQuery(new DateOnly(2026, 3, 10)),
            CancellationToken.None);

        Assert.True(agingResult.IsSuccess);
        var agingRows = agingResult.Value!;
        Assert.Contains(agingRows, r => r.VendorId == vendorId.Value && r.TotalBalance == 1_100_000m);
    }
}
