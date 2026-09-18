using Accounting.Domain.Organization;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Organization;

public class CompanySettingConfiguration : IEntityTypeConfiguration<CompanySetting>
{
    public void Configure(EntityTypeBuilder<CompanySetting> builder)
    {
        builder.ToTable("org_company_settings");

        builder.HasKey(c => c.Id);

        builder.Property(c => c.TaxCode)
            .HasMaxLength(20)
            .IsRequired();

        builder.Property(c => c.CompanyName)
            .HasMaxLength(250)
            .IsRequired();

        builder.Property(c => c.Address)
            .HasMaxLength(500)
            .IsRequired();

        builder.Property(c => c.LegalRepresentative)
            .HasMaxLength(150)
            .IsRequired();

        builder.Property(c => c.ChiefAccountant)
            .HasMaxLength(150)
            .IsRequired();

        builder.Property(c => c.CurrencyCode)
            .HasMaxLength(10)
            .IsRequired();

        builder.Property(c => c.GoverningCircular)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(c => c.FiscalYearStartMonth)
            .IsRequired();

        builder.Property(c => c.ContactPhone)
            .HasMaxLength(50);

        builder.Property(c => c.ContactEmail)
            .HasMaxLength(150);

        builder.Property(c => c.TaxOffice)
            .HasMaxLength(200);

        builder.Property(c => c.CreatedAtUtc).IsRequired();
        builder.Property(c => c.CreatedBy).HasMaxLength(100);
        builder.Property(c => c.UpdatedAtUtc);
        builder.Property(c => c.UpdatedBy).HasMaxLength(100);

        builder.HasIndex(c => c.TaxCode).IsUnique();
    }
}
