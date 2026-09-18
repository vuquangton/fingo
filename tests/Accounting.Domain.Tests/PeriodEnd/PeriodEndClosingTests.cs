using System.Data;
using System.Xml;
using System.Xml.Schema;
using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Services;
using Accounting.Application.Features.PeriodEnd;
using Accounting.Application.Features.Reporting;
using Accounting.Application.Features.Tax;
using Accounting.Domain.Common;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Exceptions;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.PeriodEnd;
using Accounting.Domain.Reporting;
using Accounting.Domain.Tax;
using Accounting.Infrastructure.Persistence.Connections;
using Accounting.Infrastructure.Persistence.Context;
using Accounting.Infrastructure.Persistence.Seeding;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using Xunit;
using Account = Accounting.Domain.MasterData.Accounts.Account;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;
using Voucher = Accounting.Domain.Ledger.Voucher;
using VoucherType = Accounting.Domain.Ledger.VoucherType;

namespace Accounting.Domain.Tests.PeriodEnd;

public class PeriodEndClosingTests : IDisposable
{
    private readonly string _testDbPath;
    private readonly AccountingDbContext _context;
    private readonly ISqlConnectionFactory _connectionFactory;
    private readonly IGlVoucherBridgeService _glBridge;

    private readonly CurrencyCode _vnd = new("VND");
    private readonly CurrencyCode _usd = new("USD");

    public PeriodEndClosingTests()
    {
        _testDbPath = $"period_end_test_{Guid.NewGuid():N}.db";

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
        var acc111 = new Account(new AccountId("111"), "Ti?n m?t", AccountType.Asset, BalanceNature.DebitBalance, isParent: true);
        var acc1111 = new Account(new AccountId("1111"), "Ti?n Vi?t Nam", AccountType.Asset, BalanceNature.DebitBalance, parentAccountId: new AccountId("111"));
        var acc112 = new Account(new AccountId("112"), "Ti?n g?i ng�n h�ng", AccountType.Asset, BalanceNature.DebitBalance, isParent: true);
        var acc1121 = new Account(new AccountId("1121"), "Ti?n Vi?t Nam", AccountType.Asset, BalanceNature.DebitBalance, parentAccountId: new AccountId("112"));
        var acc1122 = new Account(new AccountId("1122"), "Ngo?i t?", AccountType.Asset, BalanceNature.DebitBalance, parentAccountId: new AccountId("112"));
        var acc133 = new Account(new AccountId("133"), "Thu? GTGT du?c kh?u tr?", AccountType.Asset, BalanceNature.DebitBalance, isParent: true);
        var acc1331 = new Account(new AccountId("1331"), "Thu? GTGT du?c kh?u tr? c?a HHDV", AccountType.Asset, BalanceNature.DebitBalance, parentAccountId: new AccountId("133"));

        var acc333 = new Account(new AccountId("333"), "Thu? v� c�c kho?n ph?i n?p Nh� nu?c", AccountType.Liability, BalanceNature.CreditBalance, isParent: true);
        var acc3331 = new Account(new AccountId("3331"), "Thu? gi� tr? gia tang ph?i n?p", AccountType.Liability, BalanceNature.CreditBalance, parentAccountId: new AccountId("333"));

        var acc411 = new Account(new AccountId("411"), "V?n d?u tu c?a ch? s? h?u", AccountType.Equity, BalanceNature.CreditBalance, isParent: true);
        var acc4111 = new Account(new AccountId("4111"), "V?n g�p c?a ch? s? h?u", AccountType.Equity, BalanceNature.CreditBalance, parentAccountId: new AccountId("411"));
        var acc413 = new Account(new AccountId("413"), "Ch�nh l?ch t? gi� h?i do�i", AccountType.Equity, BalanceNature.Bilateral, isParent: true);
        var acc4131 = new Account(new AccountId("4131"), "Ch�nh l?ch t? gi� do d�nh gi� l?i", AccountType.Equity, BalanceNature.Bilateral, parentAccountId: new AccountId("413"));
        var acc421 = new Account(new AccountId("421"), "L?i nhu?n sau thu? chua ph�n ph?i", AccountType.Equity, BalanceNature.Bilateral, isParent: true);
        var acc4212 = new Account(new AccountId("4212"), "L?i nhu?n sau thu? chua ph�n ph?i nam nay", AccountType.Equity, BalanceNature.Bilateral, parentAccountId: new AccountId("421"));

        var acc511 = new Account(new AccountId("511"), "Doanh thu b�n h�ng v� cung c?p d?ch v?", AccountType.Revenue, BalanceNature.ZeroBalance, isParent: true);
        var acc5111 = new Account(new AccountId("5111"), "Doanh thu b�n h�ng h�a", AccountType.Revenue, BalanceNature.ZeroBalance, parentAccountId: new AccountId("511"));
        var acc515 = new Account(new AccountId("515"), "Doanh thu ho?t d?ng t�i ch�nh", AccountType.Revenue, BalanceNature.ZeroBalance);

        var acc635 = new Account(new AccountId("635"), "Chi ph� t�i ch�nh", AccountType.Expense, BalanceNature.ZeroBalance);
        var acc642 = new Account(new AccountId("642"), "Chi ph� qu?n l� doanh nghi?p", AccountType.Expense, BalanceNature.ZeroBalance, isParent: true);
        var acc6422 = new Account(new AccountId("6422"), "Chi ph� nh�n vi�n qu?n l�", AccountType.Expense, BalanceNature.ZeroBalance, parentAccountId: new AccountId("642"));

        var acc911 = new Account(new AccountId("911"), "X�c d?nh k?t qu? kinh doanh", AccountType.Equity, BalanceNature.ZeroBalance);

        _context.MasterAccounts.AddRange(
            acc111, acc1111, acc112, acc1121, acc1122, acc133, acc1331,
            acc333, acc3331,
            acc411, acc4111, acc413, acc4131, acc421, acc4212,
            acc511, acc5111, acc515,
            acc635, acc642, acc6422,
            acc911);

        // 2. Fiscal Periods
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
    public async Task PnLClosing_ClearanceInvariant_BalancesToZeroAndTransfersNetProfit()
    {
        // 1. Initial Capital: N? 1111 (100M) / C� 4111 (100M)
        var initVoucher = new Voucher(VoucherId.New(), "V-INIT-01", VoucherType.GeneralJournal, new DateOnly(2026, 1, 1), new DateOnly(2026, 1, 1), "V?n g�p", _vnd, 1.0m);
        initVoucher.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 100_000_000m, "V?n g�p");
        initVoucher.AddLine(new AccountId("4111"), LedgerEntryType.Credit, 100_000_000m, "V?n g�p");
        await _glBridge.PostOperationalVoucherAsync(initVoucher, "tester");

        // 2. Revenue 100M: N? 1121 (100M) / C� 5111 (100M)
        var revVoucher = new Voucher(VoucherId.New(), "V-REV-01", VoucherType.GeneralJournal, new DateOnly(2026, 1, 15), new DateOnly(2026, 1, 15), "Doanh thu b�n h�ng", _vnd, 1.0m);
        revVoucher.AddLine(new AccountId("1121"), LedgerEntryType.Debit, 100_000_000m, "Thu ti?n b�n h�ng");
        revVoucher.AddLine(new AccountId("5111"), LedgerEntryType.Credit, 100_000_000m, "Doanh thu b�n h�ng");
        await _glBridge.PostOperationalVoucherAsync(revVoucher, "tester");

        // 3. Expense 40M: N? 6422 (40M) / C� 1111 (40M)
        var expVoucher = new Voucher(VoucherId.New(), "V-EXP-01", VoucherType.GeneralJournal, new DateOnly(2026, 1, 20), new DateOnly(2026, 1, 20), "Chi ph� QLDN", _vnd, 1.0m);
        expVoucher.AddLine(new AccountId("6422"), LedgerEntryType.Debit, 40_000_000m, "Chi ph� qu?n l�");
        expVoucher.AddLine(new AccountId("1111"), LedgerEntryType.Credit, 40_000_000m, "Chi ti?n m?t");
        await _glBridge.PostOperationalVoucherAsync(expVoucher, "tester");

        // 4. Action: Execute P&L Closing for 2026/01
        var handler = new ExecutePnLClosingCommandHandler(_context, _glBridge);
        var result = await handler.Handle(new ExecutePnLClosingCommand(2026, 1, "test_accountant"), CancellationToken.None);

        Assert.True(result.IsSuccess);
        Assert.NotEqual(PeriodClosingRunId.Empty, result.Value);

        // 5. Verification: Voucher Lines transferred 5111 -> 911, 6422 -> 911, and 911 -> 4212 (60M)
        var closingRun = await _context.PeriodClosingRuns.FindAsync(result.Value!);
        Assert.NotNull(closingRun);
        Assert.Equal(ClosingStatus.Success, closingRun.Status);
        Assert.Single(closingRun.GeneratedVoucherIds);

        var voucherId = closingRun.GeneratedVoucherIds.First();
        var closingVoucher = await _context.GlVouchers.Include(v => v.Lines).FirstOrDefaultAsync(v => v.Id == voucherId);
        Assert.NotNull(closingVoucher);
        Assert.Equal(VoucherStatus.Posted, closingVoucher.Status);

        // Lines:
        // Debit 5111: 100M
        // Credit 911: 100M
        // Debit 911: 40M
        // Credit 6422: 40M
        // Debit 911: 60M (net profit)
        // Credit 4212: 60M (net profit)
        var line5111 = closingVoucher.Lines.FirstOrDefault(l => l.AccountId == new AccountId("5111"));
        Assert.NotNull(line5111);
        Assert.Equal(LedgerEntryType.Debit, line5111.EntryType);
        Assert.Equal(100_000_000m, line5111.AmountBase);

        var line6422 = closingVoucher.Lines.FirstOrDefault(l => l.AccountId == new AccountId("6422"));
        Assert.NotNull(line6422);
        Assert.Equal(LedgerEntryType.Credit, line6422.EntryType);
        Assert.Equal(40_000_000m, line6422.AmountBase);

        var line4212 = closingVoucher.Lines.FirstOrDefault(l => l.AccountId == new AccountId("4212"));
        Assert.NotNull(line4212);
        Assert.Equal(LedgerEntryType.Credit, line4212.EntryType);
        Assert.Equal(60_000_000m, line4212.AmountBase);

        // 6. Invariant Verification: All Class 5, 6, 7, 8, 9 accounts MUST have balance of EXACTLY zero
        var glEntries = await _context.GeneralLedgerEntries.ToListAsync();
        var zeroAccounts = new[] { "5111", "6422", "911" };

        foreach (var accCode in zeroAccounts)
        {
            var accDebit = glEntries.Where(e => e.AccountId == new AccountId(accCode)).Sum(e => e.DebitAmount);
            var accCredit = glEntries.Where(e => e.AccountId == new AccountId(accCode)).Sum(e => e.CreditAmount);
            Assert.Equal(0m, accDebit - accCredit);
        }
    }

    [Fact]
    public async Task PnLClosing_IdempotencyGuard_ThrowsPeriodAlreadyClosedException()
    {
        // Execute once
        var handler = new ExecutePnLClosingCommandHandler(_context, _glBridge);
        var result = await handler.Handle(new ExecutePnLClosingCommand(2026, 2, "tester"), CancellationToken.None);
        Assert.True(result.IsSuccess);

        // Attempt second execution on same period -> must throw PeriodAlreadyClosedException
        await Assert.ThrowsAsync<PeriodAlreadyClosedException>(() =>
            handler.Handle(new ExecutePnLClosingCommand(2026, 2, "tester"), CancellationToken.None));
    }

    [Fact]
    public async Task PnLClosing_UnbalancedTrialBalance_ThrowsUnbalancedTrialBalanceException()
    {
        // Directly corrupt GL entries to simulate out-of-balance condition
        var corruptEntry = new GeneralLedgerEntry(
            Guid.NewGuid(),
            VoucherId.New(),
            new DateOnly(2026, 3, 15),
            FiscalPeriodId.FromYearMonth(2026, 3),
            new AccountId("1111"),
            debitAmount: 10_000_000m,
            creditAmount: 0m);

        _context.AddEntity(corruptEntry);
        await _context.SaveChangesAsync();

        var handler = new ExecutePnLClosingCommandHandler(_context, _glBridge);
        await Assert.ThrowsAsync<UnbalancedTrialBalanceException>(() =>
            handler.Handle(new ExecutePnLClosingCommand(2026, 3, "tester"), CancellationToken.None));
    }

    [Fact]
    public async Task FxRevaluation_MonetaryForeignItems_RevaluesAndPostsDifferenceTo413And515()
    {
        // Post USD voucher: $1,000 @ 25,000 VND = 25,000,000 VND
        var foreignVoucher = new Voucher(
            VoucherId.New(),
            "V-USD-01",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 1, 10),
            new DateOnly(2026, 1, 10),
            "Thu ti?n USD",
            _usd,
            25_000m);

        foreignVoucher.AddLine(new AccountId("1122"), LedgerEntryType.Debit, 1_000m, "USD deposit");
        foreignVoucher.AddLine(new AccountId("515"), LedgerEntryType.Credit, 1_000m, "Financial income");
        await _glBridge.PostOperationalVoucherAsync(foreignVoucher, "tester");

        // Action: Revalue at 25,400 VND/USD (Gain = 400,000 VND)
        var handler = new ExecuteFxRevaluationCommandHandler(_context, _glBridge);
        var result = await handler.Handle(new ExecuteFxRevaluationCommand(2026, 1, "USD", 25_400m, "tester"), CancellationToken.None);

        Assert.True(result.IsSuccess);

        var run = await _context.PeriodClosingRuns.FindAsync(result.Value!);
        Assert.NotNull(run);
        Assert.Equal(ClosingStep.FxRevaluation, run.ClosingStep);
        Assert.Single(run.GeneratedVoucherIds);

        var voucher = await _context.GlVouchers.Include(v => v.Lines).FirstOrDefaultAsync(v => v.Id == run.GeneratedVoucherIds.First());
        Assert.NotNull(voucher);
        Assert.Equal(VoucherStatus.Posted, voucher.Status);

        // Lines:
        // N? 1122: 400,000
        // C� 4131: 400,000
        // N? 4131: 400,000
        // C� 515: 400,000
        var debit1122 = voucher.Lines.FirstOrDefault(l => l.AccountId == new AccountId("1122") && l.EntryType == LedgerEntryType.Debit);
        Assert.NotNull(debit1122);
        Assert.Equal(400_000m, debit1122.AmountBase);

        var credit515 = voucher.Lines.FirstOrDefault(l => l.AccountId == new AccountId("515") && l.EntryType == LedgerEntryType.Credit);
        Assert.NotNull(credit515);
        Assert.Equal(400_000m, credit515.AmountBase);
    }

    [Fact]
    public async Task FinancialStatement_BalanceSheet_TotalAssetsEqualsTotalLiabilitiesAndEquity()
    {
        // Seed statutory report templates
        await ReportTemplateSeeder.SeedReportTemplatesAsync(_context);

        // Seed transactions:
        // 1. Initial capital: N? 1111 (50M) / C� 4111 (50M)
        var initVoucher = new Voucher(VoucherId.New(), "V-BCTC-INIT", VoucherType.GeneralJournal, new DateOnly(2026, 1, 1), new DateOnly(2026, 1, 1), "V?n", _vnd, 1.0m);
        initVoucher.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 50_000_000m, "Ti?n m?t");
        initVoucher.AddLine(new AccountId("4111"), LedgerEntryType.Credit, 50_000_000m, "V?n g�p");
        await _glBridge.PostOperationalVoucherAsync(initVoucher, "tester");

        // 2. Revenue: N? 1121 (30M) / C� 5111 (30M)
        var revVoucher = new Voucher(VoucherId.New(), "V-BCTC-REV", VoucherType.GeneralJournal, new DateOnly(2026, 1, 15), new DateOnly(2026, 1, 15), "Doanh thu", _vnd, 1.0m);
        revVoucher.AddLine(new AccountId("1121"), LedgerEntryType.Debit, 30_000_000m, "Ti?n g?i");
        revVoucher.AddLine(new AccountId("5111"), LedgerEntryType.Credit, 30_000_000m, "Doanh thu");
        await _glBridge.PostOperationalVoucherAsync(revVoucher, "tester");

        // 3. P&L Closing to transfer revenue 30M -> 4212
        var pnlHandler = new ExecutePnLClosingCommandHandler(_context, _glBridge);
        await pnlHandler.Handle(new ExecutePnLClosingCommand(2026, 1, "tester"), CancellationToken.None);

        // 4. Query B01-DN Balance Sheet
        var queryHandler = new GetFinancialStatementQueryHandler(_context, _connectionFactory);
        var queryRes = await queryHandler.Handle(
            new GetFinancialStatementQuery("B01-DN", new DateOnly(2026, 1, 1), new DateOnly(2026, 1, 31)),
            CancellationToken.None);

        Assert.True(queryRes.IsSuccess);
        var bctc = queryRes.Value!;

        // Total Assets (270) = 50M (1111) + 30M (1121) = 80M
        // Total Liabilities & Equity (440) = 50M (4111) + 30M (4212) = 80M
        Assert.Equal(80_000_000m, bctc.TotalAssets);
        Assert.Equal(80_000_000m, bctc.TotalLiabilitiesAndEquity);
        Assert.True(bctc.IsBalanced);
    }

    [Fact]
    public async Task ExportTaxXml_ValidVatDeclaration_GeneratesXmlAndValidatesAgainstXsd()
    {
        // 1. Post input VAT (TK 1331 = 12M) and output VAT (TK 3331 = 20M)
        var vatVoucher = new Voucher(VoucherId.New(), "V-VAT-01", VoucherType.GeneralJournal, new DateOnly(2026, 1, 15), new DateOnly(2026, 1, 15), "Giao d?ch VAT", _vnd, 1.0m);
        vatVoucher.AddLine(new AccountId("1331"), LedgerEntryType.Debit, 12_000_000m, "VAT d?u v�o");
        vatVoucher.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 12_000_000m, "Thanh to�n");
        vatVoucher.AddLine(new AccountId("1121"), LedgerEntryType.Debit, 20_000_000m, "Thu ti?n");
        vatVoucher.AddLine(new AccountId("3331"), LedgerEntryType.Credit, 20_000_000m, "VAT d?u ra");
        await _glBridge.PostOperationalVoucherAsync(vatVoucher, "tester");

        // 2. Export Tax XML
        var handler = new ExportTaxXmlCommandHandler(_context);
        var result = await handler.Handle(new ExportTaxXmlCommand(
            Year: 2026,
            Month: 1,
            CompanyTaxCode: "0109998888",
            CompanyName: "C�NG TY C? PH?N PH?N M?M K? TO�N",
            DirectorName: "NGUY?N VAN GI�M �?C"), CancellationToken.None);

        Assert.True(result.IsSuccess);
        var dto = result.Value!;

        Assert.Equal(12_000_000m, dto.InputVat);
        Assert.Equal(20_000_000m, dto.OutputVat);
        Assert.Equal(8_000_000m, dto.VatPayable);
        Assert.Equal(0m, dto.VatCarriedForward);
        Assert.False(string.IsNullOrWhiteSpace(dto.XmlContent));

        // 3. Validate against local XSD schema
        var xsdPath = Path.Combine(AppContext.BaseDirectory, "PeriodEnd", "htkk_01_gtgt.xsd");
        if (!File.Exists(xsdPath))
        {
            xsdPath = Path.Combine(Directory.GetCurrentDirectory(), "PeriodEnd", "htkk_01_gtgt.xsd");
        }

        Assert.True(File.Exists(xsdPath), $"XSD schema file not found at {xsdPath}");

        var schemas = new XmlSchemaSet();
        schemas.Add(null, xsdPath);

        var xmlSettings = new XmlReaderSettings
        {
            ValidationType = ValidationType.Schema,
            Schemas = schemas
        };

        var validationErrors = new List<string>();
        xmlSettings.ValidationEventHandler += (s, e) =>
        {
            validationErrors.Add(e.Message);
        };

        using var stringReader = new StringReader(dto.XmlContent);
        using var xmlReader = XmlReader.Create(stringReader, xmlSettings);
        while (xmlReader.Read()) { }

        Assert.Empty(validationErrors);

        // 4. Validate metadata requirement
        await Assert.ThrowsAsync<InvalidTaxDeclarationException>(() =>
            handler.Handle(new ExportTaxXmlCommand(2026, 1, "", "Company", "Director"), CancellationToken.None));
    }

    [Fact]
    public async Task Live_MariaDb_PeriodEndClosingAndBctc_Succeeds()
    {
        var serverConnStr = "Server=localhost;Port=3306;User=dev;Password=123456;TreatTinyAsBoolean=true;CharSet=utf8mb4;";
        var connStr = "Server=localhost;Port=3306;Database=accounting_period_test;User=dev;Password=123456;TreatTinyAsBoolean=true;CharSet=utf8mb4;";
        var serverVersion = new MariaDbServerVersion(new Version(12, 3, 0));

        try
        {
            using var pingConn = new MySqlConnector.MySqlConnection(serverConnStr);
            await pingConn.OpenAsync();
            using var cmd = pingConn.CreateCommand();
            cmd.CommandText = "CREATE DATABASE IF NOT EXISTS accounting_period_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;";
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
            await DbInitializer.SeedAsync(efContext);
        }
        await ReportTemplateSeeder.SeedReportTemplatesAsync(efContext);

        var glBridge = new GlVoucherBridgeService(efContext);

        // Post revenue and expense
        var v = new Voucher(VoucherId.New(), $"VM-{Guid.NewGuid():N}"[..15].ToUpperInvariant(), VoucherType.GeneralJournal, new DateOnly(2026, 3, 10), new DateOnly(2026, 3, 10), "MariaDB Closing Test", _vnd, 1.0m);
        v.AddLine(new AccountId("1121"), LedgerEntryType.Debit, 20_000_000m, "Ti?n g?i");
        v.AddLine(new AccountId("5111"), LedgerEntryType.Credit, 20_000_000m, "Doanh thu");
        v.AddLine(new AccountId("6422"), LedgerEntryType.Debit, 5_000_000m, "Chi ph� QLDN");
        v.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 5_000_000m, "Ti?n g?i");
        await glBridge.PostOperationalVoucherAsync(v, "tester");

        var existingRuns = await efContext.PeriodClosingRuns.Where(r => r.FiscalPeriodId == FiscalPeriodId.FromYearMonth(2026, 3)).ToListAsync();
        if (existingRuns.Count > 0)
        {
            efContext.PeriodClosingRuns.RemoveRange(existingRuns);
            await efContext.SaveChangesAsync();
        }

        var pnlHandler = new ExecutePnLClosingCommandHandler(efContext, glBridge);
        var closeRes = await pnlHandler.Handle(new ExecutePnLClosingCommand(2026, 3, "tester"), CancellationToken.None);

        Assert.True(closeRes.IsSuccess);
    }
}
