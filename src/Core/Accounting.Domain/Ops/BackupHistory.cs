using Accounting.Domain.Common;

namespace Accounting.Domain.Ops;

public class BackupHistory : Entity<BackupRecordId>
{
    public DateTime TimestampUtc { get; private set; } = DateTime.UtcNow;
    public string FileName { get; private set; } = string.Empty;
    public string FilePath { get; private set; } = string.Empty;
    public long FileSizeKb { get; private set; }
    public BackupEncryptionStatus EncryptionStatus { get; private set; }
    public string ChecksumSha256 { get; private set; } = string.Empty;
    public string InitiatedBy { get; private set; } = string.Empty;
    public BackupStatus Status { get; private set; }
    public string? ErrorMessage { get; private set; }

    private BackupHistory() { } // EF Core

    public BackupHistory(
        BackupRecordId id,
        string fileName,
        string filePath,
        long fileSizeKb,
        BackupEncryptionStatus encryptionStatus,
        string checksumSha256,
        string initiatedBy,
        BackupStatus status = BackupStatus.Success,
        string? errorMessage = null)
    {
        Id = id;
        TimestampUtc = DateTime.UtcNow;
        FileName = fileName;
        FilePath = filePath;
        FileSizeKb = fileSizeKb;
        EncryptionStatus = encryptionStatus;
        ChecksumSha256 = checksumSha256;
        InitiatedBy = initiatedBy;
        Status = status;
        ErrorMessage = errorMessage;
    }
}
