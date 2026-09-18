using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Dimensions;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.MasterData;

public class ExpenseItemConfiguration : IEntityTypeConfiguration<ExpenseItem>
{
    public void Configure(EntityTypeBuilder<ExpenseItem> builder)
    {
        builder.ToTable("md_expense_items");

        builder.HasKey(e => e.Id);
        builder.Property(e => e.Id)
            .HasConversion(id => id.Value, value => new ExpenseItemId(value))
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(e => e.Name)
            .HasMaxLength(200)
            .IsRequired();

        builder.Property(e => e.ParentId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : null,
                v => v != null ? new ExpenseItemId(v) : (ExpenseItemId?)null)
            .HasMaxLength(50);

        builder.HasOne(e => e.Parent)
            .WithMany()
            .HasForeignKey(e => e.ParentId)
            .OnDelete(DeleteBehavior.Restrict);

        builder.Property(e => e.IsActive).IsRequired();

        builder.Property(e => e.CreatedAtUtc).IsRequired();
        builder.Property(e => e.CreatedBy).HasMaxLength(100);
        builder.Property(e => e.UpdatedAtUtc);
        builder.Property(e => e.UpdatedBy).HasMaxLength(100);

        builder.Property(e => e.IsDeleted).IsRequired();
        builder.Property(e => e.DeletedAtUtc);
        builder.Property(e => e.DeletedBy).HasMaxLength(100);
    }
}
