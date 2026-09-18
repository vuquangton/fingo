using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Currencies;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;

namespace Accounting.Infrastructure.Persistence.Configurations.MasterData;

public class ExchangeRateConfiguration : IEntityTypeConfiguration<ExchangeRate>
{
    public void Configure(EntityTypeBuilder<ExchangeRate> builder)
    {
        builder.ToTable("md_exchange_rates");

        builder.HasKey(r => r.Id);
        builder.Property(r => r.Id)
            .HasConversion(id => id.Value, value => new ExchangeRateId(value))
            .IsRequired();

        builder.Property(r => r.CurrencyCode)
            .HasConversion(c => c.Value, v => new CurrencyCode(v))
            .HasMaxLength(3)
            .IsRequired();

        builder.Property(r => r.ValidDate).IsRequired();
        builder.Property(r => r.BuyingRate).HasPrecision(18, 4).IsRequired();
        builder.Property(r => r.SellingRate).HasPrecision(18, 4).IsRequired();
        builder.Property(r => r.AverageRate).HasPrecision(18, 4).IsRequired();

        builder.HasIndex(r => new { r.CurrencyCode, r.ValidDate }).IsUnique();

        builder.HasOne(r => r.Currency)
            .WithMany()
            .HasForeignKey(r => r.CurrencyCode)
            .OnDelete(DeleteBehavior.Restrict);
    }
}
