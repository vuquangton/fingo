using System.Data;
using Accounting.Application.Common.Interfaces;
using Accounting.Application.Common.Models;
using Accounting.Domain.Common;
using Accounting.Domain.Ops;
using Dapper;
using MediatR;

namespace Accounting.Application.Features.Ops;

public class ExecuteDatabaseBackupCommandHandler(
    ICryptoBackupEngine backupEngine,
    IAccountingDbContext context,
    ICurrentUserContext currentUser) : IRequestHandler<ExecuteDatabaseBackupCommand, Result<BackupResultDto>>
{
    public async Task<Result<BackupResultDto>> Handle(ExecuteDatabaseBackupCommand request, CancellationToken cancellationToken)
    {
        try
        {
            var isEncrypted = request.EncryptionKey.HasValue && !request.EncryptionKey.Value.IsEmpty;
            var (filePath, checksum, sizeKb) = await backupEngine.BackupAsync(
                request.DestinationDirectory,
                request.EncryptionKey,
                cancellationToken);

            var fileName = Path.GetFileName(filePath);
            var recordId = BackupRecordId.New();

            var history = new BackupHistory(
                recordId,
                fileName,
                filePath,
                sizeKb,
                isEncrypted ? BackupEncryptionStatus.Aes256 : BackupEncryptionStatus.None,
                checksum,
                currentUser.Username);

            context.AddEntity(history);
            await context.SaveChangesAsync(cancellationToken);

            return Result<BackupResultDto>.Success(new BackupResultDto(
                recordId.Value,
                filePath,
                sizeKb,
                checksum,
                isEncrypted));
        }
        catch (Exception ex)
        {
            return Result<BackupResultDto>.Failure($"Backup failed: {ex.Message}");
        }
    }
}

public class RestoreDatabaseBackupCommandHandler(
    ICryptoBackupEngine backupEngine) : IRequestHandler<RestoreDatabaseBackupCommand, Result<bool>>
{
    public async Task<Result<bool>> Handle(RestoreDatabaseBackupCommand request, CancellationToken cancellationToken)
    {
        try
        {
            var restored = await backupEngine.RestoreAsync(
                request.BackupFilePath,
                request.EncryptionKey,
                request.TargetDatabasePath,
                cancellationToken);

            return Result<bool>.Success(restored);
        }
        catch (Exception ex)
        {
            return Result<bool>.Failure($"Restore failed: {ex.Message}");
        }
    }
}

public class OptimizeDatabaseCommandHandler(
    ISqlConnectionFactory connectionFactory) : IRequestHandler<OptimizeDatabaseCommand, Result<string>>
{
    public async Task<Result<string>> Handle(OptimizeDatabaseCommand request, CancellationToken cancellationToken)
    {
        try
        {
            var provider = connectionFactory.ProviderName;
            using var conn = connectionFactory.CreateConnection();
            if (conn.State != ConnectionState.Open)
                conn.Open();

            if (provider.Equals("MariaDb", StringComparison.OrdinalIgnoreCase) ||
                provider.Equals("MySql", StringComparison.OrdinalIgnoreCase))
            {
                await conn.ExecuteAsync("OPTIMIZE TABLE gl_entries, gl_vouchers, gl_voucher_lines;");
                return Result<string>.Success("MariaDB table optimization (OPTIMIZE TABLE) executed successfully.");
            }
            else
            {
                await conn.ExecuteAsync("VACUUM;");
                return Result<string>.Success("SQLite database optimization (VACUUM) executed successfully.");
            }
        }
        catch (Exception ex)
        {
            return Result<string>.Failure($"Optimization failed: {ex.Message}");
        }
    }
}
