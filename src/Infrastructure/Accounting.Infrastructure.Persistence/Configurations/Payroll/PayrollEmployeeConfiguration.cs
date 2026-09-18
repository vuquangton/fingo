using Accounting.Domain.Common;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.Payroll;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Payroll;

public class PayrollEmployeeConfiguration : IEntityTypeConfiguration<PayrollEmployee>
{
    public void Configure(EntityTypeBuilder<PayrollEmployee> builder)
    {
        builder.ToTable("pay_employees");

        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id)
            .HasConversion(id => id.Value, value => new EmployeeId(value))
            .IsRequired();

        builder.Property(x => x.EmployeeCode)
            .HasMaxLength(50)
            .IsRequired();
        builder.HasIndex(x => x.EmployeeCode).IsUnique();

        builder.Property(x => x.FullName)
            .HasMaxLength(250)
            .IsRequired();

        builder.Property(x => x.DepartmentId)
            .HasConversion(id => id.Value, value => new DepartmentId(value))
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(x => x.IdentityCard)
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(x => x.TaxCode)
            .HasMaxLength(50);

        builder.Property(x => x.ContractType)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.BaseSalary)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.InsuranceSalary)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.Allowances)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.NonTaxableAllowances)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.DependentCount)
            .IsRequired();

        builder.Property(x => x.ExpenseAccountId)
            .HasConversion(id => id.Value, value => new AccountId(value))
            .HasMaxLength(20)
            .IsRequired();

        builder.Property(x => x.CostCenterId)
            .HasConversion(id => id.HasValue ? id.Value.Value : null, value => !string.IsNullOrEmpty(value) ? new CostCenterId(value) : null)
            .HasMaxLength(50);

        builder.Property(x => x.IsActive).IsRequired();

        builder.Property(x => x.CreatedAtUtc).IsRequired();
        builder.Property(x => x.CreatedBy).HasMaxLength(100);
        builder.Property(x => x.UpdatedAtUtc);
        builder.Property(x => x.UpdatedBy).HasMaxLength(100);

        builder.Property(x => x.IsDeleted).IsRequired();
        builder.Property(x => x.DeletedAtUtc);
        builder.Property(x => x.DeletedBy).HasMaxLength(100);
    }
}
