using System.Data;
using Accounting.Application.Common.Interfaces;
using Accounting.Application.Features.Ledger;
using Accounting.Domain.Common;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Compliance;
using Accounting.Infrastructure.Persistence.Connections;
using Accounting.Infrastructure.Persistence.Context;
using Dapper;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using Xunit;

// Explicit aliases to eliminate all ambiguity with legacy GeneralLedger namespace
using Account = Accounting.Domain.MasterData.Accounts.Account;
using Voucher = Accounting.Domain.Ledger.Voucher;
using VoucherType = Accounting.Domain.Ledger.VoucherType;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;

namespace Accounting.Domain.Tests.Ledger;

public class VoucherDomainTests
{
    private readonly CurrencyCode _vnd = new("VND");
    private readonly CurrencyCode _usd = new("USD");

    [Fact]
    public void Post_UnbalancedBaseEntries_ThrowsUnbalancedJournalException()
    {
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-001",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 1),
            new DateOnly(2026, 2, 1),
            "Unbalanced voucher test",
            _vnd,
            1.0m);

        voucher.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 1_000_000m, "Debit cash");
        voucher.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 900_000m, "Credit bank");

        var ex = Assert.Throws<UnbalancedJournalException>(() => voucher.Post("test_user"));
        Assert.Contains("Debit", ex.Message);
        Assert.Contains("Credit", ex.Message);
    }

    [Fact]
    public void Post_UnbalancedForeignCurrencyEntries_ThrowsUnbalancedJournalException()
    {
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-USD-001",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 1),
            new DateOnly(2026, 2, 1),
            "USD Voucher",
            _usd,
            25_000m);

        // Lines unbalanced in original currency: $100 vs $90
        voucher.AddLine(new AccountId("1112"), LedgerEntryType.Debit, 100m, "USD debit");
        voucher.AddLine(new AccountId("1122"), LedgerEntryType.Credit, 90m, "USD credit");

        Assert.Throws<UnbalancedJournalException>(() => voucher.Post("test_user"));
    }

    [Fact]
    public void Post_LessThanTwoLines_ThrowsUnbalancedJournalException()
    {
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-SINGLE-001",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 1),
            new DateOnly(2026, 2, 1),
            "Single line voucher",
            _vnd,
            1.0m);

        voucher.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 1_000_000m, "Only debit");

        Assert.Throws<UnbalancedJournalException>(() => voucher.Post("test_user"));
    }

    [Fact]
    public void Post_OnlyDebits_ThrowsUnbalancedJournalException()
    {
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-NODEBIT-001",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 1),
            new DateOnly(2026, 2, 1),
            "Debits only",
            _vnd,
            1.0m);

        voucher.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 500_000m, "Debit 1");
        voucher.AddLine(new AccountId("1112"), LedgerEntryType.Debit, 500_000m, "Debit 2");

        Assert.Throws<UnbalancedJournalException>(() => voucher.Post("test_user"));
    }

    [Fact]
    public void Post_BalancedEntries_SucceedsAndSetsStatusPosted()
    {
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-OK-001",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 1),
            new DateOnly(2026, 2, 1),
            "Balanced voucher",
            _vnd,
            1.0m);

        voucher.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 5_000_000m, "Debit cash");
        voucher.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 5_000_000m, "Credit bank");

        voucher.Post("accountant_a");

        Assert.Equal(VoucherStatus.Posted, voucher.Status);
        Assert.Equal("accountant_a", voucher.PostedBy);
        Assert.NotNull(voucher.PostedAtUtc);
    }

    [Fact]
    public void Post_AlreadyPostedVoucher_ThrowsImmutablePostedVoucherException()
    {
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-DOUBLE-001",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 1),
            new DateOnly(2026, 2, 1),
            "Double post test",
            _vnd,
            1.0m);

        voucher.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 1_000_000m, "Debit");
        voucher.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 1_000_000m, "Credit");
        voucher.Post("accountant_a");

        Assert.Throws<ImmutablePostedVoucherException>(() => voucher.Post("accountant_b"));
    }

    [Fact]
    public void AddLine_OnPostedVoucher_ThrowsImmutablePostedVoucherException()
    {
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-MUTATE-001",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 1),
            new DateOnly(2026, 2, 1),
            "Mutation test",
            _vnd,
            1.0m);

        voucher.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 1_000_000m, "Debit");
        voucher.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 1_000_000m, "Credit");
        voucher.Post("accountant_a");

        Assert.Throws<ImmutablePostedVoucherException>(() =>
            voucher.AddLine(new AccountId("131"), LedgerEntryType.Debit, 500_000m, "Illegal mutation"));
    }

    [Fact]
    public void CreateReversal_InvertsDebitAndCreditAndMarksOriginalReversed()
    {
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-ORIG-001",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 1),
            new DateOnly(2026, 2, 1),
            "Original transaction",
            _vnd,
            1.0m);

        var partnerId = PartnerId.New();
        voucher.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 2_000_000m, "Debit cash");
        voucher.AddLine(new AccountId("131"), LedgerEntryType.Credit, 2_000_000m, "Credit customer", partnerId: partnerId);
        voucher.Post("accountant_a");

        var reversal = voucher.CreateReversal(
            "REV-PKT-001",
            "accountant_b",
            new DateOnly(2026, 2, 15),
            "Error correction");

        Assert.Equal(VoucherStatus.Reversed, voucher.Status);
        Assert.Equal(VoucherStatus.Draft, reversal.Status);
        Assert.Equal(voucher.Id, reversal.ReversalOfVoucherId);
        Assert.Equal(2, reversal.Lines.Count);

        // Counter entries: Line 1 was Debit 1111 -> now Credit 1111
        var revLine1 = reversal.Lines.First(l => l.AccountId == new AccountId("1111"));
        Assert.Equal(LedgerEntryType.Credit, revLine1.EntryType);
        Assert.Equal(2_000_000m, revLine1.AmountBase);

        // Line 2 was Credit 131 -> now Debit 131
        var revLine2 = reversal.Lines.First(l => l.AccountId == new AccountId("131"));
        Assert.Equal(LedgerEntryType.Debit, revLine2.EntryType);
        Assert.Equal(2_000_000m, revLine2.AmountBase);
        Assert.Equal(partnerId, revLine2.PartnerId);
    }
}

public class LedgerPostingAndComplianceIntegrationTests : IDisposable
{
    private readonly string _testDbPath;
    private readonly AccountingDbContext _context;
    private readonly ISqlConnectionFactory _connectionFactory;

    public LedgerPostingAndComplianceIntegrationTests()
    {
        _testDbPath = $"ledger_test_{Guid.NewGuid():N}.db";

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

        SeedMasterData();
    }

    private void SeedMasterData()
    {
        // 1. Chart of Accounts
        // Parent non-postable account
        var acc111 = new Account(new AccountId("111"), "Tiền mặt", AccountType.Asset, BalanceNature.DebitBalance, isParent: true);
        // Active postable leaf accounts
        var acc1111 = new Account(new AccountId("1111"), "Tiền Việt Nam", AccountType.Asset, BalanceNature.DebitBalance, parentAccountId: new AccountId("111"));
        var acc1121 = new Account(new AccountId("1121"), "Tiền gửi ngân hàng VNĐ", AccountType.Asset, BalanceNature.DebitBalance);
        // Dimension-constrained accounts
        var acc131 = new Account(new AccountId("131"), "Phải thu của khách hàng", AccountType.Asset, BalanceNature.DebitBalance, requiresPartner: true);
        var acc331 = new Account(new AccountId("331"), "Phải trả cho người bán", AccountType.Liability, BalanceNature.CreditBalance, requiresPartner: true);
        var acc1561 = new Account(new AccountId("1561"), "Giá mua hàng hóa", AccountType.Asset, BalanceNature.DebitBalance, requiresWarehouse: true);
        var acc6421 = new Account(new AccountId("6421"), "Chi phí bán hàng", AccountType.Expense, BalanceNature.DebitBalance, requiresCostCenter: true);
        var acc5111 = new Account(new AccountId("5111"), "Doanh thu bán hàng hóa", AccountType.Revenue, BalanceNature.CreditBalance);

        // Inactive / Expired accounts for compliance guardrail testing
        var accInactive = new Account(new AccountId("811_INACTIVE"), "Chi phí khác (Ngừng dùng)", AccountType.Expense, BalanceNature.DebitBalance);
        accInactive.SetActive(false);

        var accExpired = new Account(
            new AccountId("811_EXPIRED"),
            "Chi phí khác (Hết hạn)",
            AccountType.Expense,
            BalanceNature.DebitBalance,
            effectiveFrom: new DateOnly(2024, 1, 1),
            effectiveTo: new DateOnly(2025, 12, 31));

        _context.MasterAccounts.AddRange(acc111, acc1111, acc1121, acc131, acc331, acc1561, acc6421, acc5111, accInactive, accExpired);

        // 2. Fiscal Periods
        var periodJan = new FiscalPeriod(2026, 1);
        periodJan.HardLock(Guid.NewGuid()); // Jan 2026 is hard-locked

        var periodFeb = new FiscalPeriod(2026, 2); // Feb 2026 is open

        _context.FiscalPeriods.AddRange(periodJan, periodFeb);
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
    public async Task PostVoucher_ParentAccount_ThrowsStatutoryComplianceException()
    {
        var handler = new PostVoucherCommandHandler(_context);
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-PARENT-01",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 10),
            new DateOnly(2026, 2, 10),
            "Post on parent account 111",
            new CurrencyCode("VND"),
            1.0m);

        // Account 111 is IsParent = true (Synthetic / non-postable)
        voucher.AddLine(new AccountId("111"), LedgerEntryType.Debit, 1_000_000m, "Debit parent");
        voucher.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 1_000_000m, "Credit bank");

        _context.GlVouchers.Add(voucher);
        await _context.SaveChangesAsync();

        var ex = await Assert.ThrowsAsync<StatutoryComplianceException>(() =>
            handler.Handle(new PostVoucherCommand(voucher.Id, "auditor"), CancellationToken.None));

        Assert.Contains("parent account cannot accept direct postings", ex.Message);
    }

    [Fact]
    public async Task PostVoucher_InactiveAccount_ThrowsStatutoryComplianceException()
    {
        var handler = new PostVoucherCommandHandler(_context);
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-INACTIVE-01",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 10),
            new DateOnly(2026, 2, 10),
            "Post on inactive account",
            new CurrencyCode("VND"),
            1.0m);

        voucher.AddLine(new AccountId("811_INACTIVE"), LedgerEntryType.Debit, 500_000m, "Debit inactive");
        voucher.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 500_000m, "Credit bank");

        _context.GlVouchers.Add(voucher);
        await _context.SaveChangesAsync();

        var ex = await Assert.ThrowsAsync<StatutoryComplianceException>(() =>
            handler.Handle(new PostVoucherCommand(voucher.Id, "auditor"), CancellationToken.None));

        Assert.Contains("account is inactive", ex.Message);
    }

    [Fact]
    public async Task PostVoucher_ExpiredAccount_ThrowsStatutoryComplianceException()
    {
        var handler = new PostVoucherCommandHandler(_context);
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-EXPIRED-01",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 10),
            new DateOnly(2026, 2, 10),
            "Post on expired account",
            new CurrencyCode("VND"),
            1.0m);

        voucher.AddLine(new AccountId("811_EXPIRED"), LedgerEntryType.Debit, 200_000m, "Debit expired");
        voucher.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 200_000m, "Credit bank");

        _context.GlVouchers.Add(voucher);
        await _context.SaveChangesAsync();

        var ex = await Assert.ThrowsAsync<StatutoryComplianceException>(() =>
            handler.Handle(new PostVoucherCommand(voucher.Id, "auditor"), CancellationToken.None));

        Assert.Contains("outside statutory validity", ex.Message);
    }

    [Fact]
    public async Task PostVoucher_MissingRequiredPartner_ThrowsStatutoryComplianceException()
    {
        var handler = new PostVoucherCommandHandler(_context);
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-NOPARTNER-01",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 10),
            new DateOnly(2026, 2, 10),
            "Missing partner on 131",
            new CurrencyCode("VND"),
            1.0m);

        // Account 131 requires PartnerId, but we pass null
        voucher.AddLine(new AccountId("131"), LedgerEntryType.Debit, 3_000_000m, "Receivable without partner", partnerId: null);
        voucher.AddLine(new AccountId("5111"), LedgerEntryType.Credit, 3_000_000m, "Revenue");

        _context.GlVouchers.Add(voucher);
        await _context.SaveChangesAsync();

        var ex = await Assert.ThrowsAsync<StatutoryComplianceException>(() =>
            handler.Handle(new PostVoucherCommand(voucher.Id, "auditor"), CancellationToken.None));

        Assert.Contains("requires a valid Business Partner dimension", ex.Message);
    }

    [Fact]
    public async Task PostVoucher_MissingRequiredWarehouse_ThrowsStatutoryComplianceException()
    {
        var handler = new PostVoucherCommandHandler(_context);
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-NOWH-01",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 10),
            new DateOnly(2026, 2, 10),
            "Missing warehouse on 1561",
            new CurrencyCode("VND"),
            1.0m);

        // Account 1561 requires WarehouseId
        voucher.AddLine(new AccountId("1561"), LedgerEntryType.Debit, 10_000_000m, "Inventory without WH", warehouseId: null);
        voucher.AddLine(new AccountId("331"), LedgerEntryType.Credit, 10_000_000m, "Payable", partnerId: PartnerId.New());

        _context.GlVouchers.Add(voucher);
        await _context.SaveChangesAsync();

        var ex = await Assert.ThrowsAsync<StatutoryComplianceException>(() =>
            handler.Handle(new PostVoucherCommand(voucher.Id, "auditor"), CancellationToken.None));

        Assert.Contains("requires a valid Warehouse dimension", ex.Message);
    }

    [Fact]
    public async Task PostVoucher_MissingRequiredCostCenter_ThrowsStatutoryComplianceException()
    {
        var handler = new PostVoucherCommandHandler(_context);
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-NOCC-01",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 10),
            new DateOnly(2026, 2, 10),
            "Missing cost center on 6421",
            new CurrencyCode("VND"),
            1.0m);

        // Account 6421 requires CostCenterId
        voucher.AddLine(new AccountId("6421"), LedgerEntryType.Debit, 4_000_000m, "Selling exp without CC", costCenterId: null);
        voucher.AddLine(new AccountId("1111"), LedgerEntryType.Credit, 4_000_000m, "Cash");

        _context.GlVouchers.Add(voucher);
        await _context.SaveChangesAsync();

        var ex = await Assert.ThrowsAsync<StatutoryComplianceException>(() =>
            handler.Handle(new PostVoucherCommand(voucher.Id, "auditor"), CancellationToken.None));

        Assert.Contains("requires a valid Cost Center dimension", ex.Message);
    }

    [Fact]
    public async Task PostVoucher_HardLockedPeriod_ThrowsFiscalPeriodClosedException()
    {
        var handler = new PostVoucherCommandHandler(_context);
        // Posting in Jan 2026 which is hard-locked
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-LOCKED-01",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 1, 20),
            new DateOnly(2026, 1, 20),
            "Voucher in locked period",
            new CurrencyCode("VND"),
            1.0m);

        voucher.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 1_000_000m, "Cash");
        voucher.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 1_000_000m, "Bank");

        _context.GlVouchers.Add(voucher);
        await _context.SaveChangesAsync();

        await Assert.ThrowsAsync<FiscalPeriodClosedException>(() =>
            handler.Handle(new PostVoucherCommand(voucher.Id, "auditor"), CancellationToken.None));
    }

    [Fact]
    public async Task PostVoucher_ValidVoucher_AtomicallyPostsAndCreatesGeneralLedgerEntries()
    {
        var handler = new PostVoucherCommandHandler(_context);
        var voucher = new Voucher(
            VoucherId.New(),
            "PKT-VALID-01",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 5),
            new DateOnly(2026, 2, 5),
            "Cash deposit to bank",
            new CurrencyCode("VND"),
            1.0m);

        voucher.AddLine(new AccountId("1121"), LedgerEntryType.Debit, 25_000_000m, "Deposit bank");
        voucher.AddLine(new AccountId("1111"), LedgerEntryType.Credit, 25_000_000m, "Withdraw cash");

        _context.GlVouchers.Add(voucher);
        await _context.SaveChangesAsync();

        var result = await handler.Handle(new PostVoucherCommand(voucher.Id, "lead_accountant"), CancellationToken.None);
        Assert.True(result.IsSuccess);

        // Verify voucher status
        var loadedVoucher = await _context.GlVouchers.FindAsync(voucher.Id);
        Assert.NotNull(loadedVoucher);
        Assert.Equal(VoucherStatus.Posted, loadedVoucher.Status);
        Assert.Equal("lead_accountant", loadedVoucher.PostedBy);

        // Verify GL entries generated
        var glEntries = await _context.GeneralLedgerEntries
            .Where(e => e.VoucherId == voucher.Id)
            .ToListAsync();

        Assert.Equal(2, glEntries.Count);

        var debitEntry = glEntries.First(e => e.AccountId == new AccountId("1121"));
        Assert.Equal(25_000_000m, debitEntry.DebitAmount);
        Assert.Equal(0m, debitEntry.CreditAmount);
        Assert.Equal(new DateOnly(2026, 2, 5), debitEntry.PostingDate);

        var creditEntry = glEntries.First(e => e.AccountId == new AccountId("1111"));
        Assert.Equal(0m, creditEntry.DebitAmount);
        Assert.Equal(25_000_000m, creditEntry.CreditAmount);
        Assert.Equal(new DateOnly(2026, 2, 5), creditEntry.PostingDate);
    }

    [Fact]
    public async Task ReverseVoucher_GeneratesCounterEntriesAndZeroesNetBalance()
    {
        var postHandler = new PostVoucherCommandHandler(_context);
        var reverseHandler = new ReverseVoucherCommandHandler(_context);

        // 1. Post original voucher
        var originalVoucher = new Voucher(
            VoucherId.New(),
            "PKT-ORIG-REV",
            VoucherType.GeneralJournal,
            new DateOnly(2026, 2, 10),
            new DateOnly(2026, 2, 10),
            "Erroneous transaction",
            new CurrencyCode("VND"),
            1.0m);

        originalVoucher.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 8_000_000m, "Debit cash");
        originalVoucher.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 8_000_000m, "Credit bank");

        _context.GlVouchers.Add(originalVoucher);
        await _context.SaveChangesAsync();

        await postHandler.Handle(new PostVoucherCommand(originalVoucher.Id, "poster"), CancellationToken.None);

        // 2. Reverse voucher
        var revResult = await reverseHandler.Handle(new ReverseVoucherCommand(
            originalVoucher.Id,
            "REV-PKT-002",
            "reverser",
            new DateOnly(2026, 2, 12),
            "Reversal of mistaken voucher"), CancellationToken.None);

        Assert.True(revResult.IsSuccess);
        var reversalVoucherId = revResult.Value!;

        // Verify original voucher is marked Reversed
        var refreshedOrig = await _context.GlVouchers.FindAsync(originalVoucher.Id);
        Assert.NotNull(refreshedOrig);
        Assert.Equal(VoucherStatus.Reversed, refreshedOrig.Status);

        // Verify reversing voucher is Posted
        var reversalVoucher = await _context.GlVouchers.FindAsync(reversalVoucherId);
        Assert.NotNull(reversalVoucher);
        Assert.Equal(VoucherStatus.Posted, reversalVoucher.Status);
        Assert.Equal(originalVoucher.Id, reversalVoucher.ReversalOfVoucherId);

        // Verify net GL entries across both vouchers sum to zero
        var allEntries = await _context.GeneralLedgerEntries.ToListAsync();
        Assert.Equal(4, allEntries.Count); // 2 original + 2 counter

        var totalDebit1111 = allEntries.Where(e => e.AccountId == new AccountId("1111")).Sum(e => e.DebitAmount);
        var totalCredit1111 = allEntries.Where(e => e.AccountId == new AccountId("1111")).Sum(e => e.CreditAmount);
        Assert.Equal(8_000_000m, totalDebit1111);
        Assert.Equal(8_000_000m, totalCredit1111);
        Assert.Equal(0m, totalDebit1111 - totalCredit1111);

        var totalDebit1121 = allEntries.Where(e => e.AccountId == new AccountId("1121")).Sum(e => e.DebitAmount);
        var totalCredit1121 = allEntries.Where(e => e.AccountId == new AccountId("1121")).Sum(e => e.CreditAmount);
        Assert.Equal(8_000_000m, totalDebit1121);
        Assert.Equal(8_000_000m, totalCredit1121);
        Assert.Equal(0m, totalDebit1121 - totalCredit1121);
    }

    [Fact]
    public async Task Dapper_TrialBalance_SumDebitEqualsSumCredit()
    {
        var postHandler = new PostVoucherCommandHandler(_context);
        var partnerId = PartnerId.New();

        // Transaction 1: Revenue on credit (Debit 131: 15,000,000 / Credit 5111: 15,000,000)
        var v1 = new Voucher(VoucherId.New(), "V-TB-01", VoucherType.SalesInvoice, new DateOnly(2026, 2, 1), new DateOnly(2026, 2, 1), "Sale", new CurrencyCode("VND"), 1m);
        v1.AddLine(new AccountId("131"), LedgerEntryType.Debit, 15_000_000m, "Receivable", partnerId: partnerId);
        v1.AddLine(new AccountId("5111"), LedgerEntryType.Credit, 15_000_000m, "Revenue");
        _context.GlVouchers.Add(v1);

        // Transaction 2: Customer pays cash (Debit 1111: 10,000,000 / Credit 131: 10,000,000)
        var v2 = new Voucher(VoucherId.New(), "V-TB-02", VoucherType.CashReceipt, new DateOnly(2026, 2, 5), new DateOnly(2026, 2, 5), "Cash receipt", new CurrencyCode("VND"), 1m);
        v2.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 10_000_000m, "Cash in");
        v2.AddLine(new AccountId("131"), LedgerEntryType.Credit, 10_000_000m, "Customer paid", partnerId: partnerId);
        _context.GlVouchers.Add(v2);

        // Transaction 3: Deposit cash to bank (Debit 1121: 6,000,000 / Credit 1111: 6,000,000)
        var v3 = new Voucher(VoucherId.New(), "V-TB-03", VoucherType.BankPayment, new DateOnly(2026, 2, 8), new DateOnly(2026, 2, 8), "Bank deposit", new CurrencyCode("VND"), 1m);
        v3.AddLine(new AccountId("1121"), LedgerEntryType.Debit, 6_000_000m, "Bank");
        v3.AddLine(new AccountId("1111"), LedgerEntryType.Credit, 6_000_000m, "Cash");
        _context.GlVouchers.Add(v3);

        await _context.SaveChangesAsync();

        await postHandler.Handle(new PostVoucherCommand(v1.Id, "auditor"), CancellationToken.None);
        await postHandler.Handle(new PostVoucherCommand(v2.Id, "auditor"), CancellationToken.None);
        await postHandler.Handle(new PostVoucherCommand(v3.Id, "auditor"), CancellationToken.None);

        // Execute High-Speed Dapper Query
        var tbHandler = new GetTrialBalanceQueryHandler(_connectionFactory);
        var tbResult = await tbHandler.Handle(
            new GetTrialBalanceQuery(new DateOnly(2026, 2, 1), new DateOnly(2026, 2, 28)),
            CancellationToken.None);

        Assert.True(tbResult.IsSuccess);
        var rows = tbResult.Value!;
        Assert.NotEmpty(rows);

        // Karpathy Invariant: Trial balance total debits must strictly equal total credits
        var totalPeriodDebit = rows.Sum(r => r.PeriodDebit);
        var totalPeriodCredit = rows.Sum(r => r.PeriodCredit);
        Assert.Equal(31_000_000m, totalPeriodDebit);
        Assert.Equal(31_000_000m, totalPeriodCredit);
        Assert.Equal(totalPeriodDebit, totalPeriodCredit);

        var totalClosingDebit = rows.Sum(r => r.ClosingDebit);
        var totalClosingCredit = rows.Sum(r => r.ClosingCredit);
        Assert.Equal(totalClosingDebit, totalClosingCredit);

        // Specific Account Verifications
        // 1111 Cash: Debit 10M, Credit 6M -> Closing Debit 4M
        var row1111 = rows.First(r => r.AccountCode == "1111");
        Assert.Equal(10_000_000m, row1111.PeriodDebit);
        Assert.Equal(6_000_000m, row1111.PeriodCredit);
        Assert.Equal(4_000_000m, row1111.ClosingDebit);
        Assert.Equal(0m, row1111.ClosingCredit);

        // 1121 Bank: Debit 6M -> Closing Debit 6M
        var row1121 = rows.First(r => r.AccountCode == "1121");
        Assert.Equal(6_000_000m, row1121.PeriodDebit);
        Assert.Equal(6_000_000m, row1121.ClosingDebit);

        // 131 AR: Debit 15M, Credit 10M -> Closing Debit 5M
        var row131 = rows.First(r => r.AccountCode == "131");
        Assert.Equal(15_000_000m, row131.PeriodDebit);
        Assert.Equal(10_000_000m, row131.PeriodCredit);
        Assert.Equal(5_000_000m, row131.ClosingDebit);

        // 5111 Revenue: Credit 15M -> Closing Credit 15M
        var row5111 = rows.First(r => r.AccountCode == "5111");
        Assert.Equal(15_000_000m, row5111.PeriodCredit);
        Assert.Equal(15_000_000m, row5111.ClosingCredit);
    }

    [Fact]
    public async Task Dapper_AccountLedgerDetail_CalculatesRunningBalance()
    {
        var postHandler = new PostVoucherCommandHandler(_context);

        // Post 3 vouchers on account 1111 (Cash)
        var v1 = new Voucher(VoucherId.New(), "V-SO-01", VoucherType.CashReceipt, new DateOnly(2026, 2, 2), new DateOnly(2026, 2, 2), "Receipt 1", new CurrencyCode("VND"), 1m);
        v1.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 10_000_000m, "Receipt 1");
        v1.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 10_000_000m, "Bank");
        _context.GlVouchers.Add(v1);

        var v2 = new Voucher(VoucherId.New(), "V-SO-02", VoucherType.CashDisbursement, new DateOnly(2026, 2, 5), new DateOnly(2026, 2, 5), "Payment 1", new CurrencyCode("VND"), 1m);
        v2.AddLine(new AccountId("6421"), LedgerEntryType.Debit, 3_000_000m, "Exp", costCenterId: new CostCenterId("CC_MKT"));
        v2.AddLine(new AccountId("1111"), LedgerEntryType.Credit, 3_000_000m, "Payment 1");
        _context.GlVouchers.Add(v2);

        var v3 = new Voucher(VoucherId.New(), "V-SO-03", VoucherType.CashReceipt, new DateOnly(2026, 2, 10), new DateOnly(2026, 2, 10), "Receipt 2", new CurrencyCode("VND"), 1m);
        v3.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 5_000_000m, "Receipt 2");
        v3.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 5_000_000m, "Bank");
        _context.GlVouchers.Add(v3);

        await _context.SaveChangesAsync();

        await postHandler.Handle(new PostVoucherCommand(v1.Id, "auditor"), CancellationToken.None);
        await postHandler.Handle(new PostVoucherCommand(v2.Id, "auditor"), CancellationToken.None);
        await postHandler.Handle(new PostVoucherCommand(v3.Id, "auditor"), CancellationToken.None);

        // Execute High-Speed Dapper Query for Sổ Cái TK 1111
        var detailHandler = new GetAccountLedgerDetailQueryHandler(_connectionFactory);
        var detailResult = await detailHandler.Handle(
            new GetAccountLedgerDetailQuery("1111", new DateOnly(2026, 2, 1), new DateOnly(2026, 2, 28)),
            CancellationToken.None);

        Assert.True(detailResult.IsSuccess);
        var rows = detailResult.Value!;
        Assert.Equal(3, rows.Count);

        // Entry 1: Debit 10M -> Running Balance = 10M
        Assert.Equal("V-SO-01", rows[0].VoucherNumber);
        Assert.Equal(10_000_000m, rows[0].DebitAmount);
        Assert.Equal(0m, rows[0].CreditAmount);
        Assert.Equal(10_000_000m, rows[0].RunningBalance);

        // Entry 2: Credit 3M -> Running Balance = 10M - 3M = 7M
        Assert.Equal("V-SO-02", rows[1].VoucherNumber);
        Assert.Equal(0m, rows[1].DebitAmount);
        Assert.Equal(3_000_000m, rows[1].CreditAmount);
        Assert.Equal(7_000_000m, rows[1].RunningBalance);

        // Entry 3: Debit 5M -> Running Balance = 7M + 5M = 12M
        Assert.Equal("V-SO-03", rows[2].VoucherNumber);
        Assert.Equal(5_000_000m, rows[2].DebitAmount);
        Assert.Equal(0m, rows[2].CreditAmount);
        Assert.Equal(12_000_000m, rows[2].RunningBalance);
    }

    [Fact]
    public async Task MariaDb_LiveLedgerPostingAndDapperTrialBalance_ShouldSucceed()
    {
        var serverConnStr = "Server=localhost;Port=3306;User=dev;Password=123456;TreatTinyAsBoolean=true;CharSet=utf8mb4;";
        var connStr = "Server=localhost;Port=3306;Database=accounting_gl_test;User=dev;Password=123456;TreatTinyAsBoolean=true;CharSet=utf8mb4;";
        var serverVersion = new MariaDbServerVersion(new Version(12, 3, 0));

        // Test connectivity first & create isolated database
        try
        {
            using var pingConn = new MySqlConnector.MySqlConnection(serverConnStr);
            await pingConn.OpenAsync();
            using var cmd = pingConn.CreateCommand();
            cmd.CommandText = "CREATE DATABASE IF NOT EXISTS accounting_gl_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;";
            await cmd.ExecuteNonQueryAsync();
        }
        catch
        {
            // If MariaDB server is offline in environment, skip gracefully
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

        // Check if accounts exist, if not seed
        if (!await efContext.MasterAccounts.AnyAsync())
        {
            await Accounting.Infrastructure.Persistence.Seeding.DbInitializer.SeedAsync(efContext);
        }

        // Post a test voucher on live MariaDB
        var postHandler = new PostVoucherCommandHandler(efContext);
        var voucherNumber = $"PKT-MDB-{Guid.NewGuid():N}"[..15].ToUpperInvariant();
        var voucher = new Voucher(
            VoucherId.New(),
            voucherNumber,
            VoucherType.GeneralJournal,
            new DateOnly(2026, 3, 1),
            new DateOnly(2026, 3, 1),
            "Live MariaDB GL posting",
            new CurrencyCode("VND"),
            1.0m);

        voucher.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 12_000_000m, "Debit cash MariaDB");
        voucher.AddLine(new AccountId("1121"), LedgerEntryType.Credit, 12_000_000m, "Credit bank MariaDB");

        efContext.GlVouchers.Add(voucher);
        await efContext.SaveChangesAsync();

        var postResult = await postHandler.Handle(new PostVoucherCommand(voucher.Id, "mariadb_auditor"), CancellationToken.None);
        Assert.True(postResult.IsSuccess);

        // Run Dapper Trial Balance Query on Live MariaDB
        var tbHandler = new GetTrialBalanceQueryHandler(dapperFactory);
        var tbResult = await tbHandler.Handle(
            new GetTrialBalanceQuery(new DateOnly(2026, 3, 1), new DateOnly(2026, 3, 31)),
            CancellationToken.None);

        Assert.True(tbResult.IsSuccess);
        var rows = tbResult.Value!;
        Assert.NotEmpty(rows);

        var totalDebit = rows.Sum(r => r.PeriodDebit);
        var totalCredit = rows.Sum(r => r.PeriodCredit);
        Assert.True(totalDebit >= 12_000_000m);
        Assert.Equal(totalDebit, totalCredit);

        // Run Dapper Sổ Cái Query on Live MariaDB
        var detailHandler = new GetAccountLedgerDetailQueryHandler(dapperFactory);
        var detailResult = await detailHandler.Handle(
            new GetAccountLedgerDetailQuery("1111", new DateOnly(2026, 3, 1), new DateOnly(2026, 3, 31)),
            CancellationToken.None);

        Assert.True(detailResult.IsSuccess);
        var detailRows = detailResult.Value!;
        Assert.Contains(detailRows, r => r.VoucherNumber == voucherNumber && r.DebitAmount == 12_000_000m);
    }
}
