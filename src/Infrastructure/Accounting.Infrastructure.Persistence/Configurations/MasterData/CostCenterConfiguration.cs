using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Dimensions;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.MasterData;

public class CostCenterConfiguration : IEntityTypeConfiguration<CostCenter>
{
    public void Configure(EntityTypeBuilder<CostCenter> builder)
    {
        builder.ToTable("md_cost_centers");

        builder.HasKey(c => c.Id);
        builder.Property(c => c.Id)
            .HasConversion(id => id.Value, value => new CostCenterId(value))
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(c => c.CostCenterName)
            .HasMaxLength(200)
            .IsRequired();

        builder.Property(c => c.ParentCostCenterId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : null,
                v => v != null ? new CostCenterId(v) : (CostCenterId?)null)
            .HasMaxLength(50);

        builder.HasOne(c => c.ParentCostCenter)
            .WithMany()
            .HasForeignKey(c => c.ParentCostCenterId)
            .OnDelete(DeleteBehavior.Restrict);

        builder.Property(c => c.IsActive).IsRequired();

        builder.Property(c => c.CreatedAtUtc).IsRequired();
        builder.Property(c => c.CreatedBy).HasMaxLength(100);
        builder.Property(c => c.UpdatedAtUtc);
        builder.Property(c => c.UpdatedBy).HasMaxLength(100);

        builder.Property(c => c.IsDeleted).IsRequired();
        builder.Property(c => c.DeletedAtUtc);
        builder.Property(c => c.DeletedBy).HasMaxLength(100);
    }
}
