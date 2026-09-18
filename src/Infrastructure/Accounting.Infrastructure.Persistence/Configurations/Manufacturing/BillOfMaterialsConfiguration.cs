using Accounting.Domain.Common;
using Accounting.Domain.Manufacturing;
using Accounting.Domain.MasterData.Common;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Manufacturing;

public class BillOfMaterialsConfiguration : IEntityTypeConfiguration<BillOfMaterials>
{
    public void Configure(EntityTypeBuilder<BillOfMaterials> builder)
    {
        builder.ToTable("mfg_boms");

        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id)
            .HasConversion(id => id.Value, value => new BomId(value))
            .IsRequired();

        builder.Property(x => x.FinishedGoodItemId)
            .HasConversion(id => id.Value, value => new InventoryItemId(value))
            .IsRequired();

        builder.Property(x => x.Version)
            .HasMaxLength(20)
            .IsRequired();

        builder.Property(x => x.Description)
            .HasMaxLength(500);

        builder.Property(x => x.IsActive)
            .IsRequired();

        builder.Property(x => x.CreatedAtUtc).IsRequired();
        builder.Property(x => x.CreatedBy).HasMaxLength(100);
        builder.Property(x => x.UpdatedAtUtc);
        builder.Property(x => x.UpdatedBy).HasMaxLength(100);

        builder.Property(x => x.IsDeleted).IsRequired();
        builder.Property(x => x.DeletedAtUtc);
        builder.Property(x => x.DeletedBy).HasMaxLength(100);

        builder.HasMany(x => x.Lines)
            .WithOne()
            .HasForeignKey(l => l.BomId)
            .OnDelete(DeleteBehavior.Cascade);

        builder.HasIndex(x => new { x.FinishedGoodItemId, x.Version }).IsUnique();
    }
}

public class BomLineConfiguration : IEntityTypeConfiguration<BomLine>
{
    public void Configure(EntityTypeBuilder<BomLine> builder)
    {
        builder.ToTable("mfg_bom_lines");

        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id)
            .HasConversion(id => id.Value, value => new BomLineId(value))
            .IsRequired();

        builder.Property(x => x.BomId)
            .HasConversion(id => id.Value, value => new BomId(value))
            .IsRequired();

        builder.Property(x => x.MaterialItemId)
            .HasConversion(id => id.Value, value => new InventoryItemId(value))
            .IsRequired();

        builder.Property(x => x.StandardQuantity)
            .HasPrecision(18, 4)
            .IsRequired();

        builder.Property(x => x.ScrapPercentage)
            .HasPrecision(5, 2)
            .IsRequired();
    }
}
