using Accounting.Domain.MasterData.Common;
using Accounting.Domain.MasterData.Partners;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.MasterData;

public class BusinessPartnerConfiguration : IEntityTypeConfiguration<BusinessPartner>
{
    public void Configure(EntityTypeBuilder<BusinessPartner> builder)
    {
        builder.ToTable("md_business_partners");

        builder.HasKey(p => p.Id);
        builder.Property(p => p.Id)
            .HasConversion(id => id.Value, value => new PartnerId(value))
            .IsRequired();

        builder.Property(p => p.PartnerCode)
            .HasMaxLength(50)
            .IsRequired();
        builder.HasIndex(p => p.PartnerCode).IsUnique();

        builder.Property(p => p.Name)
            .HasMaxLength(200)
            .IsRequired();

        builder.Property(p => p.PartnerType)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(p => p.TaxCode)
            .HasMaxLength(20);
        builder.HasIndex(p => p.TaxCode);

        builder.Property(p => p.Address).HasMaxLength(500);
        builder.Property(p => p.ContactPhone).HasMaxLength(50);
        builder.Property(p => p.ContactEmail).HasMaxLength(150);

        builder.Property(p => p.CreditLimit).HasPrecision(18, 2);
        builder.Property(p => p.PaymentTermDays).IsRequired();
        builder.Property(p => p.IsActive).IsRequired();

        builder.Property(p => p.CreatedAtUtc).IsRequired();
        builder.Property(p => p.CreatedBy).HasMaxLength(100);
        builder.Property(p => p.UpdatedAtUtc);
        builder.Property(p => p.UpdatedBy).HasMaxLength(100);

        builder.Property(p => p.IsDeleted).IsRequired();
        builder.Property(p => p.DeletedAtUtc);
        builder.Property(p => p.DeletedBy).HasMaxLength(100);
    }
}
