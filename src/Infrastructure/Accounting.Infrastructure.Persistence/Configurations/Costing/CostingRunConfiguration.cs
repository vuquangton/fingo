using Accounting.Domain.Common;
using Accounting.Domain.Costing;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Common;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Costing;

public class CostingRunConfiguration : IEntityTypeConfiguration<CostingRun>
{
    public void Configure(EntityTypeBuilder<CostingRun> builder)
    {
        builder.ToTable("cost_allocation_runs");

        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id)
            .HasConversion(id => id.Value, value => new CostAllocationRunId(value))
            .IsRequired();

        builder.Property(x => x.FiscalPeriodId)
            .HasConversion(id => id.Value, value => new FiscalPeriodId(value))
            .IsRequired();

        builder.Property(x => x.WarehouseId)
            .HasConversion(id => id.HasValue ? id.Value.Value : null, value => !string.IsNullOrEmpty(value) ? new WarehouseId(value) : null)
            .HasMaxLength(50);

        builder.Property(x => x.InventoryItemId)
            .HasConversion(id => id.HasValue ? id.Value.Value : (Guid?)null, value => value.HasValue ? new InventoryItemId(value.Value) : null);

        builder.Property(x => x.CostingMethod)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.Status)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.TotalAdjustmentAmount)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.LinkedVoucherId)
            .HasConversion(id => id.HasValue ? id.Value.Value : (Guid?)null, value => value.HasValue ? new VoucherId(value.Value) : null);

        builder.Property(x => x.CreatedAtUtc).IsRequired();
        builder.Property(x => x.CreatedBy).HasMaxLength(100);
        builder.Property(x => x.UpdatedAtUtc);
        builder.Property(x => x.UpdatedBy).HasMaxLength(100);

        builder.HasIndex(x => new { x.FiscalPeriodId, x.CostingMethod });
    }
}
