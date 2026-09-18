using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Inventory;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.MasterData;

public class UnitOfMeasureConfiguration : IEntityTypeConfiguration<UnitOfMeasure>
{
    public void Configure(EntityTypeBuilder<UnitOfMeasure> builder)
    {
        builder.ToTable("md_units_of_measure");

        builder.HasKey(u => u.Id);
        builder.Property(u => u.Id)
            .HasConversion(id => id.Value, value => new UomId(value))
            .IsRequired();

        builder.Property(u => u.UomCode)
            .HasMaxLength(20)
            .IsRequired();
        builder.HasIndex(u => u.UomCode).IsUnique();

        builder.Property(u => u.UomName)
            .HasMaxLength(100)
            .IsRequired();

        builder.Property(u => u.Description)
            .HasMaxLength(250);

        builder.Property(u => u.IsActive).IsRequired();

        builder.Property(u => u.CreatedAtUtc).IsRequired();
        builder.Property(u => u.CreatedBy).HasMaxLength(100);
        builder.Property(u => u.UpdatedAtUtc);
        builder.Property(u => u.UpdatedBy).HasMaxLength(100);

        builder.Property(u => u.IsDeleted).IsRequired();
        builder.Property(u => u.DeletedAtUtc);
        builder.Property(u => u.DeletedBy).HasMaxLength(100);
    }
}
