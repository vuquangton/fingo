using Accounting.Domain.CapitalAssets;
using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.CapitalAssets;

public class FixedAssetConfiguration : IEntityTypeConfiguration<FixedAsset>
{
    public void Configure(EntityTypeBuilder<FixedAsset> builder)
    {
        builder.ToTable("cap_fixed_assets");

        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id)
            .HasConversion(id => id.Value, value => new FixedAssetId(value))
            .IsRequired();

        builder.Property(x => x.AssetCode)
            .HasMaxLength(50)
            .IsRequired();
        builder.HasIndex(x => x.AssetCode).IsUnique();

        builder.Property(x => x.AssetName)
            .HasMaxLength(250)
            .IsRequired();

        builder.Property(x => x.AssetType)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.OriginalCost)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.ResidualValue)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.AccumulatedDepreciation)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.UsefulLifeMonths)
            .IsRequired();

        builder.Property(x => x.RemainingMonths)
            .IsRequired();

        builder.Property(x => x.CapitalizationDate)
            .IsRequired();

        builder.Property(x => x.DepreciationStartDate)
            .IsRequired();

        builder.Property(x => x.AssetAccountId)
            .HasConversion(id => id.Value, value => new AccountId(value))
            .HasMaxLength(20)
            .IsRequired();

        builder.Property(x => x.DepreciationAccountId)
            .HasConversion(id => id.Value, value => new AccountId(value))
            .HasMaxLength(20)
            .IsRequired();

        builder.Property(x => x.ExpenseAccountId)
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
