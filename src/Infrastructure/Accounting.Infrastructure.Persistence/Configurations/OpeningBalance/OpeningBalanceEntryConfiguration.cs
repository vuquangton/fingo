using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.OpeningBalance;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.OpeningBalance;

public class OpeningBalanceEntryConfiguration : IEntityTypeConfiguration<OpeningBalanceEntry>
{
    public void Configure(EntityTypeBuilder<OpeningBalanceEntry> builder)
    {
        builder.ToTable("gl_opening_balances");

        builder.HasKey(e => e.Id);

        builder.Property(e => e.FiscalYear)
            .IsRequired();

        builder.Property(e => e.AccountId)
            .HasConversion(id => id.Value, value => new AccountId(value))
            .HasMaxLength(20)
            .IsRequired();

        builder.Property(e => e.DebitAmount)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(e => e.CreditAmount)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(e => e.PartnerId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : (Guid?)null,
                value => value.HasValue ? new PartnerId(value.Value) : (PartnerId?)null);

        builder.Property(e => e.WarehouseId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : null,
                value => value != null ? new WarehouseId(value) : (WarehouseId?)null)
            .HasMaxLength(50);

        builder.Property(e => e.InventoryItemId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : (Guid?)null,
                value => value.HasValue ? new InventoryItemId(value.Value) : (InventoryItemId?)null);

        builder.Property(e => e.Quantity)
            .HasPrecision(18, 4);

        builder.Property(e => e.UnitPrice)
            .HasPrecision(18, 4);

        builder.Property(e => e.BatchId)
            .HasMaxLength(100);

        builder.Property(e => e.Description)
            .HasMaxLength(500);

        builder.Property(e => e.IsCommitted)
            .IsRequired();

        builder.Property(e => e.CommittedAtUtc);

        builder.Property(e => e.CreatedAtUtc).IsRequired();
        builder.Property(e => e.CreatedBy).HasMaxLength(100);
        builder.Property(e => e.UpdatedAtUtc);
        builder.Property(e => e.UpdatedBy).HasMaxLength(100);

        builder.HasIndex(e => new { e.FiscalYear, e.AccountId });
        builder.HasIndex(e => e.BatchId);
        builder.HasIndex(e => new { e.FiscalYear, e.PartnerId });
        builder.HasIndex(e => new { e.FiscalYear, e.InventoryItemId });
    }
}
