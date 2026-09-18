using Accounting.Domain.Common;
using Accounting.Domain.Costing;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Common;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Costing;

public class InventoryLayerConfiguration : IEntityTypeConfiguration<InventoryLayer>
{
    public void Configure(EntityTypeBuilder<InventoryLayer> builder)
    {
        builder.ToTable("cost_inventory_layers");

        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id)
            .HasConversion(id => id.Value, value => new InventoryLayerId(value))
            .IsRequired();

        builder.Property(x => x.WarehouseId)
            .HasConversion(id => id.Value, value => new WarehouseId(value))
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(x => x.InventoryItemId)
            .HasConversion(id => id.Value, value => new InventoryItemId(value))
            .IsRequired();

        builder.Property(x => x.ReceiptDate)
            .IsRequired();

        builder.Property(x => x.OriginalQuantity)
            .HasPrecision(18, 4)
            .IsRequired();

        builder.Property(x => x.RemainingQuantity)
            .HasPrecision(18, 4)
            .IsRequired();

        builder.Property(x => x.UnitCost)
            .HasPrecision(18, 4)
            .IsRequired();

        builder.Property(x => x.SourceVoucherId)
            .HasConversion(id => id.HasValue ? id.Value.Value : (Guid?)null, value => value.HasValue ? new VoucherId(value.Value) : null);

        builder.Property(x => x.CreatedAtUtc).IsRequired();
        builder.Property(x => x.CreatedBy).HasMaxLength(100);
        builder.Property(x => x.UpdatedAtUtc);
        builder.Property(x => x.UpdatedBy).HasMaxLength(100);

        builder.HasIndex(x => new { x.WarehouseId, x.InventoryItemId, x.ReceiptDate });
    }
}
