namespace Accounting.Domain.Ops;

public enum BackupEncryptionStatus
{
    None = 0,
    Aes256 = 1
}

public enum BackupStatus
{
    Success = 1,
    Failed = 2
}
