using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.Treasury;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;

namespace Accounting.Infrastructure.Persistence.Configurations.Treasury;

public class CashTransactionConfiguration : IEntityTypeConfiguration<CashTransaction>
{
    public void Configure(EntityTypeBuilder<CashTransaction> builder)
    {
        builder.ToTable("sub_cash_transactions");

        builder.HasKey(t => t.Id);
        builder.Property(t => t.Id)
            .HasConversion(id => id.Value, value => new CashVoucherId(value))
            .IsRequired();

        builder.Property(t => t.VoucherNumber)
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(t => t.TransactionType)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(t => t.TransactionDate)
            .IsRequired();

        builder.Property(t => t.BankAccountId)
            .HasMaxLength(50);

        builder.Property(t => t.PartnerId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : (Guid?)null,
                value => value.HasValue ? new PartnerId(value.Value) : (PartnerId?)null);

        builder.Property(t => t.ReceiverPayerName)
            .HasMaxLength(255)
            .IsRequired();

        builder.Property(t => t.Description)
            .HasMaxLength(500)
            .IsRequired();

        builder.Property(t => t.TotalAmount)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(t => t.CurrencyId)
            .HasConversion(c => c.Value, value => new CurrencyCode(value))
            .HasMaxLength(10)
            .IsRequired();

        builder.Property(t => t.ExchangeRate)
            .HasPrecision(18, 4)
            .IsRequired();

        builder.Property(t => t.LinkedVoucherId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : (Guid?)null,
                value => value.HasValue ? new VoucherId(value.Value) : (VoucherId?)null);

        builder.Property(t => t.CreatedAtUtc).IsRequired();
        builder.Property(t => t.CreatedBy).HasMaxLength(100);
        builder.Property(t => t.UpdatedAtUtc);
        builder.Property(t => t.UpdatedBy).HasMaxLength(100);

        builder.HasIndex(t => t.VoucherNumber).IsUnique();
        builder.HasIndex(t => t.TransactionDate);
        builder.HasIndex(t => t.PartnerId);
    }
}
