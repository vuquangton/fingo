using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Inventory;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.MasterData;

public class InventoryItemConfiguration : IEntityTypeConfiguration<InventoryItem>
{
    public void Configure(EntityTypeBuilder<InventoryItem> builder)
    {
        builder.ToTable("md_inventory_items");

        builder.HasKey(i => i.Id);
        builder.Property(i => i.Id)
            .HasConversion(id => id.Value, value => new InventoryItemId(value))
            .IsRequired();

        builder.Property(i => i.ItemCode)
            .HasMaxLength(50)
            .IsRequired();
        builder.HasIndex(i => i.ItemCode).IsUnique();

        builder.Property(i => i.ItemName)
            .HasMaxLength(200)
            .IsRequired();

        builder.Property(i => i.ItemType)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(i => i.BaseUomId)
            .HasConversion(id => id.Value, value => new UomId(value))
            .IsRequired();

        builder.HasOne(i => i.BaseUom)
            .WithMany()
            .HasForeignKey(i => i.BaseUomId)
            .OnDelete(DeleteBehavior.Restrict);

        builder.Property(i => i.DefaultCostingMethod)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(i => i.DefaultInventoryAccountId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : null,
                v => v != null ? new AccountId(v) : (AccountId?)null)
            .HasMaxLength(20);

        builder.Property(i => i.DefaultCogsAccountId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : null,
                v => v != null ? new AccountId(v) : (AccountId?)null)
            .HasMaxLength(20);

        builder.Property(i => i.DefaultRevenueAccountId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : null,
                v => v != null ? new AccountId(v) : (AccountId?)null)
            .HasMaxLength(20);

        builder.Property(i => i.TaxRate)
            .HasPrecision(6, 4)
            .IsRequired();

        builder.Property(i => i.IsActive).IsRequired();

        builder.Property(i => i.CreatedAtUtc).IsRequired();
        builder.Property(i => i.CreatedBy).HasMaxLength(100);
        builder.Property(i => i.UpdatedAtUtc);
        builder.Property(i => i.UpdatedBy).HasMaxLength(100);

        builder.Property(i => i.IsDeleted).IsRequired();
        builder.Property(i => i.DeletedAtUtc);
        builder.Property(i => i.DeletedBy).HasMaxLength(100);
    }
}
