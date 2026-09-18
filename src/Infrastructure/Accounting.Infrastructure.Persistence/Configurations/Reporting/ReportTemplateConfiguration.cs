using Accounting.Domain.Common;
using Accounting.Domain.Reporting;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Reporting;

public class ReportTemplateConfiguration : IEntityTypeConfiguration<ReportTemplate>
{
    public void Configure(EntityTypeBuilder<ReportTemplate> builder)
    {
        builder.ToTable("rep_templates");

        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id)
            .HasConversion(id => id.Value, value => new ReportTemplateId(value))
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(x => x.StatementType)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.TemplateCode)
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(x => x.TemplateName)
            .HasMaxLength(250)
            .IsRequired();

        builder.Property(x => x.Circular)
            .HasMaxLength(100)
            .IsRequired();

        builder.HasMany(x => x.Lines)
            .WithOne()
            .HasForeignKey(l => l.TemplateId)
            .OnDelete(DeleteBehavior.Cascade);
    }
}
