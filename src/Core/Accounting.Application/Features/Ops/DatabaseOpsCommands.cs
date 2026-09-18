using Accounting.Application.Common.Models;
using Accounting.Domain.Security;
using MediatR;

namespace Accounting.Application.Features.Ops;

public record BackupResultDto(
    Guid BackupId,
    string FilePath,
    long FileSizeKb,
    string ChecksumSha256,
    bool IsEncrypted);

[RequirePermission(AppPermission.DatabaseBackup)]
public record ExecuteDatabaseBackupCommand(
    string DestinationDirectory,
    SecretString? EncryptionKey = null) : IRequest<Result<BackupResultDto>>;

[RequirePermission(AppPermission.DatabaseBackup)]
public record RestoreDatabaseBackupCommand(
    string BackupFilePath,
    SecretString? EncryptionKey,
    string TargetDatabasePath) : IRequest<Result<bool>>;

[RequirePermission(AppPermission.DatabaseOptimize)]
public record OptimizeDatabaseCommand() : IRequest<Result<string>>;
