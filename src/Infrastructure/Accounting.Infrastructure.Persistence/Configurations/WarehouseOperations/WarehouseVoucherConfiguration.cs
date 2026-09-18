using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Accounts;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.WarehouseOperations;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.WarehouseOperations;

public class WarehouseVoucherConfiguration : IEntityTypeConfiguration<WarehouseVoucher>
{
    public void Configure(EntityTypeBuilder<WarehouseVoucher> builder)
    {
        builder.ToTable("sub_warehouse_vouchers");

        builder.HasKey(v => v.Id);
        builder.Property(v => v.Id)
            .HasConversion(id => id.Value, value => new WarehouseVoucherId(value))
            .IsRequired();

        builder.Property(v => v.VoucherNumber)
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(v => v.VoucherType)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(v => v.PostingDate)
            .IsRequired();

        builder.Property(v => v.WarehouseId)
            .HasConversion(id => id.Value, value => new WarehouseId(value))
            .HasMaxLength(50)
            .IsRequired();

        builder.Property(v => v.DestinationWarehouseId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : null,
                value => value != null ? new WarehouseId(value) : (WarehouseId?)null)
            .HasMaxLength(50);

        builder.Property(v => v.Description)
            .HasMaxLength(500)
            .IsRequired();

        builder.Property(v => v.TotalQuantity)
            .HasPrecision(18, 4)
            .IsRequired();

        builder.Property(v => v.TotalAmount)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(v => v.LinkedVoucherId)
            .HasConversion(
                id => id.HasValue ? id.Value.Value : (Guid?)null,
                value => value.HasValue ? new VoucherId(value.Value) : (VoucherId?)null);

        builder.Property(v => v.CreatedAtUtc).IsRequired();
        builder.Property(v => v.CreatedBy).HasMaxLength(100);
        builder.Property(v => v.UpdatedAtUtc);
        builder.Property(v => v.UpdatedBy).HasMaxLength(100);

        builder.HasMany(v => v.Lines)
            .WithOne()
            .HasForeignKey(l => l.VoucherId)
            .OnDelete(DeleteBehavior.Cascade);

        builder.HasIndex(v => v.VoucherNumber).IsUnique();
        builder.HasIndex(v => new { v.WarehouseId, v.PostingDate });
    }
}

public class WarehouseVoucherLineConfiguration : IEntityTypeConfiguration<WarehouseVoucherLine>
{
    public void Configure(EntityTypeBuilder<WarehouseVoucherLine> builder)
    {
        builder.ToTable("sub_warehouse_voucher_lines");

        builder.HasKey(l => l.Id);
        builder.Property(l => l.Id)
            .HasConversion(id => id.Value, value => new WarehouseVoucherLineId(value))
            .IsRequired();

        builder.Property(l => l.VoucherId)
            .HasConversion(id => id.Value, value => new WarehouseVoucherId(value))
            .IsRequired();

        builder.Property(l => l.InventoryItemId)
            .HasConversion(id => id.Value, value => new InventoryItemId(value))
            .IsRequired();

        builder.Property(l => l.UnitOfMeasureId)
            .HasConversion(id => id.Value, value => new UomId(value))
            .IsRequired();

        builder.Property(l => l.Quantity)
            .HasPrecision(18, 4)
            .IsRequired();

        builder.Property(l => l.UnitPrice)
            .HasPrecision(18, 4)
            .IsRequired();

        builder.Property(l => l.TotalAmount)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(l => l.DebitAccountId)
            .HasConversion(id => id.Value, value => new AccountId(value))
            .HasMaxLength(20)
            .IsRequired();

        builder.Property(l => l.CreditAccountId)
            .HasConversion(id => id.Value, value => new AccountId(value))
            .HasMaxLength(20)
            .IsRequired();

        builder.Property(l => l.Description)
            .HasMaxLength(500)
            .IsRequired();

        builder.HasIndex(l => new { l.InventoryItemId, l.VoucherId });
    }
}
