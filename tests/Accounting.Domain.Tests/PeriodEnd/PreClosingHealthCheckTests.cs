using Accounting.Application.Features.PeriodEnd;
using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Infrastructure.Persistence.Context;
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;
using Xunit;

namespace Accounting.Domain.Tests.PeriodEnd;

public class PreClosingHealthCheckTests : IDisposable
{
    private readonly SqliteConnection _connection;
    private readonly AccountingDbContext _context;

    public PreClosingHealthCheckTests()
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
    public async Task RunPreClosingCheck_ShouldDetectUnpostedVouchersAndNegativeCash()
    {
        var handler = new PreClosingCheckQueryHandlers(_context);

        var acc1111 = new Account(new AccountId("1111"), "Tien mat", AccountType.Asset, BalanceNature.DebitBalance);
        var acc331 = new Account(new AccountId("331"), "Phai tra", AccountType.Liability, BalanceNature.CreditBalance);
        _context.MasterAccounts.AddRange(acc1111, acc331);

        // 1. Unposted voucher in Jan 2026
        var draftVoucher = new Voucher(VoucherId.New(), "DRAFT-01", VoucherType.CashDisbursement, new DateOnly(2026, 1, 15), new DateOnly(2026, 1, 15), "Tam ung chua ghi so");
        _context.GlVouchers.Add(draftVoucher);

        // 2. Negative Cash in GL (Credit 1111: 50,000,000 without prior Debit)
        var glNegativeCash = new GeneralLedgerEntry(
            Guid.NewGuid(), VoucherId.New(), new DateOnly(2026, 1, 20), FiscalPeriodId.FromYearMonth(2026, 1),
            new AccountId("1111"), 0m, 50_000_000m);
        // Balance with Debit 331 so trial balance is balanced
        var glDebit331 = new GeneralLedgerEntry(
            Guid.NewGuid(), VoucherId.New(), new DateOnly(2026, 1, 20), FiscalPeriodId.FromYearMonth(2026, 1),
            new AccountId("331"), 50_000_000m, 0m);

        _context.GeneralLedgerEntries.AddRange(glNegativeCash, glDebit331);
        await _context.SaveChangesAsync();

        var report = await handler.Handle(new RunPreClosingCheckQuery(2026, 1), CancellationToken.None);

        Assert.True(report.IsSuccess);
        Assert.NotNull(report.Value);
        Assert.False(report.Value.CanProceedToClose);
        Assert.True(report.Value.ErrorCount >= 2);

        var unpostedCheck = report.Value.Checks.First(c => c.RuleCode == "CHK_UNPOSTED_VOUCHERS");
        Assert.False(unpostedCheck.Passed);
        Assert.Equal(1, unpostedCheck.OffendingRecordsCount);
        Assert.NotNull(unpostedCheck.OffendingIdentifiers);
        Assert.Contains("DRAFT-01", unpostedCheck.OffendingIdentifiers);

        var negCashCheck = report.Value.Checks.First(c => c.RuleCode == "CHK_NEGATIVE_CASH");
        Assert.False(negCashCheck.Passed);
        Assert.Equal(1, negCashCheck.OffendingRecordsCount);

        var trialBalanceCheck = report.Value.Checks.First(c => c.RuleCode == "CHK_GL_TRIAL_BALANCE");
        Assert.True(trialBalanceCheck.Passed);
    }
}
