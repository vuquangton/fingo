using Accounting.Application.Common.Interfaces;
using Accounting.Domain.Entities.Security;
using Accounting.Domain.Enums;
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;

namespace Accounting.Infrastructure.Persistence.Services;

public class CurrentUserService : ICurrentUserService, ICurrentUserContext
{
    public Guid? UserId { get; set; } = Guid.Parse("00000000-0000-0000-0000-000000000001");
    string? ICurrentUserContext.UserId => UserId?.ToString();
    public string Username { get; set; } = "admin";
    public string MachineName => Environment.MachineName;
    public string? IpAddress => "127.0.0.1";
    public bool IsAuthenticated => UserId.HasValue;
    public List<Accounting.Domain.Security.AppPermission> Permissions { get; set; } = Enum.GetValues<Accounting.Domain.Security.AppPermission>().ToList();
    IReadOnlyCollection<Accounting.Domain.Security.AppPermission> ICurrentUserContext.Permissions => Permissions;
    public bool HasPermission(Accounting.Domain.Security.AppPermission permission) =>
        Permissions.Contains(Accounting.Domain.Security.AppPermission.SystemAdmin) || Permissions.Contains(permission);
}

public class DateTimeService : IDateTimeService
{
    public DateTime UtcNow => DateTime.UtcNow;
    public DateTime Now => DateTime.Now;
}

public class AuditLogService : IAuditLogService
{
    private readonly IAccountingDbContext _context;
    private readonly ICurrentUserService _currentUser;

    public AuditLogService(IAccountingDbContext context, ICurrentUserService currentUser)
    {
        _context = context;
        _currentUser = currentUser;
    }

    public async Task LogAsync(
        AuditAction action,
        string entityName,
        string entityId,
        string? oldValuesJson = null,
        string? newValuesJson = null,
        string? diffSummary = null,
        CancellationToken cancellationToken = default)
    {
        var log = new AuditLog(
            _currentUser.UserId,
            _currentUser.Username,
            _currentUser.MachineName,
            action,
            entityName,
            entityId,
            oldValuesJson,
            newValuesJson,
            diffSummary,
            _currentUser.IpAddress);

        _context.AddEntity(log);
        await _context.SaveChangesAsync(cancellationToken);
    }
}

public class DatabaseBackupService : IDatabaseBackupService
{
    private readonly IConfiguration _configuration;
    private readonly IAccountingDbContext _context;

    public DatabaseBackupService(IConfiguration configuration, IAccountingDbContext context)
    {
        _configuration = configuration;
        _context = context;
    }

    public async Task<string> BackupDatabaseAsync(string destinationPath, CancellationToken cancellationToken = default)
    {
        var provider = _configuration.GetValue<string>("DatabaseProvider") ?? "Sqlite";

        if (string.Equals(provider, "Sqlite", StringComparison.OrdinalIgnoreCase))
        {
            var connStr = _configuration.GetConnectionString("SqliteConnection") ?? "Data Source=accounting.db";
            var builder = new SqliteConnectionStringBuilder(connStr);
            var sourceFile = builder.DataSource;

            if (!File.Exists(sourceFile))
            {
                throw new FileNotFoundException($"Source SQLite database '{sourceFile}' not found.");
            }

            var dir = Path.GetDirectoryName(destinationPath);
            if (!string.IsNullOrEmpty(dir) && !Directory.Exists(dir))
            {
                Directory.CreateDirectory(dir);
            }

            // Perform online SQLite VACUUM INTO backup
            using (var sourceConnection = new SqliteConnection(connStr))
            {
                await sourceConnection.OpenAsync(cancellationToken);
                using var command = sourceConnection.CreateCommand();
                command.CommandText = $"VACUUM INTO '{destinationPath.Replace("'", "''")}';";
                await command.ExecuteNonQueryAsync(cancellationToken);
            }

            return destinationPath;
        }
        else
        {
            // PostgreSQL client-side placeholder backup log
            return destinationPath;
        }
    }

    public Task RestoreDatabaseAsync(string backupFilePath, CancellationToken cancellationToken = default)
    {
        if (!File.Exists(backupFilePath))
            throw new FileNotFoundException($"Backup file '{backupFilePath}' not found.");

        var connStr = _configuration.GetConnectionString("SqliteConnection") ?? "Data Source=accounting.db";
        var builder = new SqliteConnectionStringBuilder(connStr);
        var sourceFile = builder.DataSource;

        File.Copy(backupFilePath, sourceFile, overwrite: true);
        return Task.CompletedTask;
    }

    public async Task OptimizeDatabaseAsync(CancellationToken cancellationToken = default)
    {
        var provider = _configuration.GetValue<string>("DatabaseProvider") ?? "Sqlite";
        if (string.Equals(provider, "Sqlite", StringComparison.OrdinalIgnoreCase))
        {
            var connStr = _configuration.GetConnectionString("SqliteConnection") ?? "Data Source=accounting.db";
            using var connection = new SqliteConnection(connStr);
            await connection.OpenAsync(cancellationToken);
            using var command = connection.CreateCommand();
            command.CommandText = "PRAGMA optimize; VACUUM;";
            await command.ExecuteNonQueryAsync(cancellationToken);
        }
    }
}
