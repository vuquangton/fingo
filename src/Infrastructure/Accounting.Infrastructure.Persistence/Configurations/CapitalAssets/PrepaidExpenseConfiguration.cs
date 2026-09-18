using Accounting.Domain.CapitalAssets;
using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.CapitalAssets;

public class PrepaidExpenseConfiguration : IEntityTypeConfiguration<PrepaidExpense>
{
    public void Configure(EntityTypeBuilder<PrepaidExpense> builder)
    {
        builder.ToTable("cap_prepaid_expenses");

        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id)
            .HasConversion(id => id.Value, value => new PrepaidExpenseId(value))
            .IsRequired();

        builder.Property(x => x.ExpenseCode)
            .HasMaxLength(50)
            .IsRequired();
        builder.HasIndex(x => x.ExpenseCode).IsUnique();

        builder.Property(x => x.Name)
            .HasMaxLength(250)
            .IsRequired();

        builder.Property(x => x.TotalAmount)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.AllocatedAmount)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.TotalPeriods)
            .IsRequired();

        builder.Property(x => x.RemainingPeriods)
            .IsRequired();

        builder.Property(x => x.StartDate)
            .IsRequired();

        builder.Property(x => x.SourceAccountId)
            .HasConversion(id => id.Value, value => new AccountId(value))
            .HasMaxLength(20)
            .IsRequired();

        builder.Property(x => x.TargetExpenseAccountId)
            .HasConversion(id => id.Value, value => new AccountId(value))
            .HasMaxLength(20)
            .IsRequired();

        builder.Property(x => x.CostCenterId)
            .HasConversion(id => id.HasValue ? id.Value.Value : null, value => !string.IsNullOrEmpty(value) ? new CostCenterId(value) : null)
            .HasMaxLength(50);

        builder.Property(x => x.Status)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.CreatedAtUtc).IsRequired();
        builder.Property(x => x.CreatedBy).HasMaxLength(100);
        builder.Property(x => x.UpdatedAtUtc);
        builder.Property(x => x.UpdatedBy).HasMaxLength(100);

        builder.Property(x => x.IsDeleted).IsRequired();
        builder.Property(x => x.DeletedAtUtc);
        builder.Property(x => x.DeletedBy).HasMaxLength(100);

        builder.HasIndex(x => x.Status);
    }
}
