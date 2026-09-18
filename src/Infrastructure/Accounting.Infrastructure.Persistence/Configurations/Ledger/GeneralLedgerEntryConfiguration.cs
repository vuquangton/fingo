using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Ledger;

public class GeneralLedgerEntryConfiguration : IEntityTypeConfiguration<GeneralLedgerEntry>
{
    public void Configure(EntityTypeBuilder<GeneralLedgerEntry> builder)
    {
        builder.ToTable("gl_entries");

        builder.HasKey(e => e.Id);

        builder.Property(e => e.VoucherId)
            .HasConversion(id => id.Value, value => new VoucherId(value))
            .IsRequired();

        builder.Property(e => e.PostingDate)
            .IsRequired();

        builder.Property(e => e.FiscalPeriodId)
            .HasConversion(id => id.Value, value => new FiscalPeriodId(value))
            .IsRequired();

        builder.Property(e => e.AccountId)
            .HasConversion(id => id.Value, value => new AccountId(value))
            .HasMaxLength(20)
            .IsRequired();

        builder.Property(e => e.DebitAmount)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(e => e.CreditAmount)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(e => e.PartnerId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : (Guid?)null,
                value => value.HasValue ? new PartnerId(value.Value) : (PartnerId?)null);

        builder.Property(e => e.CostCenterId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : null,
                value => value != null ? new CostCenterId(value) : (CostCenterId?)null)
            .HasMaxLength(50);

        builder.Property(e => e.WarehouseId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : null,
                value => value != null ? new WarehouseId(value) : (WarehouseId?)null)
            .HasMaxLength(50);

        builder.Property(e => e.CreatedAtUtc)
            .IsRequired();

        // Optimized composite index for Sổ Cái & Trial Balance aggregation
        builder.HasIndex(e => new { e.AccountId, e.PostingDate });
        builder.HasIndex(e => e.FiscalPeriodId);
        builder.HasIndex(e => e.VoucherId);
        builder.HasIndex(e => e.PostingDate);
        builder.HasIndex(e => e.PartnerId);
    }
}
