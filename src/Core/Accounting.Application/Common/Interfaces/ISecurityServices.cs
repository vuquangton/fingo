using Accounting.Domain.Security;

namespace Accounting.Application.Common.Interfaces;

public interface IPasswordHasher
{
    string HashPassword(string password);
    bool VerifyPassword(string password, string passwordHash);
}

public interface ICryptoBackupEngine
{
    Task<(string FilePath, string ChecksumSha256, long FileSizeKb)> BackupAsync(
        string destinationDirectory,
        SecretString? encryptionKey,
        CancellationToken cancellationToken = default);

    Task<bool> RestoreAsync(
        string backupFilePath,
        SecretString? encryptionKey,
        string targetRestorePath,
        CancellationToken cancellationToken = default);
}
