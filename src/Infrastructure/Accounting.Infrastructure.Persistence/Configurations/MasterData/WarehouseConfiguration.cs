using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Inventory;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.MasterData;

public class WarehouseConfiguration : IEntityTypeConfiguration<Warehouse>
{
    public void Configure(EntityTypeBuilder<Warehouse> builder)
    {
        builder.ToTable("md_warehouses");

        builder.HasKey(w => w.Id);
        builder.Property(w => w.Id)
            .HasConversion(id => id.Value, value => new WarehouseId(value))
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(w => w.WarehouseName)
            .HasMaxLength(200)
            .IsRequired();

        builder.Property(w => w.Address)
            .HasMaxLength(500);

        builder.Property(w => w.IsActive).IsRequired();

        builder.Property(w => w.CreatedAtUtc).IsRequired();
        builder.Property(w => w.CreatedBy).HasMaxLength(100);
        builder.Property(w => w.UpdatedAtUtc);
        builder.Property(w => w.UpdatedBy).HasMaxLength(100);

        builder.Property(w => w.IsDeleted).IsRequired();
        builder.Property(w => w.DeletedAtUtc);
        builder.Property(w => w.DeletedBy).HasMaxLength(100);
    }
}
