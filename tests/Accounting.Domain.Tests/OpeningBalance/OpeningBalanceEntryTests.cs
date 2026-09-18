using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.OpeningBalance;
using Xunit;

namespace Accounting.Domain.Tests.OpeningBalance;

public class OpeningBalanceEntryTests
{
    [Fact]
    public void Constructor_ValidInputs_ShouldInitializeProperly()
    {
        var id = Guid.NewGuid();
        var accountId = new AccountId("1111");
        var partnerId = PartnerId.New();
        var entry = new OpeningBalanceEntry(
            id,
            2026,
            accountId,
            1000000m,
            0m,
            partnerId,
            description: "Tiền mặt tồn quỹ");

        Assert.Equal(id, entry.Id);
        Assert.Equal(2026, entry.FiscalYear);
        Assert.Equal(accountId, entry.AccountId);
        Assert.Equal(1000000m, entry.DebitAmount);
        Assert.Equal(0m, entry.CreditAmount);
        Assert.Equal(partnerId, entry.PartnerId);
        Assert.False(entry.IsCommitted);
    }

    [Fact]
    public void Constructor_NegativeDebitOrCredit_ShouldThrowArgumentOutOfRangeException()
    {
        Assert.Throws<ArgumentOutOfRangeException>(() =>
            new OpeningBalanceEntry(
                Guid.NewGuid(),
                2026,
                new AccountId("1111"),
                -100m,
                0m));

        Assert.Throws<ArgumentOutOfRangeException>(() =>
            new OpeningBalanceEntry(
                Guid.NewGuid(),
                2026,
                new AccountId("1111"),
                0m,
                -500m));
    }

    [Fact]
    public void Constructor_ZeroDebitAndCreditAndQuantity_ShouldThrowArgumentException()
    {
        Assert.Throws<ArgumentException>(() =>
            new OpeningBalanceEntry(
                Guid.NewGuid(),
                2026,
                new AccountId("1111"),
                0m,
                0m,
                quantity: 0m));
    }

    [Fact]
    public void Constructor_QuantityPositiveWithoutInventoryItemId_ShouldThrowArgumentException()
    {
        Assert.Throws<ArgumentException>(() =>
            new OpeningBalanceEntry(
                Guid.NewGuid(),
                2026,
                new AccountId("1561"),
                100000m,
                0m,
                quantity: 10m,
                inventoryItemId: null));
    }

    [Fact]
    public void MarkCommitted_ShouldSetFlagAndTimestamp()
    {
        var entry = new OpeningBalanceEntry(
            Guid.NewGuid(),
            2026,
            new AccountId("1111"),
            5000000m,
            0m);

        entry.MarkCommitted("admin");

        Assert.True(entry.IsCommitted);
        Assert.NotNull(entry.CommittedAtUtc);
        Assert.Equal("admin", entry.UpdatedBy);
    }
}
