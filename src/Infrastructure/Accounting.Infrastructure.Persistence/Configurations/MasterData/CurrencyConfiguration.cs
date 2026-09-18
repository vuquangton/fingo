using Accounting.Domain.MasterData.Currencies;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;
using CurrencyCode = Accounting.Domain.MasterData.Common.CurrencyCode;

namespace Accounting.Infrastructure.Persistence.Configurations.MasterData;

public class CurrencyConfiguration : IEntityTypeConfiguration<Currency>
{
    public void Configure(EntityTypeBuilder<Currency> builder)
    {
        builder.ToTable("md_currencies");

        builder.HasKey(c => c.Id);
        builder.Property(c => c.Id)
            .HasConversion(id => id.Value, value => new CurrencyCode(value))
            .HasMaxLength(3)
            .IsRequired();

        builder.Property(c => c.CurrencyName)
            .HasMaxLength(100)
            .IsRequired();

        builder.Property(c => c.Symbol)
            .HasMaxLength(10)
            .IsRequired();

        builder.Property(c => c.IsBaseCurrency).IsRequired();
        builder.Property(c => c.DecimalPlaces).IsRequired();
        builder.Property(c => c.IsActive).IsRequired();
    }
}
