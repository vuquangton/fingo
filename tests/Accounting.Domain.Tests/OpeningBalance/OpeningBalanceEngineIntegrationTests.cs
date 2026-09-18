using Accounting.Application.Features.OpeningBalance;
using Accounting.Application.Features.Organization;
using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.OpeningBalance;
using Accounting.Domain.Organization;
using Accounting.Infrastructure.Persistence.Context;
using Accounting.Infrastructure.Persistence.Services;
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;
using Xunit;

namespace Accounting.Domain.Tests.OpeningBalance;

public class OpeningBalanceEngineIntegrationTests : IDisposable
{
    private readonly SqliteConnection _connection;
    private readonly AccountingDbContext _context;

    public OpeningBalanceEngineIntegrationTests()
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
    public async Task CompanySetting_UpdateAndGet_ShouldSucceed()
    {
        var handler = new CompanySettingHandlers(_context);

        var updateRes = await handler.Handle(new UpdateCompanySettingCommand(
            "0109999888",
            "CÔNG TY CỔ PHẦN CÔNG NGHỆ ALPHA",
            "Tòa nhà Landmark 81, TP. HCM",
            "Nguyễn Giám Đốc",
            "Lê Kế Toán"), CancellationToken.None);

        Assert.True(updateRes.IsSuccess);
        Assert.Equal("0109999888", updateRes.Value!.TaxCode);

        var getRes = await handler.Handle(new GetCompanySettingQuery(), CancellationToken.None);
        Assert.True(getRes.IsSuccess);
        Assert.Equal("CÔNG TY CỔ PHẦN CÔNG NGHỆ ALPHA", getRes.Value!.CompanyName);

        // Verify ReportHeaderProvider
        var provider = new ReportHeaderProvider(_context);
        var header = await provider.GetReportHeaderAsync();
        Assert.Equal("CÔNG TY CỔ PHẦN CÔNG NGHỆ ALPHA", header.CompanyName);
        Assert.Equal("0109999888", header.TaxCode);
    }

    [Fact]
    public async Task ValidateOpeningBalances_WhenUnbalanced_ShouldReturnTrialBalanceFatalIssue()
    {
        var handler = new OpeningBalanceHandlers(_context);

        var entries = new List<SaveOpeningBalanceEntryDto>
        {
            new("1111", 10_000_000m, 0m),
            new("4111", 0m, 8_000_000m) // Unbalanced by 2M
        };

        var saveRes = await handler.Handle(new BatchSaveOpeningBalancesCommand(2026, entries), CancellationToken.None);
        Assert.True(saveRes.IsSuccess);
        Assert.Equal(2, saveRes.Value);

        var valRes = await handler.Handle(new ValidateOpeningBalancesQuery(2026), CancellationToken.None);
        Assert.True(valRes.IsSuccess);
        Assert.False(valRes.Value!.IsTrialBalanceBalanced);
        Assert.Contains(valRes.Value.Issues, i => i.Category == "TrialBalance" && i.IsFatal);
    }

    [Fact]
    public async Task ValidateOpeningBalances_WhenMissingPartnerForArAp_ShouldReturnSubLedgerIssues()
    {
        var handler = new OpeningBalanceHandlers(_context);

        var entries = new List<SaveOpeningBalanceEntryDto>
        {
            new("131", 5_000_000m, 0m, PartnerId: null), // Missing customer
            new("331", 0m, 5_000_000m, PartnerId: null)  // Missing vendor
        };

        await handler.Handle(new BatchSaveOpeningBalancesCommand(2026, entries), CancellationToken.None);

        var valRes = await handler.Handle(new ValidateOpeningBalancesQuery(2026), CancellationToken.None);
        Assert.True(valRes.IsSuccess);
        Assert.True(valRes.Value!.IsTrialBalanceBalanced);
        Assert.Contains(valRes.Value.Issues, i => i.Category == "SubLedgerAR");
        Assert.Contains(valRes.Value.Issues, i => i.Category == "SubLedgerAP");
    }

    [Fact]
    public async Task CommitOpeningBalances_WhenBalanced_ShouldCreatePostedGlVoucherAndMarkEntriesCommitted()
    {
        var handler = new OpeningBalanceHandlers(_context);
        var partnerId = Guid.NewGuid();

        var acc1111 = new Account(new AccountId("1111"), "Tiền mặt", AccountType.Asset, BalanceNature.DebitBalance);
        var acc4111 = new Account(new AccountId("4111"), "Vốn đầu tư của chủ sở hữu", AccountType.Equity, BalanceNature.CreditBalance);
        _context.MasterAccounts.AddRange(acc1111, acc4111);
        await _context.SaveChangesAsync();

        var entries = new List<SaveOpeningBalanceEntryDto>
        {
            new("1111", 10_000_000m, 0m, Description: "Tiền gửi quỹ"),
            new("4111", 0m, 10_000_000m, Description: "Vốn đầu tư của chủ sở hữu")
        };

        await handler.Handle(new BatchSaveOpeningBalancesCommand(2026, entries), CancellationToken.None);

        var commitRes = await handler.Handle(new CommitOpeningBalancesCommand(2026, "ChiefAccountant"), CancellationToken.None);
        Assert.True(commitRes.IsSuccess);

        // Verify GL Voucher
        var voucher = await _context.GlVouchers.Include(v => v.Lines).FirstOrDefaultAsync(v => v.VoucherNumber == "OPN-2026");
        Assert.NotNull(voucher);
        Assert.Equal(VoucherStatus.Posted, voucher.Status);
        Assert.Equal(2, voucher.Lines.Count);
        Assert.Equal(10_000_000m, voucher.TotalDebitBase);
        Assert.Equal(10_000_000m, voucher.TotalCreditBase);

        // Verify OpeningBalanceEntry marked committed
        var savedEntries = await _context.OpeningBalanceEntries.Where(e => e.FiscalYear == 2026).ToListAsync();
        Assert.All(savedEntries, e => Assert.True(e.IsCommitted));
    }

    [Fact]
    public async Task ExcelImport_ValidSpreadsheet_ShouldParseAndSaveEntries()
    {
        var parser = new Accounting.Infrastructure.Compliance.Services.OpeningBalanceExcelParser();
        var handler = new OpeningBalanceHandlers(_context, parser);

        using var workbook = new ClosedXML.Excel.XLWorkbook();
        var ws = workbook.Worksheets.Add("SoDuDauKy");
        // Header
        ws.Cell(1, 1).Value = "Mã TK";
        ws.Cell(1, 2).Value = "Dư Nợ";
        ws.Cell(1, 3).Value = "Dư Có";
        ws.Cell(1, 4).Value = "Đối Tác";
        ws.Cell(1, 5).Value = "Kho";
        ws.Cell(1, 6).Value = "Vật Tư";
        ws.Cell(1, 7).Value = "Số Lượng";
        ws.Cell(1, 8).Value = "Đơn Giá";
        ws.Cell(1, 9).Value = "Diễn Giải";

        // Row 1: 1111
        ws.Cell(2, 1).Value = "1111";
        ws.Cell(2, 2).Value = 15000000;
        ws.Cell(2, 3).Value = 0;
        ws.Cell(2, 9).Value = "Tiền mặt tại quỹ";

        // Row 2: 4111
        ws.Cell(3, 1).Value = "4111";
        ws.Cell(3, 2).Value = 0;
        ws.Cell(3, 3).Value = 15000000;
        ws.Cell(3, 9).Value = "Vốn điều lệ";

        using var ms = new MemoryStream();
        workbook.SaveAs(ms);
        ms.Position = 0;

        var importRes = await handler.Handle(new ImportOpeningBalancesFromExcelCommand(2026, ms), CancellationToken.None);
        Assert.True(importRes.IsSuccess);
        Assert.True(importRes.Value!.IsTrialBalanceBalanced);
        Assert.Equal(15000000m, importRes.Value.TotalDebit);
        Assert.Equal(15000000m, importRes.Value.TotalCredit);

        var saved = await _context.OpeningBalanceEntries.Where(e => e.FiscalYear == 2026).ToListAsync();
        Assert.Equal(2, saved.Count);
    }

    [Fact]
    public async Task StatutoryBooks_ShouldIncludeAuthoritativeCompanyHeaders()
    {
        // 1. Setup Company
        var compHandler = new CompanySettingHandlers(_context);
        await compHandler.Handle(new UpdateCompanySettingCommand(
            "0315556677",
            "CÔNG TY TNHH MINH BẢO",
            "Quận Bình Thạnh, TP. Hồ Chí Minh",
            "Đặng Minh Giám Đốc",
            "Phạm Kế Toán Trưởng"), CancellationToken.None);

        var headerProvider = new ReportHeaderProvider(_context);
        var bookHandler = new Accounting.Application.Features.Books.StatutoryBookQueryHandlers(_context, headerProvider);

        // 2. Query S03a
        var s03aRes = await bookHandler.Handle(
            new Accounting.Application.Features.Books.GetS03aJournalBookQuery(new DateOnly(2026, 1, 1), new DateOnly(2026, 1, 31)),
            CancellationToken.None);

        Assert.True(s03aRes.IsSuccess);
        Assert.NotNull(s03aRes.Value!.Header);
        Assert.Equal("CÔNG TY TNHH MINH BẢO", s03aRes.Value.Header.CompanyName);
        Assert.Equal("0315556677", s03aRes.Value.Header.TaxCode);

        // 3. Query S03b
        var s03bRes = await bookHandler.Handle(
            new Accounting.Application.Features.Books.GetS03bGeneralLedgerBookQuery("1111", new DateOnly(2026, 1, 1), new DateOnly(2026, 1, 31)),
            CancellationToken.None);

        Assert.True(s03bRes.IsSuccess);
        Assert.NotNull(s03bRes.Value!.Header);
        Assert.Equal("CÔNG TY TNHH MINH BẢO", s03bRes.Value.Header.CompanyName);
        Assert.Equal("Đặng Minh Giám Đốc", s03bRes.Value.Header.LegalRepresentative);
    }
}
