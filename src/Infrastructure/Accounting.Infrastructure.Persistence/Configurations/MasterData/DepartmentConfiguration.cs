using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Dimensions;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.MasterData;

public class DepartmentConfiguration : IEntityTypeConfiguration<Department>
{
    public void Configure(EntityTypeBuilder<Department> builder)
    {
        builder.ToTable("md_departments");

        builder.HasKey(d => d.Id);
        builder.Property(d => d.Id)
            .HasConversion(id => id.Value, value => new DepartmentId(value))
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(d => d.DepartmentName)
            .HasMaxLength(200)
            .IsRequired();

        builder.Property(d => d.ParentDepartmentId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : null,
                v => v != null ? new DepartmentId(v) : (DepartmentId?)null)
            .HasMaxLength(50);

        builder.HasOne(d => d.ParentDepartment)
            .WithMany()
            .HasForeignKey(d => d.ParentDepartmentId)
            .OnDelete(DeleteBehavior.Restrict);

        builder.Property(d => d.IsActive).IsRequired();

        builder.Property(d => d.CreatedAtUtc).IsRequired();
        builder.Property(d => d.CreatedBy).HasMaxLength(100);
        builder.Property(d => d.UpdatedAtUtc);
        builder.Property(d => d.UpdatedBy).HasMaxLength(100);

        builder.Property(d => d.IsDeleted).IsRequired();
        builder.Property(d => d.DeletedAtUtc);
        builder.Property(d => d.DeletedBy).HasMaxLength(100);
    }
}
