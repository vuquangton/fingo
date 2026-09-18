using Accounting.Domain.Common;
using Accounting.Domain.Reporting;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Reporting;

public class ReportLineConfiguration : IEntityTypeConfiguration<ReportLine>
{
    public void Configure(EntityTypeBuilder<ReportLine> builder)
    {
        builder.ToTable("rep_lines");

        builder.HasKey(x => x.Id);

        builder.Property(x => x.TemplateId)
            .HasConversion(id => id.Value, value => new ReportTemplateId(value))
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(x => x.LineCode)
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(x => x.LineNumber)
            .IsRequired();

        builder.Property(x => x.ItemName)
            .HasMaxLength(250)
            .IsRequired();

        builder.Property(x => x.PrintStyle)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.NodeType)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.AccountPattern)
            .HasMaxLength(250);

        builder.Property(x => x.CalculationLogic)
            .HasMaxLength(250);

        builder.Property(x => x.BalanceSide)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.InvertSign)
            .IsRequired();

        builder.HasIndex(x => new { x.TemplateId, x.LineNumber });
    }
}
