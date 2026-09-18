using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.MasterData;

public class AccountConfiguration : IEntityTypeConfiguration<Account>
{
    public void Configure(EntityTypeBuilder<Account> builder)
    {
        builder.ToTable("md_accounts");

        builder.HasKey(a => a.Id);
        builder.Property(a => a.Id)
            .HasConversion(id => id.Value, value => new AccountId(value))
            .HasMaxLength(20)
            .IsRequired();

        builder.Property(a => a.AccountName)
            .HasMaxLength(200)
            .IsRequired();

        builder.Property(a => a.AccountType)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(a => a.BalanceNature)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(a => a.ParentAccountId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : null,
                value => value != null ? new AccountId(value) : (AccountId?)null)
            .HasMaxLength(20);

        builder.HasOne(a => a.ParentAccount)
            .WithMany(a => a.SubAccounts)
            .HasForeignKey(a => a.ParentAccountId)
            .OnDelete(DeleteBehavior.Restrict);

        builder.Property(a => a.AccountLevel).IsRequired();
        builder.Property(a => a.IsParent).IsRequired();
        builder.Property(a => a.IsActive).IsRequired();

        builder.Property(a => a.GoverningCircular)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(a => a.EffectiveFrom)
            .IsRequired();

        builder.Property(a => a.EffectiveTo);

        builder.Property(a => a.RequiresPartner).IsRequired();
        builder.Property(a => a.RequiresWarehouse).IsRequired();
        builder.Property(a => a.RequiresCostCenter).IsRequired();
        builder.Property(a => a.RequiresProject).IsRequired();

        builder.Ignore(a => a.CanPost);

        builder.Property(a => a.CreatedAtUtc).IsRequired();
        builder.Property(a => a.CreatedBy).HasMaxLength(100);
        builder.Property(a => a.UpdatedAtUtc);
        builder.Property(a => a.UpdatedBy).HasMaxLength(100);

        builder.HasIndex(a => a.AccountType);
        builder.HasIndex(a => a.AccountLevel);
    }
}
