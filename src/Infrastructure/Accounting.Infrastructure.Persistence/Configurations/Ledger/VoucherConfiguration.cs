using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Common;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Ledger;

public class VoucherConfiguration : IEntityTypeConfiguration<Voucher>
{
    public void Configure(EntityTypeBuilder<Voucher> builder)
    {
        builder.ToTable("gl_vouchers");

        builder.HasKey(v => v.Id);
        builder.Property(v => v.Id)
            .HasConversion(id => id.Value, value => new VoucherId(value))
            .IsRequired();

        builder.Property(v => v.VoucherNumber)
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(v => v.VoucherType)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(v => v.VoucherDate)
            .IsRequired();

        builder.Property(v => v.PostingDate)
            .IsRequired();

        builder.Property(v => v.Status)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(v => v.Description)
            .HasMaxLength(500)
            .IsRequired();

        builder.Property(v => v.CurrencyId)
            .HasConversion(c => c.Value, value => new CurrencyCode(value))
            .HasMaxLength(10)
            .IsRequired();

        builder.Property(v => v.ExchangeRate)
            .HasPrecision(18, 4)
            .IsRequired();

        builder.Property(v => v.TotalDebitBase)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(v => v.TotalCreditBase)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(v => v.TotalDebitOriginal)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(v => v.TotalCreditOriginal)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(v => v.PostedAtUtc);
        builder.Property(v => v.PostedBy).HasMaxLength(100);

        builder.Property(v => v.ReversalOfVoucherId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : (Guid?)null,
                value => value.HasValue ? new VoucherId(value.Value) : (VoucherId?)null);

        builder.Property(v => v.CreatedAtUtc).IsRequired();
        builder.Property(v => v.CreatedBy).HasMaxLength(100);
        builder.Property(v => v.UpdatedAtUtc);
        builder.Property(v => v.UpdatedBy).HasMaxLength(100);

        builder.HasMany(v => v.Lines)
            .WithOne()
            .HasForeignKey(l => l.VoucherId)
            .OnDelete(DeleteBehavior.Cascade);

        builder.HasIndex(v => v.VoucherNumber).IsUnique();
        builder.HasIndex(v => v.PostingDate);
        builder.HasIndex(v => v.Status);
    }
}
