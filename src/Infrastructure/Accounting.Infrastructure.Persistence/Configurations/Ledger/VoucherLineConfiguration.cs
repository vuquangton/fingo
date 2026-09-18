using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Ledger;

public class VoucherLineConfiguration : IEntityTypeConfiguration<VoucherLine>
{
    public void Configure(EntityTypeBuilder<VoucherLine> builder)
    {
        builder.ToTable("gl_voucher_lines");

        builder.HasKey(l => l.Id);
        builder.Property(l => l.Id)
            .HasConversion(id => id.Value, value => new VoucherLineId(value))
            .IsRequired();

        builder.Property(l => l.VoucherId)
            .HasConversion(id => id.Value, value => new VoucherId(value))
            .IsRequired();

        builder.Property(l => l.LineNumber).IsRequired();

        builder.Property(l => l.AccountId)
            .HasConversion(id => id.Value, value => new AccountId(value))
            .HasMaxLength(20)
            .IsRequired();

        builder.Property(l => l.EntryType)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(l => l.AmountOriginal)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(l => l.AmountBase)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(l => l.Description)
            .HasMaxLength(500)
            .IsRequired();

        builder.Property(l => l.PartnerId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : (Guid?)null,
                value => value.HasValue ? new PartnerId(value.Value) : (PartnerId?)null);

        builder.Property(l => l.WarehouseId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : null,
                value => value != null ? new WarehouseId(value) : (WarehouseId?)null)
            .HasMaxLength(50);

        builder.Property(l => l.CostCenterId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : null,
                value => value != null ? new CostCenterId(value) : (CostCenterId?)null)
            .HasMaxLength(50);

        builder.Property(l => l.ProjectId)
            .HasMaxLength(50);

        builder.HasOne(l => l.Account)
            .WithMany()
            .HasForeignKey(l => l.AccountId)
            .OnDelete(DeleteBehavior.Restrict);

        builder.HasIndex(l => l.VoucherId);
        builder.HasIndex(l => l.AccountId);
        builder.HasIndex(l => l.PartnerId);
    }
}
