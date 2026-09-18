using Accounting.Domain.Common;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Ledger;
using Accounting.Domain.Payroll;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Payroll;

public class PayrollRunConfiguration : IEntityTypeConfiguration<PayrollRun>
{
    public void Configure(EntityTypeBuilder<PayrollRun> builder)
    {
        builder.ToTable("pay_payroll_runs");

        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id)
            .HasConversion(id => id.Value, value => new PayrollRunId(value))
            .IsRequired();

        builder.Property(x => x.FiscalPeriodId)
            .HasConversion(id => id.Value, value => new FiscalPeriodId(value))
            .IsRequired();

        builder.Property(x => x.Title)
            .HasMaxLength(250)
            .IsRequired();

        builder.Property(x => x.Status)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.TotalGrossPay)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.TotalEmployeeInsurance)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.TotalPersonalIncomeTax)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.TotalEmployeeDeductions)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.TotalNetPay)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.TotalEmployerContributions)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.TotalEmployerCost)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.LinkedVoucherId)
            .HasConversion(id => id.HasValue ? id.Value.Value : (Guid?)null, value => value.HasValue ? new VoucherId(value.Value) : null);

        builder.Property(x => x.CreatedAtUtc).IsRequired();
        builder.Property(x => x.CreatedBy).HasMaxLength(100);
        builder.Property(x => x.UpdatedAtUtc);
        builder.Property(x => x.UpdatedBy).HasMaxLength(100);

        builder.HasMany(x => x.Payslips)
            .WithOne()
            .HasForeignKey(p => p.PayrollRunId)
            .OnDelete(DeleteBehavior.Cascade);

        builder.HasIndex(x => new { x.FiscalPeriodId, x.Status });
    }
}
