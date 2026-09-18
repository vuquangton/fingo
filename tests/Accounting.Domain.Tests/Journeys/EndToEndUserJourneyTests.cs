using Accounting.Application.Common.Interfaces;
using Accounting.Domain.Common;
using Accounting.Domain.Entities.Security;
using Accounting.Domain.Enums;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Inventory;
using Accounting.Domain.MasterData.Partners;
using Accounting.Domain.Payables;
using Accounting.Domain.Receivables;
using Accounting.Infrastructure.Persistence.Context;
using Accounting.Infrastructure.Persistence.Interceptors;
using Accounting.Infrastructure.Persistence.Seeding;
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;
using Xunit;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;
using Voucher = Accounting.Domain.Ledger.Voucher;
using VoucherType = Accounting.Domain.Ledger.VoucherType;
using VoucherStatus = Accounting.Domain.Ledger.VoucherStatus;
using PartnerType = Accounting.Domain.MasterData.Common.PartnerType;

namespace Accounting.Domain.Tests.Journeys;

public class EndToEndUserJourneyTests
{
    private class TestCurrentUserService : ICurrentUserService
    {
        public Guid? UserId { get; set; } = Guid.Parse("11111111-2222-3333-4444-555555555555");
        public string Username { get; set; } = "chief_accountant";
        public string MachineName { get; set; } = "WORKSTATION-01";
        public string? IpAddress { get; set; } = "127.0.0.1";
    }

    private class TestDateTimeService : IDateTimeService
    {
        public DateTime UtcNow => new(2026, 3, 31, 17, 0, 0, DateTimeKind.Utc);
        public DateTime Now => UtcNow.ToLocalTime();
    }

    private static AccountingDbContext CreateContext(SqliteConnection connection)
    {
        var userService = new TestCurrentUserService();
        var timeService = new TestDateTimeService();

        var auditInterceptor = new AuditTrailInterceptor(userService, timeService);
        var softDeleteInterceptor = new SoftDeleteInterceptor(userService, timeService);

        var options = new DbContextOptionsBuilder<AccountingDbContext>()
            .UseSqlite(connection)
            .AddInterceptors(auditInterceptor, softDeleteInterceptor)
            .Options;

        var context = new AccountingDbContext(options);
        context.Database.EnsureCreated();
        return context;
    }

    [Fact]
    public async Task CompleteQuarter_EnterpriseAccountingUserJourney_SucceedsWithAllInvariants()
    {
        // Setup SQLite In-Memory DB
        using var connection = new SqliteConnection("Data Source=:memory:;Mode=Memory;Cache=Shared");
        connection.Open();

        using (var initContext = CreateContext(connection))
        {
            // Seed statutory TT 99/2025 COA, currencies, UoMs, Warehouses
            await StatutorySeeder.SeedAsync(initContext);
        }

        using var db = CreateContext(connection);

        // =========================================================================
        // JOURNEY 1: KHỞI TẠO HỆ THỐNG & SỐ DƯ ĐẦU KỲ (Opening Balances)
        // =========================================================================
        var periodQ1 = new FiscalPeriodId(202601);
        var opDate = new DateOnly(2026, 1, 1);

        // Create Partners
        var vendorId = PartnerId.New();
        var vendor = new BusinessPartner(
            vendorId,
            "NCC-SAMSUNG-01",
            "Công ty TNHH Điện Tử Samsung Vina",
            PartnerType.Vendor,
            taxCode: "0100109106",
            address: "KCN Yên Phong, Bắc Ninh",
            creditLimit: 500_000_000m);
        vendor.AddBankAccount("Vietcombank", "VCB", "0011009998888", "SAMSUNG VINA", "Bắc Ninh", isDefault: true);

        var customerId = PartnerId.New();
        var customer = new BusinessPartner(
            customerId,
            "KH-FPT-RETAIL",
            "Công ty Cổ phần Bán lẻ Kỹ thuật số FPT",
            PartnerType.Customer,
            taxCode: "0101234567",
            address: "261 Khánh Hội, Quận 4, TP. HCM",
            creditLimit: 200_000_000m);
        customer.AddDeliveryAddress("Kho FPT Long An", city: "Long An", isDefault: true);

        db.BusinessPartners.AddRange(vendor, customer);

        // Record Opening Balance Voucher (1111: 100M, 1121: 400M, 1561: 200M, 4111: 700M)
        var opVoucher = new Voucher(
            VoucherId.New(),
            "SDDK-2026-001",
            VoucherType.GeneralJournal,
            opDate,
            opDate,
            "Số dư đầu kỳ năm tài chính 2026");

        opVoucher.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 100_000_000m, "Dư đầu kỳ Tiền mặt VND");
        opVoucher.AddLine(new AccountId("1121"), LedgerEntryType.Debit, 400_000_000m, "Dư đầu kỳ Tiền gửi NH VCB");
        opVoucher.AddLine(new AccountId("1561"), LedgerEntryType.Debit, 200_000_000m, "Dư đầu kỳ Hàng hóa tồn kho");
        opVoucher.AddLine(new AccountId("4111"), LedgerEntryType.Credit, 700_000_000m, "Dư đầu kỳ Vốn đầu tư của CSH");

        opVoucher.Post("chief_accountant");
        db.GlVouchers.Add(opVoucher);

        // Generate corresponding GL entries
        foreach (var l in opVoucher.Lines)
        {
            db.GeneralLedgerEntries.Add(new GeneralLedgerEntry(
                Guid.NewGuid(),
                opVoucher.Id,
                opVoucher.PostingDate,
                periodQ1,
                l.AccountId,
                l.EntryType == LedgerEntryType.Debit ? l.AmountBase : 0m,
                l.EntryType == LedgerEntryType.Credit ? l.AmountBase : 0m));
        }

        await db.SaveChangesAsync();

        // INVARIANT 1: Total Debit == Total Credit on Opening
        Assert.Equal(opVoucher.TotalDebitBase, opVoucher.TotalCreditBase);
        Assert.Equal(700_000_000m, opVoucher.TotalDebitBase);

        // =========================================================================
        // JOURNEY 2: CHU TRÌNH MUA HÀNG & PHẢI TRẢ (Procure-to-Pay / AP Cycle)
        // =========================================================================
        var purchaseInvoiceDate = new DateOnly(2026, 1, 15);
        var purchaseDueDate = new DateOnly(2026, 2, 15);
        var purchaseInvoice = new PurchaseInvoice(
            PurchaseInvoiceId.New(),
            "HDM-00129",
            "1C26TBB",
            purchaseInvoiceDate,
            purchaseDueDate,
            vendorId);

        // Line 1: Hàng hóa 100M, VAT 10% = 10M
        purchaseInvoice.AddLine(new AccountId("1561"), 10m, 10_000_000m, 10m, "Lô màn hình máy tính SamSung 27 inch");
        db.SubPurchaseInvoices.Add(purchaseInvoice);

        // AP Accounting Voucher: Nợ 1561: 100M, Nợ 1331: 10M / Có 331: 110M
        var apVoucher = new Voucher(
            VoucherId.New(),
            "PKT-MH-001",
            VoucherType.PurchaseInvoice,
            purchaseInvoiceDate,
            purchaseInvoiceDate,
            $"Mua hàng NCC Samsung hóa đơn {purchaseInvoice.InvoiceNumber}");

        apVoucher.AddLine(new AccountId("1561"), LedgerEntryType.Debit, 100_000_000m, "Tiền hàng mua nhập kho", partnerId: vendorId);
        apVoucher.AddLine(new AccountId("1331"), LedgerEntryType.Debit, 10_000_000m, "Thuế GTGT đầu vào được khấu trừ", partnerId: vendorId);
        apVoucher.AddLine(new AccountId("331"), LedgerEntryType.Credit, 110_000_000m, "Phải trả nhà cung cấp Samsung", partnerId: vendorId);
        apVoucher.Post("chief_accountant");
        db.GlVouchers.Add(apVoucher);

        foreach (var l in apVoucher.Lines)
        {
            db.GeneralLedgerEntries.Add(new GeneralLedgerEntry(
                Guid.NewGuid(), apVoucher.Id, apVoucher.PostingDate, periodQ1, l.AccountId,
                l.EntryType == LedgerEntryType.Debit ? l.AmountBase : 0m,
                l.EntryType == LedgerEntryType.Credit ? l.AmountBase : 0m,
                l.PartnerId));
        }

        // Adjust Vendor AP Balance
        vendor.AdjustPayableBalance(110_000_000m);
        await db.SaveChangesAsync();

        Assert.Equal(110_000_000m, vendor.CurrentPayableBalance);

        // =========================================================================
        // JOURNEY 3: CHU TRÌNH BÁN HÀNG & PHẢI THU (Order-to-Cash / AR Cycle)
        // =========================================================================
        var salesDate = new DateOnly(2026, 2, 10);
        var salesDueDate = new DateOnly(2026, 3, 10);

        // Check Credit Limit before sale (Proposed sale: 132M VND, Limit: 200M VND -> OK)
        customer.CheckCreditLimit(132_000_000m);

        var salesInvoice = new SalesInvoice(
            SalesInvoiceId.New(),
            "HDB-00889",
            salesDate,
            salesDueDate,
            customerId);

        salesInvoice.AddLine(new AccountId("5111"), 10m, 12_000_000m, 10m, "Bán màn hình SamSung 27 inch cho FPT");
        db.SubSalesInvoices.Add(salesInvoice);

        // Sales Accounting Voucher: Nợ 131: 132M / Có 5111: 120M, Có 33311: 12M
        var arVoucher = new Voucher(
            VoucherId.New(),
            "PKT-BH-001",
            VoucherType.SalesInvoice,
            salesDate,
            salesDate,
            $"Doanh thu bán hàng FPT Retail hóa đơn {salesInvoice.InvoiceNumber}");

        arVoucher.AddLine(new AccountId("131"), LedgerEntryType.Debit, 132_000_000m, "Phải thu khách hàng FPT", partnerId: customerId);
        arVoucher.AddLine(new AccountId("5111"), LedgerEntryType.Credit, 120_000_000m, "Doanh thu bán hàng hóa", partnerId: customerId);
        arVoucher.AddLine(new AccountId("33311"), LedgerEntryType.Credit, 12_000_000m, "Thuế GTGT đầu ra phải nộp", partnerId: customerId);
        arVoucher.Post("chief_accountant");
        db.GlVouchers.Add(arVoucher);

        // COGS Voucher: Nợ 632: 70M / Có 1561: 70M
        var cogsVoucher = new Voucher(
            VoucherId.New(),
            "PXK-001",
            VoucherType.InventoryIssue,
            salesDate,
            salesDate,
            "Xuất kho giá vốn bán hàng cho FPT");
        cogsVoucher.AddLine(new AccountId("632"), LedgerEntryType.Debit, 70_000_000m, "Giá vốn hàng bán");
        cogsVoucher.AddLine(new AccountId("1561"), LedgerEntryType.Credit, 70_000_000m, "Xuất kho hàng hóa");
        cogsVoucher.Post("chief_accountant");
        db.GlVouchers.Add(cogsVoucher);

        foreach (var l in arVoucher.Lines.Concat(cogsVoucher.Lines))
        {
            db.GeneralLedgerEntries.Add(new GeneralLedgerEntry(
                Guid.NewGuid(), l.VoucherId, salesDate, periodQ1, l.AccountId,
                l.EntryType == LedgerEntryType.Debit ? l.AmountBase : 0m,
                l.EntryType == LedgerEntryType.Credit ? l.AmountBase : 0m,
                l.PartnerId));
        }

        customer.AdjustReceivableBalance(132_000_000m);
        await db.SaveChangesAsync();

        Assert.Equal(132_000_000m, customer.CurrentReceivableBalance);
        Assert.Equal(PartnerRiskTier.Medium, customer.RiskTier); // 132M / 200M = 66% -> Medium

        // =========================================================================
        // JOURNEY 4: TIỀN MẶT & NGÂN HÀNG (Treasury & Cash Management)
        // =========================================================================
        // Customer pays 132M via Bank Transfer (Nợ 1121 / Có 131: 132M)
        var payDate = new DateOnly(2026, 2, 20);
        var bankRecVoucher = new Voucher(
            VoucherId.New(),
            "GNT-001",
            VoucherType.GeneralJournal,
            payDate,
            payDate,
            "FPT thanh toán tiền mua hàng chuyển khoản VCB");
        bankRecVoucher.AddLine(new AccountId("1121"), LedgerEntryType.Debit, 132_000_000m, "Thu chuyển khoản ngân hàng", partnerId: customerId);
        bankRecVoucher.AddLine(new AccountId("131"), LedgerEntryType.Credit, 132_000_000m, "Giảm công nợ FPT", partnerId: customerId);
        bankRecVoucher.Post("chief_accountant");
        db.GlVouchers.Add(bankRecVoucher);

        // Settle customer invoice
        customer.AdjustReceivableBalance(-132_000_000m);
        customer.UpdateRiskTier();
        Assert.Equal(0m, customer.CurrentReceivableBalance);
        Assert.Equal(PartnerRiskTier.Low, customer.RiskTier);

        // Pay Vendor 110M via Bank Transfer (Nợ 331 / Có 1121: 110M)
        var bankPayVoucher = new Voucher(
            VoucherId.New(),
            "UNC-001",
            VoucherType.BankPayment,
            payDate,
            payDate,
            "Thanh toán tiền hàng Samsung Vina UNC VCB");
        bankPayVoucher.AddLine(new AccountId("331"), LedgerEntryType.Debit, 110_000_000m, "Thanh toán nợ Samsung", partnerId: vendorId);
        bankPayVoucher.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 110_000_000m, "Rút tiền gửi ngân hàng chi trả", partnerId: vendorId);
        bankPayVoucher.Post("chief_accountant");
        db.GlVouchers.Add(bankPayVoucher);

        vendor.AdjustPayableBalance(-110_000_000m);
        Assert.Equal(0m, vendor.CurrentPayableBalance);

        foreach (var l in bankRecVoucher.Lines.Concat(bankPayVoucher.Lines))
        {
            db.GeneralLedgerEntries.Add(new GeneralLedgerEntry(
                Guid.NewGuid(), l.VoucherId, payDate, periodQ1, l.AccountId,
                l.EntryType == LedgerEntryType.Debit ? l.AmountBase : 0m,
                l.EntryType == LedgerEntryType.Credit ? l.AmountBase : 0m,
                l.PartnerId));
        }

        await db.SaveChangesAsync();

        // =========================================================================
        // JOURNEY 5: CHI PHÍ QUẢN LÝ & TIỀN LƯƠNG (Operating Expenses)
        // =========================================================================
        // Accrue salary: Nợ 6422: 20M / Có 334: 20M; Then Pay: Nợ 334 / Có 1121: 20M
        var expDate = new DateOnly(2026, 3, 25);
        var salaryVoucher = new Voucher(
            VoucherId.New(),
            "PKT-LUONG-001",
            VoucherType.GeneralJournal,
            expDate,
            expDate,
            "Chi phí lương bộ phận quản lý Tháng 3");
        salaryVoucher.AddLine(new AccountId("6422"), LedgerEntryType.Debit, 20_000_000m, "Chi phí tiền lương nhân viên");
        salaryVoucher.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 20_000_000m, "Chi trả lương chuyển khoản ngân hàng");
        salaryVoucher.Post("chief_accountant");
        db.GlVouchers.Add(salaryVoucher);

        foreach (var l in salaryVoucher.Lines)
        {
            db.GeneralLedgerEntries.Add(new GeneralLedgerEntry(
                Guid.NewGuid(), salaryVoucher.Id, expDate, periodQ1, l.AccountId,
                l.EntryType == LedgerEntryType.Debit ? l.AmountBase : 0m,
                l.EntryType == LedgerEntryType.Credit ? l.AmountBase : 0m));
        }
        await db.SaveChangesAsync();

        // =========================================================================
        // JOURNEY 6: KHÓA SỔ CUỐI KỲ & KẾT CHUYỂN KQKD (Period Close & PnL 911 -> 4212)
        // =========================================================================
        // Revenue (5111): 120M
        // COGS (632): 70M
        // OpEx (6422): 20M
        // Net Profit before Tax = 120M - 70M - 20M = 30M VND
        var closeDate = new DateOnly(2026, 3, 31);
        var pnlCloseVoucher = new Voucher(
            VoucherId.New(),
            "KC-KQKD-Q1",
            VoucherType.GeneralJournal,
            closeDate,
            closeDate,
            "Kết chuyển xác định kết quả kinh doanh Quý 1/2026");

        // 1. Clear Revenue to 911: Nợ 5111 / Có 911: 120M
        pnlCloseVoucher.AddLine(new AccountId("5111"), LedgerEntryType.Debit, 120_000_000m, "Kết chuyển doanh thu thuần");
        pnlCloseVoucher.AddLine(new AccountId("911"), LedgerEntryType.Credit, 120_000_000m, "Ghi tăng kết quả kinh doanh");

        // 2. Clear Expenses to 911: Nợ 911: 90M / Có 632: 70M, Có 6422: 20M
        pnlCloseVoucher.AddLine(new AccountId("911"), LedgerEntryType.Debit, 90_000_000m, "Kết chuyển chi phí trong kỳ");
        pnlCloseVoucher.AddLine(new AccountId("632"), LedgerEntryType.Credit, 70_000_000m, "Kết chuyển giá vốn");
        pnlCloseVoucher.AddLine(new AccountId("6422"), LedgerEntryType.Credit, 20_000_000m, "Kết chuyển chi phí quản lý");

        // 3. Transfer Profit to 4212: Nợ 911: 30M / Có 4212: 30M
        pnlCloseVoucher.AddLine(new AccountId("911"), LedgerEntryType.Debit, 30_000_000m, "Kết chuyển lãi thuần sang LN sau thuế");
        pnlCloseVoucher.AddLine(new AccountId("4212"), LedgerEntryType.Credit, 30_000_000m, "Lợi nhuận sau thuế chưa phân phối năm nay");

        pnlCloseVoucher.Post("chief_accountant");
        db.GlVouchers.Add(pnlCloseVoucher);

        foreach (var l in pnlCloseVoucher.Lines)
        {
            db.GeneralLedgerEntries.Add(new GeneralLedgerEntry(
                Guid.NewGuid(), pnlCloseVoucher.Id, closeDate, periodQ1, l.AccountId,
                l.EntryType == LedgerEntryType.Debit ? l.AmountBase : 0m,
                l.EntryType == LedgerEntryType.Credit ? l.AmountBase : 0m));
        }
        await db.SaveChangesAsync();

        // =========================================================================
        // JOURNEY 7: INVARIANT VERIFICATIONS ACROSS ALL MODULES
        // =========================================================================
        var allGlEntries = await db.GeneralLedgerEntries.ToListAsync();

        // INVARIANT 1: Double-Entry Conservation across whole enterprise
        var sumDebit = allGlEntries.Sum(e => e.DebitAmount);
        var sumCredit = allGlEntries.Sum(e => e.CreditAmount);
        Assert.Equal(sumDebit, sumCredit);
        Assert.True(sumDebit > 0m);

        // INVARIANT 2: Sub-ledger AR/AP = GL TK 131/331 = 0 (both paid in full)
        var gl131Debit = allGlEntries.Where(e => e.AccountId.Value == "131").Sum(e => e.DebitAmount - e.CreditAmount);
        var gl331Credit = allGlEntries.Where(e => e.AccountId.Value == "331").Sum(e => e.CreditAmount - e.DebitAmount);
        Assert.Equal(0m, gl131Debit);
        Assert.Equal(0m, gl331Credit);
        Assert.Equal(0m, customer.CurrentReceivableBalance);
        Assert.Equal(0m, vendor.CurrentPayableBalance);

        // INVARIANT 3: Inventory value matches GL TK 1561
        // Opening 200M + Purchase 100M - COGS 70M = 230M VND
        var gl1561Balance = allGlEntries.Where(e => e.AccountId.Value == "1561").Sum(e => e.DebitAmount - e.CreditAmount);
        Assert.Equal(230_000_000m, gl1561Balance);

        // INVARIANT 4: Cash + Bank matches GL
        // Cash (1111): 100M
        // Bank (1121): Opening 400M + Rec 132M - Pay 110M - Salary 20M = 402M VND
        var gl1111 = allGlEntries.Where(e => e.AccountId.Value == "1111").Sum(e => e.DebitAmount - e.CreditAmount);
        var gl1121 = allGlEntries.Where(e => e.AccountId.Value == "1121").Sum(e => e.DebitAmount - e.CreditAmount);
        Assert.Equal(100_000_000m, gl1111);
        Assert.Equal(402_000_000m, gl1121);

        // Total Cash & Equivalents = 502M VND
        var totalCash = gl1111 + gl1121;
        Assert.Equal(502_000_000m, totalCash);

        // INVARIANT 5: Retained Earnings (4212) = 30M VND
        var gl4212 = allGlEntries.Where(e => e.AccountId.Value == "4212").Sum(e => e.CreditAmount - e.DebitAmount);
        Assert.Equal(30_000_000m, gl4212);

        // INVARIANT 6: Balance Sheet Equation: Assets = Liabilities + Equity
        // Assets = Cash (502M) + VAT Deductible (1331: 10M) + Inventory (1561: 230M) = 742M VND
        // Liabilities = Output VAT Payable (33311: 12M)
        // Equity = Capital (4111: 700M) + Profit (4212: 30M) = 730M VND
        // Total Liabilities & Equity = 12M + 730M = 742M VND
        var gl1331 = allGlEntries.Where(e => e.AccountId.Value == "1331").Sum(e => e.DebitAmount - e.CreditAmount);
        var gl33311 = allGlEntries.Where(e => e.AccountId.Value == "33311").Sum(e => e.CreditAmount - e.DebitAmount);
        var gl4111 = allGlEntries.Where(e => e.AccountId.Value == "4111").Sum(e => e.CreditAmount - e.DebitAmount);

        var totalAssets = totalCash + gl1331 + gl1561Balance;
        var totalLiabEquity = gl33311 + gl4111 + gl4212;

        Assert.Equal(742_000_000m, totalAssets);
        Assert.Equal(742_000_000m, totalLiabEquity);
        Assert.Equal(totalAssets, totalLiabEquity); // Exact Balance Sheet Balance!
    }
}
