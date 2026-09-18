using Accounting.Domain.Common;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Ledger;
using Accounting.Domain.PeriodEnd;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.PeriodEnd;

public class PeriodClosingRunConfiguration : IEntityTypeConfiguration<PeriodClosingRun>
{
    public void Configure(EntityTypeBuilder<PeriodClosingRun> builder)
    {
        builder.ToTable("gl_period_closing_runs");

        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id)
            .HasConversion(id => id.Value, value => new PeriodClosingRunId(value))
            .IsRequired();

        builder.Property(x => x.FiscalPeriodId)
            .HasConversion(id => id.Value, value => new FiscalPeriodId(value))
            .IsRequired();

        builder.Property(x => x.RunDate)
            .IsRequired();

        builder.Property(x => x.PerformedBy)
            .HasMaxLength(100)
            .IsRequired();

        builder.Property(x => x.ClosingStep)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.Status)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.Notes)
            .HasMaxLength(500);

        builder.Property(x => x.GeneratedVoucherIds)
            .HasConversion(
                v => string.Join(";", v.Select(id => id.Value)),
                s => string.IsNullOrEmpty(s)
                    ? new List<VoucherId>()
                    : s.Split(';', StringSplitOptions.RemoveEmptyEntries).Select(Guid.Parse).Select(g => new VoucherId(g)).ToList())
            .HasMaxLength(2000);

        builder.Property(x => x.CreatedAtUtc).IsRequired();
        builder.Property(x => x.CreatedBy).HasMaxLength(100);
        builder.Property(x => x.UpdatedAtUtc);
        builder.Property(x => x.UpdatedBy).HasMaxLength(100);

        builder.HasIndex(x => new { x.FiscalPeriodId, x.ClosingStep, x.Status });
    }
}
