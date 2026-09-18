using Accounting.Application.Features.Books;
using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Compliance;
using Accounting.Infrastructure.Persistence.Context;
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;
using Xunit;

namespace Accounting.Domain.Tests.Books;

public class StatutoryBooksTests : IDisposable
{
    private readonly SqliteConnection _connection;
    private readonly AccountingDbContext _context;

    public StatutoryBooksTests()
    {
        _connection = new SqliteConnection("DataSource=:memory:");
        _connection.Open();

        var options = new DbContextOptionsBuilder<AccountingDbContext>()
            .UseSqlite(_connection)
            .Options;

        _context = new AccountingDbContext(options);
        _context.Database.EnsureCreated();
    }

    public void Dispose()
    {
        _context.Dispose();
        _connection.Dispose();
    }

    [Fact]
    public async Task S03aJournalBook_ShouldAggregatePostedVouchersChronologically()
    {
        var handler = new StatutoryBookQueryHandlers(_context);

        var acc1111 = new Account(new AccountId("1111"), "Tiền mặt", AccountType.Asset, BalanceNature.DebitBalance);
        var acc5111 = new Account(new AccountId("5111"), "Doanh thu", AccountType.Revenue, BalanceNature.CreditBalance);
        _context.MasterAccounts.AddRange(acc1111, acc5111);
        await _context.SaveChangesAsync();

        var v1 = new Voucher(VoucherId.New(), "PKT-001", VoucherType.GeneralJournal, new DateOnly(2026, 1, 10), new DateOnly(2026, 1, 10), "Thu tien ban hang");
        v1.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 10_000_000m, "Thu tien mat");
        v1.AddLine(new AccountId("5111"), LedgerEntryType.Credit, 10_000_000m, "Doanh thu ban hang");
        v1.Post("admin");

        _context.GlVouchers.Add(v1);
        _context.GeneralLedgerEntries.AddRange(
            new GeneralLedgerEntry(Guid.NewGuid(), v1.Id, v1.PostingDate, FiscalPeriodId.FromYearMonth(2026, 1), new AccountId("1111"), 10_000_000m, 0m),
            new GeneralLedgerEntry(Guid.NewGuid(), v1.Id, v1.PostingDate, FiscalPeriodId.FromYearMonth(2026, 1), new AccountId("5111"), 0m, 10_000_000m)
        );
        await _context.SaveChangesAsync();

        var res = await handler.Handle(new GetS03aJournalBookQuery(new DateOnly(2026, 1, 1), new DateOnly(2026, 1, 31)), CancellationToken.None);

        Assert.True(res.IsSuccess);
        Assert.NotNull(res.Value);
        Assert.Single(res.Value.Lines);
        Assert.Equal(10_000_000m, res.Value.TotalAmount);
        Assert.Equal("1111", res.Value.Lines[0].DebitAccountNumber);
        Assert.Equal("5111", res.Value.Lines[0].CreditAccountNumber);
    }

    [Fact]
    public async Task S03bGeneralLedgerBook_ShouldCalculateOpeningMovementsAndClosingBalances()
    {
        var handler = new StatutoryBookQueryHandlers(_context);

        var acc = new Account(new AccountId("1111"), "Tien mat Viet Nam", AccountType.Asset, BalanceNature.DebitBalance);
        var accBank = new Account(new AccountId("1121"), "Tien gui ngan hang", AccountType.Asset, BalanceNature.DebitBalance);
        _context.MasterAccounts.AddRange(acc, accBank);
        await _context.SaveChangesAsync();

        // Seed opening balance entry before 2026-02-01
        var openEntry = new GeneralLedgerEntry(Guid.NewGuid(), VoucherId.New(), new DateOnly(2026, 1, 15), FiscalPeriodId.FromYearMonth(2026, 1), new AccountId("1111"), 5_000_000m, 0m);
        _context.GeneralLedgerEntries.Add(openEntry);

        // Seed period voucher in Feb 2026
        var v = new Voucher(VoucherId.New(), "PKT-002", VoucherType.GeneralJournal, new DateOnly(2026, 2, 10), new DateOnly(2026, 2, 10), "Rut tien gui nhap quy");
        v.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 3_000_000m, "Nhap quy");
        v.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 3_000_000m, "Tien gui");
        v.Post("admin");

        _context.GlVouchers.Add(v);
        _context.GeneralLedgerEntries.Add(new GeneralLedgerEntry(Guid.NewGuid(), v.Id, v.PostingDate, FiscalPeriodId.FromYearMonth(2026, 2), new AccountId("1111"), 3_000_000m, 0m));
        await _context.SaveChangesAsync();

        var res = await handler.Handle(new GetS03bGeneralLedgerBookQuery("1111", new DateOnly(2026, 2, 1), new DateOnly(2026, 2, 28)), CancellationToken.None);

        Assert.True(res.IsSuccess);
        Assert.NotNull(res.Value);
        Assert.Equal(5_000_000m, res.Value.OpeningDebit);
        Assert.Equal(0m, res.Value.OpeningCredit);
        Assert.Equal(3_000_000m, res.Value.TotalDebitMovement);
        Assert.Equal(8_000_000m, res.Value.ClosingDebit);
    }

    [Fact]
    public async Task DetailedPartnerBook_ShouldTrackCustomerReceivables131()
    {
        var handler = new StatutoryBookQueryHandlers(_context);
        var partnerId = PartnerId.New();

        var partner = new Domain.MasterData.Partners.BusinessPartner(
            partnerId, "CUST-001", "Cong ty TNHH Thuong Mai ABC", Domain.MasterData.Common.PartnerType.Customer);
        _context.BusinessPartners.Add(partner);

        var acc131 = new Account(new AccountId("131"), "Phai thu khach hang", AccountType.Asset, BalanceNature.DebitBalance);
        var acc511 = new Account(new AccountId("511"), "Doanh thu", AccountType.Revenue, BalanceNature.CreditBalance);
        var acc112 = new Account(new AccountId("112"), "Tien gui", AccountType.Asset, BalanceNature.DebitBalance);
        _context.MasterAccounts.AddRange(acc131, acc511, acc112);
        await _context.SaveChangesAsync();

        // Invoice sale
        var v1 = new Voucher(VoucherId.New(), "HD-001", VoucherType.SalesInvoice, new DateOnly(2026, 3, 5), new DateOnly(2026, 3, 5), "Ban hang chua thu tien");
        v1.AddLine(new AccountId("131"), LedgerEntryType.Debit, 20_000_000m, "Phai thu", partnerId: partnerId);
        v1.AddLine(new AccountId("511"), LedgerEntryType.Credit, 20_000_000m, "Doanh thu");
        v1.Post("accountant");

        // Customer partial payment
        var v2 = new Voucher(VoucherId.New(), "BN-001", VoucherType.CashReceipt, new DateOnly(2026, 3, 10), new DateOnly(2026, 3, 10), "Khach hang tra tien");
        v2.AddLine(new AccountId("112"), LedgerEntryType.Debit, 12_000_000m, "Nhan tien gui");
        v2.AddLine(new AccountId("131"), LedgerEntryType.Credit, 12_000_000m, "Giam phai thu", partnerId: partnerId);
        v2.Post("accountant");

        _context.GlVouchers.AddRange(v1, v2);
        _context.GeneralLedgerEntries.AddRange(
            new GeneralLedgerEntry(Guid.NewGuid(), v1.Id, v1.PostingDate, FiscalPeriodId.FromYearMonth(2026, 3), new AccountId("131"), 20_000_000m, 0m, partnerId: partnerId),
            new GeneralLedgerEntry(Guid.NewGuid(), v2.Id, v2.PostingDate, FiscalPeriodId.FromYearMonth(2026, 3), new AccountId("131"), 0m, 12_000_000m, partnerId: partnerId)
        );
        await _context.SaveChangesAsync();

        var query = new GetDetailedPartnerBookQuery("131", partnerId.Value, new DateOnly(2026, 3, 1), new DateOnly(2026, 3, 31));
        var res = await handler.Handle(query, CancellationToken.None);

        Assert.True(res.IsSuccess);
        Assert.NotNull(res.Value);
        Assert.Equal(2, res.Value.Lines.Count);
        Assert.Equal(20_000_000m, res.Value.TotalDebit);
        Assert.Equal(12_000_000m, res.Value.TotalCredit);
        Assert.Equal(8_000_000m, res.Value.ClosingBalance);
    }

    [Fact]
    public void ExcelAndPrintGenerators_ShouldProduceValidOutputs()
    {
        var excelSvc = new Accounting.Infrastructure.Compliance.Services.ExcelExportService();
        var printSvc = new Accounting.Infrastructure.Compliance.Services.PrintDocumentGenerator();

        var headers = new List<string> { "Ngay", "So CT", "Dien giai", "So tien" };
        var sampleRows = new List<object[]>
        {
            new object[] { new DateOnly(2026, 1, 10), "PKT-001", "Giao dich mau", 5_000_000m }
        };

        var excelBytes = excelSvc.ExportToExcel("SoNhatKyChung", headers, sampleRows, r => r, "Tu ngay 01/01/2026 den ngay 31/01/2026");
        Assert.NotNull(excelBytes);
        Assert.True(excelBytes.Length > 0);

        var html = printSvc.GenerateHtmlPrintout("SO NHAT KY CHUNG", "Mau S03a-DN", headers, sampleRows);
        Assert.NotNull(html);
        Assert.Contains("SO NHAT KY CHUNG", html);
        Assert.Contains("5,000,000.00", html);
    }
}