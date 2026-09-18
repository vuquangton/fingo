using Accounting.Domain.Common;
using Accounting.Domain.Ledger;
using Accounting.Domain.MasterData.Common;
using Accounting.Domain.Settlement;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Settlement;

public class InvoicePaymentAllocationConfiguration : IEntityTypeConfiguration<InvoicePaymentAllocation>
{
    public void Configure(EntityTypeBuilder<InvoicePaymentAllocation> builder)
    {
        builder.ToTable("sub_invoice_payment_allocations");

        builder.HasKey(a => a.Id);
        builder.Property(a => a.Id)
            .HasConversion(id => id.Value, value => new InvoicePaymentAllocationId(value))
            .IsRequired();

        builder.Property(a => a.InvoiceId)
            .IsRequired();

        builder.Property(a => a.InvoiceType)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(a => a.PaymentVoucherId)
            .HasConversion(id => id.Value, value => new VoucherId(value))
            .IsRequired();

        builder.Property(a => a.PartnerId)
            .HasConversion(id => id.Value, value => new PartnerId(value))
            .IsRequired();

        builder.Property(a => a.AllocationDate)
            .IsRequired();

        builder.Property(a => a.AllocatedAmount)
            .HasPrecision(18, 2)
            .IsRequired();

        builder.Property(a => a.Description)
            .HasMaxLength(500)
            .IsRequired();

        builder.Property(a => a.IsReversed)
            .IsRequired();

        builder.Property(a => a.ReversedAtUtc);
        builder.Property(a => a.ReversedBy).HasMaxLength(100);

        builder.Property(a => a.CreatedAtUtc).IsRequired();
        builder.Property(a => a.CreatedBy).HasMaxLength(100);
        builder.Property(a => a.UpdatedAtUtc);
        builder.Property(a => a.UpdatedBy).HasMaxLength(100);

        builder.HasIndex(a => a.InvoiceId);
        builder.HasIndex(a => a.PaymentVoucherId);
        builder.HasIndex(a => a.PartnerId);
        builder.HasIndex(a => a.AllocationDate);
    }
}
