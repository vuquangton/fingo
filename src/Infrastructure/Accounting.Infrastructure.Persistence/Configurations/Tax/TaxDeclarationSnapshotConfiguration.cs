using Accounting.Domain.Tax;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Tax;

public class TaxDeclarationSnapshotConfiguration : IEntityTypeConfiguration<TaxDeclarationSnapshot>
{
    public void Configure(EntityTypeBuilder<TaxDeclarationSnapshot> builder)
    {
        builder.ToTable("tax_declaration_snapshots");

        builder.HasKey(x => x.Id);

        builder.Property(x => x.TaxType)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.Period)
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(x => x.CompanyTaxCode)
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(x => x.CompanyName)
            .HasMaxLength(250)
            .IsRequired();

        builder.Property(x => x.DirectorName)
            .HasMaxLength(100)
            .IsRequired();

        builder.Property(x => x.XmlPayload)
            .IsRequired();

        builder.Property(x => x.CreatedAtUtc)
            .IsRequired();

        builder.HasIndex(x => new { x.TaxType, x.Period });
    }
}
