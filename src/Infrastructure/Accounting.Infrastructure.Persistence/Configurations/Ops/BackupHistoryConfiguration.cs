using Accounting.Domain.Common;
using Accounting.Domain.Ops;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Metadata.Builders;

namespace Accounting.Infrastructure.Persistence.Configurations.Ops;

public class BackupHistoryConfiguration : IEntityTypeConfiguration<BackupHistory>
{
    public void Configure(EntityTypeBuilder<BackupHistory> builder)
    {
        builder.ToTable("ops_backup_history");

        builder.HasKey(x => x.Id);

        builder.Property(x => x.Id)
            .HasConversion(id => id.Value, value => new BackupRecordId(value))
            .IsRequired();

        builder.Property(x => x.TimestampUtc)
            .IsRequired();

        builder.Property(x => x.FileName)
            .HasMaxLength(255)
            .IsRequired();

        builder.Property(x => x.FilePath)
            .HasMaxLength(1000)
            .IsRequired();

        builder.Property(x => x.FileSizeKb)
            .IsRequired();

        builder.Property(x => x.EncryptionStatus)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.ChecksumSha256)
            .HasMaxLength(64)
            .IsRequired();

        builder.Property(x => x.InitiatedBy)
            .HasMaxLength(100)
            .IsRequired();

        builder.Property(x => x.Status)
            .HasConversion<int>()
            .IsRequired();

        builder.Property(x => x.ErrorMessage)
            .HasMaxLength(1000);

        builder.HasIndex(x => x.TimestampUtc);
    }
}
