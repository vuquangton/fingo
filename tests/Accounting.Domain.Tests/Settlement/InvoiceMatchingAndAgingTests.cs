using Accounting.Application.Features.Settlement;
using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Partners;
using Accounting.Domain.Payables;
using Accounting.Domain.Receivables;
using Accounting.Domain.Settlement;
using Accounting.Infrastructure.Persistence.Context;
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;
using Xunit;

namespace Accounting.Domain.Tests.Settlement;

public class InvoiceMatchingAndAgingTests : IDisposable
{
    private readonly SqliteConnection _connection;
    private readonly AccountingDbContext _context;

    public InvoiceMatchingAndAgingTests()
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
    public async Task AllocatePayment_SalesInvoice_PartialAndFullAllocation_ShouldUpdateStatus()
    {
        var handler = new SettlementHandlers(_context);
        var custId = PartnerId.New();

        var customer = new BusinessPartner(custId, "CUST-01", "Cong ty Alpha", PartnerType.Customer);
        _context.BusinessPartners.Add(customer);

        var acc131 = new Account(new AccountId("131"), "Phai thu", AccountType.Asset, BalanceNature.DebitBalance);
        var acc1111 = new Account(new AccountId("1111"), "Tien mat", AccountType.Asset, BalanceNature.DebitBalance);
        _context.MasterAccounts.AddRange(acc131, acc1111);

        var invId = SalesInvoiceId.New();
        var inv = new SalesInvoice(invId, "INV-001", new DateOnly(2026, 1, 1), new DateOnly(2026, 1, 31), custId);
        inv.AddLine(new AccountId("131"), 10, 1_000_000m, 10m, "Dich vu phan mem"); // 10tr + 1tr VAT = 11tr
        _context.SubSalesInvoices.Add(inv);

        var pmtVoucher = new Voucher(VoucherId.New(), "PT-001", VoucherType.CashReceipt, new DateOnly(2026, 1, 15), new DateOnly(2026, 1, 15), "Thu tien Alpha");
        pmtVoucher.AddLine(new AccountId("1111"), LedgerEntryType.Debit, 11_000_000m, "Thu tien mat");
        pmtVoucher.AddLine(new AccountId("131"), LedgerEntryType.Credit, 11_000_000m, "Thu tien");
        pmtVoucher.Post("admin");
        _context.GlVouchers.Add(pmtVoucher);

        await _context.SaveChangesAsync();

        // 1. Partial allocation: 5,000,000
        var alloc1 = await handler.Handle(new AllocatePaymentCommand(
            inv.Id.Value,
            AllocationInvoiceType.SalesInvoice,
            pmtVoucher.Id.Value,
            custId.Value,
            new DateOnly(2026, 1, 15),
            5_000_000m,
            "Dot 1",
            "admin"), CancellationToken.None);

        Assert.True(alloc1.IsSuccess);
        Assert.Equal(SalesInvoiceStatus.PartiallyPaid, inv.Status);
        Assert.Equal(5_000_000m, inv.ReceivedAmount);
        Assert.Equal(6_000_000m, inv.RemainingAmount);

        // 2. Full allocation remaining: 6,000,000
        var alloc2 = await handler.Handle(new AllocatePaymentCommand(
            inv.Id.Value,
            AllocationInvoiceType.SalesInvoice,
            pmtVoucher.Id.Value,
            custId.Value,
            new DateOnly(2026, 1, 20),
            6_000_000m,
            "Dot 2 thanh toan het",
            "admin"), CancellationToken.None);

        Assert.True(alloc2.IsSuccess);
        Assert.Equal(SalesInvoiceStatus.FullyPaid, inv.Status);
        Assert.Equal(11_000_000m, inv.ReceivedAmount);
        Assert.Equal(0m, inv.RemainingAmount);

        // 3. Reverse allocation 2
        var rev = await handler.Handle(new ReverseAllocationCommand(alloc2.Value, "admin", "Huy phan bo dot 2 do nham"), CancellationToken.None);
        Assert.True(rev.IsSuccess);
        Assert.Equal(SalesInvoiceStatus.PartiallyPaid, inv.Status);
        Assert.Equal(5_000_000m, inv.ReceivedAmount);
        Assert.Equal(6_000_000m, inv.RemainingAmount);
    }

    [Fact]
    public async Task AgingReport_ShouldCorrectlyDistributeInvoicesIntoBuckets()
    {
        var handler = new SettlementHandlers(_context);
        var custId = PartnerId.New();
        var customer = new BusinessPartner(custId, "CUST-02", "Cong ty Beta", PartnerType.Customer);
        _context.BusinessPartners.Add(customer);

        var acc131 = new Account(new AccountId("131"), "Phai thu", AccountType.Asset, BalanceNature.DebitBalance);
        _context.MasterAccounts.Add(acc131);

        var asOfDate = new DateOnly(2026, 3, 31);

        // Inv 1: Not due yet (Due: 2026-04-15) -> 10m
        var inv1 = new SalesInvoice(SalesInvoiceId.New(), "INV-NOT-DUE", new DateOnly(2026, 3, 1), new DateOnly(2026, 4, 15), custId);
        inv1.AddLine(new AccountId("131"), 10, 1_000_000m, 0m, "Hang hoa");
        _context.SubSalesInvoices.Add(inv1);

        // Inv 2: Overdue 20 days (Due: 2026-03-11) -> 20m (Bucket 1-30)
        var inv2 = new SalesInvoice(SalesInvoiceId.New(), "INV-OD-20", new DateOnly(2026, 2, 1), new DateOnly(2026, 3, 11), custId);
        inv2.AddLine(new AccountId("131"), 20, 1_000_000m, 0m, "Hang hoa");
        _context.SubSalesInvoices.Add(inv2);

        // Inv 3: Overdue 45 days (Due: 2026-02-14) -> 30m (Bucket 31-60)
        var inv3 = new SalesInvoice(SalesInvoiceId.New(), "INV-OD-45", new DateOnly(2026, 1, 1), new DateOnly(2026, 2, 14), custId);
        inv3.AddLine(new AccountId("131"), 30, 1_000_000m, 0m, "Hang hoa");
        _context.SubSalesInvoices.Add(inv3);

        // Inv 4: Overdue 100 days (Due: 2025-12-21) -> 40m (Bucket >90)
        var inv4 = new SalesInvoice(SalesInvoiceId.New(), "INV-OD-100", new DateOnly(2025, 11, 1), new DateOnly(2025, 12, 21), custId);
        inv4.AddLine(new AccountId("131"), 40, 1_000_000m, 0m, "Hang hoa");
        _context.SubSalesInvoices.Add(inv4);

        await _context.SaveChangesAsync();

        var report = await handler.Handle(new GetArAgingReportQuery(asOfDate), CancellationToken.None);

        Assert.True(report.IsSuccess);
        Assert.NotNull(report.Value);
        Assert.Equal(100_000_000m, report.Value.GrandTotalOutstanding);

        var betaSummary = report.Value.PartnerSummaries.First(p => p.PartnerId == custId.Value);
        Assert.Equal(10_000_000m, betaSummary.CurrentNotDue);
        Assert.Equal(20_000_000m, betaSummary.Overdue1To30);
        Assert.Equal(30_000_000m, betaSummary.Overdue31To60);
        Assert.Equal(0m, betaSummary.Overdue61To90);
        Assert.Equal(40_000_000m, betaSummary.OverdueAbove90);
    }
}
