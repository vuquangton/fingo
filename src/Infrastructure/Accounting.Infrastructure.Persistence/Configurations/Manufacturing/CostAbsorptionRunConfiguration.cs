using Accounting.Domain.Common;
using Accounting.Domain.Entities.GeneralLedger;
using Accounting.Domain.Ledger;
using Accounting.Domain.Manufacturing;
using Accounting.Domain.MasterData.Common;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Manufacturing;

public class CostAbsorptionRunConfiguration : IEntityTypeConfiguration<CostAbsorptionRun>
{
    public void Configure(EntityTypeBuilder<CostAbsorptionRun> builder)
    {
        builder.ToTable("mfg_cost_absorption_runs");

        builder.HasKey(x => x.Id);
        builder.Property(x => x.Id)
            .HasConversion(id => id.Value, value => new CostAbsorptionRunId(value))
            .IsRequired();

        builder.Property(x => x.FiscalPeriodId)
            .HasConversion(id => id.Value, value => new FiscalPeriodId(value))
            .IsRequired();

        builder.Property(x => x.FinishedGoodItemId)
            .HasConversion(id => id.Value, value => new InventoryItemId(value))
            .IsRequired();

        builder.Property(x => x.ProducedQuantity)
            .HasPrecision(18, 4)
            .IsRequired();

        builder.Property(x => x.DirectMaterialCost)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.DirectLaborCost)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.OverheadCost)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.WipBeginning)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.WipEnding)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.TotalManufacturingCost)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(x => x.UnitCostPerItem)
            .HasPrecision(18, 4)
            .IsRequired();

        builder.Property(x => x.Status)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.LinkedClearanceVoucherId)
            .HasConversion(id => id.HasValue ? id.Value.Value : (Guid?)null, value => value.HasValue ? new VoucherId(value.Value) : null);

        builder.Property(x => x.LinkedReceiptVoucherId)
            .HasConversion(id => id.HasValue ? id.Value.Value : (Guid?)null, value => value.HasValue ? new VoucherId(value.Value) : null);

        builder.Property(x => x.CreatedAtUtc).IsRequired();
        builder.Property(x => x.CreatedBy).HasMaxLength(100);
        builder.Property(x => x.UpdatedAtUtc);
        builder.Property(x => x.UpdatedBy).HasMaxLength(100);

        builder.HasIndex(x => new { x.FiscalPeriodId, x.FinishedGoodItemId });
    }
}
