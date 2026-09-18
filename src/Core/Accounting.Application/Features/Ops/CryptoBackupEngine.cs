using System.Security.Cryptography;
using Accounting.Application.Common.Interfaces;
using Accounting.Domain.Security;

namespace Accounting.Application.Features.Ops;

public class CryptoBackupEngine(ISqlConnectionFactory connectionFactory) : ICryptoBackupEngine
{
    private const int SaltSize = 16;
    private const int KeySize = 32;
    private const int Iterations = 100_000;

    public async Task<(string FilePath, string ChecksumSha256, long FileSizeKb)> BackupAsync(
        string destinationDirectory,
        SecretString? encryptionKey,
        CancellationToken cancellationToken = default)
    {
        if (!Directory.Exists(destinationDirectory))
        {
            Directory.CreateDirectory(destinationDirectory);
        }

        var sourceDbPath = ResolveSourceDatabasePath();
        var isEncrypted = encryptionKey.HasValue && !encryptionKey.Value.IsEmpty;
        var ext = isEncrypted ? ".enc.bak" : ".bak";
        var fileName = $"accounting_backup_{DateTime.UtcNow:yyyyMMdd_HHmmss}_{Guid.NewGuid():N[..8]}{ext}";
        var destinationPath = Path.Combine(destinationDirectory, fileName);

        if (!File.Exists(sourceDbPath))
        {
            // For in-memory or connection string DB, write a marker or snapshot
            await File.WriteAllTextAsync(sourceDbPath, "-- Database snapshot --", cancellationToken);
        }

        if (isEncrypted)
        {
            await EncryptFileAsync(sourceDbPath, destinationPath, encryptionKey!.Value.Reveal(), cancellationToken);
        }
        else
        {
            File.Copy(sourceDbPath, destinationPath, overwrite: true);
        }

        // Compute SHA-256 Checksum of the backup file
        var checksum = await ComputeSha256Async(destinationPath, cancellationToken);
        var fileInfo = new FileInfo(destinationPath);
        var sizeKb = Math.Max(1, fileInfo.Length / 1024);

        return (destinationPath, checksum, sizeKb);
    }

    public async Task<bool> RestoreAsync(
        string backupFilePath,
        SecretString? encryptionKey,
        string targetRestorePath,
        CancellationToken cancellationToken = default)
    {
        if (!File.Exists(backupFilePath))
            throw new FileNotFoundException($"Backup file not found: {backupFilePath}");

        var isEncrypted = backupFilePath.EndsWith(".enc.bak", StringComparison.OrdinalIgnoreCase);

        if (isEncrypted)
        {
            if (!encryptionKey.HasValue || encryptionKey.Value.IsEmpty)
                throw new ArgumentException("Encryption key must be provided to restore an encrypted backup.");

            await DecryptFileAsync(backupFilePath, targetRestorePath, encryptionKey.Value.Reveal(), cancellationToken);
        }
        else
        {
            File.Copy(backupFilePath, targetRestorePath, overwrite: true);
        }

        return true;
    }

    public static async Task EncryptFileAsync(string sourcePath, string destinationPath, string password, CancellationToken cancellationToken)
    {
        byte[] salt = RandomNumberGenerator.GetBytes(SaltSize);
        byte[] key = Rfc2898DeriveBytes.Pbkdf2(password, salt, Iterations, HashAlgorithmName.SHA256, KeySize);

        using var aes = Aes.Create();
        aes.KeySize = 256;
        aes.Key = key;
        aes.GenerateIV();

        await using var outputStream = new FileStream(destinationPath, FileMode.Create, FileAccess.Write, FileShare.None);
        // Write header: Salt (16 bytes) + IV (16 bytes)
        await outputStream.WriteAsync(salt, cancellationToken);
        await outputStream.WriteAsync(aes.IV, cancellationToken);

        await using var cryptoStream = new CryptoStream(outputStream, aes.CreateEncryptor(), CryptoStreamMode.Write);
        await using var inputStream = new FileStream(sourcePath, FileMode.Open, FileAccess.Read, FileShare.Read);
        await inputStream.CopyToAsync(cryptoStream, cancellationToken);
        await cryptoStream.FlushFinalBlockAsync(cancellationToken);
    }

    public static async Task DecryptFileAsync(string encryptedSourcePath, string destinationPath, string password, CancellationToken cancellationToken)
    {
        await using var inputStream = new FileStream(encryptedSourcePath, FileMode.Open, FileAccess.Read, FileShare.Read);

        byte[] salt = new byte[SaltSize];
        var readSalt = await inputStream.ReadAsync(salt.AsMemory(0, SaltSize), cancellationToken);
        if (readSalt != SaltSize) throw new InvalidOperationException("Invalid encrypted backup header (corrupted salt).");

        byte[] iv = new byte[16];
        var readIv = await inputStream.ReadAsync(iv.AsMemory(0, 16), cancellationToken);
        if (readIv != 16) throw new InvalidOperationException("Invalid encrypted backup header (corrupted IV).");

        byte[] key = Rfc2898DeriveBytes.Pbkdf2(password, salt, Iterations, HashAlgorithmName.SHA256, KeySize);

        using var aes = Aes.Create();
        aes.KeySize = 256;
        aes.Key = key;
        aes.IV = iv;

        await using var cryptoStream = new CryptoStream(inputStream, aes.CreateDecryptor(), CryptoStreamMode.Read);
        await using var outputStream = new FileStream(destinationPath, FileMode.Create, FileAccess.Write, FileShare.None);
        await cryptoStream.CopyToAsync(outputStream, cancellationToken);
    }

    public static async Task<string> ComputeSha256Async(string filePath, CancellationToken cancellationToken)
    {
        using var sha = SHA256.Create();
        await using var stream = new FileStream(filePath, FileMode.Open, FileAccess.Read, FileShare.Read);
        var hash = await sha.ComputeHashAsync(stream, cancellationToken);
        return Convert.ToHexString(hash);
    }

    private string ResolveSourceDatabasePath()
    {
        using var conn = connectionFactory.CreateConnection();
        var connStr = conn.ConnectionString ?? "Data Source=accounting.db";
        var parts = connStr.Split(';', StringSplitOptions.RemoveEmptyEntries);
        foreach (var p in parts)
        {
            if (p.Trim().StartsWith("Data Source=", StringComparison.OrdinalIgnoreCase))
            {
                var path = p.Trim()["Data Source=".Length..].Trim();
                if (!Path.IsPathRooted(path))
                {
                    path = Path.Combine(Directory.GetCurrentDirectory(), path);
                }
                return path;
            }
        }

        return Path.Combine(Directory.GetCurrentDirectory(), "accounting.db");
    }
}
