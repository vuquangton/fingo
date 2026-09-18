using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.Payroll;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Payroll;

public class PayslipConfiguration : IEntityTypeConfiguration<Payslip>
{
    public void Configure(EntityTypeBuilder<Payslip> builder)
    {
        builder.ToTable("pay_payslips");

        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id)
            .HasConversion(id => id.Value, value => new PayslipId(value))
            .IsRequired();

        builder.Property(x => x.PayrollRunId)
            .HasConversion(id => id.Value, value => new PayrollRunId(value))
            .IsRequired();

        builder.Property(x => x.EmployeeId)
            .HasConversion(id => id.Value, value => new EmployeeId(value))
            .IsRequired();

        builder.Property(x => x.StandardWorkingDays)
            .HasPrecision(4, 1)
            .IsRequired();

        builder.Property(x => x.ActualWorkedDays)
            .HasPrecision(4, 1)
            .IsRequired();

        builder.Property(x => x.BaseSalary).HasPrecision(18, 2).IsRequired();
        builder.Property(x => x.Allowances).HasPrecision(18, 2).IsRequired();
        builder.Property(x => x.NonTaxableAllowances).HasPrecision(18, 2).IsRequired();
        builder.Property(x => x.GrossSalary).HasPrecision(18, 2).IsRequired();

        builder.Property(x => x.SocialInsuranceEmployee).HasPrecision(18, 2).IsRequired();
        builder.Property(x => x.HealthInsuranceEmployee).HasPrecision(18, 2).IsRequired();
        builder.Property(x => x.UnemploymentInsuranceEmployee).HasPrecision(18, 2).IsRequired();
        builder.Property(x => x.TaxableIncome).HasPrecision(18, 2).IsRequired();
        builder.Property(x => x.PersonalIncomeTax).HasPrecision(18, 2).IsRequired();

        builder.Property(x => x.SocialInsuranceEmployer).HasPrecision(18, 2).IsRequired();
        builder.Property(x => x.HealthInsuranceEmployer).HasPrecision(18, 2).IsRequired();
        builder.Property(x => x.UnemploymentInsuranceEmployer).HasPrecision(18, 2).IsRequired();
        builder.Property(x => x.TradeUnionFeeEmployer).HasPrecision(18, 2).IsRequired();

        builder.Property(x => x.ExpenseAccountId)
            .HasConversion(id => id.Value, value => new AccountId(value))
            .HasMaxLength(20)
            .IsRequired();

        builder.Property(x => x.CostCenterId)
            .HasConversion(id => id.HasValue ? id.Value.Value : null, value => !string.IsNullOrEmpty(value) ? new CostCenterId(value) : null)
            .HasMaxLength(50);

        builder.HasIndex(x => new { x.PayrollRunId, x.EmployeeId });
    }
}
